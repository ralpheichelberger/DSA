package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dropshipagent/agent/config"
	metaint "github.com/dropshipagent/agent/internal/integrations/meta"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/integrations/openai"
	"github.com/dropshipagent/agent/internal/integrations/sup"
	tiktokint "github.com/dropshipagent/agent/internal/integrations/tiktok"
	"github.com/dropshipagent/agent/internal/store"
	"go.uber.org/zap"
)

type Agent struct {
	cfg        *config.Config
	store      *store.Store
	reasoner   openai.Reasoner
	discoverer minea.Discoverer
	supplier   sup.Supplier
	meta       metaint.AdPlatform
	tiktok     tiktokint.AdPlatform
	outbox     chan Message
	approval   chan ApprovalResponse
	logger     *zap.Logger
}

type Message struct {
	Type    string
	Subject string
	Body    string
	Data    interface{}
}

type ApprovalResponse struct {
	ProductTestID string
	Approved      bool
	Note          string
}

func New(cfg *config.Config, storeDB *store.Store, reasoner openai.Reasoner,
	discoverer minea.Discoverer, supplier sup.Supplier,
	meta metaint.AdPlatform, tiktok tiktokint.AdPlatform,
	logger *zap.Logger) *Agent {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Agent{
		cfg:        cfg,
		store:      storeDB,
		reasoner:   reasoner,
		discoverer: discoverer,
		supplier:   supplier,
		meta:       meta,
		tiktok:     tiktok,
		outbox:     make(chan Message, 100),
		approval:   make(chan ApprovalResponse, 10),
		logger:     logger,
	}
}

func (a *Agent) Outbox() <-chan Message                { return a.outbox }
func (a *Agent) ApprovalChan() chan<- ApprovalResponse { return a.approval }

// Approvals exposes the receive side of the approval channel (e.g. for API tests).
func (a *Agent) Approvals() <-chan ApprovalResponse { return a.approval }

