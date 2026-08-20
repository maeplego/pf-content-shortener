package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/auth"
	"github.com/portfolio/pf-content-shortener/internal/link"
	"github.com/portfolio/pf-content-shortener/internal/ratelimit"
	"github.com/portfolio/pf-content-shortener/internal/target"
)

type Server struct {
	svc        *link.Service
	auth       *auth.Middleware
	cors       string
	publicBase string
	ready      func() error
	redirectRL *ratelimit.Limiter
	now        func() time.Time
}

func New(svc *link.Service, mw *auth.Middleware, cors, publicBase string, ready func() error, redirectRPM int) *Server {
	if ready == nil {
		ready = func() error { return nil }
	}
	return &Server{
		svc: svc, auth: mw, cors: cors, publicBase: strings.TrimRight(publicBase, "/"), ready: ready,
		redirectRL: ratelimit.New(redirectRPM, time.Minute),
		now:        time.Now,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) {
		if err := s.ready(); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	mux.Handle("POST /v1/links", s.auth.Handler(http.HandlerFunc(s.create)))
	mux.Handle("GET /v1/links", s.auth.Handler(http.HandlerFunc(s.list)))
	mux.Handle("GET /v1/links/{id}", s.auth.Handler(http.HandlerFunc(s.get)))
	mux.Handle("GET /v1/links/{id}/stats", s.auth.Handler(http.HandlerFunc(s.stats)))
	mux.HandleFunc("GET /{code}", s.redirect)
	return s.withCORS(mux)
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cors != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cors)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Dev-User-Sub")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type createBody struct {
	URL       string  `json:"url"`
	Slug      string  `json:"slug"`
	ExpiresAt *string `json:"expiresAt"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	var body createBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid", "invalid json")
		return
	}
	var exp *time.Time
	if body.ExpiresAt != nil && strings.TrimSpace(*body.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.ExpiresAt))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid", "expiresAt must be RFC3339")
			return
		}
		exp = &t
	}
	l, err := s.svc.Create(r.Context(), link.CreateInput{
		URL: body.URL, Slug: body.Slug, ExpiresAt: exp, CreatedBy: u.Sub,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toJSON(l))
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	list, err := s.svc.List(r.Context(), u.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, l := range list {
		out = append(out, s.toJSON(l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	l, err := s.svc.Get(r.Context(), r.PathValue("id"), u.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toJSON(l))
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	l, days, err := s.svc.Stats(r.Context(), r.PathValue("id"), u.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	daily := make([]map[string]any, 0, len(days))
	for _, d := range days {
		daily = append(daily, map[string]any{"date": d.Day.Format("2006-01-02"), "count": d.Count})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"link":  s.toJSON(l),
		"daily": daily,
	})
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	ip := ipHash(r)
	if !s.redirectRL.Allow(ip, s.now()) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many redirects from this client")
		return
	}
	l, err := s.svc.Resolve(r.Context(), code)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) || errors.Is(err, link.ErrInactive) {
			http.NotFound(w, r)
			return
		}
		writeDomainError(w, err)
		return
	}
	// Hot path: write 302 first. Clicks are fire-and-forget so Redis/DB lag
	// does not sit on the redirect. Raw IP is hashed, not stored.
	s.svc.RecordClickAsync(l.ID)
	http.Redirect(w, r, l.URL, http.StatusFound)
}

func ipHash(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:])
}

func (s *Server) toJSON(l link.Link) map[string]any {
	m := map[string]any{
		"id":        l.ID,
		"code":      l.Code,
		"url":       l.URL,
		"shortUrl":  s.publicBase + "/" + l.Code,
		"active":    l.Active,
		"createdBy": l.CreatedBy,
		"createdAt": l.CreatedAt.UTC().Format(time.RFC3339),
		"clicks":    l.Clicks,
	}
	if l.ExpiresAt != nil {
		m["expiresAt"] = l.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return m
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func writeDomainError(w http.ResponseWriter, err error) {
	st, code, msg := mapErr(err)
	writeError(w, st, code, msg)
}

func mapErr(err error) (int, string, string) {
	switch {
	case errors.Is(err, link.ErrNotFound):
		return http.StatusNotFound, "not_found", "not found"
	case errors.Is(err, link.ErrForbidden):
		return http.StatusForbidden, "forbidden", "forbidden"
	case errors.Is(err, link.ErrConflict):
		return http.StatusConflict, "conflict", "code already exists"
	case errors.Is(err, link.ErrInactive):
		return http.StatusNotFound, "not_found", "inactive or expired"
	case errors.Is(err, target.ErrAllowlist):
		return http.StatusBadRequest, "host_not_allowed", err.Error()
	case errors.Is(err, target.ErrScheme), errors.Is(err, target.ErrEmpty), errors.Is(err, target.ErrInvalid), errors.Is(err, target.ErrHost), errors.Is(err, target.ErrUserinfo):
		return http.StatusBadRequest, "invalid_url", err.Error()
	case errors.Is(err, link.ErrInvalid):
		return http.StatusBadRequest, "invalid", err.Error()
	default:
		return http.StatusBadRequest, "invalid", err.Error()
	}
}
