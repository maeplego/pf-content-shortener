package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/auth"
	"github.com/portfolio/pf-content-shortener/internal/cache"
	"github.com/portfolio/pf-content-shortener/internal/config"
	"github.com/portfolio/pf-content-shortener/internal/link"
	"github.com/portfolio/pf-content-shortener/internal/store/memory"
	"github.com/portfolio/pf-content-shortener/internal/store/postgres"
	"github.com/portfolio/pf-content-shortener/internal/web"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	var (
		store  link.Store
		ready  func() error
		closer func()
	)
	if cfg.DatabaseURL != "" {
		pg, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		store = pg
		ready = func() error { return pg.Ping(context.Background()) }
		closer = pg.Close
		log.Printf("store=postgres")
	} else {
		st := memory.New()
		store = st
		ready = func() error { return st.Ping(context.Background()) }
		log.Printf("store=memory (set SHORTENER_DATABASE_URL for Postgres)")
	}

	var c link.Cache = memory.NewCache()
	if cfg.RedisURL != "" {
		r, err := cache.Open(cfg.RedisURL)
		if err != nil {
			log.Printf("redis unavailable, using memory cache: %v", err)
		} else {
			c = r
			defer r.Close()
			log.Printf("cache=redis")
		}
	}

	svc := link.NewService(store, c, time.Now, nil, cfg.AllowHosts, cfg.CacheTTL)
	handler := web.New(svc, auth.New(cfg.DevAuth), cfg.CORSOrigin, cfg.PublicBase, ready).Routes()
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("shortener listening on %s allowHosts=%v", cfg.HTTPAddr, cfg.AllowHosts)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	shctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shctx)
	if closer != nil {
		closer()
	}
}
