package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/link"
)

type Store struct {
	mu     sync.Mutex
	byID   map[string]link.Link
	byCode map[string]string
	daily  map[string]map[string]int64
}

func New() *Store {
	return &Store{
		byID:   map[string]link.Link{},
		byCode: map[string]string{},
		daily:  map[string]map[string]int64{},
	}
}

func (s *Store) Create(_ context.Context, l link.Link) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byCode[l.Code]; ok {
		return link.ErrConflict
	}
	s.byID[l.ID] = l
	s.byCode[l.Code] = l.ID
	return nil
}

func (s *Store) ByID(_ context.Context, id string) (link.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.byID[id]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}
	return l, nil
}

func (s *Store) ByCode(_ context.Context, code string) (link.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byCode[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}
	return s.byID[id], nil
}

func (s *Store) ListByOwner(_ context.Context, sub string) ([]link.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]link.Link, 0)
	for _, l := range s.byID {
		if l.CreatedBy == sub {
			out = append(out, l)
		}
	}
	return out, nil
}

func (s *Store) RecordClick(_ context.Context, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.byID[id]
	if !ok {
		return link.ErrNotFound
	}
	l.Clicks++
	s.byID[id] = l
	day := at.UTC().Format("2006-01-02")
	if s.daily[id] == nil {
		s.daily[id] = map[string]int64{}
	}
	s.daily[id][day]++
	return nil
}

func (s *Store) Daily(_ context.Context, id string) ([]link.Daily, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.daily[id]
	out := make([]link.Daily, 0, len(m))
	for k, n := range m {
		t, _ := time.Parse("2006-01-02", k)
		out = append(out, link.Daily{Day: t, Count: n})
	}
	return out, nil
}

func (s *Store) Ping(context.Context) error { return nil }

type Cache struct {
	mu   sync.Mutex
	data map[string]string
}

func NewCache() *Cache {
	return &Cache{data: map[string]string{}}
}

func (c *Cache) Get(_ context.Context, code string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	u, ok := c.data[strings.TrimSpace(code)]
	return u, ok, nil
}

func (c *Cache) Set(_ context.Context, code, url string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[code] = url
	return nil
}
