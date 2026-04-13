package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dropshipagent/agent/config"
	metaint "github.com/dropshipagent/agent/internal/integrations/meta"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/integrations/openai"
	"github.com/dropshipagent/agent/internal/integrations/sup"
	tiktokint "github.com/dropshipagent/agent/internal/integrations/tiktok"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockReasoner struct {
	reasonResponse string
	creativeBriefs []openai.CreativeBrief
	lessons        []openai.LessonDraft
}

func (m *mockReasoner) Reason(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	return m.reasonResponse, nil
}
func (m *mockReasoner) GenerateCreativeBriefs(ctx context.Context, productName string, niche string, angles []string) ([]openai.CreativeBrief, error) {
	return m.creativeBriefs, nil
}
func (m *mockReasoner) ExtractLessons(ctx context.Context, productSummary string, campaignSummary string) ([]openai.LessonDraft, error) {
	return m.lessons, nil
}

type mockDiscoverer struct {
	products []minea.ProductCandidate
	err      error
}

func (m *mockDiscoverer) GetTrendingProducts(ctx context.Context, niche string, country string, limit int) ([]minea.ProductCandidate, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []minea.ProductCandidate
	for _, p := range m.products {
		if niche == "" || p.Niche == niche {
			out = append(out, p)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *mockDiscoverer) GetProductDetails(ctx context.Context, productID string) (*minea.ProductCandidate, error) {
	return nil, errors.New("not implemented")
}

type mockSupplier struct {
	data      *sup.SupplierData
	err       error
	errByProd map[string]error
}

func (m *mockSupplier) GetProductCost(ctx context.Context, productID string) (*sup.SupplierData, error) {
	if m.errByProd != nil {
		if err, ok := m.errByProd[productID]; ok {
			return nil, err
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	cp := *m.data
	cp.ProductID = productID
	return &cp, nil
}
func (m *mockSupplier) ImportProduct(ctx context.Context, productID string, shopifyDomain string) error {
	return nil
}

type mockMetaPlatform struct {
	campaignID string
	metrics    *metaint.CampaignMetrics
	paused     bool
	scaled     bool
}

func (m *mockMetaPlatform) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []metaint.AdCreative) (string, error) {
	if m.campaignID == "" {
		m.campaignID = "meta-c-1"
	}
	return m.campaignID, nil
}
func (m *mockMetaPlatform) GetMetrics(ctx context.Context, campaignID string) (*metaint.CampaignMetrics, error) {
	if m.metrics == nil {
		return &metaint.CampaignMetrics{CampaignID: campaignID}, nil
	}
	return m.metrics, nil
}
func (m *mockMetaPlatform) PauseCampaign(ctx context.Context, campaignID string) error {
	m.paused = true
	return nil
}
func (m *mockMetaPlatform) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	m.scaled = true
	return nil
}

type mockTikTokPlatform struct {
	campaignID string
	metrics    *tiktokint.CampaignMetrics
	paused     bool
	scaled     bool
}

func (m *mockTikTokPlatform) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []tiktokint.AdCreative) (string, error) {
	if m.campaignID == "" {
		m.campaignID = "tt-c-1"
	}
	return m.campaignID, nil
}
func (m *mockTikTokPlatform) GetMetrics(ctx context.Context, campaignID string) (*tiktokint.CampaignMetrics, error) {
	if m.metrics == nil {
		return &tiktokint.CampaignMetrics{CampaignID: campaignID}, nil
	}
	return m.metrics, nil
}
func (m *mockTikTokPlatform) PauseCampaign(ctx context.Context, campaignID string) error {
	m.paused = true
	return nil
}
func (m *mockTikTokPlatform) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	m.scaled = true
	return nil
}
func (m *mockTikTokPlatform) GetTrendingAudio(ctx context.Context) ([]string, error) {
	return []string{"a1"}, nil
}

