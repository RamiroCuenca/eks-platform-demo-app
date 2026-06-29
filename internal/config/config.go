// Package config loads runtime configuration from the environment. Secret values
// (DB password, Redis AUTH token) are read with a file-first strategy so they can be
// delivered by the Secrets Store CSI mount (tmpfs) rather than baked into env vars.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Mode string

const (
	ModeServer Mode = "server"
	ModeWorker Mode = "worker"
)

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	TLS      bool
	QueueKey string
}

type Config struct {
	Mode  Mode
	Port  string
	DB    DBConfig
	Redis RedisConfig
}

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	cfg := Config{
		Mode: Mode(getenv("APP_MODE", string(ModeServer))),
		Port: getenv("PORT", "8080"),
		DB: DBConfig{
			Host:    os.Getenv("DB_HOST"),
			Port:    getenv("DB_PORT", "5432"),
			Name:    getenv("DB_NAME", "appdb"),
			User:    os.Getenv("DB_USER"),
			SSLMode: getenv("DB_SSLMODE", "require"),
		},
		Redis: RedisConfig{
			Addr:     os.Getenv("REDIS_ADDR"),
			TLS:      getenv("REDIS_TLS", "true") == "true",
			QueueKey: getenv("REDIS_QUEUE_KEY", "demo:jobs"),
		},
	}

	pw, err := readSecret("DB_PASSWORD")
	if err != nil {
		return cfg, fmt.Errorf("read DB password: %w", err)
	}
	cfg.DB.Password = pw

	rpw, err := readSecret("REDIS_PASSWORD")
	if err != nil {
		return cfg, fmt.Errorf("read Redis password: %w", err)
	}
	cfg.Redis.Password = rpw

	if cfg.Mode != ModeServer && cfg.Mode != ModeWorker {
		return cfg, fmt.Errorf("invalid APP_MODE %q (want %q or %q)", cfg.Mode, ModeServer, ModeWorker)
	}
	return cfg, nil
}

// readSecret returns the contents of the file at ${NAME}_FILE if set (the CSI-mount path),
// otherwise the value of ${NAME}. The file form is preferred for secret material.
func readSecret(name string) (string, error) {
	if path := os.Getenv(name + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return os.Getenv(name), nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
