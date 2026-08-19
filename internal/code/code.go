package code

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

// Alphabet omits I, l, O, 0 so generated codes are harder to misread.
// Length 7 keeps the public path short without being a sequential integer.
const (
	Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	Length   = 7
)

var customRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

var reserved = map[string]struct{}{
	"health":     {},
	"ready":      {},
	"api":        {},
	"v1":         {},
	"links":      {},
	"favicon.ico": {},
	"robots.txt": {},
}

func Generate() (string, error) {
	buf := make([]byte, Length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := byte(len(Alphabet))
	out := make([]byte, Length)
	for i, b := range buf {
		out[i] = Alphabet[b%n]
	}
	return string(out), nil
}

func NormalizeCustom(slug string) (string, error) {
	s := strings.TrimSpace(slug)
	if s == "" {
		return "", fmt.Errorf("empty slug")
	}
	if _, ok := reserved[strings.ToLower(s)]; ok {
		return "", fmt.Errorf("reserved slug")
	}
	if !customRe.MatchString(s) {
		return "", fmt.Errorf("slug must be 3-32 chars of A-Z a-z 0-9 _ -")
	}
	// Pure digits look like a sequential id; DESIGN forbids sequential public codes.
	if isAllDigits(s) {
		return "", fmt.Errorf("numeric-only slug is not allowed")
	}
	return s, nil
}

func IsReserved(s string) bool {
	_, ok := reserved[strings.ToLower(strings.TrimSpace(s))]
	return ok
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
