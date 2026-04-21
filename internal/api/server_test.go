package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/dropshipagent/agent/internal/agent"
	metaint "github.com/dropshipagent/agent/internal/integrations/meta"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/integrations/openai"
	"github.com/dropshipagent/agent/internal/integrations/sup"
	tiktokint "github.com/dropshipagent/agent/internal/integrations/tiktok"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockStore struct {
	products  []store.ProductTest
	campaigns []store.CampaignResult
	lessons   []store.LearnedLesson
	memory    string
	memErr    error
}

func (m *mockStore) GetAllProducts() ([]store.ProductTest, error) {
	return m.products, nil
}

func (m *mockStore) GetActiveCampaigns() ([]store.CampaignResult, error) {
	return m.campaigns, nil
}

func (m *mockStore) GetAllLessons() ([]store.LearnedLesson, error) {
	return m.lessons, nil
}

func (m *mockStore) BuildMemoryContext() (string, error) {
	if m.memErr != nil {
		return "", m.memErr
	}
	return m.memory, nil
}

func (m *mockStore) GetProductTest(id string) (*store.ProductTest, error) {
	for i := range m.products {
		if m.products[i].ID == id {
			p := m.products[i]
			return &p, nil
		}
	}
	return nil, nil
}

func (m *mockStore) SaveProductTest(pt store.ProductTest) error {
	for i := range m.products {
		if m.products[i].ID == pt.ID {
			m.products[i] = pt
			return nil
		}
	}
	m.products = append(m.products, pt)
	return nil
}

type mockReasoner struct {
	reply string
}

func (m *mockReasoner) Reason(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.reply, nil
}

func (m *mockReasoner) GenerateCreativeBriefs(ctx context.Context, productName, niche string, angles []string) ([]openai.CreativeBrief, error) {
	return nil, nil
}

func (m *mockReasoner) ExtractLessons(ctx context.Context, productSummary, campaignSummary string) ([]openai.LessonDraft, error) {
	return nil, nil
}

type mockDiscoverer struct{}

func (m *mockDiscoverer) GetTrendingProducts(ctx context.Context, niche, country string, limit int) ([]minea.ProductCandidate, error) {
	return nil, nil
}

func (m *mockDiscoverer) GetProductDetails(ctx context.Context, productID string) (*minea.ProductCandidate, error) {
	return nil, nil
}

type mockSupplier struct{}

func (m *mockSupplier) GetProductCost(ctx context.Context, productID string) (*sup.SupplierData, error) {
	return nil, nil
}

func (m *mockSupplier) ImportProduct(ctx context.Context, productID, shopifyDomain string) error {
	return nil
}

type noopMeta struct{}

func (noopMeta) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []metaint.AdCreative) (string, error) {
	return "", nil
}

func (noopMeta) GetMetrics(ctx context.Context, campaignID string) (*metaint.CampaignMetrics, error) {
	return nil, nil
}

func (noopMeta) PauseCampaign(ctx context.Context, campaignID string) error { return nil }

func (noopMeta) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	return nil
}

type noopTikTok struct{}

func (noopTikTok) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []tiktokint.AdCreative) (string, error) {
	return "", nil
}

func (noopTikTok) GetMetrics(ctx context.Context, campaignID string) (*tiktokint.CampaignMetrics, error) {
	return nil, nil
}

func (noopTikTok) PauseCampaign(ctx context.Context, campaignID string) error { return nil }

func (noopTikTok) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	return nil
}

func (noopTikTok) GetTrendingAudio(ctx context.Context) ([]string, error) { return nil, nil }

// testAgentBridge implements AgentConn with a receivable approval channel for tests.
type testAgentBridge struct {
	approvalCh chan agent.ApprovalResponse
	outboxCh   chan agent.Message
}

func newTestAgentBridge() *testAgentBridge {
	return &testAgentBridge{
		approvalCh: make(chan agent.ApprovalResponse, 10),
		outboxCh:   make(chan agent.Message, 100),
	}
}

func (b *testAgentBridge) ApprovalChan() chan<- agent.ApprovalResponse { return b.approvalCh }

func (b *testAgentBridge) Outbox() <-chan agent.Message { return b.outboxCh }

