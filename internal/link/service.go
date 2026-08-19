package link

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/code"
	"github.com/portfolio/pf-content-shortener/internal/id"
	"github.com/portfolio/pf-content-shortener/internal/target"
)

type Service struct {
	store      Store
	cache      Cache
	clock      Clock
	gen        CodeGen
	allowHosts []string
	cacheTTL   time.Duration
}

func NewService(store Store, cache Cache, clock Clock, gen CodeGen, allowHosts []string, cacheTTL time.Duration) *Service {
	if clock == nil {
		clock = time.Now
	}
	if gen == nil {
		gen = code.Generate
	}
	if cacheTTL <= 0 {
		cacheTTL = 24 * time.Hour
	}
	return &Service{store: store, cache: cache, clock: clock, gen: gen, allowHosts: allowHosts, cacheTTL: cacheTTL}
}

type CreateInput struct {
	URL       string
	Slug      string
	ExpiresAt *time.Time
	CreatedBy string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Link, error) {
	if strings.TrimSpace(in.CreatedBy) == "" {
		return Link{}, ErrInvalid
	}
	dest, err := target.Validate(in.URL, s.allowHosts)
	if err != nil {
		return Link{}, err
	}
	now := s.clock()
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		return Link{}, ErrInvalid
	}
	c, err := s.pickCode(in.Slug)
	if err != nil {
		return Link{}, err
	}
	l := Link{
		ID:        id.New(),
		Code:      c,
		URL:       dest,
		CreatedBy: in.CreatedBy,
		Active:    true,
		ExpiresAt: in.ExpiresAt,
		CreatedAt: now,
	}
	if err := s.store.Create(ctx, l); err != nil {
		return Link{}, err
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, l.Code, l.URL, s.cacheTTL)
	}
	return l, nil
}

func (s *Service) pickCode(slug string) (string, error) {
	if strings.TrimSpace(slug) != "" {
		c, err := code.NormalizeCustom(slug)
		if err != nil {
			return "", err
		}
		return c, nil
	}
	for i := 0; i < 8; i++ {
		c, err := s.gen()
		if err != nil {
			return "", err
		}
		if code.IsReserved(c) {
			continue
		}
		return c, nil
	}
	return "", errors.New("could not allocate a short code")
}

func (s *Service) Get(ctx context.Context, idStr, caller string) (Link, error) {
	if err := id.Parse(idStr); err != nil {
		return Link{}, ErrInvalid
	}
	l, err := s.store.ByID(ctx, idStr)
	if err != nil {
		return Link{}, err
	}
	if l.CreatedBy != caller {
		return Link{}, ErrForbidden
	}
	return l, nil
}

func (s *Service) List(ctx context.Context, caller string) ([]Link, error) {
	return s.store.ListByOwner(ctx, caller)
}

func (s *Service) Stats(ctx context.Context, idStr, caller string) (Link, []Daily, error) {
	l, err := s.Get(ctx, idStr, caller)
	if err != nil {
		return Link{}, nil, err
	}
	days, err := s.store.Daily(ctx, l.ID)
	if err != nil {
		return Link{}, nil, err
	}
	return l, days, nil
}

// Resolve looks up the destination for the hot path. Cache is URL-only;
// click recording is the caller's job after the 302 is written.
func (s *Service) Resolve(ctx context.Context, rawCode string) (Link, error) {
	c := strings.TrimSpace(rawCode)
	if c == "" || code.IsReserved(c) {
		return Link{}, ErrNotFound
	}
	l, err := s.store.ByCode(ctx, c)
	if err != nil {
		return Link{}, err
	}
	now := s.clock()
	if !l.Active || (l.ExpiresAt != nil && !l.ExpiresAt.After(now)) {
		return Link{}, ErrInactive
	}
	if s.cache != nil {
		_ = s.cache.Set(ctx, l.Code, l.URL, s.cacheTTL)
	}
	return l, nil
}

func (s *Service) RecordClickAsync(linkID string) {
	idCopy := linkID
	at := s.clock()
	go func() {
		_ = s.store.RecordClick(context.Background(), idCopy, at)
	}()
}
