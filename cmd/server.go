package cmd

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/supriyo-biswas/postbox/api"
	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/smtp"
	"github.com/supriyo-biswas/postbox/utils"
	"gopkg.in/natefinch/lumberjack.v2"
)

func runServerCmd(cmd *cobra.Command, args []string) error {
	cfg, err := readConfig(cmd.Root().PersistentFlags())
	if err != nil {
		return fmt.Errorf("failed to read config: %s", err)
	}

	_, err = os.Stat(path.Join(cfg.Database.Path, "db.sqlite3"))
	initCreds := false
	if err != nil {
		if os.IsNotExist(err) {
			initCreds = true
		} else {
			return fmt.Errorf("unexpected error checking database file: %s", err)
		}
	}

	d, err := openDb(cfg.Database.Path)
	if err != nil {
		return err
	}

	if initCreds {
		secret := "postbox-default"
		h := utils.HashSecret(secret)
		inbox := &ent.Inbox{
			Name:     "postbox-default",
			SmtpPass: h,
			ApiKey:   h,
		}
		if err := d.Create(inbox).Error; err != nil {
			return fmt.Errorf("failed to create initial inbox: %s", err)
		}

		log.Printf("Starting up for the first time. Created initial inbox with ID %d, "+
			"SMTP username: %s, API key/SMTP password: %s", inbox.Id, inbox.Name, secret)
	}

	if cfg.Logging.Filename != "" {
		f, err := os.OpenFile(cfg.Logging.Filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %s", err)
		}

		f.Close()
		log.SetOutput(&lumberjack.Logger{
			Filename:   cfg.Logging.Filename,
			MaxSize:    cfg.Logging.MaxSize,
			MaxBackups: cfg.Logging.MaxBackups,
			MaxAge:     cfg.Logging.MaxAge,
		})
	}

	smtpKeyFile := cfg.Server.Smtp.KeyFile
	smtpCertFile := cfg.Server.Smtp.CertFile
	var smtpCert *tls.Certificate
	if smtpKeyFile != "" && smtpCertFile != "" {
		cert, err := tls.LoadX509KeyPair(smtpCertFile, smtpKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load SMTP TLS keypair: %s", err)
		}
		smtpCert = &cert
	} else {
		smtpCert = nil
	}

	httpKeyFile := cfg.Server.Http.KeyFile
	httpCertFile := cfg.Server.Http.CertFile
	var httpCert *tls.Certificate
	if httpKeyFile != "" && httpCertFile != "" {
		cert, err := tls.LoadX509KeyPair(httpCertFile, httpKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load HTTP TLS keypair: %s", err)
		}
		httpCert = &cert
	}

	log.Printf("Starting postbox server (smtp: %s, http: %s)\n",
		cfg.Server.Smtp.Listen, cfg.Server.Http.Listen)

	smtpListener, err := net.Listen("tcp", cfg.Server.Smtp.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on SMTP port: %s", err)
	}

	httpListener, err := net.Listen("tcp", cfg.Server.Http.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on HTTP port: %s", err)
	}

	m := smtp.NewServer(d, smtpCert, cfg.Server.Smtp.MaxMsgBytes)
	go m.Serve(smtpListener)

	handler := api.NewServer(d)
	h := &http.Server{Addr: cfg.Server.Http.Listen, Handler: handler}
	if httpCert != nil {
		h.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*httpCert}}
		h.ServeTLS(httpListener, "", "")
	} else {
		h.Serve(httpListener)
	}

	return nil
}

var serverCmd = &cobra.Command{
	Use:          "server",
	Short:        "Run email and API server",
	SilenceUsage: true,
	RunE:         runServerCmd,
}

func Execute() {
	err := serverCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
