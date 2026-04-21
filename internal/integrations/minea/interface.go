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
	// AdURL is the platform ad / creative link (e.g. Meta or TikTok) when present in RPC payloads.
	AdURL string `json:"adURL"`
	// ShopURL is the storefront URL when Minea returns a domain or shop URL.
	ShopURL string `json:"shopURL"`
	// LandingURL is the ad destination / product landing page when present.
	LandingURL      string `json:"landingURL"`
	TikTokOrganic   bool
	GoogleTrending  bool
	WeeksSinceTrend int
}

type Discoverer interface {
	GetTrendingProducts(ctx context.Context, niche string, country string, limit int) ([]ProductCandidate, error)
	GetProductDetails(ctx context.Context, productID string) (*ProductCandidate, error)
}

type AdsSearchOptions struct {
	Country              string
	Limit                int
	StartPage            int
	Pages                int
	PerPage              int
	SortBy               string
	MediaTypes           []string
	MediaTypesExcludes   []string
	PublicationDate      string
	PublicationDateRange []string
	AdIsActive           []string
	AdIsActiveExcludes   []string
	AdLanguages          []string
	AdLanguagesExcludes  []string
	AdCountries          []string
	AdCountriesExcludes  []string
	AdDays               []int
	CTAs                 []string
	CTAsExcludes         []string
	OnlyEU               bool
	CPMValue             float64
	ExcludeBad           *bool
	Collapse             string
	ExtraFilters         map[string]interface{}
	// Query is the Meta ads library text search (URL param "query"), e.g. "pets".
	Query string
	// QSearchTargets selects which fields to search (URL param "q_search_targets"), e.g. "adCopy".
	QSearchTargets string
}
