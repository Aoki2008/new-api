package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string

	DBDriver string
	DBDSN    string

	WebhookSecret string
	AuthToken     string

	MaxBodyBytes    int64
	MaxSkewSeconds  int64
	ShutdownTimeout time.Duration

	TrustProxyHeaders bool
}

func LoadConfig() Config {
	cfg := Config{
		ListenAddr: getEnv("AUDIT_LISTEN_ADDR", ":8081"),
		DBDriver:   strings.ToLower(getEnv("AUDIT_DB_DRIVER", "sqlite")),
		DBDSN:      getEnv("AUDIT_DB_DSN", "audit.db"),

		WebhookSecret: getEnv("AUDIT_WEBHOOK_SECRET", ""),
		AuthToken:     getEnv("AUDIT_AUTH_TOKEN", ""),

		MaxBodyBytes:      parseInt64Env("AUDIT_MAX_BODY_BYTES", 2*1024*1024),
		MaxSkewSeconds:    parseInt64Env("AUDIT_MAX_SKEW_SECONDS", 300),
		ShutdownTimeout:   time.Duration(parseInt64Env("AUDIT_SHUTDOWN_TIMEOUT_SECONDS", 10)) * time.Second,
		TrustProxyHeaders: parseBoolEnv("AUDIT_TRUST_PROXY_HEADERS", false),
	}

	if cfg.MaxBodyBytes < 1024 {
		cfg.MaxBodyBytes = 1024
	}
	if cfg.MaxSkewSeconds < 0 {
		cfg.MaxSkewSeconds = 0
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if strings.TrimSpace(cfg.ListenAddr) == "" {
		cfg.ListenAddr = ":8081"
	}
	if strings.TrimSpace(cfg.DBDriver) == "" {
		cfg.DBDriver = "sqlite"
	}
	if strings.TrimSpace(cfg.DBDSN) == "" {
		cfg.DBDSN = "audit.db"
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}

func parseInt64Env(key string, defaultValue int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func parseBoolEnv(key string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	switch strings.ToLower(v) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}
