package tiktok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCampaign_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"campaign_id": "tt_123"}})
	}))
	defer srv.Close()

	c := New("token")
	c.baseURL = srv.URL
	id, err := c.CreateCampaign(context.Background(), "Pet Feeder", 20, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

func TestGetMetrics_ParsesSpendAndROAS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"list": []map[string]any{
					{"spend": 70.0, "revenue": 98.0, "impressions": 12000.0, "clicks": 220.0, "ctr": 1.8, "purchases": 4.0},
				},
			},
		})
	}))
	defer srv.Close()

	c := New("token")
	c.baseURL = srv.URL
	m, err := c.GetMetrics(context.Background(), "tt_1")
	require.NoError(t, err)
	assert.Equal(t, 1.4, m.ROAS)
	assert.Equal(t, 70.0, m.SpendEur)
}

func TestGetMetrics_NoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"list": []any{}}})
	}))
	defer srv.Close()

	c := New("token")
	c.baseURL = srv.URL
	m, err := c.GetMetrics(context.Background(), "tt_1")
	require.NoError(t, err)
	assert.Equal(t, 0.0, m.ROAS)
}
