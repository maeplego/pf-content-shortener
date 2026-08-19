package target

import "testing"

func TestValidate_httpsLocalhost(t *testing.T) {
	got, err := Validate("https://localhost:3007/posts/hello", []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://localhost:3007/posts/hello" {
		t.Fatalf("got %q", got)
	}
}

func TestValidate_rejectsDangerousSchemes(t *testing.T) {
	allow := []string{"localhost"}
	for _, raw := range []string{
		"javascript:alert(1)",
		"JAVASCRIPT:alert(1)",
		"data:text/html,hi",
		"file:///etc/passwd",
		"ftp://localhost/x",
		"/relative",
		"https://evil.example/phish",
		"https://user:pass@localhost/x",
	} {
		if _, err := Validate(raw, allow); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestValidate_emptyAllowlist(t *testing.T) {
	if _, err := Validate("http://localhost/", nil); err != ErrAllowlist {
		t.Fatalf("got %v", err)
	}
}

func TestValidate_ipLiteralMustBeListed(t *testing.T) {
	if _, err := Validate("http://127.0.0.1/posts/a", []string{"localhost"}); err != ErrAllowlist {
		t.Fatalf("got %v", err)
	}
	if _, err := Validate("http://127.0.0.1/posts/a", []string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
}
