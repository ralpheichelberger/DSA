package sup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProductCost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cogs":             12.5,
			"shipping_cost":    4.0,
			"shipping_days":    8,
			"warehouse_region": "EU",
			"stock_available":  true,
		})
	}))
	defer srv.Close()

	c := New("key")
	c.baseURL = srv.URL
	got, err := c.GetProductCost(context.Background(), "prod-1")
	require.NoError(t, err)
	assert.Greater(t, got.COGSEur, 0.0)
}

func TestGetProductCost_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New("key")
	c.baseURL = srv.URL
	_, err := c.GetProductCost(context.Background(), "prod-404")
	require.Error(t, err)
}

func TestImportProduct_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("key")
	c.baseURL = srv.URL
	require.NoError(t, c.ImportProduct(context.Background(), "prod-1", "shop.example.com"))
}
