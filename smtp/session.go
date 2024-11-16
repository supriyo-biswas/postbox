package smtp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/k3a/parsemail"
	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/utils"
	"gorm.io/gorm"
)

const (
	atYourServiceMultiResp = "250-Postbox at your service\r\n"
	atYourServiceResp      = "250 Postbox at your service\r\n"
	authFailedResp         = "535 Authentication failed\r\n"
	authGetPassResp        = "334 UGFzc3dvcmQ6\r\n"
	authGetUserResp        = "334 VXNlcm5hbWU6\r\n"
	authPlainCredsResp     = "334 Provide credentials\r\n"
	authReqdResp           = "530 Authentication required\r\n"
	authSuccessResp        = "235 Authentication successful\r\n"
	byeResp                = "221 Bye\r\n"
	cmdNotImplResp         = "502 Command not implemented\r\n"
	cmdSyntaxErrResp       = "500 Command syntax error\r\n"
	domainReqdResp         = "501 Domain name required\r\n"
	heloReqdResp           = "503 HELO/EHLO required\r\n"
	helpResp               = "214 Refer https://tools.ietf.org/html/rfc5321\r\n"
	mailFromRequiredResp   = "503 MAIL FROM required\r\n"
	mailboxFullResp        = "552 Mailbox full\r\n"
	messageTooBig          = "550 Message too big\r\n"
	missingArgsResp        = "501 Missing or invalid arguments\r\n"
	okResp                 = "250 OK\r\n"
	rcptToRequiredResp     = "503 RCPT TO required\r\n"
	readyResp              = "220 ESMTP Postbox Server ready\r\n"
	readyToStartTlsResp    = "220 Ready to start TLS\r\n"
	startInputResp         = "354 Start mail input; end with <CRLF>.<CRLF>\r\n"
	tlsUnavailableResp     = "454 TLS not available due to temporary reason\r\n"
	unsupportedAuthResp    = "504 Unsupported authentication type\r\n"
)

type session struct {
	conn        net.Conn
	cert        *tls.Certificate
	isTls       bool
	maxMsgBytes int
	db          *gorm.DB
	rw          *bufio.ReadWriter
	heloDone    bool
	inbox       int64
	mailFrom    string
	rcptTo      map[string]bool
}

func newSession(conn net.Conn, cert *tls.Certificate, maxSize int, db *gorm.DB) *session {
	return &session{conn: conn, cert: cert, maxMsgBytes: maxSize, db: db}
}

func (s *session) close() {
	s.conn.Close()
}

func (s *session) send(resp string) error {
	if _, err := s.rw.WriteString(resp); err != nil {
		return err
	}
	return s.rw.Flush()
}

func (s *session) handleHelo(args string) error {
	if args == "" {
		return s.send(domainReqdResp)
	}

	s.heloDone = true
	return s.send(okResp)
}

func (s *session) handleEhlo(args string) error {
	if args == "" {
		return s.send(domainReqdResp)
	}

	s.heloDone = true
	lines := atYourServiceMultiResp +
		"250-SIZE " + strconv.Itoa(s.maxMsgBytes) + "\r\n" +
		"250-PIPELINING\r\n" +
		"250-8BITMIME\r\n" +
		"250-SMTPUTF8\r\n" +
		"250-DSN\r\n" +
		"250-AUTH PLAIN LOGIN\r\n"

	if s.cert != nil {
		lines += "250-STARTTLS\r\n"
	}

	return s.send(lines + okResp)
}

func (s *session) resetState() {
	s.inbox = 0
	s.mailFrom = ""
	s.rcptTo = nil
}

func (s *session) handleRset(args string) error {
	s.resetState()
	return s.send(okResp)
}

func (s *session) handleNoop(args string) error {
	return s.send(okResp)
}

func (s *session) handleHelp(args string) error {
	return s.send(helpResp)
}

func (s *session) handleStartTls(args string) error {
	if s.cert == nil {
		return s.send(cmdNotImplResp)
	}

	if s.isTls {
		return s.send(tlsUnavailableResp)
	}

	if err := s.send(readyToStartTlsResp); err != nil {
		return err
	}

	tlsConn := tls.Server(s.conn, &tls.Config{
		Certificates: []tls.Certificate{*s.cert},
	})

	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	s.isTls = true
	s.conn = tlsConn
	s.rw = bufio.NewReadWriter(bufio.NewReader(s.conn), bufio.NewWriter(s.conn))
	s.resetState()
	return nil
}

