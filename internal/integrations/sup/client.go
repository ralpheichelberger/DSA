package sup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dropshipagent/agent/config"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://api.supdropshipping.com/v1",
	}
}

func NewSupplier(cfg *config.Config) Supplier {
	if cfg.DevMode || cfg.SupAPIKey == "" {
		return NewStub()
	}
	return New(cfg.SupAPIKey)
}

func (c *Client) GetProductCost(ctx context.Context, productID string) (*SupplierData, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/products/%s/cost", c.baseURL, productID), nil)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("sup get product cost: http %d", resp.StatusCode)
	}

	var out struct {
		COGS           float64 `json:"cogs"`
		ShippingCost   float64 `json:"shipping_cost"`
		ShippingDays   int     `json:"shipping_days"`
		Warehouse      string  `json:"warehouse_region"`
		StockAvailable bool    `json:"stock_available"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &SupplierData{
		ProductID:       productID,
		COGSEur:         out.COGS,
		ShippingCostEur: out.ShippingCost,
		ShippingDays:    out.ShippingDays,
		WarehouseRegion: out.Warehouse,
		StockAvailable:  out.StockAvailable,
	}, nil
}

func (c *Client) ImportProduct(ctx context.Context, productID string, shopifyDomain string) error {
	body, _ := json.Marshal(map[string]string{"shopify_domain": shopifyDomain})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/products/%s/import", c.baseURL, productID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sup import product: http %d", resp.StatusCode)
	}
	return nil
}
