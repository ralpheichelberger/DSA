// Command server runs PartyShow.
//
// Environment:
//
//	ADDR                  listen address (default :8080)
//	BASE_URL              public URL for join links and QR codes (default http://localhost:8080)
//	DATA_DIR              where shows are stored (default ./data)
//	OPENAI_API_KEY        enables AI generation; without it the demo generator is used
//	OPENAI_MODEL          chat model (default gpt-4o-mini)
//	DEFAULT_PLAN          plan for new shows until payments exist: free|small|medium|large (default free)
//	GENERATIONS_PER_HOUR  per client IP (default 5)
//	GENERATIONS_PER_DAY   in total, caps AI spend (default 200)
//	RETENTION_DAYS        delete shows this long after their last change (default 60)
//	TRUST_PROXY           true when running behind Cloudflare/Caddy (default false)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ralpheichelberger/partyshow/internal/generator"
	"github.com/ralpheichelberger/partyshow/internal/server"
	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/store"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
	"github.com/ralpheichelberger/partyshow/web"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return n
	}
	return def
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	st, err := store.New(env("DATA_DIR", "./data") + "/shows")
	if err != nil {
		log.Error("data dir", "err", err)
		os.Exit(1)
	}

	var gen generator.Generator = generator.Stub{}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		gen = generator.NewOpenAI(key, os.Getenv("OPENAI_MODEL"))
		log.Info("AI generation enabled")
	} else {
		log.Warn("OPENAI_API_KEY not set: using the demo generator (no AI)")
	}

	cfg := server.Config{
		BaseURL:            env("BASE_URL", "http://localhost:8080"),
		DefaultPlan:        show.Plan(env("DEFAULT_PLAN", string(show.PlanFree))),
		GenerationsPerHour: envInt("GENERATIONS_PER_HOUR", 5),
		GenerationsPerDay:  envInt("GENERATIONS_PER_DAY", 200),
		TrustProxy:         env("TRUST_PROXY", "false") == "true",
	}
	srv := server.New(cfg, st, gen, webtext.New(), web.Files, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go srv.Janitor(ctx, time.Duration(envInt("RETENTION_DAYS", 60))*24*time.Hour)

	httpSrv := &http.Server{
		Addr:              env("ADDR", ":8080"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	log.Info("listening", "addr", httpSrv.Addr, "base_url", cfg.BaseURL)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
