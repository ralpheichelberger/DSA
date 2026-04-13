package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoreProduct(t *testing.T) {
	testCases := []struct {
		name        string
		input       ProductInput
		expected    int
		delta       int
		wantViable  bool
		wantBEROAS  float64
		beroasDelta float64
		wantMargin  float64
		marginDelta float64
	}{
		{
			name: "high margin four signals fast shipping two weeks",
			input: ProductInput{
				COGS:            26,
				SellPrice:       50,
				ShippingCost:    4,
				ShippingDays:    5,
				WeeksSinceTrend: 2,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp:  true,
					TikTokOrganic:     true,
					MultipleAdSellers: true,
					ShopifyVelocity:   true,
					WeeklyGrowthPct:   30,
				},
			},
			expected:    90,
			delta:       1,
			wantViable:  true,
			wantBEROAS:  2.50,
			beroasDelta: 0.02,
		},
		{
			name: "low margin all signals fast shipping",
			input: ProductInput{
				COGS:            20,
				SellPrice:       30,
				ShippingCost:    4,
				ShippingDays:    6,
				WeeksSinceTrend: 2,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp:  true,
					TikTokOrganic:     true,
					MultipleAdSellers: true,
					ShopifyVelocity:   true,
					WeeklyGrowthPct:   40,
				},
			},
			expected:   70,
			delta:      1,
			wantViable: false,
		},
		{
			name: "good margin one signal not viable",
			input: ProductInput{
				COGS:            31,
				SellPrice:       60,
				ShippingCost:    5,
				ShippingDays:    7,
				WeeksSinceTrend: 2,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp: true,
				},
			},
			expected:   53,
			delta:      1,
			wantViable: false,
		},
		{
			name: "good margin three signals slow shipping",
			input: ProductInput{
				COGS:            15,
				SellPrice:       45,
				ShippingCost:    6,
				ShippingDays:    20,
				WeeksSinceTrend: 2,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp:  true,
					TikTokOrganic:     true,
					MultipleAdSellers: true,
				},
			},
			expected:   68,
			delta:      1,
			wantViable: true,
		},
		{
			name: "good margin three signals eight weeks trending penalty",
			input: ProductInput{
				COGS:            15,
				SellPrice:       50,
				ShippingCost:    5,
				ShippingDays:    6,
				WeeksSinceTrend: 8,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp: true,
					TikTokOrganic:    true,
					ShopifyVelocity:  true,
					WeeklyGrowthPct:  10,
				},
			},
			expected:   77,
			delta:      1,
			wantViable: true,
		},
		{
			name: "zero sell price returns 999 beroas not viable",
			input: ProductInput{
				COGS:            10,
				SellPrice:       0,
				ShippingCost:    2,
				ShippingDays:    5,
				WeeksSinceTrend: 1,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp: true,
				},
			},
			expected:    32,
			delta:       1,
			wantViable:  false,
			wantBEROAS:  999,
			beroasDelta: 0,
		},
		{
			name: "exactly thirty percent margin boundary viable if score high enough",
			input: ProductInput{
				COGS:            31,
				SellPrice:       50,
				ShippingCost:    4,
				ShippingDays:    8,
				WeeksSinceTrend: 2,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp: true,
					TikTokOrganic:    true,
					ShopifyVelocity:  true,
					WeeklyGrowthPct:  25,
				},
			},
			expected:    80,
			delta:       1,
			wantViable:  true,
			wantMargin:  30,
			marginDelta: 0.01,
		},
		{
			name: "perfect product scores one hundred",
			input: ProductInput{
				COGS:            15,
				SellPrice:       60,
				ShippingCost:    3,
				ShippingDays:    4,
				WeeksSinceTrend: 1,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp:  true,
					TikTokOrganic:     true,
					MultipleAdSellers: true,
					ShopifyVelocity:   true,
					WeeklyGrowthPct:   30,
				},
			},
			expected:   100,
			delta:      0,
			wantViable: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScoreProduct(tc.input)
			assert.InDelta(t, tc.expected, got.Score, float64(tc.delta))
			assert.Equal(t, tc.wantViable, got.Viable)
			assert.NotEmpty(t, got.Reasoning)

			if tc.wantBEROAS > 0 {
				assert.InDelta(t, tc.wantBEROAS, got.BEROAS, tc.beroasDelta)
			}
			if tc.wantMargin > 0 {
				assert.InDelta(t, tc.wantMargin, got.MarginPct, tc.marginDelta)
			}
		})
	}
}

