package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/dropshipagent/agent/internal/agent"
	"github.com/dropshipagent/agent/internal/integrations/minea"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dev := false
	if s.cfg != nil {
		dev = s.cfg.DevMode
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"dev_mode": dev,
		"version":  serverVersion,
	})
}

// productTestIsWinnerCandidate approximates the pipeline shortlist: promoted tests, scaling wins,
// or watching rows that already have supplier COGS and decent unit economics.
// Raw Minea-only rows (cogs_eur = 0) stay off this list — use Scraped for the full ad feed.
func productTestIsWinnerCandidate(pt store.ProductTest) bool {
	st := strings.ToLower(strings.TrimSpace(pt.Status))
	if st == "killed" {
		return false
	}
	if st == "scaling" || st == "testing" {
		return true
	}
	if pt.COGSEur <= 0 {
		return false
	}
	if pt.Score >= 60 && pt.GrossMarginPct >= 35 {
		return true
	}
	if pt.GrossMarginPct >= 40 && pt.BEROAS >= 1.5 {
		return true
	}
	if pt.Score >= 55 && pt.GrossMarginPct >= 38 && pt.BEROAS >= 1.4 {
		return true
	}
	return false
}

func (s *Server) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	products, err := s.store.GetAllProducts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" {
		filtered := products[:0]
		for _, p := range products {
			if p.Status == status {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("view")), "winners") {
		filtered := products[:0]
		for _, p := range products {
			if productTestIsWinnerCandidate(p) {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(products)
}

func (s *Server) handleGetCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := s.store.GetActiveCampaigns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(campaigns)
}

func (s *Server) handleGetLessons(w http.ResponseWriter, r *http.Request) {
	lessons, err := s.store.GetAllLessons()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(lessons, func(i, j int) bool {
		return lessons[i].Confidence > lessons[j].Confidence
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(lessons)
}

func (s *Server) handleGetMineaScraped(w http.ResponseWriter, r *http.Request) {
	products, err := s.store.GetAllProducts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	wantNiche := strings.TrimSpace(r.URL.Query().Get("niche"))
	out := make([]interface{}, 0, len(products))
	for _, p := range products {
		if !strings.EqualFold(strings.TrimSpace(p.SourcePlatform), "minea") {
			continue
		}
		if wantNiche != "" && !strings.EqualFold(strings.TrimSpace(p.Niche), wantNiche) {
			continue
		}
		out = append(out, p)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type approveBody struct {
	ProductTestID string `json:"product_test_id"`
	Approved      bool   `json:"approved"`
	Note          string `json:"note"`
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body approveBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.ProductTestID == "" {
		http.Error(w, "product_test_id required", http.StatusBadRequest)
		return
	}
	if s.agent == nil {
		http.Error(w, "agent not configured", http.StatusServiceUnavailable)
		return
	}
	s.agent.ApprovalChan() <- agent.ApprovalResponse{
		ProductTestID: body.ProductTestID,
		Approved:      body.Approved,
		Note:          body.Note,
	}
	w.WriteHeader(http.StatusOK)
}

type chatBody struct {
	Message string `json:"message"`
}

type chatResponse struct {
	Reply string `json:"reply"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Message == "" {
		http.Error(w, "message required", http.StatusBadRequest)
		return
	}
	if s.reasoner == nil {
		http.Error(w, "reasoner not configured", http.StatusServiceUnavailable)
		return
	}
	mem, err := s.store.BuildMemoryContext()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	system := agent.AgentKnowledgeCore
	if mem != "" {
		system += "\n\n" + mem
	}
	reply, err := s.reasoner.Reason(r.Context(), system, body.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatResponse{Reply: reply})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("websocket upgrade failed", zap.Error(err))
		return
	}

	dev := false
	if s.cfg != nil {
		dev = s.cfg.DevMode
	}
	welcome, _ := json.Marshal(map[string]interface{}{
		"type":    "connected",
		"message": formatConnectedMessage(dev),
	})
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, welcome); err != nil {
		_ = conn.Close()
		return
	}

	s.hub.register <- conn

	go func() {
		defer func() {
			s.hub.unregister <- conn
			_ = conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.Debug("websocket read ended", zap.Error(err))
				}
				return
			}
			// Owner commands ignored for PoC.
		}
	}()
}

func formatConnectedMessage(devMode bool) string {
	if devMode {
		return "Agent online. DEV_MODE: true"
	}
	return "Agent online. DEV_MODE: false"
}

type mineaSearchBody struct {
	Country              string                 `json:"country"`
	Limit                int                    `json:"limit"`
	StartPage            int                    `json:"start_page"`
	Pages                int                    `json:"pages"`
	PerPage              int                    `json:"per_page"`
	SortBy               string                 `json:"sort_by"`
	MediaTypes           []string               `json:"media_types"`
	MediaTypesExcludes   []string               `json:"media_types_excludes"`
	PublicationDate      string                 `json:"ad_publication_date"`
	PublicationDateRange []string               `json:"ad_publication_date_range"`
	AdIsActive           []string               `json:"ad_is_active"`
	AdIsActiveExcludes   []string               `json:"ad_is_active_excludes"`
	AdLanguages          []string               `json:"ad_languages"`
	AdLanguagesExcludes  []string               `json:"ad_languages_excludes"`
	AdCountries          []string               `json:"ad_countries"`
	AdCountriesExcludes  []string               `json:"ad_countries_excludes"`
	AdDays               []int                  `json:"ad_days_running"`
	CTAs                 []string               `json:"ctas"`
	CTAsExcludes         []string               `json:"ctas_excludes"`
	OnlyEU               bool                   `json:"only_eu"`
	CPMValue             float64                `json:"cpm_value"`
	ExcludeBad           *bool                  `json:"exclude_bad_data"`
	Collapse             string                 `json:"collapse"`
	ExtraFilters         map[string]interface{} `json:"extra_filters"`
	// Query is the Meta ads library keyword search (same as URL param "query").
	Query string `json:"query"`
	// QSearchTargets is which fields to search (URL param "q_search_targets"), e.g. "adCopy".
	QSearchTargets string `json:"q_search_targets"`
	// ScrapeNiche labels persisted rows (e.g. "pets"); does not change the Minea search query.
	ScrapeNiche string `json:"scrape_niche"`
}

// clampScore0to100 aligns persisted rows with the agent's ScoreProduct scale (0–100).
// Minea often returns engagement/reach as an unbounded metric, not a 0–100 score.
func clampScore0to100(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// mergeMineaSearchRow overlays a live search hit onto a product_tests row. When existing is nil,
// a new row is created with minimal fields; otherwise agent-populated economics are preserved.
func mergeMineaSearchRow(existing *store.ProductTest, c minea.ProductCandidate, now time.Time) store.ProductTest {
	id := strings.TrimSpace(c.ID)
	var pt store.ProductTest
	if existing != nil {
		pt = *existing
	} else {
		pt = store.ProductTest{
			ID:             id,
			SourcePlatform: "minea",
			Status:         "watching",
			CreatedAt:      now,
		}
	}
	pt.ID = id
	pt.SourcePlatform = "minea"
	pt.UpdatedAt = now

	if strings.TrimSpace(c.Name) != "" {
		pt.ProductName = strings.TrimSpace(c.Name)
	}
	if strings.TrimSpace(c.ImageURL) != "" {
		pt.ProductImageURL = strings.TrimSpace(c.ImageURL)
	}
	if strings.TrimSpace(c.Niche) != "" {
		pt.Niche = strings.TrimSpace(c.Niche)
	}
	if strings.TrimSpace(c.ShopifyStore) != "" {
		pt.ShopifyStore = strings.TrimSpace(c.ShopifyStore)
	}
	if strings.TrimSpace(c.SupplierID) != "" {
		pt.Supplier = strings.TrimSpace(c.SupplierID)
	}
	if strings.TrimSpace(c.AdURL) != "" {
		pt.AdURL = strings.TrimSpace(c.AdURL)
	}
	if strings.TrimSpace(c.ShopURL) != "" {
		pt.ShopURL = strings.TrimSpace(c.ShopURL)
	}
	if strings.TrimSpace(c.LandingURL) != "" {
		pt.LandingURL = strings.TrimSpace(c.LandingURL)
	}
	if c.EstimatedSellEur > 0 {
		pt.SellPriceEur = c.EstimatedSellEur
	}
	searchScore := clampScore0to100(int(c.EngagementScore))
	if existing != nil {
		if searchScore > pt.Score {
			pt.Score = searchScore
		}
	} else {
		pt.Score = searchScore
	}
	if pt.Score > 100 {
		pt.Score = 100
	}
	return pt
}

func (s *Server) persistMineaSearchResults(products []minea.ProductCandidate, scrapeNiche string) {
	if s.store == nil {
		return
	}
	now := time.Now().UTC()
	scrapeNiche = strings.TrimSpace(scrapeNiche)
	var saved, skipped int
	for _, c := range products {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			skipped++
			continue
		}
		existing, err := s.store.GetProductTest(id)
		if err != nil {
			s.logger.Warn("minea search persist: load failed", zap.String("id", id), zap.Error(err))
			continue
		}
		pt := mergeMineaSearchRow(existing, c, now)
		if scrapeNiche != "" {
			pt.Niche = scrapeNiche
		}
		if err := s.store.SaveProductTest(pt); err != nil {
			s.logger.Warn("minea search persist: save failed", zap.String("id", id), zap.Error(err))
			continue
		}
		saved++
	}
	if saved > 0 || skipped > 0 {
		s.logger.Info("minea search persisted",
			zap.Int("saved", saved),
			zap.Int("skipped_empty_id", skipped),
			zap.Int("result_count", len(products)))
	}
}

func (s *Server) handleMineaSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.cfg == nil || strings.TrimSpace(s.cfg.MineaEmail) == "" || strings.TrimSpace(s.cfg.MineaPassword) == "" {
		http.Error(w, "MINEA_EMAIL and MINEA_PASSWORD required", http.StatusServiceUnavailable)
		return
	}
	var body mineaSearchBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	scraper := minea.NewScraper(s.cfg.MineaEmail, s.cfg.MineaPassword, "./data/minea_session.json", s.logger)
	products, err := scraper.SearchAds(r.Context(), minea.AdsSearchOptions{
		Country:              body.Country,
		Limit:                body.Limit,
		StartPage:            body.StartPage,
		Pages:                body.Pages,
		PerPage:              body.PerPage,
		SortBy:               body.SortBy,
		MediaTypes:           body.MediaTypes,
		MediaTypesExcludes:   body.MediaTypesExcludes,
		PublicationDate:      body.PublicationDate,
		PublicationDateRange: body.PublicationDateRange,
		AdIsActive:           body.AdIsActive,
		AdIsActiveExcludes:   body.AdIsActiveExcludes,
		AdLanguages:          body.AdLanguages,
		AdLanguagesExcludes:  body.AdLanguagesExcludes,
		AdCountries:          body.AdCountries,
		AdCountriesExcludes:  body.AdCountriesExcludes,
		AdDays:               body.AdDays,
		CTAs:                 body.CTAs,
		CTAsExcludes:         body.CTAsExcludes,
		OnlyEU:               body.OnlyEU,
		CPMValue:             body.CPMValue,
		ExcludeBad:           body.ExcludeBad,
		Collapse:             body.Collapse,
		ExtraFilters:         body.ExtraFilters,
		Query:                strings.TrimSpace(body.Query),
		QSearchTargets:       strings.TrimSpace(body.QSearchTargets),
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("minea search failed: %v", err), http.StatusInternalServerError)
		return
	}
	s.persistMineaSearchResults(products, body.ScrapeNiche)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(products)
}