func (s *session) handleMail(args string) error {
	if !s.heloDone {
		return s.send(heloReqdResp)
	}

	if s.inbox == 0 {
		return s.send(authReqdResp)
	}

	aType, addr, err := parseEmailArgs(args)
	if err != nil || aType != "FROM" {
		return s.send(missingArgsResp)
	}

	s.mailFrom = addr
	return s.send(okResp)
}

func (s *session) handleRcpt(args string) error {
	if !s.heloDone {
		return s.send(heloReqdResp)
	}

	if s.inbox == 0 {
		return s.send(authReqdResp)
	}

	argType, addr, err := parseEmailArgs(args)
	if err != nil || argType != "TO" {
		return s.send(missingArgsResp)
	}

	if s.rcptTo == nil {
		s.rcptTo = make(map[string]bool)
	}

	s.rcptTo[addr] = true
	return s.send(okResp)
}

func (s *session) saveEmail(data []byte, inbox int64) error {
	e, err := parsemail.Parse(bytes.NewReader(data))
	if err != nil {
		log.Printf("failed to parse email from %s: %s", s.conn.RemoteAddr().String(), err)
	}

	addr := make([]ent.Address, len(e.From)+len(e.To)+len(e.Cc)+len(e.Bcc))
	i := 0

	for _, r := range e.From {
		addr[i] = ent.Address{Type: ent.FromAddr, Address: r.Address, Name: r.Name}
		i++
	}

	for _, r := range e.To {
		addr[i] = ent.Address{Type: ent.ToAddr, Address: r.Address, Name: r.Name}
		i++
	}

	for _, r := range e.Cc {
		addr[i] = ent.Address{Type: ent.CcAddr, Address: r.Address, Name: r.Name}
		i++
	}

	for _, r := range e.Bcc {
		addr[i] = ent.Address{Type: ent.BccAddr, Address: r.Address, Name: r.Name}
		i++
	}

	content := []ent.EmailContent{
		{
			Relationship: ent.RelRaw,
			Content:      data,
			MimeType:     "message/rfc822",
			Size:         len(data),
		},
	}

	if len(e.TextBody) > 0 {
		content = append(content, ent.EmailContent{
			Relationship: ent.RelText,
			Content:      []byte(e.TextBody),
			MimeType:     "text/plain",
			Size:         len(e.TextBody),
		})
	}

	if len(e.HTMLBody) > 0 {
		content = append(content, ent.EmailContent{
			Relationship: ent.RelHTML,
			Content:      []byte(e.HTMLBody),
			MimeType:     "text/html",
			Size:         len(e.HTMLBody),
		})
	}

	for _, a := range e.Attachments {
		if data, err := io.ReadAll(a.Data); err == nil {
			content = append(content, ent.EmailContent{
				Relationship: ent.RelAttach,
				Content:      data,
				MimeType:     a.ContentType,
				FileName:     a.Filename,
				Size:         len(data),
			})
		} else {
			log.Printf("failed to read attachment from %s: %s", s.conn.RemoteAddr().String(), err)
		}
	}

	for _, a := range e.EmbeddedFiles {
		if data, err := io.ReadAll(a.Data); err == nil {
			content = append(content, ent.EmailContent{
				Relationship: ent.RelEmbedded,
				Content:      data,
				MimeType:     a.ContentType,
				FileName:     "",
				Size:         len(data),
			})
		} else {
			log.Printf("failed to read embedded file from %s: %s", s.conn.RemoteAddr().String(), err)
		}
	}

	h, err := json.Marshal(e.Header)
	if err != nil {
		h = []byte("{}")
	}

	email := ent.Email{
		InboxId:     inbox,
		ClientIP:    s.conn.RemoteAddr().String(),
		IsRead:      false,
		ParseError:  err != nil,
		MailFrom:    s.mailFrom,
		Subject:     e.Subject,
		HeadersJson: h,
		Addresses:   addr,
		Contents:    content,
	}

	return s.db.Create(&email).Error
}

