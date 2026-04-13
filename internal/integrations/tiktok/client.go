package tiktok

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
	accessToken string
	httpClient  *http.Client
	baseURL     string
}

func New(accessToken string) *Client {
	return &Client{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 12 * time.Second},
		baseURL:     "https://business-api.tiktok.com/open_api/v1.3",
	}
}

func NewTikTokPlatform(cfg *config.Config) AdPlatform {
	if cfg.DevMode || cfg.TikTokAccessToken == "" {
		return NewStub()
	}
	return New(cfg.TikTokAccessToken)
}

func (c *Client) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []AdCreative) (string, error) {
	_ = creatives
	body, _ := json.Marshal(map[string]any{
		"campaign_name": productName,
		"budget":        dailyBudgetEur,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/campaign/create/", bytes.NewReader(body))
	req.Header.Set("Access-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tiktok create campaign: http %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			CampaignID string `json:"campaign_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Data.CampaignID, nil
}

func (c *Client) GetMetrics(ctx context.Context, campaignID string) (*CampaignMetrics, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/report/integrated/get/?campaign_id="+campaignID, nil)
	req.Header.Set("Access-Token", c.accessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tiktok get metrics: http %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data.List) == 0 {
		return &CampaignMetrics{CampaignID: campaignID}, nil
	}
	row := out.Data.List[0]
	spend := toFloat(row["spend"])
	revenue := toFloat(row["revenue"])
	roas := 0.0
	if spend > 0 {
		roas = revenue / spend
	}
	purchases := int64(toFloat(row["purchases"]))
	cpa := 0.0
	if purchases > 0 {
		cpa = spend / float64(purchases)
	}
	return &CampaignMetrics{
		CampaignID:  campaignID,
		SpendEur:    spend,
		RevenueEur:  revenue,
		ROAS:        roas,
		CTRPct:      toFloat(row["ctr"]),
		CPAEur:      cpa,
		Impressions: int64(toFloat(row["impressions"])),
		Clicks:      int64(toFloat(row["clicks"])),
		Purchases:   purchases,
	}, nil
}

func (c *Client) PauseCampaign(ctx context.Context, campaignID string) error {
	return c.postUpdate(ctx, c.baseURL+"/campaign/status/update/", map[string]any{"campaign_id": campaignID, "status": "DISABLE"})
}

func (c *Client) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	return c.postUpdate(ctx, c.baseURL+"/campaign/update/", map[string]any{"campaign_id": campaignID, "budget": newDailyBudgetEur})
}

func (c *Client) GetTrendingAudio(ctx context.Context) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/creative/music/recommend/", nil)
	req.Header.Set("Access-Token", c.accessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tiktok trending audio: http %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	result := make([]string, 0, 10)
	for _, row := range out.Data.List {
		if id, ok := row["music_id"].(string); ok && id != "" {
			result = append(result, id)
		}
		if len(result) >= 10 {
			break
		}
	}
	return result, nil
}

func (c *Client) postUpdate(ctx context.Context, url string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Access-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tiktok update: http %d", resp.StatusCode)
	}
	return nil
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}
