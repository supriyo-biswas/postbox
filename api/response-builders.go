package api

import (
	"errors"

	"github.com/dustin/go-humanize"
	ent "github.com/supriyo-biswas/postbox/entities"
	"gorm.io/gorm"
)

func (s *Server) buildMessageResponse(email *ent.Email) (*Message, error) {
	var recipients []ent.Address
	err := s.db.Where("email_id = ?", email.Id).Find(&recipients).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	msgRecipients := map[string][]MailAddress{
		"from": make([]MailAddress, 0),
		"to":   make([]MailAddress, 0),
		"cc":   make([]MailAddress, 0),
		"bcc":  make([]MailAddress, 0),
	}

	for _, recipient := range recipients {
		var key string
		if recipient.Type == ent.FromAddr {
			key = "from"
		} else if recipient.Type == ent.ToAddr {
			key = "to"
		} else if recipient.Type == ent.CcAddr {
			key = "cc"
		} else if recipient.Type == ent.BccAddr {
			key = "bcc"
		}

		if key != "" {
			msgRecipients[key] = append(msgRecipients[key], MailAddress{
				Name:    recipient.Name,
				Address: recipient.Address,
			})
		}
	}

	var fromEmail, fromName *string
	if len(msgRecipients["from"]) > 0 {
		fromEmail = &msgRecipients["from"][0].Address
		fromName = &msgRecipients["from"][0].Name
	}

	var toEmail, toName *string
	if len(msgRecipients["to"]) > 0 {
		toEmail = &msgRecipients["to"][0].Address
		toName = &msgRecipients["to"][0].Name
	}

	var content []ent.EmailContent
	tx := s.db.Select("size, relationship").Model(&ent.EmailContent{}).
		Where(
			"email_id = ? and relationship in ?",
			email.Id,
			[]ent.RelType{ent.RelRaw, ent.RelHTML, ent.RelText},
		).Find(&content)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var emailSize, htmlBodySize, textBodySize int
	for _, c := range content {
		if c.Relationship == ent.RelRaw {
			emailSize = c.Size
		} else if c.Relationship == ent.RelHTML {
			htmlBodySize = c.Size
		} else if c.Relationship == ent.RelText {
			textBodySize = c.Size
		}
	}

	result := Message{
		Id:           email.Id,
		InboxId:      email.InboxId,
		Subject:      email.Subject,
		SentAt:       email.CreatedAt.Format(timestampFormat),
		CreatedAt:    email.CreatedAt.Format(timestampFormat),
		UpdatedAt:    email.UpdatedAt.Format(timestampFormat),
		IsRead:       email.IsRead,
		FromEmail:    fromEmail,
		FromName:     fromName,
		ToEmail:      toEmail,
		ToName:       toName,
		Recipients:   msgRecipients,
		EmailSize:    emailSize,
		HTMLBodySize: htmlBodySize,
		TextBodySize: textBodySize,
		HumanSize:    humanize.Bytes(uint64(emailSize)),
		SmtpInfo: MessageSmtpInfo{
			Ok: true,
			Data: MessageSmtpInfoData{
				MailFromAddr: email.MailFrom,
				ClientIP:     email.ClientIP,
			},
		},
	}

	return &result, nil
}

func (s *Server) buildInboxResponse(inbox *ent.Inbox) (*Inbox, error) {
	var count int64
	tx := s.db.Select("count(*)").Model(&ent.Email{}).
		Where("inbox_id = ?", inbox.Id).
		Count(&count)

	if tx.Error != nil {
		return nil, tx.Error
	}

	var unreadCount int64
	tx = s.db.Select("count(*)").Model(&ent.Email{}).
		Where("inbox_id = ? and is_read = ?", inbox.Id, false).
		Count(&unreadCount)

	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, tx.Error
	}

	var lastSent ent.Email
	tx = s.db.Model(&ent.Email{}).Where("inbox_id = ?", inbox.Id).
		Order("created_at desc").First(&lastSent)

	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, tx.Error
	}

	var lastSentTs *string
	if lastSent.Id > 0 {
		s := lastSent.CreatedAt.Format(timestampFormat)
		lastSentTs = &s
	}

	result := Inbox{
		Id:                   inbox.Id,
		Name:                 inbox.Name,
		Username:             inbox.Name,
		Status:               "active",
		EmailUsername:        inbox.Name,
		EmailUsernameEnabled: true,
		SentMessagesCount:    count,
		EmailsCount:          count,
		EmailsUnreadCount:    unreadCount,
		LastMessageSentAt:    lastSentTs,
	}

	return &result, nil
}

func (s *Server) buildAttachmentResponse(email *ent.Email, attach *ent.EmailContent) (*Attachment, error) {
	var attachType string
	if attach.Relationship == ent.RelAttach {
		attachType = "attachment"
	} else if attach.Relationship == ent.RelEmbedded {
		attachType = "inline"
	} else {
		return nil, errors.New("invalid attachment type")
	}

	var filename *string
	if attach.Relationship == ent.RelAttach {
		filename = &attach.FileName
	}

	result := Attachment{
		Id:             attach.Id,
		MessageId:      attach.EmailId,
		Filename:       filename,
		AttachmentType: attachType,
		ContentType:    attach.MimeType,
		AttachmentSize: len(attach.Content),
		HumanSize:      humanize.Bytes(uint64(len(attach.Content))),
		CreatedAt:      email.CreatedAt.Format(timestampFormat),
		UpdatedAt:      email.UpdatedAt.Format(timestampFormat),
	}

	return &result, nil
}
