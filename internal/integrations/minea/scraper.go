package minea

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"go.uber.org/zap"
)

var ErrAuthExpired = errors.New("minea: auth token expired")

const (
	creditCostPerSearch    = 20.0
	minSafeCredits         = 100.0
	alertCredits           = 200.0
	maxCreditsPerSession   = 200.0
	defaultMineaSession    = "./data/minea_session.json"
	defaultMineaGraphqlURL = "https://www.minea.com/graphql"
)

type Scraper struct {
	email       string
	password    string
	sessionPath string
	authToken   string
	userID      string
	httpClient  *http.Client
	logger      *zap.Logger
	graphqlURL  string
	creditsUsed float64
}

type sessionFile struct {
	AuthToken string    `json:"auth_token"`
	UserID    string    `json:"user_id"`
	SavedAt   time.Time `json:"saved_at"`
}

type creditBalance struct {
	Credits         float64
	TotalCredits    float64
	CreditsRefillAt time.Time
}

func NewScraper(email, password, sessionPath string, logger *zap.Logger) *Scraper {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scraper{
		email:       email,
		password:    password,
		sessionPath: sessionPath,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		logger:      logger,
		graphqlURL:  defaultMineaGraphqlURL,
	}
}

func NewDiscoverer(cfg *config.Config, logger *zap.Logger) Discoverer {
	if cfg.DevMode || strings.TrimSpace(cfg.MineaEmail) == "" || strings.TrimSpace(cfg.MineaPassword) == "" {
		return NewStub()
	}
	return NewScraper(cfg.MineaEmail, cfg.MineaPassword, defaultMineaSession, logger)
}

func (s *Scraper) graphql(ctx context.Context, operationName string, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"operationName": operationName,
		"query":         query,
		"variables":     variables,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("minea %s: %w", operationName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.authToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("minea %s: %w", operationName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		s.authToken = ""
		s.userID = ""
		_ = os.Remove(s.sessionPath)
		return nil, fmt.Errorf("minea %s: %w", operationName, ErrAuthExpired)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("minea %s: http %d", operationName, resp.StatusCode)
	}

	var decoded map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("minea %s: decode response: %w", operationName, err)
	}
	data, _ := decoded["data"].(map[string]interface{})
	if data == nil {
		return map[string]interface{}{}, nil
	}
	return data, nil
}

func (s *Scraper) GetCredits(ctx context.Context) (*creditBalance, error) {
	data, err := s.graphql(ctx, "GET_PROFILE", `query GET_PROFILE($id: ID!) { getUser(id: $id) { id credits totalCredits creditsRefillAt } }`, map[string]interface{}{"id": s.userID})
	if err != nil {
		return nil, err
	}
	user, _ := data["getUser"].(map[string]interface{})
	if user == nil {
		return nil, fmt.Errorf("minea get credits: empty user profile")
	}

	refill, _ := time.Parse(time.RFC3339Nano, getString(user, "creditsRefillAt"))
	b := &creditBalance{
		Credits:         getFloat(user, "credits"),
		TotalCredits:    getFloat(user, "totalCredits"),
		CreditsRefillAt: refill,
	}
	s.logger.Info("minea credits", zap.Float64("credits", b.Credits), zap.Float64("total_credits", b.TotalCredits), zap.Time("refill_at", b.CreditsRefillAt))
	return b, nil
}

func (s *Scraper) canAffordSearch(ctx context.Context) error {
	balance, err := s.GetCredits(ctx)
	if err != nil {
		return err
	}
	if balance.Credits < creditCostPerSearch {
		return fmt.Errorf("insufficient credits: %.0f remaining (need 20, refill at: %s)", balance.Credits, balance.CreditsRefillAt.Format(time.RFC3339))
	}
	if balance.Credits < minSafeCredits {
		s.logger.Warn("credit balance critically low", zap.Float64("credits", balance.Credits))
	} else if balance.Credits < alertCredits {
		s.logger.Warn("credit balance low", zap.Float64("credits", balance.Credits), zap.Time("refill_at", balance.CreditsRefillAt))
	}
	return nil
}

func (s *Scraper) loadSession() bool {
	raw, err := os.ReadFile(s.sessionPath)
	if err != nil {
		return false
	}
	var sf sessionFile
	if err := json.Unmarshal(raw, &sf); err != nil {
		return false
	}
	if sf.SavedAt.Before(time.Now().AddDate(0, 0, -30)) {
		return false
	}
	s.authToken = sf.AuthToken
	s.userID = sf.UserID
	return s.authToken != "" && s.userID != ""
}

