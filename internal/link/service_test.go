package link_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/link"
	"github.com/portfolio/pf-content-shortener/internal/store/memory"
	"github.com/portfolio/pf-content-shortener/internal/target"
)

func svc(t *testing.T) (*link.Service, *memory.Store) {
	t.Helper()
	st := memory.New()
	clk := func() time.Time { return time.Date(2026, 8, 19, 5, 0, 0, 0, time.UTC) }
	s := link.NewService(st, memory.NewCache(), clk, nil, []string{"localhost", "127.0.0.1"}, time.Hour)
	return s, st
}

func TestCreate_rejectsJavascriptAndOffAllowlist(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	_, err := s.Create(ctx, link.CreateInput{URL: "javascript:alert(1)", CreatedBy: "ed"})
	if !errors.Is(err, target.ErrScheme) {
		t.Fatalf("got %v", err)
	}
	_, err = s.Create(ctx, link.CreateInput{URL: "https://phish.example/login", CreatedBy: "ed"})
	if !errors.Is(err, target.ErrAllowlist) {
		t.Fatalf("got %v", err)
	}
}

func TestCreate_andResolve(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	l, err := s.Create(ctx, link.CreateInput{
		URL:       "http://localhost:3007/posts/why-redirect-is-not-nextjs",
		CreatedBy: "editor",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Code) != 7 {
		t.Fatalf("code %q", l.Code)
	}
	got, err := s.Resolve(ctx, l.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != l.URL || got.ID != l.ID {
		t.Fatalf("resolve mismatch %+v", got)
	}
}

func TestCreate_customSlugConflict(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	in := link.CreateInput{URL: "http://localhost/a", Slug: "harbor", CreatedBy: "ed"}
	if _, err := s.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, in); !errors.Is(err, link.ErrConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestResolve_expired(t *testing.T) {
	ctx := context.Background()
	st := memory.New()
	clk := time.Date(2026, 8, 19, 5, 0, 0, 0, time.UTC)
	s := link.NewService(st, memory.NewCache(), func() time.Time { return clk }, nil, []string{"localhost"}, time.Hour)
	past := clk.Add(-time.Hour)
	if _, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed", ExpiresAt: &past}); !errors.Is(err, link.ErrInvalid) {
		t.Fatalf("expired-at-create should fail, got %v", err)
	}
	exp := clk.Add(time.Minute)
	l, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed", ExpiresAt: &exp})
	if err != nil {
		t.Fatal(err)
	}
	later := link.NewService(st, memory.NewCache(), func() time.Time { return clk.Add(2 * time.Minute) }, nil, []string{"localhost"}, time.Hour)
	if _, err := later.Resolve(ctx, l.Code); !errors.Is(err, link.ErrInactive) {
		t.Fatalf("got %v", err)
	}
}

func TestGet_forbiddenOtherOwner(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	l, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, l.ID, "other"); !errors.Is(err, link.ErrForbidden) {
		t.Fatalf("got %v", err)
	}
}

func TestRecordClickAsync_increments(t *testing.T) {
	s, st := svc(t)
	ctx := context.Background()
	l, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed"})
	if err != nil {
		t.Fatal(err)
	}
	s.RecordClickAsync(l.ID)
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := st.ByID(ctx, l.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Clicks >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("click was not recorded asynchronously")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCreate_codesDiffer(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	a, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/b", CreatedBy: "ed"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Code == b.Code {
		t.Fatal("codes collided")
	}
	if a.Code == "1" || b.Code == "2" {
		t.Fatal("looks sequential")
	}
}

func TestConcurrentCreate_uniqueCodes(t *testing.T) {
	s, _ := svc(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	codes := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l, err := s.Create(ctx, link.CreateInput{URL: "http://localhost/a", CreatedBy: "ed"})
			if err != nil {
				t.Errorf("create: %v", err)
				return
			}
			codes <- l.Code
		}()
	}
	wg.Wait()
	close(codes)
	seen := map[string]struct{}{}
	for c := range codes {
		if _, ok := seen[c]; ok {
			t.Fatalf("duplicate code %s", c)
		}
		seen[c] = struct{}{}
	}
}
