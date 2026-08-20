package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr    string
	DevAuth     bool
	DatabaseURL string
	RedisURL    string
	PublicBase  string
	AllowHosts  []string
	CORSOrigin  string
	CacheTTL    time.Duration
	RedirectRPM int
}

func FromEnv() (Config, error) {
	port := strings.TrimSpace(os.Getenv("SHORTENER_HTTP_PORT"))
	if port == "" {
		port = "8094"
	}
	devAuth := strings.EqualFold(os.Getenv("SHORTENER_DEV_AUTH"), "true") || os.Getenv("SHORTENER_DEV_AUTH") == "1"
	hosts := splitCSV(os.Getenv("SHORTENER_ALLOW_HOSTS"))
	if len(hosts) == 0 {
		hosts = []string{"localhost", "127.0.0.1"}
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SHORTENER_PUBLIC_BASE")), "/")
	if base == "" {
		base = "http://localhost:8094"
	}
	cors := strings.TrimSpace(os.Getenv("SHORTENER_CORS_ORIGIN"))
	if cors == "" {
		cors = "http://localhost:3007"
	}
	cfg := Config{
		HTTPAddr:    ":" + port,
		DevAuth:     devAuth,
		DatabaseURL: strings.TrimSpace(os.Getenv("SHORTENER_DATABASE_URL")),
		RedisURL:    strings.TrimSpace(os.Getenv("SHORTENER_REDIS_URL")),
		PublicBase:  base,
		AllowHosts:  hosts,
		CORSOrigin:  cors,
		CacheTTL:    24 * time.Hour,
		RedirectRPM: envInt("SHORTENER_REDIRECT_RPM", 120),
	}
	if !cfg.DevAuth {
		return cfg, fmt.Errorf("SHORTENER_DEV_AUTH=true is required in this slice (P01 OIDC is not wired yet)")
	}
	return cfg, nil
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