func TestDecideCampaign(t *testing.T) {
	testCases := []struct {
		name                string
		input               CampaignInput
		wantAction          string
		wantUrgency         string
		wantRequiresApprove bool
	}{
		{
			name: "day one always wait",
			input: CampaignInput{
				DaysRunning:     1,
				SpendEur:        100,
				RevenueEur:      300,
				Purchases:       3,
				CTRPct:          1.0,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "wait",
			wantUrgency: "low",
		},
		{
			name: "day four no purchases spent too much kill",
			input: CampaignInput{
				DaysRunning:     4,
				SpendEur:        15,
				RevenueEur:      0,
				Purchases:       0,
				CTRPct:          1.2,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "kill",
			wantUrgency: "immediate",
		},
		{
			name: "day five cpa above gross profit kill",
			input: CampaignInput{
				DaysRunning:     5,
				SpendEur:        70,
				RevenueEur:      40,
				Purchases:       2,
				CTRPct:          1.1,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "kill",
			wantUrgency: "immediate",
		},
		{
			name: "day six roas above scale threshold",
			input: CampaignInput{
				DaysRunning:     6,
				SpendEur:        100,
				RevenueEur:      380,
				Purchases:       8,
				CTRPct:          1.4,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:          "scale",
			wantUrgency:         "next_cycle",
			wantRequiresApprove: true,
		},
		{
			name: "day five roas exactly beroas hold",
			input: CampaignInput{
				DaysRunning:     5,
				SpendEur:        100,
				RevenueEur:      176.470588,
				Purchases:       10,
				CTRPct:          1.0,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "hold",
			wantUrgency: "low",
		},
		{
			name: "day three ctr below half rotate creative",
			input: CampaignInput{
				DaysRunning:     3,
				SpendEur:        20,
				RevenueEur:      25,
				Purchases:       2,
				CTRPct:          0.3,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "rotate_creative",
			wantUrgency: "next_cycle",
		},
		{
			name: "ctr low but day one wait wins",
			input: CampaignInput{
				DaysRunning:     1,
				SpendEur:        20,
				RevenueEur:      0,
				Purchases:       0,
				CTRPct:          0.3,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "wait",
			wantUrgency: "low",
		},
		{
			name: "day seven roas above threshold scale",
			input: CampaignInput{
				DaysRunning:     7,
				SpendEur:        100,
				RevenueEur:      750,
				Purchases:       12,
				CTRPct:          1.6,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:          "scale",
			wantUrgency:         "next_cycle",
			wantRequiresApprove: true,
		},
		{
			name: "zero spend wait",
			input: CampaignInput{
				DaysRunning:     4,
				SpendEur:        0,
				RevenueEur:      0,
				Purchases:       0,
				CTRPct:          0,
				COGSEur:         10,
				SellPriceEur:    30,
				ShippingCostEur: 3,
			},
			wantAction:  "wait",
			wantUrgency: "low",
		},
		{
			name: "negative margin should not scale",
			input: CampaignInput{
				DaysRunning:     8,
				SpendEur:        100,
				RevenueEur:      0,
				Purchases:       1,
				CTRPct:          2,
				COGSEur:         40,
				SellPriceEur:    30,
				ShippingCostEur: 5,
			},
			wantAction:  "hold",
			wantUrgency: "low",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideCampaign(tc.input)
			assert.Equal(t, tc.wantAction, got.Action)
			assert.Equal(t, tc.wantUrgency, got.Urgency)
			assert.Equal(t, tc.wantRequiresApprove, got.RequiresApproval)
			assert.NotEmpty(t, got.Reasoning)
		})
	}
}

func TestGrossProfit(t *testing.T) {
	testCases := []struct {
		name     string
		input    CampaignInput
		expected float64
	}{
		{
			name: "normal gross profit",
			input: CampaignInput{
				COGSEur:         10,
				SellPriceEur:    35,
				ShippingCostEur: 3,
			},
			expected: 22,
		},
		{
			name: "zero cogs",
			input: CampaignInput{
				COGSEur:         0,
				SellPriceEur:    35,
				ShippingCostEur: 3,
			},
			expected: 32,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.input.GrossProfit())
		})
	}
}