func (s *Scraper) saveSession() error {
	sf := sessionFile{AuthToken: s.authToken, UserID: s.userID, SavedAt: time.Now().UTC()}
	raw, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.sessionPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.sessionPath, raw, 0o600)
}

func (s *Scraper) login(ctx context.Context) error {
	u := launcher.New().Headless(true).Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36").MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	defer func() {
		_ = browser.Close()
	}()

	page := browser.MustPage("")
	defer func() { _ = page.Close() }()

	if err := page.Navigate("https://www.minea.com/login"); err != nil {
		return err
	}
	page.MustWaitLoad()

	emailEl := page.MustElement("input[type='email']")
	for _, ch := range s.email {
		emailEl.MustInput(string(ch))
		humanDelay()
	}
	passEl := page.MustElement("input[type='password']")
	for _, ch := range s.password {
		passEl.MustInput(string(ch))
		humanDelay()
	}
	page.MustElement("button[type='submit']").MustClick()
	time.Sleep(3 * time.Second)

	token := strings.TrimSpace(page.MustEval(`() => localStorage.getItem('token') || localStorage.getItem('authToken') || ''`).String())
	if token == "" {
		cookies := browser.MustGetCookies()
		for _, c := range cookies {
			if strings.HasPrefix(c.Value, "ey") {
				token = c.Value
				break
			}
		}
	}
	if token == "" {
		return fmt.Errorf("minea login: unable to extract auth token")
	}

	s.authToken = strings.Trim(token, `"`)
	if err := s.getProfileFromToken(ctx); err != nil {
		return err
	}
	return s.saveSession()
}

func (s *Scraper) getProfileFromToken(ctx context.Context) error {
	data, err := s.graphql(ctx, "GET_PROFILE", `query GET_PROFILE($id: ID!) { getUser(id: $id) { id } }`, map[string]interface{}{"id": ""})
	if err == nil {
		if user, _ := data["getUser"].(map[string]interface{}); user != nil {
			if id := getString(user, "id"); id != "" {
				s.userID = id
				return nil
			}
		}
	}

	parts := strings.Split(s.authToken, ".")
	if len(parts) < 2 {
		return fmt.Errorf("minea profile: unable to resolve user id")
	}
	decoded, decErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decErr != nil {
		return fmt.Errorf("minea profile: unable to decode token")
	}
	var claims map[string]interface{}
	if jsonErr := json.Unmarshal(decoded, &claims); jsonErr != nil {
		return fmt.Errorf("minea profile: unable to parse token claims")
	}
	id := getString(claims, "sub", "id")
	if id == "" {
		return fmt.Errorf("minea profile: user id not found")
	}
	s.userID = id
	return nil
}

func humanDelay() {
	time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
}

func (s *Scraper) ensureAuth(ctx context.Context) error {
	if s.authToken != "" && s.userID != "" {
		return nil
	}
	if s.loadSession() {
		s.logger.Info("Session restored")
		return nil
	}
	if err := s.login(ctx); err != nil {
		return err
	}
	s.logger.Info("Login successful")
	return nil
}

func (s *Scraper) EnsureAuth(ctx context.Context) error {
	return s.ensureAuth(ctx)
}

func (s *Scraper) GetTrendingProducts(ctx context.Context, niche string, country string, limit int) ([]ProductCandidate, error) {
	if err := s.ensureAuth(ctx); err != nil {
		return nil, err
	}
	if s.creditsUsed+creditCostPerSearch > maxCreditsPerSession {
		return nil, fmt.Errorf("session credit quota reached: used %.0f/%.0f", s.creditsUsed, maxCreditsPerSession)
	}
	if err := s.canAffordSearch(ctx); err != nil {
		return nil, err
	}

	data, err := s.graphql(ctx, "GET_TRENDING_PRODUCTS", `query GET_TRENDING_PRODUCTS($niche: String, $country: String, $limit: Int) {
		getSuccessRadar(niche: $niche, country: $country, limit: $limit)
	}`, map[string]interface{}{"niche": niche, "country": country, "limit": limit})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll("./data", 0o755); err == nil {
		if raw, marshalErr := json.MarshalIndent(data, "", "  "); marshalErr == nil {
			_ = os.WriteFile("./data/minea_response_sample.json", raw, 0o644)
		}
	}

	products, err := s.parseGraphQLProducts(data, limit)
	if err != nil {
		return nil, err
	}
	s.creditsUsed += creditCostPerSearch
	if bal, bErr := s.GetCredits(ctx); bErr == nil {
		s.logger.Info("credits used: 20", zap.Float64("remaining", bal.Credits-creditCostPerSearch))
	}
	return products, nil
}

