package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	s, err := New(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() {
		_ = s.Close()
	})

	return s
}

func TestSaveAndGetProductTest(t *testing.T) {
	testCases := []struct {
		name      string
		productID string
		exists    bool
	}{
		{name: "roundtrip existing product", productID: "pt-1", exists: true},
		{name: "non-existent product returns nil", productID: "missing", exists: false},
	}

	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	product := ProductTest{
		ID:              "pt-1",
		ProductName:     "Magnetic Cable",
		ProductImageURL: "https://img.example.com/magnetic-cable.jpg",
		AdURL:           "https://www.facebook.com/ads/library/?id=1",
		ShopURL:         "https://store.example.com",
		LandingURL:      "https://store.example.com/products/cable",
		Niche:           "tech",
		ShopifyStore:    "tech",
		SourcePlatform:  "minea",
		Supplier:        "Supplier A",
		COGSEur:         9.5,
		SellPriceEur:    29.99,
		GrossMarginPct:  55.0,
		BEROAS:          1.82,
		ShippingCostEur: 3.2,
		ShippingDays:    7,
		Status:          "watching",
		KillReason:      "",
		Score:           78,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, s.SaveProductTest(product))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.GetProductTest(tc.productID)
			require.NoError(t, err)

			if !tc.exists {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, product.ID, got.ID)
			assert.Equal(t, product.ProductName, got.ProductName)
			assert.Equal(t, product.ProductImageURL, got.ProductImageURL)
			assert.Equal(t, product.AdURL, got.AdURL)
			assert.Equal(t, product.ShopURL, got.ShopURL)
			assert.Equal(t, product.LandingURL, got.LandingURL)
			assert.Equal(t, product.Niche, got.Niche)
			assert.Equal(t, product.ShopifyStore, got.ShopifyStore)
			assert.Equal(t, product.SourcePlatform, got.SourcePlatform)
			assert.Equal(t, product.Supplier, got.Supplier)
			assert.Equal(t, product.COGSEur, got.COGSEur)
			assert.Equal(t, product.SellPriceEur, got.SellPriceEur)
			assert.Equal(t, product.GrossMarginPct, got.GrossMarginPct)
			assert.Equal(t, product.BEROAS, got.BEROAS)
			assert.Equal(t, product.ShippingCostEur, got.ShippingCostEur)
			assert.Equal(t, product.ShippingDays, got.ShippingDays)
			assert.Equal(t, product.Status, got.Status)
			assert.Equal(t, product.KillReason, got.KillReason)
			assert.Equal(t, product.Score, got.Score)
			assert.WithinDuration(t, product.CreatedAt, got.CreatedAt, time.Second)
			assert.WithinDuration(t, product.UpdatedAt, got.UpdatedAt, time.Second)
		})
	}
}

func TestUpdateProductStatus(t *testing.T) {
	testCases := []struct {
		name       string
		nextStatus string
		killReason string
	}{
		{name: "updates status and reason", nextStatus: "killed", killReason: "cpa exceeded gross profit"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			initialTime := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
			product := ProductTest{
				ID:              "pt-status",
				ProductName:     "Pet Groomer",
				Niche:           "pets",
				ShopifyStore:    "pets",
				SourcePlatform:  "manual",
				Supplier:        "Supplier B",
				COGSEur:         7,
				SellPriceEur:    24,
				GrossMarginPct:  45,
				BEROAS:          2.22,
				ShippingCostEur: 2,
				ShippingDays:    10,
				Status:          "watching",
				Score:           65,
				CreatedAt:       initialTime,
				UpdatedAt:       initialTime,
			}
			require.NoError(t, s.SaveProductTest(product))

			require.NoError(t, s.UpdateProductStatus(product.ID, tc.nextStatus, tc.killReason))

			got, err := s.GetProductTest(product.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tc.nextStatus, got.Status)
			assert.Equal(t, tc.killReason, got.KillReason)
			assert.True(t, got.UpdatedAt.After(initialTime))
		})
	}
}

func TestGetProductsByStatus(t *testing.T) {
	testCases := []struct {
		name         string
		filterStatus string
		wantCount    int
	}{
		{name: "returns only testing products", filterStatus: "testing", wantCount: 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			now := time.Now().UTC()

			products := []ProductTest{
				{ID: "p1", ProductName: "A", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "minea", Status: "testing", CreatedAt: now, UpdatedAt: now},
				{ID: "p2", ProductName: "B", Niche: "pets", ShopifyStore: "pets", SourcePlatform: "minea", Status: "watching", CreatedAt: now, UpdatedAt: now},
				{ID: "p3", ProductName: "C", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "manual", Status: "testing", CreatedAt: now, UpdatedAt: now},
			}

			for _, p := range products {
				require.NoError(t, s.SaveProductTest(p))
			}

			got, err := s.GetProductsByStatus(tc.filterStatus)
			require.NoError(t, err)
			require.Len(t, got, tc.wantCount)
			for _, p := range got {
				assert.Equal(t, tc.filterStatus, p.Status)
			}
		})
	}
}

