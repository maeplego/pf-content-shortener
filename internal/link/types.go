package link

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("link not found")
	ErrConflict  = errors.New("code already exists")
	ErrInvalid   = errors.New("invalid link")
	ErrInactive  = errors.New("link is inactive or expired")
	ErrForbidden = errors.New("forbidden")
)

type Link struct {
	ID        string
	Code      string
	URL       string
	CreatedBy string
	Active    bool
	ExpiresAt *time.Time
	CreatedAt time.Time
	Clicks    int64
}

type Daily struct {
	Day   time.Time
	Count int64
}

type Store interface {
	Create(ctx context.Context, l Link) error
	ByID(ctx context.Context, id string) (Link, error)
	ByCode(ctx context.Context, code string) (Link, error)
	ListByOwner(ctx context.Context, sub string) ([]Link, error)
	RecordClick(ctx context.Context, id string, at time.Time) error
	Daily(ctx context.Context, id string) ([]Daily, error)
}

type Cache interface {
	Get(ctx context.Context, code string) (url string, ok bool, err error)
	Set(ctx context.Context, code, url string, ttl time.Duration) error
}

type Clock func() time.Time

type CodeGen func() (string, error)