func (s *Scraper) GetProductDetails(ctx context.Context, productID string) (*ProductCandidate, error) {
	_ = ctx
	return nil, fmt.Errorf("product not found: %s", productID)
}

func (s *Scraper) parseGraphQLProducts(data map[string]interface{}, limit int) ([]ProductCandidate, error) {
	items := extractItems(data)
	if len(items) == 0 {
		return []ProductCandidate{}, nil
	}
	now := time.Now().UTC()
	out := make([]ProductCandidate, 0, len(items))
	for _, item := range items {
		name := getString(item, "name", "product_name", "title")
		niche := getString(item, "niche", "category")
		pc := ProductCandidate{
			ID:               getString(item, "id", "_id", "product_id"),
			Name:             name,
			Niche:            niche,
			ShopifyStore:     deriveShopifyStore(name, niche),
			FirstSeenDate:    now,
			Platforms:        getStringSlice(item, "platforms", "platform"),
			ActiveAdCount:    getInt(item, "ad_count", "ads_count", "active_ads"),
			EngagementScore:  getFloat(item, "engagement", "engagement_score", "score"),
			EstimatedSellEur: getFloat(item, "price", "selling_price", "estimated_price"),
			SupplierID:       getString(item, "supplier_id", "supplierId"),
			ImageURL:         getString(item, "image", "image_url", "thumbnail"),
			WeeksSinceTrend:  getInt(item, "weeks_trending", "trend_weeks"),
			TikTokOrganic:    getBool(item, "tiktok_organic", "has_tiktok"),
			GoogleTrending:   getBool(item, "google_trending", "google_trend"),
		}
		if rawDate := getString(item, "first_seen_date", "firstSeenDate"); rawDate != "" {
			if t, err := time.Parse(time.RFC3339Nano, rawDate); err == nil {
				pc.FirstSeenDate = t
			}
		}
		out = append(out, pc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func extractItems(data map[string]interface{}) []map[string]interface{} {
	if arr := asArray(data["getSuccessRadar"]); len(arr) > 0 {
		return arr
	}
	if obj, _ := data["searchProducts"].(map[string]interface{}); obj != nil {
		if arr := asArray(obj["items"]); len(arr) > 0 {
			return arr
		}
		if arr := asArray(obj["data"]); len(arr) > 0 {
			return arr
		}
	}
	if obj, _ := data["searchAds"].(map[string]interface{}); obj != nil {
		if arr := asArray(obj["items"]); len(arr) > 0 {
			return arr
		}
		if arr := asArray(obj["data"]); len(arr) > 0 {
			return arr
		}
	}
	if arr := asArray(data["items"]); len(arr) > 0 {
		return arr
	}
	for _, v := range data {
		if arr := asArray(v); len(arr) > 0 {
			return arr
		}
	}
	return nil
}

func asArray(v interface{}) []map[string]interface{} {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func getString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return t
				}
			}
		}
	}
	return ""
}

func getFloat(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return t
			case int:
				return float64(t)
			}
		}
	}
	return 0
}

func getInt(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case int:
				return t
			case float64:
				return int(t)
			}
		}
	}
	return 0
}

func getBool(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return false
}

func getStringSlice(m map[string]interface{}, keys ...string) []string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case []interface{}:
				out := make([]string, 0, len(t))
				for _, e := range t {
					if s, ok := e.(string); ok && s != "" {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					return out
				}
			case string:
				if t != "" {
					return []string{t}
				}
			}
		}
	}
	return nil
}

func deriveShopifyStore(name, niche string) string {
	hay := strings.ToLower(name + " " + niche)
	for _, kw := range []string{"pet", "dog", "cat", "animal", "fur", "paw"} {
		if strings.Contains(hay, kw) {
			return "pets"
		}
	}
	return "tech"
}