func newTestAgent(t *testing.T) (*Agent, *store.Store, *mockMetaPlatform, *mockTikTokPlatform) {
	t.Helper()
	db, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		DevMode:              true,
		AutoApprove:          true,
		AgentIntervalHours:   6,
		ShopifyTechDomain:    "tech.example.com",
		ShopifyPetsDomain:    "pets.example.com",
		NotifyMinSeverity:    "critical",
		NotifySMTPHost:       "smtp.gmail.com",
		NotifySMTPPort:       587,
		NotifyTelegramChatID: "chat",
	}
	metaMock := &mockMetaPlatform{}
	ttMock := &mockTikTokPlatform{}
	a := New(cfg, db,
		&mockReasoner{reasonResponse: `[{"product_id":"p1","platform":"both","daily_budget_eur":20,"creative_angles":["problem_solution","transformation"]}]`},
		&mockDiscoverer{},
		&mockSupplier{data: &sup.SupplierData{COGSEur: 10, ShippingCostEur: 3, ShippingDays: 7, WarehouseRegion: "EU", StockAvailable: true}},
		metaMock, ttMock,
		zap.NewNop(),
	)
	return a, db, metaMock, ttMock
}

func TestRunDiscovery_FiltersLowScoreProducts(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	a.discoverer = &mockDiscoverer{products: []minea.ProductCandidate{
		{ID: "p1", Name: "Tech 1", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 40, GoogleTrending: true, TikTokOrganic: true},
		{ID: "p2", Name: "Tech 2", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 50, GoogleTrending: true, TikTokOrganic: true},
		{ID: "p3", Name: "Pet 1", Niche: "pets", ShopifyStore: "pets", EstimatedSellEur: 45, GoogleTrending: true, TikTokOrganic: true},
		{ID: "p4", Name: "Low 1", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 18},
		{ID: "p5", Name: "Low 2", Niche: "pets", ShopifyStore: "pets", EstimatedSellEur: 17},
	}}
	a.supplier = &mockSupplier{data: &sup.SupplierData{COGSEur: 10, ShippingCostEur: 3, ShippingDays: 7, StockAvailable: true}}

	got, err := a.runDiscovery(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestRunDiscovery_SkipsProductsWithNoCost(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	a.discoverer = &mockDiscoverer{products: []minea.ProductCandidate{
		{ID: "p1", Name: "Tech 1", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 40, GoogleTrending: true},
		{ID: "p2", Name: "Tech 2", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 42, GoogleTrending: true},
	}}
	a.supplier = &mockSupplier{
		data:      &sup.SupplierData{COGSEur: 10, ShippingCostEur: 3, ShippingDays: 7, StockAvailable: true},
		errByProd: map[string]error{"p2": errors.New("missing cost")},
	}
	got, err := a.runDiscovery(context.Background())
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "p1", got[0].ID)
}

func TestRunMonitoring_KillsUnprofitableCampaign(t *testing.T) {
	a, db, metaMock, _ := newTestAgent(t)
	now := time.Now().UTC()
	require.NoError(t, db.SaveProductTest(store.ProductTest{
		ID: "p1", ProductName: "Prod", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "minea", Supplier: "s",
		COGSEur: 20, SellPriceEur: 40, ShippingCostEur: 0, GrossMarginPct: 50, BEROAS: 2, ShippingDays: 7,
		Status: "testing", Score: 80, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, db.SaveCampaignResult(store.CampaignResult{
		ID: "c1", ProductTestID: "p1", Platform: "meta", CampaignID: "m1", DaysRunning: 3, SnapshotDate: now, CreatedAt: now,
	}))
	metaMock.metrics = &metaint.CampaignMetrics{CampaignID: "m1", SpendEur: 25, RevenueEur: 0, Purchases: 0, DaysRunning: 3, CTRPct: 1.0}

	require.NoError(t, a.runMonitoring(context.Background()))
	pt, err := db.GetProductTest("p1")
	require.NoError(t, err)
	require.NotNil(t, pt)
	assert.Equal(t, "killed", pt.Status)
}

func TestRunMonitoring_ScalesWinningCampaign(t *testing.T) {
	a, db, _, ttMock := newTestAgent(t)
	a.cfg.AutoApprove = true
	now := time.Now().UTC()
	require.NoError(t, db.SaveProductTest(store.ProductTest{
		ID: "p2", ProductName: "Winner", Niche: "pets", ShopifyStore: "pets", SourcePlatform: "minea", Supplier: "s",
		COGSEur: 10, SellPriceEur: 25, ShippingCostEur: 0, GrossMarginPct: 60, BEROAS: 3, ShippingDays: 7,
		Status: "testing", Score: 90, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, db.SaveCampaignResult(store.CampaignResult{
		ID: "c2", ProductTestID: "p2", Platform: "tiktok", CampaignID: "t1", DaysRunning: 6, SnapshotDate: now, CreatedAt: now,
	}))
	ttMock.metrics = &tiktokint.CampaignMetrics{CampaignID: "t1", SpendEur: 100, RevenueEur: 450, ROAS: 4.5, Purchases: 10, DaysRunning: 6, CTRPct: 1.0}

	require.NoError(t, a.runMonitoring(context.Background()))
	select {
	case msg := <-a.outbox:
		assert.NotEmpty(t, msg.Type)
	default:
		t.Fatal("expected outbox message for scale action")
	}
}

func TestRunLearning_SavesLessons(t *testing.T) {
	a, db, _, _ := newTestAgent(t)
	now := time.Now().UTC()
	require.NoError(t, db.SaveProductTest(store.ProductTest{
		ID: "p3", ProductName: "Killed Product", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "minea", Supplier: "s",
		COGSEur: 10, SellPriceEur: 30, ShippingCostEur: 2, GrossMarginPct: 60, BEROAS: 2, ShippingDays: 7,
		Status: "killed", Score: 50, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, db.SaveCampaignResult(store.CampaignResult{
		ID: "c3", ProductTestID: "p3", Platform: "meta", CampaignID: "m3", SpendEur: 40, RevenueEur: 0, ROAS: 0, Purchases: 0, DaysRunning: 4, SnapshotDate: now, CreatedAt: now,
	}))
	a.reasoner = &mockReasoner{lessons: []openai.LessonDraft{
		{Category: "creative", Lesson: "Hook failed in first 3 seconds", Confidence: 0.8},
		{Category: "platform", Lesson: "TikTok better for discovery", Confidence: 0.7},
	}}

	require.NoError(t, a.runLearning(context.Background()))
	lessons, err := db.GetAllLessons()
	require.NoError(t, err)
	assert.Len(t, lessons, 2)
}

func TestAgent_FullCycleDevMode(t *testing.T) {
	a, _, _, _ := newTestAgent(t)
	a.discoverer = &mockDiscoverer{products: []minea.ProductCandidate{
		{ID: "p1", Name: "Wireless Charger", Niche: "tech", ShopifyStore: "tech", EstimatedSellEur: 39, GoogleTrending: true, TikTokOrganic: true, ActiveAdCount: 10},
	}}
	a.supplier = &mockSupplier{data: &sup.SupplierData{COGSEur: 10, ShippingCostEur: 3, ShippingDays: 7, StockAvailable: true}}
	a.reasoner = &mockReasoner{
		reasonResponse: `[{"product_id":"p1","platform":"both","daily_budget_eur":20,"creative_angles":["problem_solution","transformation"]}]`,
		creativeBriefs: []openai.CreativeBrief{{Angle: "problem_solution", Headline: "H1", Body: "B", CTA: "Buy", Platform: "both"}},
		lessons:        []openai.LessonDraft{{Category: "product", Lesson: "Good margin", Confidence: 0.6}},
	}

	require.NoError(t, a.runCycle(context.Background()))
	select {
	case <-a.outbox:
	default:
		t.Fatal("expected at least one outbox message")
	}
}
