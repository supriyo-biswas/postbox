package api

import (
	"encoding/json"
	"log"
	"net/http"
)

const timestampFormat = "2006-01-02T15:04:05.000Z"

const (
	attachmentNotFoundMsg  = "attachment not found"
	inboxNameMissingMsg    = "missing inbox name"
	inboxNotFoundMsg       = "inbox not found"
	internalServerErrorMsg = "an internal error occurred"
	invalidApiKeyMsg       = "invalid API key"
	invalidAttachmentIdMsg = "invalid attachment id"
	invalidMessageIdMsg    = "invalid message id"
	invalidRequestMsg      = "invalid request"
	messageNotFoundMsg     = "message not found"
	missingAuthTokenMsg    = "missing auth token"
	unknownAuthTypeMsg     = "unknown auth type"
)

type Inbox struct {
	Id                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Username             string  `json:"username"`
	Status               string  `json:"status"`
	EmailUsername        string  `json:"email_username"`
	EmailUsernameEnabled bool    `json:"email_username_enabled"`
	SentMessagesCount    int64   `json:"sent_messages_count"`
	EmailsCount          int64   `json:"emails_count"`
	EmailsUnreadCount    int64   `json:"emails_unread_count"`
	LastMessageSentAt    *string `json:"last_message_sent_at"`
}

type MailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type MessageSmtpInfoData struct {
	MailFromAddr string `json:"mail_from_addr"`
	ClientIP     string `json:"client_ip"`
}

type MessageSmtpInfo struct {
	Ok   bool                `json:"ok"`
	Data MessageSmtpInfoData `json:"data"`
}

type Message struct {
	Id           int64           `json:"id"`
	InboxId      int64           `json:"inbox_id"`
	Subject      string          `json:"subject"`
	SentAt       string          `json:"sent_at"`
	FromEmail    *string         `json:"from_email"`
	FromName     *string         `json:"from_name"`
	ToEmail      *string         `json:"to_email"`
	ToName       *string         `json:"to_name"`
	EmailSize    int             `json:"email_size"`
	IsRead       bool            `json:"is_read"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	HTMLBodySize int             `json:"html_body_size"`
	TextBodySize int             `json:"text_body_size"`
	HumanSize    string          `json:"human_size"`
	SmtpInfo     MessageSmtpInfo `json:"smtp_information"`

	// custom extension that provides a parsed list of all recipients
	Recipients map[string][]MailAddress `json:"recipients"`
}

type MessageHeaders struct {
	Headers map[string]string `json:"headers"`

	// custom extension that provides a multivalued list of headers
	// (the same header can appear multiple times with different values)
	MultiHeaders map[string][]string `json:"multi_headers"`
}

type Attachment struct {
	Id             int64   `json:"id"`
	MessageId      int64   `json:"message_id"`
	Filename       *string `json:"filename"`
	AttachmentType string  `json:"attachment_type"`
	ContentType    string  `json:"content_type"`
	ContentID      any     `json:"content_id"`
	AttachmentSize int     `json:"attachment_size"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	HumanSize      string  `json:"attachment_human_size"`
}

type Error struct {
	Message string `json:"message"`
}

func sendError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Error{msg})
}

func sendResponse(w http.ResponseWriter, status int, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal response: %s", err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
