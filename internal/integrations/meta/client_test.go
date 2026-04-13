package meta

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
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "camp_123"})
	}))
	defer srv.Close()

	c := New("token", "acc")
	c.baseURL = srv.URL
	id, err := c.CreateCampaign(context.Background(), "Smart Lamp", 25, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

func TestGetMetrics_ParsesSpendAndROAS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"spend":       "100",
					"revenue":     "260",
					"impressions": "10000",
					"clicks":      "300",
					"ctr":         "3.0",
					"actions": []map[string]any{
						{"action_type": "purchase", "value": "5"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := New("token", "acc")
	c.baseURL = srv.URL
	m, err := c.GetMetrics(context.Background(), "camp_1")
	require.NoError(t, err)
	assert.Equal(t, 2.6, m.ROAS)
	assert.Equal(t, 100.0, m.SpendEur)
}

func TestGetMetrics_NoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	c := New("token", "acc")
	c.baseURL = srv.URL
	m, err := c.GetMetrics(context.Background(), "camp_1")
	require.NoError(t, err)
	assert.Equal(t, 0.0, m.SpendEur)
	assert.Equal(t, 0.0, m.ROAS)
}