func (a *Agent) Run(ctx context.Context) {
	interval := time.Duration(a.cfg.AgentIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	a.logger.Info("agent scheduler started", zap.Duration("interval_until_first_cycle", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.runCycle(ctx)
		}
	}
}

func (a *Agent) runCycle(ctx context.Context) error {
	start := time.Now()
	logPhase := func(name string, phaseStart time.Time, err error) {
		if err != nil {
			a.logger.Error("phase failed", zap.String("phase", name), zap.Duration("duration", time.Since(phaseStart)), zap.Error(err))
			return
		}
		a.logger.Info("phase complete", zap.String("phase", name), zap.Duration("duration", time.Since(phaseStart)))
	}

	p := time.Now()
	candidates, err := a.runDiscovery(ctx)
	logPhase("discover", p, err)

	p = time.Now()
	shortlist, err2 := a.runReasoning(ctx, candidates)
	logPhase("reason", p, err2)

	p = time.Now()
	approved, err3 := a.runApprovalGate(ctx, shortlist)
	logPhase("approve", p, err3)

	p = time.Now()
	err4 := a.runLaunch(ctx, approved)
	logPhase("launch", p, err4)

	p = time.Now()
	err5 := a.runMonitoring(ctx)
	logPhase("monitor", p, err5)

	p = time.Now()
	err6 := a.runLearning(ctx)
	logPhase("learn", p, err6)

	a.logger.Info("cycle finished", zap.Duration("duration", time.Since(start)))
	if err != nil {
		return err
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}
	if err4 != nil {
		return err4
	}
	if err5 != nil {
		return err5
	}
	return err6
}

// shopifyStoreFromDiscoveryNiche maps the discovery bucket to launch routing (pets vs tech Shopify domain).
func shopifyStoreFromDiscoveryNiche(niche string) string {
	if strings.EqualFold(strings.TrimSpace(niche), "pets") {
		return "pets"
	}
	return "tech"
}

func (a *Agent) runDiscovery(ctx context.Context) ([]store.ProductTest, error) {
	var viable []store.ProductTest
	niches := a.cfg.DiscoveryNiches
	if len(niches) == 0 {
		niches = []string{"tech", "pets"}
	}
	for _, niche := range niches {
		candidates, err := a.discoverer.GetTrendingProducts(ctx, niche, "US", 10)
		if err != nil {
			a.logger.Warn("discover failed for niche", zap.String("niche", niche), zap.Error(err))
			continue
		}
		for _, c := range candidates {
			cost, err := a.supplier.GetProductCost(ctx, c.ID)
			if err != nil || cost == nil {
				a.logger.Warn("skip product without cost", zap.String("product_id", c.ID), zap.Error(err))
				continue
			}
			price := c.EstimatedSellEur
			if price <= 0 {
				price = cost.COGSEur + cost.ShippingCostEur + 15
			}
			scoreRes := ScoreProduct(ProductInput{
				COGS:            cost.COGSEur,
				SellPrice:       price,
				ShippingCost:    cost.ShippingCostEur,
				ShippingDays:    cost.ShippingDays,
				WeeksSinceTrend: c.WeeksSinceTrend,
				PlatformSignals: PlatformSignals{
					GoogleTrendingUp:  c.GoogleTrending,
					TikTokOrganic:     c.TikTokOrganic,
					MultipleAdSellers: c.ActiveAdCount >= 2,
					ShopifyVelocity:   c.EngagementScore >= 60,
					WeeklyGrowthPct:   25,
				},
			})
			isViable := scoreRes.Score >= 60
			pt := store.ProductTest{
				ID:              c.ID,
				ProductName:     c.Name,
				ProductImageURL: c.ImageURL,
				AdURL:           c.AdURL,
				ShopURL:         c.ShopURL,
				LandingURL:      c.LandingURL,
				// Stamp the discovery query niche, not Minea's payload (ads often have missing/wrong category).
				Niche:           niche,
				ShopifyStore:    shopifyStoreFromDiscoveryNiche(niche),
				SourcePlatform:  "minea",
				Supplier:        c.SupplierID,
				COGSEur:         cost.COGSEur,
				SellPriceEur:    price,
				GrossMarginPct:  scoreRes.MarginPct,
				BEROAS:          scoreRes.BEROAS,
				ShippingCostEur: cost.ShippingCostEur,
				ShippingDays:    cost.ShippingDays,
				Status:          "watching",
				Score:           scoreRes.Score,
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			}
			if err := a.store.SaveProductTest(pt); err != nil {
				a.logger.Warn("save candidate failed", zap.String("product_id", c.ID), zap.Error(err))
				continue
			}
			if isViable {
				viable = append(viable, pt)
			}
		}
	}
	return viable, nil
}

func (a *Agent) runReasoning(ctx context.Context, candidates []store.ProductTest) ([]store.ProductTest, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	mem, _ := a.store.BuildMemoryContext()
	raw, _ := json.Marshal(candidates)
	niche := candidates[0].Niche
	system := BuildSystemPrompt("product_evaluation", niche, mem)
	user := fmt.Sprintf("Here are today's candidate products: %s.\nBased on our knowledge and past results, select the best 1-2 to test.\nFor each: explain why, specify platform strategy (meta|tiktok|both), daily test budget in EUR (€10 minimum), creative angles to test (2-3), and any red flags. Return as JSON array.", string(raw))
	resp, err := a.reasoner.Reason(ctx, system, user)
	if err != nil {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
		if len(candidates) > 2 {
			candidates = candidates[:2]
		}
		return candidates, nil
	}

	type selection struct {
		ProductID string   `json:"product_id"`
		ID        string   `json:"id"`
		Platform  string   `json:"platform"`
		Budget    float64  `json:"daily_budget_eur"`
		Angles    []string `json:"creative_angles"`
	}
	var picks []selection
	if err := json.Unmarshal([]byte(resp), &picks); err != nil {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
		if len(candidates) > 2 {
			candidates = candidates[:2]
		}
		return candidates, nil
	}
	byID := map[string]store.ProductTest{}
	for _, c := range candidates {
		byID[c.ID] = c
	}
	short := make([]store.ProductTest, 0, 2)
	for _, p := range picks {
		id := p.ProductID
		if id == "" {
			id = p.ID
		}
		if c, ok := byID[id]; ok {
			short = append(short, c)
		}
		if len(short) >= 2 {
			break
		}
	}
	if len(short) == 0 {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
		short = candidates
		if len(short) > 2 {
			short = short[:2]
		}
	}
	return short, nil
}

func (a *Agent) runApprovalGate(ctx context.Context, shortlist []store.ProductTest) ([]store.ProductTest, error) {
	if len(shortlist) == 0 {
		return nil, nil
	}
	a.outbox <- Message{Type: "awaiting_approval", Subject: "Product shortlist approval", Body: "Review shortlist for launch", Data: shortlist}
	if a.cfg.AutoApprove {
		return shortlist, nil
	}
	timer := time.NewTimer(24 * time.Hour)
	defer timer.Stop()
	approved := map[string]bool{}
	for range shortlist {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-a.approval:
			if r.Approved {
				approved[r.ProductTestID] = true
			}
		case <-timer.C:
			a.outbox <- Message{Type: "alert", Subject: "Approval timeout", Body: "Auto-approving top product due to 24h timeout"}
			return shortlist[:1], nil
		}
	}
	var out []store.ProductTest
	for _, p := range shortlist {
		if approved[p.ID] {
			out = append(out, p)
		}
	}
	return out, nil
}

