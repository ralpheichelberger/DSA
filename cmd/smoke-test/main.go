package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
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
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// stubReasoner satisfies openai.Reasoner without calling the real API (smoke test only).
type stubReasoner struct{}

func (stubReasoner) Reason(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	_ = ctx
	_ = systemPrompt
	if strings.Contains(strings.ToLower(userPrompt), "beroas") && strings.Contains(userPrompt, "35") {
		return "For 35% gross margin, BEROAS = 100/35, approximately 2.86.", nil
	}
	return "ok", nil
}

func (stubReasoner) GenerateCreativeBriefs(ctx context.Context, productName, niche string, angles []string) ([]openai.CreativeBrief, error) {
	return nil, nil
}

func (stubReasoner) ExtractLessons(ctx context.Context, productSummary, campaignSummary string) ([]openai.LessonDraft, error) {
	return nil, nil
}

func main() {
	_ = godotenv.Load()
	_ = os.Setenv("DEV_MODE", "true")
	_ = os.Setenv("AUTO_APPROVE", "true")
	_ = os.Setenv("DB_PATH", ":memory:")

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-test failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	logger := zap.NewNop()

	st, err := store.New(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer func() { _ = st.Close() }()

	discoverer := minea.NewDiscoverer(cfg, logger)
	supplier := sup.NewSupplier(cfg)
	metaClient := metaint.NewMetaPlatform(cfg)
	tikTokClient := tiktokint.NewTikTokPlatform(cfg)
	reasoner := stubReasoner{}

	theAgent := agent.New(cfg, st, reasoner, discoverer, supplier, metaClient, tikTokClient, logger)
	server := api.New(cfg, theAgent, st, reasoner, logger)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = ln.Close() }()

	baseURL := "http://" + ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		theAgent.Run(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Serve(ctx, ln)
	}()

	time.Sleep(3 * time.Second)

	client := &http.Client{Timeout: 15 * time.Second}

	if err := checkHealth(client, baseURL); err != nil {
		return fmt.Errorf("health: %w", err)
	}
	if err := checkProducts(client, baseURL); err != nil {
		return fmt.Errorf("products: %w", err)
	}
	if err := checkWebSocket(baseURL); err != nil {
		return fmt.Errorf("websocket: %w", err)
	}
	if err := checkChat(client, baseURL); err != nil {
		return fmt.Errorf("chat: %w", err)
	}

	cancel()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		return fmt.Errorf("shutdown wait timed out")
	}

	return nil
}

func checkHealth(c *http.Client, base string) error {
	resp, err := c.Get(base + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	var m map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	if m["status"] != "ok" {
		return fmt.Errorf("expected status ok, got %v", m["status"])
	}
	return nil
}

func checkProducts(c *http.Client, base string) error {
	resp, err := c.Get(base + "/api/products")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if !json.Valid(body) {
		return fmt.Errorf("invalid json: %s", body)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(body, &arr); err != nil {
		return fmt.Errorf("decode array: %w", err)
	}
	return nil
}

func checkWebSocket(base string) error {
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/ws"
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	s := strings.ToLower(string(data))
	if !strings.Contains(s, "connected") {
		return fmt.Errorf("welcome message missing connected: %s", data)
	}
	return nil
}

func checkChat(c *http.Client, base string) error {
	payload := map[string]string{"message": "What is the BEROAS for a product with 35% margin?"}
	b, _ := json.Marshal(payload)
	resp, err := c.Post(base+"/api/chat", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Reply string `json:"reply"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	r := out.Reply
	if !strings.Contains(r, "2.8") && !strings.Contains(r, "2.86") {
		return fmt.Errorf("reply missing BEROAS value: %q", r)
	}
	return nil
}
