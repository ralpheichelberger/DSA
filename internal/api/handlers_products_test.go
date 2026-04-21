package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dropshipagent/agent/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestProductTestIsWinnerCandidate(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		pt   store.ProductTest
		want bool
	}{
		{"killed never", store.ProductTest{Status: "killed", Score: 99, UpdatedAt: now}, false},
		{"scaling", store.ProductTest{Status: "scaling", Score: 1, UpdatedAt: now}, true},
		{"testing", store.ProductTest{Status: "testing", Score: 1, UpdatedAt: now}, true},
		{"watching economics score path", store.ProductTest{Status: "watching", Score: 60, COGSEur: 5, GrossMarginPct: 36, UpdatedAt: now}, true},
		{"watching economics beroas path", store.ProductTest{Status: "watching", Score: 40, COGSEur: 5, GrossMarginPct: 42, BEROAS: 1.6, UpdatedAt: now}, true},
		{"watching weak score", store.ProductTest{Status: "watching", Score: 50, COGSEur: 5, GrossMarginPct: 40, UpdatedAt: now}, false},
		{"watching scrape no cogs", store.ProductTest{Status: "watching", Score: 95, COGSEur: 0, UpdatedAt: now}, false},
		{"watching borderline economics", store.ProductTest{Status: "watching", Score: 55, COGSEur: 5, GrossMarginPct: 38, BEROAS: 1.4, UpdatedAt: now}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, productTestIsWinnerCandidate(tc.pt), tc.name)
		})
	}
}

func TestGetProducts_ViewWinners(t *testing.T) {
	now := time.Now().UTC()
	ms := &mockStore{
		products: []store.ProductTest{
			{ID: "w1", ProductName: "Scaling", Status: "scaling", Score: 10, CreatedAt: now, UpdatedAt: now},
			{ID: "w2", ProductName: "Minea scrape only", Status: "watching", Score: 95, COGSEur: 0, GrossMarginPct: 0, BEROAS: 0, CreatedAt: now, UpdatedAt: now},
			{ID: "w3", ProductName: "Priced candidate", Status: "watching", Score: 62, COGSEur: 8, GrossMarginPct: 40, BEROAS: 1.6, CreatedAt: now, UpdatedAt: now},
		},
	}
	srv := &Server{store: ms, logger: zap.NewNop()}
	ts := httptest.NewServer(http.HandlerFunc(srv.handleGetProducts))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/products?view=winners")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got []store.ProductTest
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	ids := make(map[string]bool)
	for _, p := range got {
		ids[p.ID] = true
	}
	assert.True(t, ids["w1"])
	assert.True(t, ids["w3"])
	assert.False(t, ids["w2"])
}
