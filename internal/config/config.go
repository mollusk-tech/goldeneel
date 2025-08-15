package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DBDSN       string
	JWTSecret   string
	DataDir     string
	EnableTLS   bool
	HTTPPort    int
	HTTPSPort   int
	TLSCertPath string
	TLSKeyPath  string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getint(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getbool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if v == "1" || v == "true" || v == "TRUE" || v == "yes" {
			return true
		}
		return false
	}
	return def
}

func Load() Config {
	dataDir := getenv("DATA_DIR", "/data")
	return Config{
		DBDSN:       getenv("DB_DSN", "postgres://postgres:postgres@localhost:5432/messaging?sslmode=disable"),
		JWTSecret:   getenv("JWT_SECRET", "change-me"),
		DataDir:     dataDir,
		EnableTLS:   getbool("ENABLE_TLS", false),
		HTTPPort:    getint("HTTP_PORT", 8081),
		HTTPSPort:   getint("HTTPS_PORT", 8443),
		TLSCertPath: filepath.Join(dataDir, "tls", "server.crt"),
		TLSKeyPath:  filepath.Join(dataDir, "tls", "server.key"),
	}
}