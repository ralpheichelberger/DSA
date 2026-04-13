package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dropshipagent/agent/config"
)

type Client struct {
	accessToken string
	adAccountID string
	httpClient  *http.Client
	baseURL     string
}

func New(accessToken string, adAccountID string) *Client {
	return &Client{
		accessToken: accessToken,
		adAccountID: adAccountID,
		httpClient:  &http.Client{Timeout: 12 * time.Second},
		baseURL:     "https://graph.facebook.com/v21.0",
	}
}

func NewMetaPlatform(cfg *config.Config) AdPlatform {
	if cfg.DevMode || cfg.MetaAccessToken == "" {
		return NewStub()
	}
	return New(cfg.MetaAccessToken, cfg.MetaAdAccountID)
}

func (c *Client) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []AdCreative) (string, error) {
	_ = creatives
	campaignID, err := c.postForID(ctx, fmt.Sprintf("%s/act_%s/campaigns", c.baseURL, c.adAccountID), map[string]any{
		"name":                  productName,
		"objective":             "OUTCOME_SALES",
		"special_ad_categories": []string{},
		"access_token":          c.accessToken,
	})
	if err != nil {
		return "", err
	}
	_, _ = c.postForID(ctx, fmt.Sprintf("%s/act_%s/adsets", c.baseURL, c.adAccountID), map[string]any{
		"name":         productName + " Adset",
		"campaign_id":  campaignID,
		"daily_budget": int(dailyBudgetEur * 100),
		"access_token": c.accessToken,
	})
	_, _ = c.postForID(ctx, fmt.Sprintf("%s/act_%s/ads", c.baseURL, c.adAccountID), map[string]any{
		"name":         productName + " Ad",
		"campaign_id":  campaignID,
		"access_token": c.accessToken,
	})
	return campaignID, nil
}

func (c *Client) GetMetrics(ctx context.Context, campaignID string) (*CampaignMetrics, error) {
	url := fmt.Sprintf("%s/%s/insights?fields=spend,revenue,impressions,clicks,actions,ctr&date_preset=lifetime&access_token=%s", c.baseURL, campaignID, c.accessToken)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("meta get metrics: http %d", resp.StatusCode)
	}

	var decoded struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data) == 0 {
		return &CampaignMetrics{CampaignID: campaignID}, nil
	}
	row := decoded.Data[0]
	spend := parseAnyFloat(row["spend"])
	revenue := parseAnyFloat(row["revenue"])
	if revenue == 0 {
		if actions, ok := row["actions"].([]any); ok {
			for _, a := range actions {
				m, _ := a.(map[string]any)
				if m["action_type"] == "purchase" {
					// keep purchase count parsing; revenue may be absent
				}
			}
		}
	}
	roas := 0.0
	if spend > 0 {
		roas = revenue / spend
	}
	return &CampaignMetrics{
		CampaignID:  campaignID,
		SpendEur:    spend,
		RevenueEur:  revenue,
		ROAS:        roas,
		CTRPct:      parseAnyFloat(row["ctr"]),
		Impressions: int64(parseAnyFloat(row["impressions"])),
		Clicks:      int64(parseAnyFloat(row["clicks"])),
	}, nil
}

func (c *Client) PauseCampaign(ctx context.Context, campaignID string) error {
	return c.postNoContent(ctx, fmt.Sprintf("%s/%s", c.baseURL, campaignID), map[string]any{
		"status":       "PAUSED",
		"access_token": c.accessToken,
	})
}

func (c *Client) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	return c.postNoContent(ctx, fmt.Sprintf("%s/%s", c.baseURL, campaignID), map[string]any{
		"daily_budget": int(newDailyBudgetEur * 100),
		"access_token": c.accessToken,
	})
}

func (c *Client) postForID(ctx context.Context, url string, payload map[string]any) (string, error) {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("meta post id: http %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) postNoContent(ctx context.Context, url string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meta post update: http %d", resp.StatusCode)
	}
	return nil
}

func parseAnyFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	default:
		return 0
	}
}
