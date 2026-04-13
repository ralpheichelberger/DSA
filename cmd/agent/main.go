package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/dropshipagent/agent/internal/agent"
	"github.com/dropshipagent/agent/internal/api"
	metaint "github.com/dropshipagent/agent/internal/integrations/meta"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/integrations/openai"
	"github.com/dropshipagent/agent/internal/integrations/sup"
	tiktokint "github.com/dropshipagent/agent/internal/integrations/tiktok"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	var logger *zap.Logger
	if cfg.DevMode {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	if err := os.MkdirAll("./data", 0o755); err != nil {
		logger.Fatal("ensure data directory", zap.Error(err))
	}

	st, err := store.New(cfg.DatabasePath)
	if err != nil {
		logger.Fatal("open store", zap.Error(err))
	}
	defer func() { _ = st.Close() }()

	discoverer := minea.NewDiscoverer(cfg, logger)
	supplier := sup.NewSupplier(cfg)
	metaClient := metaint.NewMetaPlatform(cfg)
	tikTokClient := tiktokint.NewTikTokPlatform(cfg)
	aiClient := openai.New(cfg.OpenAIAPIKey)

	theAgent := agent.New(cfg, st, aiClient, discoverer, supplier, metaClient, tikTokClient, logger)
	server := api.New(cfg, theAgent, st, aiClient, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		theAgent.Run(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(ctx, httpAddr(cfg.Port)); err != nil && ctx.Err() == nil {
			logger.Error("http server", zap.Error(err))
		}
	}()

	logger.Sugar().Infof("Agent running on %s — DEV_MODE: %v", httpAddr(cfg.Port), cfg.DevMode)

	<-ctx.Done()
	logger.Info("Shutting down")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		logger.Warn("graceful shutdown timed out after 5s")
	}
}

// httpAddr normalizes cfg.Port ("8080" -> ":8080"; leaves "host:port" unchanged).
func httpAddr(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return ":8080"
	}
	if strings.HasPrefix(port, ":") {
		return port
	}
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}
