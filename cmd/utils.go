package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	ent "github.com/supriyo-biswas/postbox/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const dbOptions = "?_journal_mode=WAL&_foreign_keys=true"

func openDb(dir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %s", err)
	}

	dbPath := path.Join(dir, "db.sqlite3")
	d, err := gorm.Open(sqlite.Open(dbPath+dbOptions), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %s", err)
	}

	if err := d.AutoMigrate(
		&ent.Inbox{},
		&ent.Email{},
		&ent.Address{},
		&ent.EmailContent{},
	); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %s", err)
	}

	return d, nil
}

func newCredentials(file string) (*CredentialConfig, error) {
	if file == "" {
		smtpPass := make([]byte, 32)
		_, err := rand.Read(smtpPass)
		if err != nil {
			return nil, fmt.Errorf("failed to generate credentials: %s", err)
		}

		apiKey := make([]byte, 32)
		_, err = rand.Read(apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to generate credentials: %s", err)
		}

		c := CredentialConfig{
			SmtpPass: base64.RawURLEncoding.EncodeToString(smtpPass),
			ApiKey:   base64.RawURLEncoding.EncodeToString(apiKey),
		}
		return &c, nil
	}

	var f *os.File
	var err error
	if file == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %s", file, err)
		}

		defer f.Close()
	}

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %s", file, err)
	}

	creds := CredentialConfig{}
	if err := json.Unmarshal(b, &creds); err != nil {
		return nil, fmt.Errorf("unable to parse %s: %s", file, err)
	}

	if creds.ApiKey == "" || creds.SmtpPass == "" {
		return nil, fmt.Errorf("credentials file %s has empty values", file)
	}

	return &creds, nil
}
