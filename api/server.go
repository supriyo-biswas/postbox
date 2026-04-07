package api

import (
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sym01/htmlsanitizer"
	"gorm.io/gorm"
)

type ServerContextKey string

type Server struct {
	db        *gorm.DB
	router    *mux.Router
	fs        http.FileSystem
	sanitizer *htmlsanitizer.HTMLSanitizer
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func NewServer(db *gorm.DB) *Server {
	r := mux.NewRouter()
	s := &Server{
		db:        db,
		router:    r,
		sanitizer: htmlsanitizer.NewHTMLSanitizer(),
	}

	s.sanitizer.GlobalAttr = []string{"style", "class", "id"}

	r.Use(s.applyBodyLimit)
	r.HandleFunc("/", s.indexHandler).Methods("GET")

	if useEmbed {
		fSys, _ := fs.Sub(embedFiles, "dist")
		s.fs = http.FS(fSys)
	} else {
		s.fs = http.Dir("api/dist")
	}

	web := r.PathPrefix("/web").Subrouter()
	wapi := web.PathPrefix("/api").Subrouter()
	web.PathPrefix("").Handler(http.StripPrefix("/web", http.FileServer(s.fs)))

	wapi.Use(s.enforceBasicAuth)
	wapi.HandleFunc("/info", s.webApiGetInfo).Methods("GET")

	v1 := r.PathPrefix("/api/v1").Subrouter()
	v2 := r.PathPrefix("/api/accounts/{account}").Subrouter()

	v1Inbox := v1.PathPrefix("/inboxes/{inbox}").Subrouter()
	v2Inbox := v2.PathPrefix("/inboxes/{inbox}").Subrouter()

	for _, sr := range []*mux.Router{v1Inbox, v2Inbox} {
		sr.Use(s.bindInbox)
	}

	inboxRouters := []*mux.Router{v1Inbox, v2Inbox, wapi}
	for _, sr := range inboxRouters {
		sr.HandleFunc("", s.getInbox).Methods("GET")
		sr.HandleFunc("/clean", s.cleanInbox).Methods("PATCH")
		sr.HandleFunc("/all_read", s.markReadInbox).Methods("PATCH")
		sr.HandleFunc("/messages", s.listInboxMessages).Methods("GET")
	}

	v1Message := v1Inbox.PathPrefix("/messages/{message}").Subrouter()
	v2Message := v2Inbox.PathPrefix("/messages/{message}").Subrouter()
	wMessage := wapi.PathPrefix("/messages/{message}").Subrouter()

	messageRouters := []*mux.Router{v1Message, v2Message, wMessage}
	for _, sr := range messageRouters {
		sr.Use(s.bindMessage)
		sr.HandleFunc("", s.getMessage).Methods("GET")
		sr.HandleFunc("", s.updateMessage).Methods("PATCH")
		sr.HandleFunc("", s.deleteMessage).Methods("DELETE")
		sr.HandleFunc("/headers", s.getMessageHeaders).Methods("GET")
		sr.HandleFunc("/body.txt", s.getTextBody).Methods("GET")
		sr.HandleFunc("/body.html", s.getSanitizedHTMLBody).Methods("GET")
		sr.HandleFunc("/body.htmlsource", s.getHTMLBody).Methods("GET")
		sr.HandleFunc("/body.eml", s.getRawSource).Methods("GET")
		sr.HandleFunc("/body.raw", s.getRawSource).Methods("GET")
		sr.HandleFunc("/attachments", s.listAttachments).Methods("GET")
	}

	v1Attachment := v1Message.PathPrefix("/attachments/{attachment}").Subrouter()
	v2Attachment := v2Message.PathPrefix("/attachments/{attachment}").Subrouter()
	wAttachment := wMessage.PathPrefix("/attachments/{attachment}").Subrouter()

	attachmentRouters := []*mux.Router{v1Attachment, v2Attachment, wAttachment}
	for _, sr := range attachmentRouters {
		sr.Use(s.bindAttachment)
		sr.HandleFunc("", s.getAttachment).Methods("GET")
		sr.HandleFunc("/download", s.downloadAttachment).Methods("GET")
	}

	return s
}
