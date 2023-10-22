package smtp

import (
	"crypto/tls"
	"net"

	"gorm.io/gorm"
)

type Server struct {
	db          *gorm.DB
	cert        *tls.Certificate
	maxMsgBytes int
}

func NewServer(db *gorm.DB, cert *tls.Certificate, maxMsgBytes int) *Server {
	return &Server{db, cert, maxMsgBytes}
}

func (s *Server) SetCertificate(cert *tls.Certificate) {
	s.cert = cert
}

func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		session := newSession(conn, s.cert, s.maxMsgBytes, s.db)
		go session.handle()
	}
}