func (s *session) handleData(args string) error {
	if !s.heloDone {
		return s.send(heloReqdResp)
	}

	if s.inbox == 0 {
		return s.send(authReqdResp)
	}

	if s.mailFrom == "" {
		return s.send(mailFromRequiredResp)
	}

	if s.rcptTo == nil || len(s.rcptTo) == 0 {
		return s.send(rcptToRequiredResp)
	}

	s.send(startInputResp)
	var buf bytes.Buffer
	for {
		line, err := s.rw.ReadBytes('\n')
		if err != nil {
			return err
		}

		if string(line) == ".\r\n" {
			break
		}

		if line[0] == '.' {
			line = line[1:]
		}

		if buf.Len()+len(line) > s.maxMsgBytes {
			return s.send(messageTooBig)
		}

		buf.WriteString(string(line))
	}

	if err := s.saveEmail(buf.Bytes(), s.inbox); err != nil {
		log.Printf("failed to save email from %s: %s", s.conn.RemoteAddr().String(), err)
		return s.send(mailboxFullResp)
	}

	return s.send(okResp)
}

func (s *session) readPlainCreds() (string, string, error) {
	if err := s.send(authPlainCredsResp); err != nil {
		return "", "", err
	}

	line, err := s.rw.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return "", "", errCredentialDecode
	}

	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 {
		return "", "", errCredentialDecode
	}

	return parts[1], parts[2], nil
}

func (s *session) readLoginPassword() (string, error) {
	if err := s.send(authGetPassResp); err != nil {
		return "", err
	}

	line, err := s.rw.ReadString('\n')
	if err != nil {
		return "", err
	}

	pass, err := base64.StdEncoding.DecodeString(string(line))
	if err != nil {
		return "", errCredentialDecode
	}

	return string(pass), nil
}

func (s *session) readLoginCreds() (string, string, error) {
	if err := s.send(authGetUserResp); err != nil {
		return "", "", err
	}

	line, err := s.rw.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	user, err := base64.StdEncoding.DecodeString(string(line))
	if err != nil {
		return "", "", errCredentialDecode
	}

	pass, nil := s.readLoginPassword()
	if err != nil {
		return "", "", err
	}

	return string(user), pass, nil
}

func (s *session) handleAuth(args string) error {
	if !s.heloDone {
		return s.send(heloReqdResp)
	}

	var e error
	var user, pass string

	authType, args := parseCommand(args)
	switch authType {
	case "PLAIN":
		if args != "" {
			user, pass, e = parsePlainCreds(args)
		} else {
			user, pass, e = s.readPlainCreds()
		}
	case "LOGIN":
		if args == "" {
			user, pass, e = s.readLoginCreds()
		} else {
			u, err := base64.StdEncoding.DecodeString(args)
			if err != nil {
				return errCredentialDecode
			}

			user = string(u)
			pass, e = s.readLoginPassword()
		}
	default:
		return s.send(unsupportedAuthResp)
	}

	if e != nil {
		if errors.Is(e, errCredentialDecode) {
			return s.send(authFailedResp)
		}
		return e
	}

	ip := s.conn.RemoteAddr().String()

	var inbox ent.Inbox
	err := s.db.Where("name = ?", user).First(&inbox).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("failed auth from %s: user %s not found", ip, user)
		} else {
			log.Printf("failed to lookup user %s: %s", user, err)
		}
		return s.send(authFailedResp)
	}

	r, err := utils.VerifySecret(pass, inbox.SmtpPass)
	if err != nil {
		log.Printf("failed to verify password for user %s: %s", user, err)
	}

	if !r {
		log.Printf("failed auth from %s: invalid password for user %s", ip, user)
		return s.send(authFailedResp)
	}

	s.inbox = inbox.Id
	return s.send(authSuccessResp)
}

func (s *session) handle() {
	defer s.close()
	s.rw = bufio.NewReadWriter(bufio.NewReader(s.conn), bufio.NewWriter(s.conn))
	s.send(readyResp)

outer:
	for {
		line, err := s.rw.ReadString('\n')
		if err != nil {
			break
		}

		cmd, args := parseCommand(line)
		switch cmd {
		case "AUTH":
			err = s.handleAuth(args)
		case "DATA":
			err = s.handleData(args)
		case "EHLO":
			err = s.handleEhlo(args)
		case "HELO":
			err = s.handleHelo(args)
		case "HELP":
			err = s.handleHelp(args)
		case "MAIL":
			err = s.handleMail(args)
		case "NOOP":
			err = s.handleNoop(args)
		case "QUIT":
			s.send(byeResp)
			break outer
		case "RCPT":
			err = s.handleRcpt(args)
		case "RSET":
			err = s.handleRset(args)
		case "STARTTLS":
			err = s.handleStartTls(args)
		default:
			err = s.send(cmdNotImplResp)
		}

		if err != nil {
			break
		}
	}
}
