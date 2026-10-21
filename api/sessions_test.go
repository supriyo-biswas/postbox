package api

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func sessionTestServer(t *testing.T) (*Server, *gorm.DB, ent.Inbox) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.sqlite3")+"?_foreign_keys=true"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&ent.Inbox{}, &ent.Session{}, &ent.Email{}, &ent.Address{}, &ent.EmailContent{}); err != nil {
		t.Fatal(err)
	}
	inbox := ent.Inbox{Name: "alice", ApiKey: utils.HashSecret("password"), SmtpPass: utils.HashSecret("smtp-password")}
	if err := db.Create(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	return NewServer(db), db, inbox
}

func sessionRequest(s *Server, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://postbox.test"+path, strings.NewReader(body))
	r.TLS = &tls.ConnectionState{}
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func sessionLogin(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	w := sessionRequest(s, "POST", "/web/auth", `{"username":"alice","password":"password"}`, nil)
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 1 {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}

func TestSessionLifecycle(t *testing.T) {
	s, db, inbox := sessionTestServer(t)
	for _, body := range []string{`{"username":"alice","password":"wrong"}`, `{"username":"missing","password":"password"}`, `{"username":"alice","password":""}`} {
		if w := sessionRequest(s, "POST", "/web/auth", body, nil); w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 0 {
			t.Fatalf("invalid credentials accepted: %d", w.Code)
		}
	}
	before := time.Now().UTC()
	cookie := sessionLogin(t, s)
	if len(cookie.Value) != 43 || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/web" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 43200 {
		t.Fatalf("incorrect cookie attributes: %+v", cookie)
	}
	var session ent.Session
	if err := db.First(&session).Error; err != nil {
		t.Fatal(err)
	}
	if session.UserId != inbox.Id || session.Token == cookie.Value || session.Token != utils.HashSecret(cookie.Value) {
		t.Fatal("database must store only the token hash, linked to the account")
	}
	if session.RefreshedAt.Before(before) || session.RefreshedAt.After(time.Now().UTC()) || session.ExpiresAt.Sub(session.RefreshedAt) != 12*time.Hour {
		t.Fatal("incorrect initial session timestamps")
	}
	for _, path := range []string{"/web/api", "/web/api/", "/web/api/messages"} {
		if w := sessionRequest(s, "GET", path, "", cookie); w.Code != http.StatusOK || len(w.Result().Cookies()) != 0 {
			t.Fatalf("fresh session at %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := sessionRequest(s, "GET", "/web/api/info", "", cookie); w.Code != http.StatusNotFound {
		t.Fatalf("old info endpoint still exists: %d", w.Code)
	}
	// A visit after five minutes extends both the database expiry and the cookie.
	oldRefresh := time.Now().UTC().Add(-5 * time.Minute)
	if err := db.Model(&ent.Session{}).Where("token = ?", session.Token).Updates(map[string]any{"refreshed_at": oldRefresh, "expires_at": oldRefresh.Add(12 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	w := sessionRequest(s, "GET", "/web/api/messages", "", cookie)
	if w.Code != http.StatusOK || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].Value != cookie.Value {
		t.Fatalf("session did not renew: %d", w.Code)
	}
	var refreshed ent.Session
	if err := db.First(&refreshed).Error; err != nil {
		t.Fatal(err)
	}
	if !refreshed.RefreshedAt.After(oldRefresh) || refreshed.ExpiresAt.Sub(refreshed.RefreshedAt) != 12*time.Hour || !w.Result().Cookies()[0].Expires.Equal(refreshed.ExpiresAt.Truncate(time.Second)) {
		t.Fatal("renewal did not persist matching cookie/database expiry")
	}
	w = sessionRequest(s, "DELETE", "/web/auth", "", cookie)
	if w.Code != http.StatusNoContent || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout failed to clear cookie: %d", w.Code)
	}
	var count int64
	db.Model(&ent.Session{}).Count(&count)
	if count != 0 || sessionRequest(s, "GET", "/web/api/messages", "", cookie).Code != http.StatusUnauthorized {
		t.Fatal("logged-out session remains usable")
	}
}

func TestSessionExpiryIsolationAndCascade(t *testing.T) {
	s, db, inbox := sessionTestServer(t)
	cookie := sessionLogin(t, s)
	other := ent.Inbox{Name: "bob", ApiKey: utils.HashSecret("other-key"), SmtpPass: utils.HashSecret("other-smtp")}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	message := ent.Email{InboxId: other.Id, HeadersJson: []byte("{}")}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	if w := sessionRequest(s, "GET", "/web/api/messages/1", "", cookie); w.Code != http.StatusNotFound {
		t.Fatalf("another inbox's message was accessible: %d", w.Code)
	}
	if w := sessionRequest(s, "GET", "/api/v1/inboxes/2/messages", "", cookie); w.Code == http.StatusOK {
		t.Fatal("web cookie authenticated a token API request")
	}
	if err := db.Model(&ent.Session{}).Where("user_id = ?", inbox.Id).Update("expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if w := sessionRequest(s, "GET", "/web/api/messages", "", cookie); w.Code != http.StatusUnauthorized || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("expired session was not rejected and cleared")
	}
	sessionLogin(t, s)
	if err := db.Delete(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&ent.Session{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("account deletion did not cascade to sessions: count=%d, err=%v", count, err)
	}
}

func TestWebAuthBoundaries(t *testing.T) {
	s, _, _ := sessionTestServer(t)
	r := httptest.NewRequest("GET", "/web/api/messages", nil)
	r.SetBasicAuth("alice", "password")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized || w.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("Basic auth is still supported or challenged")
	}
	r = httptest.NewRequest("GET", "/api/v1/inboxes/1/messages", nil)
	r.Header.Set("Api-Token", "password")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal("token API stopped working")
	}
	for _, origin := range []string{"https://attacker.test", "null"} {
		r = httptest.NewRequest("POST", "https://postbox.test/web/auth", strings.NewReader(`{"username":"alice","password":"password"}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("cross-origin login accepted: %d", w.Code)
		}
	}
}

func TestExpiredSessionCleanup(t *testing.T) {
	s, db, _ := sessionTestServer(t)
	stale := sessionLogin(t, s)
	active := sessionLogin(t, s)
	if err := db.Model(&ent.Session{}).Where("token = ?", utils.HashSecret(stale.Value)).Update("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	done := make(chan struct{})
	go func() {
		s.runSessionCleanup(ctx, ticks)
		close(done)
	}()
	ticks <- time.Now().UTC()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var count int64
		if err := db.Model(&ent.Session{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expired session was not cleaned up")
		}
		time.Sleep(time.Millisecond)
	}
	if w := sessionRequest(s, "GET", "/web/api/messages", "", active); w.Code != http.StatusOK {
		t.Fatal("cleanup removed an active session")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not stop on cancellation")
	}
}
