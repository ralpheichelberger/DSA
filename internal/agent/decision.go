package agent

import (
	"fmt"
	"math"
)

type ProductInput struct {
	COGS            float64
	SellPrice       float64
	ShippingCost    float64
	ShippingDays    int
	WeeksSinceTrend int
	PlatformSignals PlatformSignals
}

type PlatformSignals struct {
	GoogleTrendingUp  bool
	TikTokOrganic     bool
	MultipleAdSellers bool
	ShopifyVelocity   bool
	WeeklyGrowthPct   float64
}

type ScoreResult struct {
	Score     int
	Reasoning string
	BEROAS    float64
	MarginPct float64
	Viable    bool
}

func ScoreProduct(input ProductInput) ScoreResult {
	marginPct := 0.0
	if input.SellPrice > 0 {
		marginPct = ((input.SellPrice - input.COGS - input.ShippingCost) / input.SellPrice) * 100
	}

	marginPoints := 0.0
	switch {
	case marginPct < 25:
		marginPoints = 0
	case marginPct < 30:
		marginPoints = 10
	case marginPct <= 40:
		marginPoints = 20
	default:
		marginPoints = 30
	}

	signals := 0
	if input.PlatformSignals.GoogleTrendingUp {
		signals++
	}
	if input.PlatformSignals.TikTokOrganic {
		signals++
	}
	if input.PlatformSignals.MultipleAdSellers {
		signals++
	}
	if input.PlatformSignals.ShopifyVelocity {
		signals++
	}
	if input.PlatformSignals.WeeklyGrowthPct >= 20 {
		signals++
	}
	if signals > 4 {
		signals = 4
	}
	signalPoints := float64(signals) * 12.5

	shippingPoints := 0.0
	switch {
	case input.ShippingDays <= 7:
		shippingPoints = 20
	case input.ShippingDays <= 14:
		shippingPoints = 10
	default:
		shippingPoints = 0
	}

	penalty := 0.0
	switch {
	case input.WeeksSinceTrend > 6:
		penalty = 10
	case input.WeeksSinceTrend >= 4:
		penalty = 5
	}

	total := marginPoints + signalPoints + shippingPoints - penalty
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}

	beroas := 999.0
	if marginPct > 0 {
		beroas = 100 / marginPct
	}

	score := int(math.Round(total))
	viable := score >= 60 && marginPct >= 30.0
	reasoning := fmt.Sprintf(
		"margin: %.1f%% (%.0f pts), signals: %d/4 (%.1f pts), shipping: %d days (%.0f pts), saturation penalty: -%.0f, total: %d",
		marginPct, marginPoints, signals, signalPoints, input.ShippingDays, shippingPoints, penalty, score,
	)

	return ScoreResult{
		Score:     score,
		Reasoning: reasoning,
		BEROAS:    beroas,
		MarginPct: marginPct,
		Viable:    viable,
	}
}

type CampaignInput struct {
	DaysRunning     int
	SpendEur        float64
	RevenueEur      float64
	Purchases       int
	CTRPct          float64
	COGSEur         float64
	SellPriceEur    float64
	ShippingCostEur float64
}

type CampaignDecision struct {
	Action           string
	Reasoning        string
	RequiresApproval bool
	Urgency          string
}

func (ci CampaignInput) GrossProfit() float64 {
	return ci.SellPriceEur - ci.COGSEur - ci.ShippingCostEur
}

func (ci CampaignInput) ActualROAS() float64 {
	if ci.SpendEur == 0 {
		return 0
	}
	return ci.RevenueEur / ci.SpendEur
}

func (ci CampaignInput) BEROAS() float64 {
	if ci.SellPriceEur <= 0 {
		return 999
	}
	marginPct := ((ci.SellPriceEur - ci.COGSEur - ci.ShippingCostEur) / ci.SellPriceEur) * 100
	if marginPct <= 0 {
		return 999
	}
	return 100 / marginPct
}

func DecideCampaign(input CampaignInput) CampaignDecision {
	if input.DaysRunning < 3 {
		return CampaignDecision{
			Action:           "wait",
			Reasoning:        "too early to judge - minimum 3 days data needed",
			RequiresApproval: false,
			Urgency:          "low",
		}
	}

	if input.SpendEur <= 0 {
		return CampaignDecision{
			Action:           "wait",
			Reasoning:        "no spend data yet - wait for meaningful signal",
			RequiresApproval: false,
			Urgency:          "low",
		}
	}

	grossProfit := input.GrossProfit()
	if input.SpendEur > grossProfit*0.5 && input.Purchases == 0 {
		return CampaignDecision{
			Action:           "kill",
			Reasoning:        "spent 50% of per-unit profit with zero purchases",
			RequiresApproval: false,
			Urgency:          "immediate",
		}
	}

	cpa := input.SpendEur / float64(maxInt(input.Purchases, 1))
	if input.ActualROAS() > 0 && input.COGSEur > 0 && cpa > grossProfit {
		return CampaignDecision{
			Action:           "kill",
			Reasoning:        "CPA exceeds gross profit per unit",
			RequiresApproval: false,
			Urgency:          "immediate",
		}
	}

	actualROAS := input.ActualROAS()
	beroas := input.BEROAS()
	scaleThreshold := beroas * 1.25
	if actualROAS >= scaleThreshold && input.DaysRunning >= 5 {
		return CampaignDecision{
			Action:           "scale",
			Reasoning:        fmt.Sprintf("ROAS at %.2f exceeds scale threshold of %.2f", actualROAS, scaleThreshold),
			RequiresApproval: true,
			Urgency:          "next_cycle",
		}
	}

	if actualROAS >= beroas && actualROAS < scaleThreshold {
		return CampaignDecision{
			Action:           "hold",
			Reasoning:        "breaking even but insufficient buffer to scale safely",
			RequiresApproval: false,
			Urgency:          "low",
		}
	}

	if input.CTRPct < 0.5 && input.DaysRunning >= 2 {
		return CampaignDecision{
			Action:           "rotate_creative",
			Reasoning:        "CTR below 0.5% - hook is not working",
			RequiresApproval: false,
			Urgency:          "next_cycle",
		}
	}

	return CampaignDecision{
		Action:           "hold",
		Reasoning:        "within normal range, continue monitoring",
		RequiresApproval: false,
		Urgency:          "low",
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
