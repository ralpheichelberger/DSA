package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()
	logger := zap.NewExample()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}
	if cfg.MineaEmail == "" || cfg.MineaPassword == "" {
		logger.Fatal("MINEA_EMAIL and MINEA_PASSWORD required")
	}

	start := time.Now()
	s := minea.NewScraper(cfg.MineaEmail, cfg.MineaPassword, "./data/minea_session.json", logger)
	ctx := context.Background()

	if err := s.EnsureAuth(ctx); err != nil {
		logger.Fatal("auth failed", zap.Error(err))
	}

	balance, err := s.GetCredits(ctx)
	if err != nil {
		logger.Fatal("get credits failed", zap.Error(err))
	}
	fmt.Printf("credits: %.0f, refill at: %s\n", balance.Credits, balance.CreditsRefillAt.Format(time.RFC3339))

	products, err := s.GetTrendingProducts(ctx, "", "US", 10)
	if err != nil {
		logger.Fatal("get trending products failed", zap.Error(err))
	}

	raw, _ := json.MarshalIndent(products, "", "  ")
	_, _ = os.Stdout.Write(raw)
	_, _ = os.Stdout.Write([]byte("\n"))
	fmt.Printf("completed in %s\n", time.Since(start).Round(time.Millisecond))
}
