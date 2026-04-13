package minea

import (
	"context"
	"time"
)

type ProductCandidate struct {
	ID               string
	Name             string
	Niche            string
	ShopifyStore     string
	FirstSeenDate    time.Time
	Platforms        []string
	ActiveAdCount    int
	EngagementScore  float64
	EstimatedSellEur float64
	SupplierID       string
	ImageURL         string
	TikTokOrganic    bool
	GoogleTrending   bool
	WeeksSinceTrend  int
}

type Discoverer interface {
	GetTrendingProducts(ctx context.Context, niche string, country string, limit int) ([]ProductCandidate, error)
	GetProductDetails(ctx context.Context, productID string) (*ProductCandidate, error)
}
