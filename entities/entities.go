package entities

import (
	"time"
)

type AddressType string

const (
	FromAddr AddressType = "from"
	ToAddr   AddressType = "to"
	CcAddr   AddressType = "cc"
	BccAddr  AddressType = "bcc"
)

type RelType string

const (
	RelRaw      RelType = "raw"
	RelHTML     RelType = "html"
	RelText     RelType = "text"
	RelAttach   RelType = "attachment"
	RelEmbedded RelType = "embedded"
)

type Inbox struct {
	Id        int64   `gorm:"primaryKey;not null"`
	Name      string  `gorm:"unique;not null"`
	SmtpPass  string  `gorm:"not null"`
	ApiKey    string  `gorm:"unique;not null"`
	Emails    []Email `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Email struct {
	Id          int64          `gorm:"primaryKey;not null"`
	InboxId     int64          `gorm:"index;not null"`
	ClientIP    string         `gorm:"not null"`
	IsRead      bool           `gorm:"not null"`
	ParseError  bool           `gorm:"not null"`
	MailFrom    string         `gorm:"not null"`
	Subject     string         `gorm:"not null"`
	HeadersJson []byte         `gorm:"not null"`
	Addresses   []Address      `gorm:"constraint:OnDelete:CASCADE;"`
	Contents    []EmailContent `gorm:"constraint:OnDelete:CASCADE;"`
	CreatedAt   time.Time      `gorm:"not null"`
	UpdatedAt   time.Time      `gorm:"not null"`
}

type Address struct {
	EmailId int64       `gorm:"not null"`
	Type    AddressType `gorm:"not null"`
	Address string
	Name    string
}

type EmailContent struct {
	Id           int64   `gorm:"primaryKey;not null"`
	Relationship RelType `gorm:"not null"`
	EmailId      int64   `gorm:"not null"`
	Content      []byte  `gorm:"not null"`
	MimeType     string  `gorm:"not null"`
	FileName     string  `gorm:"not null"`
	Size         int     `gorm:"not null"`
}
