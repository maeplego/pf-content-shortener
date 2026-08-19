package target

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrEmpty     = errors.New("url is required")
	ErrInvalid   = errors.New("url is not a valid absolute http(s) URL")
	ErrScheme    = errors.New("only http and https are allowed")
	ErrHost      = errors.New("url host is required")
	ErrUserinfo  = errors.New("userinfo in URL is not allowed")
	ErrAllowlist = errors.New("destination host is not on the demo allowlist")
)

// Validate accepts only http(s) URLs whose hostname is on allowHosts.
// Open redirects and javascript: / data: targets are rejected before parse extras.
func Validate(raw string, allowHosts []string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrEmpty
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "javascript:"),
		strings.HasPrefix(lower, "data:"),
		strings.HasPrefix(lower, "file:"),
		strings.HasPrefix(lower, "vbscript:"),
		strings.HasPrefix(lower, "blob:"):
		return "", ErrScheme
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrScheme
	}
	if u.Host == "" || u.Hostname() == "" {
		return "", ErrHost
	}
	if u.User != nil {
		return "", ErrUserinfo
	}
	if u.Fragment != "" {
		u.Fragment = ""
	}
	host := strings.ToLower(u.Hostname())
	if !hostAllowed(host, allowHosts) {
		return "", ErrAllowlist
	}
	return u.String(), nil
}

func hostAllowed(host string, allowHosts []string) bool {
	if len(allowHosts) == 0 {
		return false
	}
	for _, raw := range allowHosts {
		want := strings.ToLower(strings.TrimSpace(raw))
		if want == "" {
			continue
		}
		if h, _, err := net.SplitHostPort(want); err == nil {
			want = h
		}
		if host == want {
			return true
		}
	}
	return false
}
