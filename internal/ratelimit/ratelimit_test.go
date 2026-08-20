package ratelimit_test

import (
	"testing"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/ratelimit"
)

func TestLimiterBlocksAfterBurst(t *testing.T) {
	l := ratelimit.New(2, time.Minute)
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	if !l.Allow("1.2.3.4", now) || !l.Allow("1.2.3.4", now) {
		t.Fatal("first two should pass")
	}
	if l.Allow("1.2.3.4", now) {
		t.Fatal("third should be blocked")
	}
	if !l.Allow("9.9.9.9", now) {
		t.Fatal("other key should pass")
	}
}
