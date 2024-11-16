package cmd

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
	"github.com/spf13/pflag"
)

type CredentialConfig struct {
	SmtpPass string `json:"smtp_pass"`
	ApiKey   string `json:"api_key"`
}

type Config struct {
	Server   *ServerConfig   `toml:"server"`
	Database *DatabaseConfig `toml:"database"`
	Logging  *LoggingConfig  `toml:"logging"`
}

type ServerConfig struct {
	Smtp *SmtpConfig `toml:"smtp"`
	Http *HttpConfig `toml:"http"`
}

type SmtpConfig struct {
	Listen      string `toml:"listen"`
	MaxMsgBytes int    `toml:"max_message_bytes"`
	KeyFile     string `toml:"key_file"`
	CertFile    string `toml:"cert_file"`
}

type HttpConfig struct {
	Listen   string `toml:"listen"`
	KeyFile  string `toml:"key_file"`
	CertFile string `toml:"cert_file"`
}

type DatabaseConfig struct {
	Path string `toml:"path"`
}

type LoggingConfig struct {
	Filename   string `toml:"filename"`
	MaxSize    int    `toml:"max_size"`
	MaxBackups int    `toml:"max_backups"`
	MaxAge     int    `toml:"max_age"`
}

func getDefaultPath(inputPath, basePath, file string) string {
	if inputPath == "" {
		return ""
	}

	if filepath.IsAbs(inputPath) {
		return inputPath
	}

	return path.Join(basePath, inputPath, file)
}

func readConfig(flags *pflag.FlagSet) (*Config, error) {
	cfgFromArgs := true
	cfgFile, err := flags.GetString("config")

	if err != nil || cfgFile == "" {
		cfgFromArgs = false
		defaultCfgFile, err := xdg.ConfigFile("postbox/config.toml")
		if err != nil {
			return nil, err
		} else {
			cfgFile = defaultCfgFile
		}
	}

	var cfg Config
	f, err := os.Open(cfgFile)

	if err == nil {
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}

		if toml.Unmarshal(data, &cfg) != nil {
			return nil, err
		}
	} else if cfgFromArgs || !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if cfg.Server == nil {
		cfg.Server = &ServerConfig{}
	}

	if cfg.Server.Smtp == nil {
		cfg.Server.Smtp = &SmtpConfig{}
	}

	if cfg.Server.Smtp.Listen == "" {
		cfg.Server.Smtp.Listen = ":8025"
	}

	dir := filepath.Dir(cfgFile)
	cfg.Server.Smtp.KeyFile = getDefaultPath(cfg.Server.Smtp.KeyFile, dir, "key.pem")
	cfg.Server.Smtp.CertFile = getDefaultPath(cfg.Server.Smtp.CertFile, dir, "cert.pem")

	if cfg.Server.Smtp.MaxMsgBytes == 0 {
		cfg.Server.Smtp.MaxMsgBytes = 1024 * 1024 * 10
	} else if cfg.Server.Smtp.MaxMsgBytes < 1024 {
		return nil, errors.New("server.smtp.max_message_bytes must be >= 1024")
	}

	if cfg.Server.Http == nil {
		cfg.Server.Http = &HttpConfig{}
	}

	if cfg.Server.Http.Listen == "" {
		cfg.Server.Http.Listen = ":8080"
	}

	cfg.Server.Http.KeyFile = getDefaultPath(cfg.Server.Http.KeyFile, dir, "key.pem")
	cfg.Server.Http.CertFile = getDefaultPath(cfg.Server.Http.CertFile, dir, "cert.pem")

	if cfg.Database == nil {
		cfg.Database = &DatabaseConfig{}
	}

	if cfg.Database.Path == "" {
		if defaultStoragePath, err := xdg.DataFile("postbox"); err != nil {
			return nil, err
		} else {
			cfg.Database.Path = defaultStoragePath
		}
	}

	if cfg.Logging == nil {
		cfg.Logging = &LoggingConfig{}
	}

	if cfg.Logging.Filename != "" && !filepath.IsAbs(cfg.Logging.Filename) {
		logPath, err := xdg.DataFile(path.Join("postbox", cfg.Logging.Filename))
		if err != nil {
			return nil, err
		}

		cfg.Logging.Filename = logPath
	}

	return &cfg, nil
}