func (a *Agent) runLaunch(ctx context.Context, approved []store.ProductTest) error {
	for _, p := range approved {
		domain := a.cfg.ShopifyTechDomain
		if p.ShopifyStore == "pets" {
			domain = a.cfg.ShopifyPetsDomain
		}
		if err := a.supplier.ImportProduct(ctx, p.ID, domain); err != nil {
			a.logger.Warn("import product failed", zap.String("product_id", p.ID), zap.Error(err))
			continue
		}
		briefs, _ := a.reasoner.GenerateCreativeBriefs(ctx, p.ProductName, p.Niche, []string{"problem_solution", "transformation"})
		metaCreatives := make([]metaint.AdCreative, 0, len(briefs))
		ttCreatives := make([]tiktokint.AdCreative, 0, len(briefs))
		for _, b := range briefs {
			metaCreatives = append(metaCreatives, metaint.AdCreative{Type: "video", Headline: b.Headline, Body: b.Body, CTA: b.CTA, MediaURL: ""})
			ttCreatives = append(ttCreatives, tiktokint.AdCreative{Type: "video", Headline: b.Headline, Body: b.Body, CTA: b.CTA, MediaURL: ""})
		}
		now := time.Now().UTC()
		if id, err := a.meta.CreateCampaign(ctx, p.ProductName, 20, metaCreatives); err == nil {
			_ = a.store.SaveCampaignResult(store.CampaignResult{ID: p.ID + "-meta-" + now.Format("150405"), ProductTestID: p.ID, Platform: "meta", CampaignID: id, DaysRunning: 1, SnapshotDate: now, CreatedAt: now})
		}
		if id, err := a.tiktok.CreateCampaign(ctx, p.ProductName, 20, ttCreatives); err == nil {
			_ = a.store.SaveCampaignResult(store.CampaignResult{ID: p.ID + "-tiktok-" + now.Format("150405"), ProductTestID: p.ID, Platform: "tiktok", CampaignID: id, DaysRunning: 1, SnapshotDate: now, CreatedAt: now})
		}
		_ = a.store.UpdateProductStatus(p.ID, "testing", "")
		a.outbox <- Message{Type: "action_taken", Subject: "Campaign launched", Body: p.ProductName}
	}
	return nil
}

