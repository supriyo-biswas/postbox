package api

import (
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	ent "github.com/supriyo-biswas/postbox/entities"
)

var searchSpecial = regexp.MustCompile(`[\s%_]+`)

func (s *Server) sendInboxResponse(w http.ResponseWriter, inbox *ent.Inbox) {
	result, err := s.buildInboxResponse(inbox)
	if err != nil {
		log.Printf("failed to get inbox %d: %s", inbox.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	sendResponse(w, http.StatusOK, result)
}

func (s *Server) getInbox(w http.ResponseWriter, r *http.Request) {
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	s.sendInboxResponse(w, inbox)
}

func (s *Server) cleanInbox(w http.ResponseWriter, r *http.Request) {
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	id := inbox.Id

	tx := s.db.Model(&ent.Email{}).Where("inbox_id = ?", id).Delete(&ent.Email{})
	if tx.Error != nil {
		log.Printf("failed to clear inbox %d: %s", id, tx.Error)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	s.sendInboxResponse(w, inbox)
}

func (s *Server) markReadInbox(w http.ResponseWriter, r *http.Request) {
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	id := inbox.Id

	tx := s.db.Model(&ent.Email{}).Where("inbox_id = ?", id).Update("is_read", true)
	if tx.Error != nil {
		log.Printf("failed to clear inbox %d: %s", id, tx.Error)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	s.sendInboxResponse(w, inbox)
}

func (s *Server) listInboxMessages(w http.ResponseWriter, r *http.Request) {
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil {
		page = 0
	} else if page <= 0 {
		sendError(w, http.StatusBadRequest, invalidRequestMsg)
		return
	} else {
		page--
	}

	size, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil {
		size = 30
	} else if size <= 1 {
		sendError(w, http.StatusBadRequest, invalidRequestMsg)
		return
	}

	tx := s.db.Limit(size).
		Offset(page * size).
		Order("id DESC")

	search := searchSpecial.ReplaceAllString(strings.TrimSpace(r.URL.Query().Get("search")), "%")
	if search != "" {
		tx = tx.Where("subject LIKE ?", "%"+search+"%")
	}

	var emails []ent.Email
	tx = tx.Where("inbox_id = ?", inbox.Id).Find(&emails)
	if tx.Error != nil {
		log.Printf("failed to fetch emails: %s", tx.Error)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	result := make([]Message, len(emails))
	for i, email := range emails {
		msg, err := s.buildMessageResponse(&email)
		if err != nil {
			log.Printf("failed to get email %d: %s", email.Id, err)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		}

		if msg != nil {
			result[i] = *msg
		}
	}

	sendResponse(w, http.StatusOK, result)
}
