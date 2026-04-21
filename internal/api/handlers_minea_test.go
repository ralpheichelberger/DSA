package api

import (
	"testing"
	"time"

	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMergeMineaSearchRow_New(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c := minea.ProductCandidate{
		ID:               "ad-1",
		Name:             "Widget",
		Niche:            "tech",
		ShopifyStore:     "gadget",
		SupplierID:       "sup-9",
		ImageURL:         "https://img.example/w.jpg",
		AdURL:            "https://www.facebook.com/ads/library/?id=42",
		ShopURL:          "https://gadget.myshopify.com",
		LandingURL:       "https://gadget.myshopify.com/products/widget",
		EstimatedSellEur: 29.99,
		EngagementScore:  72.4,
	}
	pt := mergeMineaSearchRow(nil, c, now)
	assert.Equal(t, "ad-1", pt.ID)
	assert.Equal(t, c.AdURL, pt.AdURL)
	assert.Equal(t, c.ShopURL, pt.ShopURL)
	assert.Equal(t, c.LandingURL, pt.LandingURL)
	assert.Equal(t, "Widget", pt.ProductName)
	assert.Equal(t, "https://img.example/w.jpg", pt.ProductImageURL)
	assert.Equal(t, "tech", pt.Niche)
	assert.Equal(t, "gadget", pt.ShopifyStore)
	assert.Equal(t, "sup-9", pt.Supplier)
	assert.Equal(t, "minea", pt.SourcePlatform)
	assert.Equal(t, "watching", pt.Status)
	assert.InDelta(t, 29.99, pt.SellPriceEur, 0.001)
	assert.Equal(t, 72, pt.Score)
	assert.True(t, pt.CreatedAt.Equal(now))
	assert.True(t, pt.UpdatedAt.Equal(now))
}

func TestMergeMineaSearchRow_PreservesAgentEconomics(t *testing.T) {
	created := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	existing := &store.ProductTest{
		ID:              "ad-1",
		ProductName:     "Old",
		SourcePlatform:  "minea",
		COGSEur:         12,
		SellPriceEur:    40,
		GrossMarginPct:  55,
		BEROAS:          2.5,
		ShippingCostEur: 4,
		ShippingDays:    9,
		Status:          "testing",
		Score:           80,
		CreatedAt:       created,
	}
	c := minea.ProductCandidate{
		ID:              "ad-1",
		Name:            "New name",
		ImageURL:        "https://img.example/n.jpg",
		EngagementScore: 50,
	}
	pt := mergeMineaSearchRow(existing, c, now)
	assert.Equal(t, 12.0, pt.COGSEur)
	assert.Equal(t, 40.0, pt.SellPriceEur)
	assert.Equal(t, 55.0, pt.GrossMarginPct)
	assert.Equal(t, 2.5, pt.BEROAS)
	assert.Equal(t, "testing", pt.Status)
	assert.Equal(t, "New name", pt.ProductName)
	assert.Equal(t, "https://img.example/n.jpg", pt.ProductImageURL)
	assert.Equal(t, 80, pt.Score)
	assert.True(t, pt.CreatedAt.Equal(created))
	assert.True(t, pt.UpdatedAt.Equal(now))
}

func TestMergeMineaSearchRow_EngagementClampedTo100(t *testing.T) {
	now := time.Now().UTC()
	c := minea.ProductCandidate{ID: "big", EngagementScore: 1791.7}
	pt := mergeMineaSearchRow(nil, c, now)
	assert.Equal(t, 100, pt.Score)
}

func TestMergeMineaSearchRow_LegacyOver100ScoreNormalized(t *testing.T) {
	now := time.Now().UTC()
	existing := &store.ProductTest{
		ID:             "x",
		SourcePlatform: "minea",
		Status:         "watching",
		Score:          202,
		CreatedAt:      now,
	}
	c := minea.ProductCandidate{ID: "x", EngagementScore: 5}
	pt := mergeMineaSearchRow(existing, c, now)
	assert.Equal(t, 100, pt.Score)
}

func TestMergeMineaSearchRow_HigherSearchScoreUpdates(t *testing.T) {
	now := time.Now().UTC()
	existing := &store.ProductTest{
		ID:             "x",
		SourcePlatform: "minea",
		Status:         "watching",
		Score:          40,
		CreatedAt:      now,
	}
	c := minea.ProductCandidate{ID: "x", EngagementScore: 90}
	pt := mergeMineaSearchRow(existing, c, now)
	assert.Equal(t, 90, pt.Score)
}

func TestPersistMineaSearchResults_MockStore(t *testing.T) {
	ms := &mockStore{}
	s := &Server{store: ms, logger: zap.NewNop()}
	now := time.Now().UTC()
	ms.SaveProductTest(store.ProductTest{
		ID: "keep", SourcePlatform: "minea", Status: "scaling", COGSEur: 5,
		CreatedAt: now, UpdatedAt: now,
	})
	s.persistMineaSearchResults([]minea.ProductCandidate{
		{ID: "a1", Name: "One", EngagementScore: 10},
		{ID: "a2", Name: "Two"},
		{Name: "no id"},
	}, "pets")
	require.Len(t, ms.products, 3)
	var ids []string
	for _, p := range ms.products {
		ids = append(ids, p.ID)
	}
	assert.ElementsMatch(t, []string{"keep", "a1", "a2"}, ids)
	for _, p := range ms.products {
		switch p.ID {
		case "a1", "a2":
			assert.Equal(t, "pets", p.Niche)
		case "keep":
			assert.Equal(t, "", p.Niche)
		}
	}
}