func (a *Agent) runMonitoring(ctx context.Context) error {
	active, err := a.store.GetActiveCampaigns()
	if err != nil {
		return err
	}
	for _, c := range active {
		pt, err := a.store.GetProductTest(c.ProductTestID)
		if err != nil || pt == nil {
			continue
		}
		var input CampaignInput
		switch c.Platform {
		case "meta":
			m, err := a.meta.GetMetrics(ctx, c.CampaignID)
			if err != nil {
				continue
			}
			_ = a.store.SaveCampaignResult(store.CampaignResult{
				ID:            c.ID + "-snap-" + time.Now().UTC().Format("150405"),
				ProductTestID: c.ProductTestID, Platform: c.Platform, CampaignID: c.CampaignID,
				SpendEur: m.SpendEur, RevenueEur: m.RevenueEur, ROAS: m.ROAS, CTRPct: m.CTRPct, CPAEur: m.CPAEur,
				Impressions: m.Impressions, Clicks: m.Clicks, Purchases: m.Purchases, DaysRunning: m.DaysRunning, SnapshotDate: time.Now().UTC(), CreatedAt: time.Now().UTC(),
			})
			input = CampaignInput{DaysRunning: m.DaysRunning, SpendEur: m.SpendEur, RevenueEur: m.RevenueEur, Purchases: int(m.Purchases), CTRPct: m.CTRPct, COGSEur: pt.COGSEur, SellPriceEur: pt.SellPriceEur, ShippingCostEur: pt.ShippingCostEur}
		case "tiktok":
			m, err := a.tiktok.GetMetrics(ctx, c.CampaignID)
			if err != nil {
				continue
			}
			_ = a.store.SaveCampaignResult(store.CampaignResult{
				ID:            c.ID + "-snap-" + time.Now().UTC().Format("150405"),
				ProductTestID: c.ProductTestID, Platform: c.Platform, CampaignID: c.CampaignID,
				SpendEur: m.SpendEur, RevenueEur: m.RevenueEur, ROAS: m.ROAS, CTRPct: m.CTRPct, CPAEur: m.CPAEur,
				Impressions: m.Impressions, Clicks: m.Clicks, Purchases: m.Purchases, DaysRunning: m.DaysRunning, SnapshotDate: time.Now().UTC(), CreatedAt: time.Now().UTC(),
			})
			input = CampaignInput{DaysRunning: m.DaysRunning, SpendEur: m.SpendEur, RevenueEur: m.RevenueEur, Purchases: int(m.Purchases), CTRPct: m.CTRPct, COGSEur: pt.COGSEur, SellPriceEur: pt.SellPriceEur, ShippingCostEur: pt.ShippingCostEur}
		default:
			continue
		}

		decision := DecideCampaign(input)
		switch decision.Action {
		case "kill":
			if c.Platform == "meta" {
				_ = a.meta.PauseCampaign(ctx, c.CampaignID)
			} else {
				_ = a.tiktok.PauseCampaign(ctx, c.CampaignID)
			}
			_ = a.store.UpdateProductStatus(c.ProductTestID, "killed", decision.Reasoning)
			a.outbox <- Message{Type: "alert", Subject: "Campaign killed", Body: decision.Reasoning}
		case "scale":
			if a.cfg.AutoApprove {
				if c.Platform == "meta" {
					_ = a.meta.ScaleBudget(ctx, c.CampaignID, 30)
				} else {
					_ = a.tiktok.ScaleBudget(ctx, c.CampaignID, 30)
				}
			}
			a.outbox <- Message{Type: "awaiting_approval", Subject: "Scale campaign", Body: decision.Reasoning}
		case "rotate_creative":
			a.outbox <- Message{Type: "alert", Subject: "Rotate creative", Body: decision.Reasoning}
		}
	}
	a.outbox <- Message{Type: "report", Subject: "Monitoring complete", Body: "Campaign metrics updated"}
	return nil
}

func (a *Agent) runLearning(ctx context.Context) error {
	products, err := a.store.GetAllProducts()
	if err != nil {
		return err
	}
	mem, _ := a.store.BuildMemoryContext()
	_ = BuildSystemPrompt("learning", "", mem)
	for _, p := range products {
		if p.Status != "killed" && p.Status != "scaling" {
			continue
		}
		results, _ := a.store.GetCampaignResultsForProduct(p.ID)
		productSummary := fmt.Sprintf("%s status=%s margin=%.1f", p.ProductName, p.Status, p.GrossMarginPct)
		campaignSummary := fmt.Sprintf("campaign snapshots=%d", len(results))
		lessons, err := a.reasoner.ExtractLessons(ctx, productSummary, campaignSummary)
		if err != nil {
			continue
		}
		existing, _ := a.store.GetAllLessons()
		for _, l := range lessons {
			prefix := l.Lesson
			if len(prefix) > 50 {
				prefix = prefix[:50]
			}
			found := false
			for _, ex := range existing {
				exPrefix := ex.Lesson
				if len(exPrefix) > 50 {
					exPrefix = exPrefix[:50]
				}
				if ex.Category == l.Category && strings.EqualFold(exPrefix, prefix) {
					_ = a.store.UpdateLessonConfidence(ex.ID, minFloat(1.0, (ex.Confidence+l.Confidence)/2), ex.EvidenceCount+1)
					found = true
					break
				}
			}
			if found {
				continue
			}
			_ = a.store.SaveLearnedLesson(store.LearnedLesson{
				ID: fmt.Sprintf("%s-%d", p.ID, time.Now().UnixNano()), Category: l.Category, Lesson: l.Lesson,
				Confidence: l.Confidence, EvidenceCount: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			})
		}
		a.outbox <- Message{Type: "report", Subject: "Lessons extracted", Body: p.ProductName}
	}
	return nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
