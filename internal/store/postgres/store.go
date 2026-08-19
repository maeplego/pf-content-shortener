package postgres

import (
	"context"
	"embed"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portfolio/pf-content-shortener/internal/link"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	raw, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	for _, stmt := range splitSQL(string(raw)) {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func splitSQL(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ";") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (s *Store) Create(ctx context.Context, l link.Link) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO links (id, code, url, created_by, is_active, expires_at, created_at, click_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, l.ID, l.Code, l.URL, l.CreatedBy, l.Active, l.ExpiresAt, l.CreatedAt, l.Clicks)
	if isUnique(err) {
		return link.ErrConflict
	}
	return err
}

func (s *Store) ByID(ctx context.Context, id string) (link.Link, error) {
	return s.scanOne(ctx, `SELECT id, code, url, created_by, is_active, expires_at, created_at, click_count FROM links WHERE id = $1`, id)
}

func (s *Store) ByCode(ctx context.Context, code string) (link.Link, error) {
	return s.scanOne(ctx, `SELECT id, code, url, created_by, is_active, expires_at, created_at, click_count FROM links WHERE code = $1`, code)
}

func (s *Store) ListByOwner(ctx context.Context, sub string) ([]link.Link, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, code, url, created_by, is_active, expires_at, created_at, click_count
		FROM links WHERE created_by = $1 ORDER BY created_at DESC
	`, sub)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []link.Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) RecordClick(ctx context.Context, id string, at time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE links SET click_count = click_count + 1 WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return link.ErrNotFound
	}
	day := at.UTC().Format("2006-01-02")
	if _, err := tx.Exec(ctx, `
		INSERT INTO click_daily (link_id, day, count) VALUES ($1, $2::date, 1)
		ON CONFLICT (link_id, day) DO UPDATE SET count = click_daily.count + 1
	`, id, day); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Daily(ctx context.Context, id string) ([]link.Daily, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT day, count FROM click_daily WHERE link_id = $1 ORDER BY day
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []link.Daily
	for rows.Next() {
		var d link.Daily
		if err := rows.Scan(&d.Day, &d.Count); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanOne(ctx context.Context, q string, arg any) (link.Link, error) {
	row := s.pool.QueryRow(ctx, q, arg)
	l, err := scanLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return link.Link{}, link.ErrNotFound
	}
	return l, err
}

func scanLink(row rowScanner) (link.Link, error) {
	var l link.Link
	if err := row.Scan(&l.ID, &l.Code, &l.URL, &l.CreatedBy, &l.Active, &l.ExpiresAt, &l.CreatedAt, &l.Clicks); err != nil {
		return link.Link{}, err
	}
	return l, nil
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