func testAgent(t *testing.T, st *store.Store) *agent.Agent {
	t.Helper()
	cfg := &config.Config{DevMode: true}
	return agent.New(cfg, st, &mockReasoner{}, &mockDiscoverer{}, &mockSupplier{}, noopMeta{}, noopTikTok{}, zap.NewNop())
}

func startTestServer(t *testing.T, srv *Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(srv.router)
	t.Cleanup(ts.Close)
	return ts
}

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{DevMode: true}
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ag := testAgent(t, st)
	srv := New(cfg, ag, st, &mockReasoner{}, zap.NewNop())

	ts := startTestServer(t, srv)
	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, true, body["dev_mode"])
	assert.Equal(t, "0.1.0", body["version"])
}

func TestGetProducts_ReturnsJSON(t *testing.T) {
	cfg := &config.Config{DevMode: false}
	ms := &mockStore{
		products: []store.ProductTest{
			{ID: "a", ProductName: "P1", Status: "watching"},
			{ID: "b", ProductName: "P2", Status: "watching"},
		},
	}
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ag := testAgent(t, st)
	srv := New(cfg, ag, ms, &mockReasoner{}, zap.NewNop())

	ts := startTestServer(t, srv)
	resp, err := http.Get(ts.URL + "/api/products")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var arr []store.ProductTest
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&arr))
	assert.Len(t, arr, 2)
}

func TestGetProducts_FilterByStatus(t *testing.T) {
	cfg := &config.Config{}
	ms := &mockStore{
		products: []store.ProductTest{
			{ID: "1", ProductName: "A", Status: "watching"},
			{ID: "2", ProductName: "B", Status: "testing"},
			{ID: "3", ProductName: "C", Status: "testing"},
		},
	}
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ag := testAgent(t, st)
	srv := New(cfg, ag, ms, &mockReasoner{}, zap.NewNop())

	ts := startTestServer(t, srv)
	resp, err := http.Get(ts.URL + "/api/products?status=testing")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var arr []store.ProductTest
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&arr))
	require.Len(t, arr, 2)
	for _, p := range arr {
		assert.Equal(t, "testing", p.Status)
	}
}

func TestApprove_SendsToChannel(t *testing.T) {
	cfg := &config.Config{}
	bridge := newTestAgentBridge()
	ms := &mockStore{}
	srv := New(cfg, bridge, ms, &mockReasoner{}, zap.NewNop())

	done := make(chan agent.ApprovalResponse, 1)
	go func() {
		select {
		case r := <-bridge.approvalCh:
			done <- r
		case <-time.After(2 * time.Second):
		}
	}()

	ts := startTestServer(t, srv)
	body := `{"product_test_id":"pid-1","approved":true,"note":"ok"}`
	resp, err := http.Post(ts.URL+"/api/approve", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case r := <-done:
		assert.Equal(t, "pid-1", r.ProductTestID)
		assert.True(t, r.Approved)
		assert.Equal(t, "ok", r.Note)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ApprovalResponse")
	}
}

func TestChatEndpoint_ReturnsReply(t *testing.T) {
	cfg := &config.Config{}
	ms := &mockStore{memory: ""}
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ag := testAgent(t, st)
	mr := &mockReasoner{reply: "test reply"}
	srv := New(cfg, ag, ms, mr, zap.NewNop())

	ts := startTestServer(t, srv)
	resp, err := http.Post(ts.URL+"/api/chat", "application/json", strings.NewReader(`{"message":"hello"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		Reply string `json:"reply"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Contains(t, out.Reply, "test reply")
}

func TestWebSocket_ConnectsAndReceivesWelcome(t *testing.T) {
	cfg := &config.Config{DevMode: true}
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	ag := testAgent(t, st)
	srv := New(cfg, ag, st, &mockReasoner{}, zap.NewNop())

	ts := startTestServer(t, srv)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &msg))
	assert.Equal(t, "connected", msg["type"])
	m, ok := msg["message"].(string)
	require.True(t, ok)
	assert.Contains(t, strings.ToLower(m), "agent online")
	assert.Contains(t, m, "DEV_MODE")
}
