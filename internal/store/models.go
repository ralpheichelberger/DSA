package store

import "time"

type ProductTest struct {
	ID              string
	ProductName     string
	Niche           string
	ShopifyStore    string
	SourcePlatform  string
	Supplier        string
	COGSEur         float64
	SellPriceEur    float64
	GrossMarginPct  float64
	BEROAS          float64
	ShippingCostEur float64
	ShippingDays    int
	Status          string
	KillReason      string
	Score           int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CampaignResult struct {
	ID            string
	ProductTestID string
	Platform      string
	CampaignID    string
	SpendEur      float64
	RevenueEur    float64
	ROAS          float64
	CTRPct        float64
	CPAEur        float64
	Impressions   int64
	Clicks        int64
	Purchases     int64
	DaysRunning   int
	SnapshotDate  time.Time
	CreatedAt     time.Time
}

type LearnedLesson struct {
	ID            string
	Category      string
	Lesson        string
	Confidence    float64
	EvidenceCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreativePerformance struct {
	ID                 string
	ProductTestID      string
	Platform           string
	CreativeType       string
	HookDescription    string
	Angle              string
	CTRPct             float64
	HookRetention3sPct float64
	SpendEur           float64
	ROAS               float64
	Won                bool
	CreatedAt          time.Time
}
