package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ServerContextKey string

type Server struct {
	db     *gorm.DB
	router *mux.Router
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func NewServer(db *gorm.DB) *Server {
	r := mux.NewRouter()
	server := &Server{db, r}

	r.Use(server.applyBodyLimit)

	v1 := r.PathPrefix("/api/v1").Subrouter()
	v2 := r.PathPrefix("/api/accounts/{account}").Subrouter()

	v1Inbox := v1.PathPrefix("/inboxes/{inbox}").Subrouter()
	v2Inbox := v2.PathPrefix("/inboxes/{inbox}").Subrouter()

	v1Inbox.Use(server.bindInbox)
	v2Inbox.Use(server.bindInbox)

	v1Inbox.HandleFunc("", server.getInbox).Methods("GET")
	v2Inbox.HandleFunc("", server.getInbox).Methods("GET")

	v1Inbox.HandleFunc("/clean", server.cleanInbox).Methods("PATCH")
	v2Inbox.HandleFunc("/clean", server.cleanInbox).Methods("PATCH")

	v1Inbox.HandleFunc("/all_read", server.markReadInbox).Methods("PATCH")
	v2Inbox.HandleFunc("/all_read", server.markReadInbox).Methods("PATCH")

	v1Inbox.HandleFunc("/messages", server.listInboxMessages).Methods("GET")
	v2Inbox.HandleFunc("/messages", server.listInboxMessages).Methods("GET")

	v1Message := v1Inbox.PathPrefix("/messages/{message}").Subrouter()
	v2Message := v2Inbox.PathPrefix("/messages/{message}").Subrouter()

	v1Message.Use(server.bindMessage)
	v2Message.Use(server.bindMessage)

	v1Message.HandleFunc("", server.getMessage).Methods("GET")
	v2Message.HandleFunc("", server.getMessage).Methods("GET")

	v1Message.HandleFunc("", server.updateMessage).Methods("PATCH")
	v2Message.HandleFunc("", server.updateMessage).Methods("PATCH")

	v1Message.HandleFunc("", server.deleteMessage).Methods("DELETE")
	v2Message.HandleFunc("", server.deleteMessage).Methods("DELETE")

	v1Message.HandleFunc("/headers", server.getMessageHeaders).Methods("GET")
	v2Message.HandleFunc("/headers", server.getMessageHeaders).Methods("GET")

	v1Message.HandleFunc("/body.txt", server.getTextBody).Methods("GET")
	v2Message.HandleFunc("/body.txt", server.getTextBody).Methods("GET")

	v1Message.HandleFunc("/body.html", server.getSanitizedHTMLBody).Methods("GET")
	v2Message.HandleFunc("/body.html", server.getSanitizedHTMLBody).Methods("GET")

	v1Message.HandleFunc("/body.htmlsource", server.getHTMLBody).Methods("GET")
	v2Message.HandleFunc("/body.htmlsource", server.getHTMLBody).Methods("GET")

	v1Message.HandleFunc("/body.eml", server.getRawSource).Methods("GET")
	v2Message.HandleFunc("/body.eml", server.getRawSource).Methods("GET")

	v1Message.HandleFunc("/body.raw", server.getRawSource).Methods("GET")
	v2Message.HandleFunc("/body.raw", server.getRawSource).Methods("GET")

	v1Message.HandleFunc("/attachments", server.listAttachments).Methods("GET")
	v2Message.HandleFunc("/attachments", server.listAttachments).Methods("GET")

	v1Attachment := v1Message.PathPrefix("/attachments/{attachment}").Subrouter()
	v2Attachment := v2Message.PathPrefix("/attachments/{attachment}").Subrouter()

	v1Attachment.Use(server.bindAttachment)
	v2Attachment.Use(server.bindAttachment)

	v1Attachment.HandleFunc("", server.getAttachment).Methods("GET")
	v2Attachment.HandleFunc("", server.getAttachment).Methods("GET")

	v1Attachment.HandleFunc("/download", server.downloadAttachment).Methods("GET")
	v2Attachment.HandleFunc("/download", server.downloadAttachment).Methods("GET")

	return server
}
