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

	s.sanitizer.GlobalAttr = []string{}

	r.Use(s.applyBodyLimit)
	r.HandleFunc("/", s.indexHandler).Methods("GET")

	v1 := r.PathPrefix("/api/v1").Subrouter()
	v2 := r.PathPrefix("/api/accounts/{account}").Subrouter()

	v1Inbox := v1.PathPrefix("/inboxes/{inbox}").Subrouter()
	v2Inbox := v2.PathPrefix("/inboxes/{inbox}").Subrouter()

	v1Inbox.Use(s.bindInbox)
	v2Inbox.Use(s.bindInbox)

	v1Inbox.HandleFunc("", s.getInbox).Methods("GET")
	v2Inbox.HandleFunc("", s.getInbox).Methods("GET")

	v1Inbox.HandleFunc("/clean", s.cleanInbox).Methods("PATCH")
	v2Inbox.HandleFunc("/clean", s.cleanInbox).Methods("PATCH")

	v1Inbox.HandleFunc("/all_read", s.markReadInbox).Methods("PATCH")
	v2Inbox.HandleFunc("/all_read", s.markReadInbox).Methods("PATCH")

	v1Inbox.HandleFunc("/messages", s.listInboxMessages).Methods("GET")
	v2Inbox.HandleFunc("/messages", s.listInboxMessages).Methods("GET")

	v1Message := v1Inbox.PathPrefix("/messages/{message}").Subrouter()
	v2Message := v2Inbox.PathPrefix("/messages/{message}").Subrouter()

	v1Message.Use(s.bindMessage)
	v2Message.Use(s.bindMessage)

	v1Message.HandleFunc("", s.getMessage).Methods("GET")
	v2Message.HandleFunc("", s.getMessage).Methods("GET")

	v1Message.HandleFunc("", s.updateMessage).Methods("PATCH")
	v2Message.HandleFunc("", s.updateMessage).Methods("PATCH")

	v1Message.HandleFunc("", s.deleteMessage).Methods("DELETE")
	v2Message.HandleFunc("", s.deleteMessage).Methods("DELETE")

	v1Message.HandleFunc("/headers", s.getMessageHeaders).Methods("GET")
	v2Message.HandleFunc("/headers", s.getMessageHeaders).Methods("GET")

	v1Message.HandleFunc("/body.txt", s.getTextBody).Methods("GET")
	v2Message.HandleFunc("/body.txt", s.getTextBody).Methods("GET")

	v1Message.HandleFunc("/body.html", s.getSanitizedHTMLBody).Methods("GET")
	v2Message.HandleFunc("/body.html", s.getSanitizedHTMLBody).Methods("GET")

	v1Message.HandleFunc("/body.htmlsource", s.getHTMLBody).Methods("GET")
	v2Message.HandleFunc("/body.htmlsource", s.getHTMLBody).Methods("GET")

	v1Message.HandleFunc("/body.eml", s.getRawSource).Methods("GET")
	v2Message.HandleFunc("/body.eml", s.getRawSource).Methods("GET")

	v1Message.HandleFunc("/body.raw", s.getRawSource).Methods("GET")
	v2Message.HandleFunc("/body.raw", s.getRawSource).Methods("GET")

	v1Message.HandleFunc("/attachments", s.listAttachments).Methods("GET")
	v2Message.HandleFunc("/attachments", s.listAttachments).Methods("GET")

	v1Attachment := v1Message.PathPrefix("/attachments/{attachment}").Subrouter()
	v2Attachment := v2Message.PathPrefix("/attachments/{attachment}").Subrouter()

	v1Attachment.Use(s.bindAttachment)
	v2Attachment.Use(s.bindAttachment)

	v1Attachment.HandleFunc("", s.getAttachment).Methods("GET")
	v2Attachment.HandleFunc("", s.getAttachment).Methods("GET")

	v1Attachment.HandleFunc("/download", s.downloadAttachment).Methods("GET")
	v2Attachment.HandleFunc("/download", s.downloadAttachment).Methods("GET")

	if useEmbed {
		fSys, _ := fs.Sub(embedFiles, "dist")
		s.fs = http.FS(fSys)
	} else {
		s.fs = http.Dir("api/dist")
	}

	web := r.PathPrefix("/web").Subrouter()
	webApi := web.PathPrefix("/api").Subrouter()
	web.PathPrefix("").Handler(http.StripPrefix("/web", http.FileServer(s.fs)))

	webApi.Use(s.enforceBasicAuth)
	webApi.HandleFunc("/info", s.webApiGetInfo).Methods("GET")

	return s
}
