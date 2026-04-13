package meta

import "context"

type AdCreative struct {
	Type     string
	Headline string
	Body     string
	CTA      string
	MediaURL string
}

type CampaignMetrics struct {
	CampaignID  string
	SpendEur    float64
	RevenueEur  float64
	ROAS        float64
	CTRPct      float64
	CPAEur      float64
	Impressions int64
	Clicks      int64
	Purchases   int64
	DaysRunning int
}

type AdPlatform interface {
	CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []AdCreative) (string, error)
	GetMetrics(ctx context.Context, campaignID string) (*CampaignMetrics, error)
	PauseCampaign(ctx context.Context, campaignID string) error
	ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error
}
