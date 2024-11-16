package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/utils"
	"gorm.io/gorm"
)

const inboxContextKey ServerContextKey = "inbox"
const messageContextKey ServerContextKey = "message"
const attachmentContextKey ServerContextKey = "attachment"

func (s *Server) applyBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maxSize := int64(10240)
		if r.ContentLength > maxSize {
			sendError(w, http.StatusRequestEntityTooLarge, invalidRequestMsg)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) enforceBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			sendError(w, http.StatusUnauthorized, basicAuthFailedMsg)
			return
		}

		var inbox ent.Inbox
		tx := s.db.Where("name = ?", u).First(&inbox)
		if tx.Error != nil {
			if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				sendError(w, http.StatusUnauthorized, basicAuthFailedMsg)
				return
			}

			log.Printf("failed to get inbox: %s", tx.Error)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		}

		res, err := utils.VerifySecret(p, inbox.ApiKey)
		if err != nil {
			log.Printf("failed to verify secret for %s: %s", p, err)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		}

		if !res {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			sendError(w, http.StatusUnauthorized, basicAuthFailedMsg)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, inboxContextKey, &inbox)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) bindInbox(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := mux.Vars(r)["inbox"]
		if name == "" {
			sendError(w, http.StatusBadRequest, inboxNameMissingMsg)
			return
		}

		var key string
		authHeader := r.Header.Get("Authorization")
		queryToken := r.URL.Query().Get("api_token")
		headerToken := r.Header.Get("Api-Token")

		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) < 2 {
				sendError(w, http.StatusBadRequest, missingAuthTokenMsg)
				return
			}

			authType := strings.ToLower(parts[0])
			if authType != "bearer" && authType != "token" {
				sendError(w, http.StatusBadRequest, unknownAuthTypeMsg)
				return
			}
		} else if queryToken != "" {
			key = queryToken
		} else if headerToken != "" {
			key = headerToken
		}

		if key == "" {
			sendError(w, http.StatusBadRequest, basicAuthFailedMsg)
			return
		}

		var tx *gorm.DB
		var inbox ent.Inbox
		if id, err := strconv.ParseInt(name, 10, 64); err == nil {
			tx = s.db.Where("id = ? or name = ?", id, name).First(&inbox)
		} else {
			tx = s.db.Where("name = ?", name).First(&inbox)
		}

		if tx.Error != nil {
			if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				sendError(w, http.StatusNotFound, inboxNotFoundMsg)
			} else {
				log.Printf("failed to get inbox: %s", tx.Error)
				sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			}
			return
		}

		if res, err := utils.VerifySecret(key, inbox.ApiKey); err != nil {
			log.Printf("failed to verify secret for %s: %s", key, err)
			sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			return
		} else if !res {
			sendError(w, http.StatusUnauthorized, invalidApiKeyMsg)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, inboxContextKey, &inbox)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) bindMessage(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
		messageId, err := strconv.ParseInt(mux.Vars(r)["message"], 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, invalidMessageIdMsg)
			return
		}

		email := &ent.Email{}
		tx := s.db.Where("inbox_id = ? AND id = ?", inbox.Id, messageId).First(&email)
		if tx.Error != nil {
			if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				sendError(w, http.StatusNotFound, messageNotFoundMsg)
			} else {
				log.Printf("failed to get email: %s", tx.Error)
				sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			}
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, messageContextKey, email)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) bindAttachment(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := r.Context().Value(messageContextKey).(*ent.Email)
		attachmentId, err := strconv.ParseInt(mux.Vars(r)["attachment"], 10, 64)
		if err != nil {
			sendError(w, http.StatusBadRequest, invalidAttachmentIdMsg)
			return
		}

		content := &ent.EmailContent{}
		tx := s.db.Where(
			"email_id = ? AND id = ? AND relationship in ?",
			email.Id,
			attachmentId,
			[]ent.RelType{ent.RelAttach, ent.RelEmbedded},
		).First(&content)
		if tx.Error != nil {
			if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				sendError(w, http.StatusNotFound, attachmentNotFoundMsg)
			} else {
				log.Printf("failed to get attachment: %s", tx.Error)
				sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
			}
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, attachmentContextKey, content)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
