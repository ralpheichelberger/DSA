package tiktok

import (
	"context"
	"crypto/rand"
	"fmt"
)

type Stub struct {
	campaigns map[string]*CampaignMetrics
}

func NewStub() *Stub {
	return &Stub{
		campaigns: map[string]*CampaignMetrics{},
	}
}

func (s *Stub) CreateCampaign(ctx context.Context, productName string, dailyBudgetEur float64, creatives []AdCreative) (string, error) {
	_ = ctx
	_ = productName
	_ = creatives

	id, err := newUUID()
	if err != nil {
		return "", err
	}

	spend := dailyBudgetEur * 3
	if spend <= 0 {
		spend = 45
	}
	roas := 1.4
	revenue := spend * roas
	purchases := int64(7)
	cpa := spend / float64(purchases)

	s.campaigns[id] = &CampaignMetrics{
		CampaignID:  id,
		SpendEur:    spend,
		RevenueEur:  revenue,
		ROAS:        roas,
		CTRPct:      1.8,
		CPAEur:      cpa,
		Impressions: 18000,
		Clicks:      320,
		Purchases:   purchases,
		DaysRunning: 3,
	}

	return id, nil
}

func (s *Stub) GetMetrics(ctx context.Context, campaignID string) (*CampaignMetrics, error) {
	_ = ctx
	m, ok := s.campaigns[campaignID]
	if !ok {
		return nil, fmt.Errorf("campaign not found: %s", campaignID)
	}
	return m, nil
}

func (s *Stub) PauseCampaign(ctx context.Context, campaignID string) error {
	_ = ctx
	_ = campaignID
	return nil
}

func (s *Stub) ScaleBudget(ctx context.Context, campaignID string, newDailyBudgetEur float64) error {
	_ = ctx
	_ = campaignID
	_ = newDailyBudgetEur
	return nil
}

func (s *Stub) GetTrendingAudio(ctx context.Context) ([]string, error) {
	_ = ctx
	return []string{
		"sound_viral_01",
		"sound_transition_22",
		"sound_ugc_voiceover_08",
	}, nil
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
