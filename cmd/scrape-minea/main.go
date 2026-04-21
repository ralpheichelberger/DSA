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
	// Development logger: readable timestamps + levels on stderr (JSON Example logger is easy to miss).
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}
	if cfg.MineaEmail == "" || cfg.MineaPassword == "" {
		logger.Fatal("MINEA_EMAIL and MINEA_PASSWORD required")
	}

	fmt.Fprintln(os.Stderr, "minea CLI: starting (auth + fetch).")
	fmt.Fprintln(os.Stderr, "  • Login tries Cognito password API first, then Rod if needed (MINEA_SKIP_COGNITO=true forces Rod).")
	fmt.Fprintln(os.Stderr, "  • Login URL defaults to app.minea.com quickview (override with MINEA_LOGIN_URL).")
	fmt.Fprintln(os.Stderr, "  • Credentials are filled in one step unless MINEA_SLOW_LOGIN=true (keystroke delays).")
	fmt.Fprintln(os.Stderr, "  • If ./data/minea_session.json is valid, login is skipped.")

	start := time.Now()
	s := minea.NewScraper(cfg.MineaEmail, cfg.MineaPassword, "./data/minea_session.json", logger)
	ctx := context.Background()

	fmt.Fprintln(os.Stderr, "minea CLI: ensuring auth…")
	if err := s.EnsureAuth(ctx); err != nil {
		logger.Fatal("auth failed", zap.Error(err))
	}
	fmt.Fprintln(os.Stderr, "minea CLI: auth OK, fetching credits…")

	balance, err := s.GetCredits(ctx)
	if err != nil {
		logger.Fatal("get credits failed", zap.Error(err))
	}
	fmt.Printf("credits: %.0f, refill at: %s\n", balance.Credits, balance.CreditsRefillAt.Format(time.RFC3339))

	fmt.Fprintln(os.Stderr, "minea CLI: fetching trending products…")
	products, err := s.GetTrendingProducts(ctx, "", "US", 0)
	if err != nil {
		logger.Fatal("get trending products failed", zap.Error(err))
	}

	raw, _ := json.MarshalIndent(products, "", "  ")
	_, _ = os.Stdout.Write(raw)
	_, _ = os.Stdout.Write([]byte("\n"))
	fmt.Printf("completed in %s\n", time.Since(start).Round(time.Millisecond))
}
