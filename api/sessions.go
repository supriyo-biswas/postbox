package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"time"

	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/utils"
	"gorm.io/gorm"
)

const (
	sessionCookieName                       = "postbox_session"
	sessionLifetime                         = 12 * time.Hour
	sessionRefreshInterval                  = 5 * time.Minute
	sessionContextKey      ServerContextKey = "session"
)

// Cookie-authenticated mutations must originate from this site, including login.
func (s *Server) protectWebRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					sendError(w, http.StatusForbidden, "cross-origin request denied")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				sendError(w, http.StatusForbidden, "cross-origin request denied")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/web",
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
		Expires: expires, MaxAge: int(sessionLifetime.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Path: "/web", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		sendError(w, http.StatusUnsupportedMediaType, "expected application/json")
		return
	}
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&credentials); err != nil {
		sendError(w, http.StatusBadRequest, invalidRequestMsg)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		sendError(w, http.StatusBadRequest, invalidRequestMsg)
		return
	}
	var inbox ent.Inbox
	err = s.db.Where("name = ?", credentials.Username).First(&inbox).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || credentials.Username == "" || credentials.Password == "" {
		sendError(w, http.StatusUnauthorized, loginFailedMsg)
		return
	}
	if err != nil {
		log.Printf("failed to find login account: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}
	valid, err := utils.VerifySecret(credentials.Password, inbox.ApiKey)
	if err != nil {
		log.Printf("failed to verify login for inbox %d: %s", inbox.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}
	if !valid {
		sendError(w, http.StatusUnauthorized, loginFailedMsg)
		return
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		log.Printf("failed to generate session token: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	now := time.Now().UTC()
	session := ent.Session{
		UserId: inbox.Id, Token: utils.HashSecret(token),
		RefreshedAt: now, ExpiresAt: now.Add(sessionLifetime),
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if err := tx.Where("token = ?", utils.HashSecret(cookie.Value)).Delete(&ent.Session{}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&session).Error
	})
	if err != nil {
		log.Printf("failed to create session: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}
	setSessionCookie(w, r, token, session.ExpiresAt)
	sendResponse(w, http.StatusOK, map[string]string{"username": inbox.Name})
}

func (s *Server) enforceSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || len(cookie.Value) != 43 {
			sendError(w, http.StatusUnauthorized, sessionRequiredMsg)
			return
		}
		now := time.Now().UTC()
		var session ent.Session
		err = s.db.Preload("User").Where("token = ? AND expires_at > ?", utils.HashSecret(cookie.Value), now).First(&session).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && session.User.Id == 0) {
			clearSessionCookie(w, r)
			sendError(w, http.StatusUnauthorized, sessionRequiredMsg)
			return
		}
		if err != nil {
			log.Printf("failed to look up session: %s", err)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		}
		if now.Sub(session.RefreshedAt) >= sessionRefreshInterval && r.URL.Path != "/web/auth" {
			expires := now.Add(sessionLifetime)
			// Only one concurrent request should refresh the session within each five-minute window.
			tx := s.db.Model(&ent.Session{}).Where("token = ? AND refreshed_at <= ? AND expires_at > ?",
				session.Token, now.Add(-sessionRefreshInterval), now).
				Updates(map[string]any{"refreshed_at": now, "expires_at": expires})
			if tx.Error != nil {
				log.Printf("failed to refresh session: %s", tx.Error)
				sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
				return
			}
			if tx.RowsAffected > 0 {
				setSessionCookie(w, r, cookie.Value, expires)
			}
		}
		ctx := context.WithValue(r.Context(), inboxContextKey, &session.User)
		ctx = context.WithValue(ctx, sessionContextKey, &session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(sessionContextKey).(*ent.Session)
	if err := s.db.Where("token = ?", session.Token).Delete(&ent.Session{}).Error; err != nil {
		log.Printf("failed to delete session: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// RunSessionCleanup runs for the HTTP server's lifetime and removes expired sessions hourly.
func (s *Server) RunSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	s.runSessionCleanup(ctx, ticker.C)
}

func (s *Server) runSessionCleanup(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticks:
			if err := s.db.WithContext(ctx).Where("expires_at <= ?", now.UTC()).Delete(&ent.Session{}).Error; err != nil {
				log.Printf("failed to clean up expired sessions: %s", err)
			}
		}
	}
}
