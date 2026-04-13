package minea

import (
	"context"
	"fmt"
	"time"
)

type Stub struct{}

func NewStub() *Stub {
	return &Stub{}
}

func (s *Stub) GetTrendingProducts(ctx context.Context, niche string, country string, limit int) ([]ProductCandidate, error) {
	_ = ctx
	_ = country

	all := []ProductCandidate{
		{
			ID:               "m-tech-1",
			Name:             "Magnetic Cable Organizer",
			Niche:            "tech",
			ShopifyStore:     "tech",
			FirstSeenDate:    time.Now().AddDate(0, 0, -18),
			Platforms:        []string{"facebook", "tiktok"},
			ActiveAdCount:    14,
			EngagementScore:  78.5,
			EstimatedSellEur: 29.99,
			SupplierID:       "sup-101",
			ImageURL:         "https://img.example.com/m-tech-1.jpg",
			TikTokOrganic:    true,
			GoogleTrending:   true,
			WeeksSinceTrend:  3,
		},
		{
			ID:               "m-tech-2",
			Name:             "Ergonomic Laptop Riser",
			Niche:            "tech",
			ShopifyStore:     "tech",
			FirstSeenDate:    time.Now().AddDate(0, 0, -25),
			Platforms:        []string{"facebook"},
			ActiveAdCount:    9,
			EngagementScore:  66.2,
			EstimatedSellEur: 39.90,
			SupplierID:       "sup-102",
			ImageURL:         "https://img.example.com/m-tech-2.jpg",
			TikTokOrganic:    false,
			GoogleTrending:   true,
			WeeksSinceTrend:  4,
		},
		{
			ID:               "m-tech-3",
			Name:             "Smart RGB Desk Lamp",
			Niche:            "tech",
			ShopifyStore:     "tech",
			FirstSeenDate:    time.Now().AddDate(0, 0, -56),
			Platforms:        []string{"facebook", "tiktok"},
			ActiveAdCount:    22,
			EngagementScore:  81.4,
			EstimatedSellEur: 49.00,
			SupplierID:       "sup-103",
			ImageURL:         "https://img.example.com/m-tech-3.jpg",
			TikTokOrganic:    true,
			GoogleTrending:   true,
			WeeksSinceTrend:  8,
		},
		{
			ID:               "m-pets-1",
			Name:             "Self-Cleaning Pet Brush",
			Niche:            "pets",
			ShopifyStore:     "pets",
			FirstSeenDate:    time.Now().AddDate(0, 0, -15),
			Platforms:        []string{"tiktok"},
			ActiveAdCount:    11,
			EngagementScore:  74.0,
			EstimatedSellEur: 24.95,
			SupplierID:       "sup-201",
			ImageURL:         "https://img.example.com/m-pets-1.jpg",
			TikTokOrganic:    true,
			GoogleTrending:   false,
			WeeksSinceTrend:  2,
		},
		{
			ID:               "m-pets-2",
			Name:             "Auto Portion Pet Feeder",
			Niche:            "pets",
			ShopifyStore:     "pets",
			FirstSeenDate:    time.Now().AddDate(0, 0, -29),
			Platforms:        []string{"facebook", "tiktok"},
			ActiveAdCount:    13,
			EngagementScore:  69.8,
			EstimatedSellEur: 59.00,
			SupplierID:       "sup-202",
			ImageURL:         "https://img.example.com/m-pets-2.jpg",
			TikTokOrganic:    true,
			GoogleTrending:   true,
			WeeksSinceTrend:  5,
		},
	}

	var filtered []ProductCandidate
	for _, p := range all {
		if niche == "" || p.Niche == niche {
			filtered = append(filtered, p)
		}
	}

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (s *Stub) GetProductDetails(ctx context.Context, productID string) (*ProductCandidate, error) {
	products, err := s.GetTrendingProducts(ctx, "", "", 0)
	if err != nil {
		return nil, err
	}
	for _, p := range products {
		if p.ID == productID {
			product := p
			return &product, nil
		}
	}
	return nil, fmt.Errorf("product not found: %s", productID)
}
