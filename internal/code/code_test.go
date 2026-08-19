package code

import (
	"strings"
	"testing"
)

func TestGenerate_lengthAndAlphabet(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 200; i++ {
		c, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != Length {
			t.Fatalf("len=%d", len(c))
		}
		for _, r := range c {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("char %q not in alphabet", r)
			}
		}
		seen[c] = struct{}{}
	}
	if len(seen) < 190 {
		t.Fatalf("too many collisions: %d unique", len(seen))
	}
}

func TestGenerate_notSequential(t *testing.T) {
	a, _ := Generate()
	b, _ := Generate()
	if a == b {
		t.Fatal("two consecutive codes collided")
	}
	if isAllDigits(a) {
		t.Fatal("generated code was numeric-only")
	}
}

func TestNormalizeCustom(t *testing.T) {
	ok, err := NormalizeCustom("  blog-post  ")
	if err != nil || ok != "blog-post" {
		t.Fatalf("got %q %v", ok, err)
	}
	for _, bad := range []string{"", "ab", "health", "HEALTH", "12345", "has space", "javascript:x", "../x"} {
		if _, err := NormalizeCustom(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}
