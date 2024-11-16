package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	ent "github.com/supriyo-biswas/postbox/entities"
	"gorm.io/gorm"
)

func (s *Server) sendMessageResponse(w http.ResponseWriter, email *ent.Email) {
	result, err := s.buildMessageResponse(email)
	if err != nil {
		log.Printf("failed to get message %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	sendResponse(w, http.StatusOK, result)
}

func (s *Server) updateMessage(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	var msg UpdateMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		sendError(w, http.StatusBadRequest, invalidRequestMsg)
		return
	}

	email.IsRead = msg.Message.IsRead
	if err := s.db.Save(&email).Error; err != nil {
		log.Printf("failed to update email: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	s.sendMessageResponse(w, email)
}

func (s *Server) getMessage(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	s.sendMessageResponse(w, email)
}

func (s *Server) getMessageHeaders(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	multiHeaders := make(map[string][]string)
	if err := json.Unmarshal(email.HeadersJson, &multiHeaders); err != nil {
		log.Printf("failed to unmarshal headers for email %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	headers := make(map[string]string)
	for k, v := range multiHeaders {
		headers[k] = v[0]
	}

	sendResponse(w, http.StatusOK, MessageHeaders{headers, multiHeaders})
}

func (s *Server) deleteMessage(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)

	result, err := s.buildMessageResponse(email)
	if err != nil {
		log.Printf("failed to get email %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	if err := s.db.Delete(&email).Error; err != nil {
		log.Printf("failed to delete email %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	sendResponse(w, http.StatusOK, result)
}

func (s *Server) getAndSendContent(w http.ResponseWriter, id int64, rel ent.RelType) {
	var content ent.EmailContent
	tx := s.db.Where("email_id = ? AND relationship = ?", id, rel).First(&content)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			http.Error(w, "", http.StatusNotFound)
		} else {
			log.Printf("failed to get %s for email %d: %s", rel, id, tx.Error)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		}
		return
	}

	w.Header().Set("Content-Type", content.MimeType)
	w.Write(content.Content)
}

func (s *Server) getTextBody(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	s.getAndSendContent(w, email.Id, ent.RelText)
}

func (s *Server) getHTMLBody(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	s.getAndSendContent(w, email.Id, ent.RelHTML)
}

func (s *Server) getRawSource(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	s.getAndSendContent(w, email.Id, ent.RelRaw)
}

func (s *Server) getSanitizedHTMLBody(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)

	var content ent.EmailContent
	tx := s.db.Where("email_id = ? AND relationship = ?", email.Id, ent.RelHTML).First(&content)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			sendError(w, http.StatusNotFound, "html body not found")
		} else {
			log.Printf("failed to get html for email %d: %s", email.Id, tx.Error)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		}
		return
	}

	sanitized, err := s.sanitizer.Sanitize(content.Content)
	if err != nil {
		log.Printf("sanitizer failed for email %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	w.Header().Set("Content-Type", content.MimeType)
	w.Write(sanitized)
}

func (s *Server) listAttachments(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)

	var attachments []ent.EmailContent
	tx := s.db.Where(
		"email_id = ? and relationship in ?",
		email.Id,
		[]ent.RelType{ent.RelAttach, ent.RelEmbedded},
	).Find(&attachments)

	if tx.Error != nil {
		log.Printf("failed to get attachments for email %d: %s", email.Id, tx.Error)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	result := make([]Attachment, len(attachments))
	for i, attachment := range attachments {
		attach, err := s.buildAttachmentResponse(email, &attachment)
		if err != nil {
			log.Printf("failed to build attachment response for email %d: %s", email.Id, err)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		}

		result[i] = *attach
	}

	sendResponse(w, http.StatusOK, result)
}

func (s *Server) getAttachment(w http.ResponseWriter, r *http.Request) {
	email := r.Context().Value(messageContextKey).(*ent.Email)
	content := r.Context().Value(attachmentContextKey).(*ent.EmailContent)

	if result, err := s.buildAttachmentResponse(email, content); err != nil {
		log.Printf("failed to build attachment response for email %d: %s", email.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
	} else {
		sendResponse(w, http.StatusOK, result)
	}
}

func (s *Server) downloadAttachment(w http.ResponseWriter, r *http.Request) {
	content := r.Context().Value(attachmentContextKey).(*ent.EmailContent)
	w.Header().Set("Content-Type", content.MimeType)
	w.Write(content.Content)
}