func TestCampaignResultRoundTrip(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{name: "save and fetch by product test id"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			now := time.Now().UTC()
			product := ProductTest{
				ID: "pt-campaign", ProductName: "Portable Blender", Niche: "tech", ShopifyStore: "tech",
				SourcePlatform: "minea", Status: "testing", CreatedAt: now, UpdatedAt: now,
			}
			require.NoError(t, s.SaveProductTest(product))

			cr := CampaignResult{
				ID:            "cr-1",
				ProductTestID: product.ID,
				Platform:      "meta",
				CampaignID:    "meta-42",
				SpendEur:      120.5,
				RevenueEur:    301.25,
				ROAS:          2.5,
				CTRPct:        1.8,
				CPAEur:        14.2,
				Impressions:   10000,
				Clicks:        210,
				Purchases:     8,
				DaysRunning:   3,
				SnapshotDate:  now,
				CreatedAt:     now,
			}
			require.NoError(t, s.SaveCampaignResult(cr))

			got, err := s.GetCampaignResultsForProduct(product.ID)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, cr.ROAS, got[0].ROAS)
			assert.Equal(t, cr.SpendEur, got[0].SpendEur)
		})
	}
}

func TestBuildMemoryContext(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(t *testing.T, s *Store)
		check func(t *testing.T, context string)
	}{
		{
			name: "empty db returns headers",
			setup: func(t *testing.T, s *Store) {
				t.Helper()
			},
			check: func(t *testing.T, context string) {
				t.Helper()
				assert.NotEmpty(t, context)
				assert.Contains(t, context, "## Lessons from past campaigns:")
				assert.Contains(t, context, "## Recent product outcomes (last 10):")
			},
		},
		{
			name: "contains lessons and products",
			setup: func(t *testing.T, s *Store) {
				t.Helper()
				now := time.Now().UTC()

				require.NoError(t, s.SaveLearnedLesson(LearnedLesson{
					ID: "l1", Category: "creative", Lesson: "UGC hooks outperform static intros",
					Confidence: 0.9, EvidenceCount: 5, CreatedAt: now, UpdatedAt: now,
				}))
				require.NoError(t, s.SaveLearnedLesson(LearnedLesson{
					ID: "l2", Category: "platform", Lesson: "Meta retargeting closes better than cold",
					Confidence: 0.8, EvidenceCount: 3, CreatedAt: now, UpdatedAt: now,
				}))

				for _, p := range []ProductTest{
					{ID: "bp1", ProductName: "Smart Lamp", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "minea", Status: "testing", GrossMarginPct: 42, Score: 81, CreatedAt: now, UpdatedAt: now},
					{ID: "bp2", ProductName: "Pet Trimmer", Niche: "pets", ShopifyStore: "pets", SourcePlatform: "manual", Status: "killed", GrossMarginPct: 33, Score: 50, CreatedAt: now, UpdatedAt: now},
					{ID: "bp3", ProductName: "Ergo Mouse", Niche: "tech", ShopifyStore: "tech", SourcePlatform: "minea", Status: "scaling", GrossMarginPct: 48, Score: 90, CreatedAt: now, UpdatedAt: now},
				} {
					require.NoError(t, s.SaveProductTest(p))
				}
			},
			check: func(t *testing.T, context string) {
				t.Helper()
				assert.Contains(t, context, "UGC hooks outperform static intros")
				assert.Contains(t, context, "Meta retargeting closes better than cold")
				assert.Contains(t, context, "Smart Lamp")
				assert.Contains(t, context, "Pet Trimmer")
				assert.Contains(t, context, "Ergo Mouse")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			tc.setup(t, s)

			context, err := s.BuildMemoryContext()
			require.NoError(t, err)
			tc.check(t, context)
		})
	}
}

func TestLearnedLessonConfidenceUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		newConfidence  float64
		newEvidenceNum int
	}{
		{name: "updates confidence and evidence count", newConfidence: 0.9, newEvidenceNum: 9},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			now := time.Now().UTC()
			initial := LearnedLesson{
				ID:            "lesson-1",
				Category:      "product",
				Lesson:        "Pet niche has lower CPM on TikTok",
				Confidence:    0.5,
				EvidenceCount: 2,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			require.NoError(t, s.SaveLearnedLesson(initial))
			require.NoError(t, s.UpdateLessonConfidence(initial.ID, tc.newConfidence, tc.newEvidenceNum))

			all, err := s.GetAllLessons()
			require.NoError(t, err)
			require.Len(t, all, 1)
			assert.Equal(t, tc.newConfidence, all[0].Confidence)
			assert.Equal(t, tc.newEvidenceNum, all[0].EvidenceCount)
		})
	}
}
