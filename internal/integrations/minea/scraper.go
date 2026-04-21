package minea

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"go.uber.org/zap"
)

var ErrAuthExpired = errors.New("minea: auth token expired")

const (
	defaultMineaSession    = "./data/minea_session.json"
	// Minea moved the app to app.minea.com; www.minea.com/login is not the real login UI.
	// quickview matches the modal login users see when opening from the marketing site.
	defaultMineaLoginURL   = "https://app.minea.com/en/login/quickview?from=%2F"
	// Real API is AWS AppSync; app.minea.com/graphql returns 307/HTML for unauthenticated clients.
	defaultMineaGraphqlURL = "https://y5ec7qy3bzbkxm6udp4u3o3lee.appsync-api.eu-west-1.amazonaws.com/graphql"
	defaultMineaGraphQLUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	defaultMineaGraphQLOrigin    = "https://app.minea.com"
)

// savedCookie is a minimal cookie snapshot persisted with the Minea session (for server-side GraphQL).
type savedCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path,omitempty"`
}

func cookieAppliesToHost(domain, host string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	h := strings.ToLower(host)
	if d == "" {
		return false
	}
	return h == d || strings.HasSuffix(h, "."+d)
}

func setMineaGraphQLAuthHeader(req *http.Request, graphqlURL, authToken string) {
	if authToken == "" {
		return
	}
	tok := strings.TrimSpace(authToken)
	if len(tok) > 7 && strings.EqualFold(tok[:7], "bearer ") {
		tok = strings.TrimSpace(tok[7:])
	}
	// Browser HAR shows raw JWT for AppSync; Vercel / legacy route used Bearer.
	if strings.Contains(strings.ToLower(graphqlURL), "appsync-api.") {
		req.Header.Set("Authorization", tok)
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok)
}

func mineaCookieHeader(cookies []savedCookie, host string) string {
	seen := make(map[string]string)
	for _, c := range cookies {
		if c.Name == "" || !cookieAppliesToHost(c.Domain, host) {
			continue
		}
		seen[c.Name] = c.Value
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, n+"="+seen[n])
	}
	return strings.Join(parts, "; ")
}

func cookiesFromBrowser(browser *rod.Browser) ([]savedCookie, error) {
	raw, err := browser.GetCookies()
	if err != nil {
		return nil, err
	}
	out := make([]savedCookie, 0, len(raw))
	for _, c := range raw {
		if c == nil {
			continue
		}
		out = append(out, savedCookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path})
	}
	return out, nil
}

// mineaStorageTokenJS scans localStorage/sessionStorage for a JWT-shaped session token.
const mineaStorageTokenJS = `() => {
	function jwtLike(s) {
		if (!s || typeof s !== 'string') return '';
		s = s.trim();
		if (s.startsWith('"') && s.endsWith('"')) try { s = JSON.parse(s); } catch(e) {}
		if (typeof s !== 'string') return '';
		const parts = s.split('.');
		if (parts.length >= 3 && s.startsWith('ey') && s.length > 40) return s;
		return '';
	}
	function fromJSON(s) {
		if (!s) return '';
		try {
			const j = JSON.parse(s);
			for (const k of ['token','access_token','accessToken','authToken','jwt']) {
				const v = jwtLike(j[k] || '');
				if (v) return v;
			}
		} catch (e) {}
		return '';
	}
	const keys = ['token','authToken','auth_token','access_token','accessToken','jwt','session','user','persist:root'];
	for (const k of keys) {
		let v = localStorage.getItem(k) || sessionStorage.getItem(k) || '';
		let t = jwtLike(v) || fromJSON(v);
		if (t) return t;
	}
	for (let i = 0; i < localStorage.length; i++) {
		const k = localStorage.key(i);
		const v = localStorage.getItem(k);
		const t = jwtLike(v) || fromJSON(v);
		if (t) return t;
	}
	for (let i = 0; i < sessionStorage.length; i++) {
		const k = sessionStorage.key(i);
		const v = sessionStorage.getItem(k);
		const t = jwtLike(v) || fromJSON(v);
		if (t) return t;
	}
	return '';
}`

var jwtCookieValue = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9._-]*$`)

func tokenFromMineaStorage(page *rod.Page) string {
	res, err := page.Eval(mineaStorageTokenJS)
	if err != nil {
		return ""
	}
	tok := strings.TrimSpace(res.Value.String())
	if tok == "" || tok == `""` {
		return ""
	}
	return tok
}

func tokenFromMineaCookies(browser *rod.Browser) string {
	cookies, err := browser.GetCookies()
	if err != nil {
		return ""
	}
	for _, c := range cookies {
		v := strings.TrimSpace(c.Value)
		if len(v) < 40 || !strings.HasPrefix(v, "ey") || strings.Count(v, ".") < 2 {
			continue
		}
		if jwtCookieValue.MatchString(v) {
			return v
		}
	}
	return ""
}

func pollMineaAuthToken(page *rod.Page, browser *rod.Browser, maxWait time.Duration) string {
	deadline := time.Now().Add(maxWait)
	step := 500 * time.Millisecond
	for time.Now().Before(deadline) {
		if t := tokenFromMineaStorage(page); t != "" {
			return t
		}
		if t := tokenFromMineaCookies(browser); t != "" {
			return t
		}
		time.Sleep(step)
	}
	return ""
}

type Scraper struct {
	email       string
	password    string
	sessionPath string
	authToken   string
	userID      string
	cookies     []savedCookie
	httpClient  *http.Client
	logger      *zap.Logger
	graphqlURL  string
	creditsUsed float64
}

type sessionFile struct {
	AuthToken string        `json:"auth_token"`
	UserID    string        `json:"user_id"`
	SavedAt   time.Time     `json:"saved_at"`
	Cookies   []savedCookie `json:"cookies,omitempty"`
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
	gql := strings.TrimSpace(os.Getenv("MINEA_GRAPHQL_URL"))
	if gql == "" {
		gql = defaultMineaGraphqlURL
	}
	return &Scraper{
		email:       email,
		password:    password,
		sessionPath: sessionPath,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger:     logger,
		graphqlURL: gql,
	}
}

func NewDiscoverer(cfg *config.Config, logger *zap.Logger) Discoverer {
	// DevMode does not imply fake Minea: local dev often uses DEV_MODE=true with real
	// MINEA_* credentials while other integrations stay stubbed. Use the stub only
	// when credentials are missing or MINEA_STUB=true (e.g. smoke-test / CI).
	if strings.EqualFold(strings.TrimSpace(os.Getenv("MINEA_STUB")), "true") {
		return NewStub()
	}
	if strings.TrimSpace(cfg.MineaEmail) == "" || strings.TrimSpace(cfg.MineaPassword) == "" {
		return NewStub()
	}
	return NewScraper(cfg.MineaEmail, cfg.MineaPassword, defaultMineaSession, logger)
}

// graphql posts to Minea's GraphQL API. If invalidateOnGraphQLRedirect is true, a 307 JSON body
// with "redirect" (unauthenticated flow) clears the saved session and returns ErrAuthExpired so
// callers can re-login. Must be false during login's getProfileFromToken so we do not wipe a fresh token.
func (s *Scraper) graphql(ctx context.Context, operationName string, query string, variables map[string]interface{}, invalidateOnGraphQLRedirect bool) (map[string]interface{}, error) {
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", mineaUserAgent())
	req.Header.Set("Origin", mineaOrigin())
	req.Header.Set("Referer", mineaOrigin()+"/")
	setMineaGraphQLAuthHeader(req, s.graphqlURL, s.authToken)
	if u, parseErr := url.Parse(s.graphqlURL); parseErr == nil && u.Host != "" {
		if ch := mineaCookieHeader(s.cookies, u.Host); ch != "" {
			req.Header.Set("Cookie", ch)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("minea %s: %w", operationName, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("minea %s: read body: %w", operationName, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		s.authToken = ""
		s.userID = ""
		s.cookies = nil
		_ = os.Remove(s.sessionPath)
		return nil, fmt.Errorf("minea %s: %w", operationName, ErrAuthExpired)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if invalidateOnGraphQLRedirect &&
			(resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusPermanentRedirect ||
				resp.StatusCode == http.StatusFound) {
			var meta map[string]interface{}
			if json.Unmarshal(respBody, &meta) == nil {
				if redirect, _ := meta["redirect"].(string); redirect != "" {
					s.authToken = ""
					s.userID = ""
					s.cookies = nil
					_ = os.Remove(s.sessionPath)
					return nil, fmt.Errorf("minea %s: %w", operationName, ErrAuthExpired)
				}
			}
		}
		if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusPermanentRedirect ||
			resp.StatusCode == http.StatusFound {
			var meta map[string]interface{}
			if json.Unmarshal(respBody, &meta) == nil {
				if redirect, _ := meta["redirect"].(string); redirect != "" {
					return nil, fmt.Errorf("minea %s: session rejected (http %d, redirect %q)", operationName, resp.StatusCode, redirect)
				}
			}
		}
		return nil, fmt.Errorf("minea %s: http %d", operationName, resp.StatusCode)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("minea %s: decode response: %w", operationName, err)
	}
	if errs := extractGraphQLErrorMessages(decoded["errors"]); len(errs) > 0 {
		return nil, fmt.Errorf("minea %s: graphql errors: %s", operationName, strings.Join(errs, "; "))
	}
	data, _ := decoded["data"].(map[string]interface{})
	if data == nil {
		return map[string]interface{}{}, nil
	}
	return data, nil
}

func extractGraphQLErrorMessages(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		msg := ""
		if m, ok := entry.(map[string]interface{}); ok {
			msg = strings.TrimSpace(getString(m, "message"))
		}
		if msg != "" {
			out = append(out, msg)
		}
	}
	return out
}

func (s *Scraper) graphqlWithAuthRetry(ctx context.Context, operationName, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	data, err := s.graphql(ctx, operationName, query, variables, true)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, ErrAuthExpired) {
		return nil, err
	}
	s.logger.Info("minea: re-login after session expired or rejected", zap.String("operation", operationName))
	if err := s.login(ctx); err != nil {
		return nil, err
	}
	return s.graphql(ctx, operationName, query, variables, true)
}

func (s *Scraper) GetCredits(ctx context.Context) (*creditBalance, error) {
	data, err := s.graphqlWithAuthRetry(ctx, "GET_PROFILE", `query GET_PROFILE($id: ID!) { getUser(id: $id) { id credits totalCredits creditsRefillAt } }`, map[string]interface{}{"id": s.userID})
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
	if balance.Credits < mineaCreditCostPerSearch() {
		return fmt.Errorf("insufficient credits: %.0f remaining (need 20, refill at: %s)", balance.Credits, balance.CreditsRefillAt.Format(time.RFC3339))
	}
	if balance.Credits < mineaMinSafeCredits() {
		s.logger.Warn("credit balance critically low", zap.Float64("credits", balance.Credits))
	} else if balance.Credits < mineaAlertCredits() {
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
	s.cookies = sf.Cookies
	return s.authToken != "" && s.userID != ""
}

func (s *Scraper) saveSession() error {
	sf := sessionFile{AuthToken: s.authToken, UserID: s.userID, SavedAt: time.Now().UTC(), Cookies: s.cookies}
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
	cogErr := s.loginViaCognitoPassword(ctx)
	if cogErr == nil {
		s.logger.Info("minea login: Cognito USER_PASSWORD_AUTH succeeded (no browser)")
		return nil
	}
	s.logger.Info("minea login: Cognito unavailable or failed; using Rod", zap.Error(cogErr))
	return s.loginViaRod(ctx)
}

func (s *Scraper) loginViaCognitoPassword(ctx context.Context) error {
	if strings.EqualFold(os.Getenv("MINEA_SKIP_COGNITO"), "1") || strings.EqualFold(os.Getenv("MINEA_SKIP_COGNITO"), "true") {
		return fmt.Errorf("minea: MINEA_SKIP_COGNITO set")
	}
	region := strings.TrimSpace(os.Getenv("MINEA_COGNITO_REGION"))
	clientID := strings.TrimSpace(os.Getenv("MINEA_COGNITO_CLIENT_ID"))
	idTok, _, _, err := cognitoPasswordAuth(ctx, region, clientID, s.email, s.password)
	if err != nil {
		return err
	}
	sub, err := cognitoSubFromJWT(idTok)
	if err != nil {
		return fmt.Errorf("minea cognito: parse id token: %w", err)
	}
	s.authToken = idTok
	s.userID = sub
	s.cookies = nil
	return s.saveSession()
}

func (s *Scraper) loginViaRod(ctx context.Context) error {
	loginURL := strings.TrimSpace(os.Getenv("MINEA_LOGIN_URL"))
	if loginURL == "" {
		loginURL = defaultMineaLoginURL
	}

	s.logger.Info("minea login: launching headless Chromium (may download on first run)")
	// Linux (and many CI/container environments) often lack a usable Chromium sandbox;
	// without --no-sandbox Rod's bundled Chromium aborts with zygote_host_impl_linux "No usable sandbox".
	u := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("user-agent", mineaUserAgent()).
		MustLaunch()
	s.logger.Info("minea login: browser connected")
	browser := rod.New().ControlURL(u).Context(ctx).MustConnect()
	defer func() {
		_ = browser.Close()
	}()

	page := browser.MustPage("")
	page = page.Timeout(4 * time.Minute)
	defer func() { _ = page.Close() }()

	s.logger.Info("minea login: opening login page", zap.String("url", loginURL))
	if err := page.Navigate(loginURL); err != nil {
		return fmt.Errorf("minea login: navigate: %w", err)
	}
	page.MustWaitLoad()
	// Next.js / React: wait for client-rendered inputs (otherwise Element() can retry until global timeout).
	time.Sleep(2 * time.Second)
	if info, err := page.Info(); err == nil {
		s.logger.Info("minea login: document ready", zap.String("title", info.Title), zap.String("url", info.URL))
	}

	slow := strings.EqualFold(os.Getenv("MINEA_SLOW_LOGIN"), "1") || strings.EqualFold(os.Getenv("MINEA_SLOW_LOGIN"), "true")
	if slow {
		emailSel := `input[type="email"], input[name="email"], input[autocomplete="username"], input[autocomplete="email"]`
		emailEl, err := page.Element(emailSel)
		if err != nil {
			return fmt.Errorf("minea login: no email field on page (wrong URL or layout changed?): %w", err)
		}
		passEl, err := page.Element(`input[type="password"], input[name="password"], input[autocomplete="current-password"]`)
		if err != nil {
			return fmt.Errorf("minea login: no password field: %w", err)
		}
		n := len(s.email) + len(s.password)
		s.logger.Info("minea login: typing credentials slowly", zap.Int("chars", n))
		for i, ch := range s.email {
			emailEl.MustInput(string(ch))
			humanDelay()
			if (i+1)%10 == 0 {
				s.logger.Info("minea login: email progress", zap.Int("chars", i+1))
			}
		}
		for _, ch := range s.password {
			passEl.MustInput(string(ch))
			humanDelay()
		}
	} else {
		s.logger.Info("minea login: filling credentials (React-safe native setter; set MINEA_SLOW_LOGIN=true for keystroke-by-keystroke)")
		fillRes, err := page.Eval(`(email, password) => {
			const q = (sel) => document.querySelector(sel);
			const emailEl = q('input[type="email"]') || q('input[autocomplete="email"]') || q('input[autocomplete="username"]') || q('input[name="email"]');
			const passEl = q('input[type="password"]') || q('input[autocomplete="current-password"]') || q('input[name="password"]');
			function setNative(el, v) {
				if (!el) return false;
				const desc = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
				if (desc && desc.set) { desc.set.call(el, String(v)); } else { el.value = String(v); }
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return true;
			}
			return setNative(emailEl, email) && setNative(passEl, password);
		}`, s.email, s.password)
		if err != nil {
			return fmt.Errorf("minea login: fill fields: %w", err)
		}
		if !fillRes.Value.Bool() {
			return fmt.Errorf("minea login: could not find email/password inputs (layout changed?)")
		}
		time.Sleep(200 * time.Millisecond)
	}

	s.logger.Info("minea login: submitting form")
	clickRes, err := page.Eval(`() => {
		const visible = (el) => el && el.offsetParent !== null;
		const btns = [...document.querySelectorAll('button')];
		const login = btns.find(b => visible(b) && /^\s*login\s*$/i.test((b.textContent || '').trim()));
		if (login) { login.click(); return true; }
		const sub = document.querySelector('button[type="submit"]');
		if (sub && visible(sub)) { sub.click(); return true; }
		const formSub = document.querySelector('form button[type="submit"]');
		if (formSub && visible(formSub)) { formSub.click(); return true; }
		return false;
	}`)
	if err != nil {
		return fmt.Errorf("minea login: click login: %w", err)
	}
	if !clickRes.Value.Bool() {
		return fmt.Errorf("minea login: could not find visible Login / submit button")
	}

	token := pollMineaAuthToken(page, browser, 25*time.Second)
	if token == "" {
		if info, err := page.Info(); err == nil {
			s.logger.Error("minea login: no token after submit", zap.String("title", info.Title), zap.String("url", info.URL))
		}
		return fmt.Errorf("minea login: unable to extract auth token")
	}

	s.authToken = strings.Trim(token, `"`)
	store, err := cookiesFromBrowser(browser)
	if err != nil {
		return fmt.Errorf("minea login: read cookies: %w", err)
	}
	s.cookies = store
	if err := s.getProfileFromToken(ctx); err != nil {
		return err
	}
	return s.saveSession()
}

func (s *Scraper) getProfileFromToken(ctx context.Context) error {
	data, err := s.graphql(ctx, "GET_PROFILE", `query GET_PROFILE($id: ID!) { getUser(id: $id) { id } }`, map[string]interface{}{"id": ""}, false)
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
	if s.creditsUsed+mineaCreditCostPerSearch() > mineaMaxCreditsPerSession() {
		return nil, fmt.Errorf("session credit quota reached: used %.0f/%.0f", s.creditsUsed, mineaMaxCreditsPerSession())
	}
	if err := s.canAffordSearch(ctx); err != nil {
		return nil, err
	}
	var beforeCredits *float64
	if bal, bErr := s.GetCredits(ctx); bErr == nil {
		beforeCredits = &bal.Credits
	}

	data, err := s.graphqlWithAuthRetry(ctx, "GET_TRENDING_PRODUCTS", `query GET_TRENDING_PRODUCTS($niche: String, $country: String, $limit: Int) {
		getSuccessRadar(niche: $niche, country: $country, limit: $limit)
	}`, map[string]interface{}{"niche": niche, "country": country, "limit": limit})
	if err != nil {
		// Some Minea plans/schemas do not expose getSuccessRadar.
		if isMissingGraphQLField(err, "getSuccessRadar") {
			s.logger.Info("minea: getSuccessRadar unavailable, falling back to searchProducts")
			data, err = s.graphqlWithAuthRetry(ctx, "SEARCH_PRODUCTS", `query SEARCH_PRODUCTS($niche: String, $country: String, $limit: Int) {
				searchProducts(niche: $niche, country: $country, limit: $limit) {
					items
				}
			}`, map[string]interface{}{"niche": niche, "country": country, "limit": limit})
			if err != nil && strings.Contains(strings.ToLower(err.Error()), "unknownargument") {
				s.logger.Info("minea: graphql searchProducts signature mismatch, falling back to rpc searchAds")
				data, err = s.rpcSearchAds(ctx, AdsSearchOptions{Country: country, Limit: limit})
			}
		}
		if err != nil {
			return nil, err
		}
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
	s.creditsUsed += mineaCreditCostPerSearch()
	if bal, bErr := s.GetCredits(ctx); bErr == nil {
		used, verified := calculateCreditUsage(beforeCredits, bal.Credits)
		if verified {
			s.logger.Info("credits used", zap.Float64("used", used), zap.Float64("remaining", bal.Credits))
		} else {
			s.logger.Warn("credit usage not verified by balance delta", zap.Float64("balance", bal.Credits), zap.Float64("expected_cost", mineaCreditCostPerSearch()))
		}
	}
	return products, nil
}

func (s *Scraper) SearchAds(ctx context.Context, opts AdsSearchOptions) ([]ProductCandidate, error) {
	if err := s.ensureAuth(ctx); err != nil {
		return nil, err
	}
	data, err := s.rpcSearchAds(ctx, opts)
	if err != nil {
		return nil, err
	}
	return s.parseGraphQLProducts(data, opts.Limit)
}

func calculateCreditUsage(before *float64, after float64) (used float64, verified bool) {
	if before == nil {
		return 0, false
	}
	delta := *before - after
	if delta <= 0 {
		return 0, false
	}
	return delta, true
}

func isMissingGraphQLField(err error, field string) bool {
	if err == nil || strings.TrimSpace(field) == "" {
		return false
	}
	msg := strings.ToLower(err.Error())
	fieldLower := strings.ToLower(field)
	return strings.Contains(msg, "fieldundefined") && strings.Contains(msg, fieldLower)
}

func (s *Scraper) rpcSearchAds(ctx context.Context, opts AdsSearchOptions) (map[string]interface{}, error) {
	startPage, totalPages, perPage := mineaRPCSearchPagination(opts)
	u, err := url.Parse(s.graphqlURL)
	if err != nil {
		return nil, fmt.Errorf("minea rpc searchAds: parse graphql url: %w", err)
	}
	rpcBase, err := url.Parse(mineaOrigin())
	if err != nil {
		return nil, fmt.Errorf("minea rpc searchAds: parse rpc base: %w", err)
	}
	hostLower := strings.ToLower(u.Host)
	if strings.Contains(hostLower, "app.minea.com") || strings.Contains(hostLower, "localhost") || strings.Contains(hostLower, "127.0.0.1") {
		rpcBase = u
	}
	rpcURL := rpcBase.Scheme + "://" + rpcBase.Host + "/rpc/meta-ads-graphql/searchAds"
	combined := make([]map[string]interface{}, 0, perPage*totalPages)
	for page := startPage; page < startPage+totalPages; page++ {
		payload := mineaRPCSearchPayload(opts, page, perPage)
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("minea rpc searchAds: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", mineaUserAgent())
		req.Header.Set("Origin", mineaOrigin())
		referer := mineaOrigin() + "/"
		if q := strings.TrimSpace(opts.Query); q != "" {
			sb := strings.TrimSpace(opts.SortBy)
			if sb == "" {
				sb = "-publication_date"
			}
			qt := strings.TrimSpace(opts.QSearchTargets)
			if qt == "" {
				qt = "adCopy"
			}
			referer = fmt.Sprintf("%s/en/ads/meta-library?sort_by=%s&q_search_targets=%s&query=%s",
				mineaOrigin(), url.QueryEscape(sb), url.QueryEscape(qt), url.QueryEscape(q))
		}
		req.Header.Set("Referer", referer)
		setMineaGraphQLAuthHeader(req, rpcURL, s.authToken)
		if ch := mineaCookieHeader(s.cookies, rpcBase.Host); ch != "" {
			req.Header.Set("Cookie", ch)
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("minea rpc searchAds: %w", err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("minea rpc searchAds: read body: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("minea rpc searchAds: http %d", resp.StatusCode)
		}
		var decoded interface{}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("minea rpc searchAds: decode response: %w", err)
		}
		combined = append(combined, extractItemsRecursive(decoded)...)
	}
	if len(combined) == 0 {
		return map[string]interface{}{}, nil
	}
	asIface := make([]interface{}, 0, len(combined))
	for _, it := range combined {
		asIface = append(asIface, it)
	}
	return map[string]interface{}{"items": asIface}, nil
}

func mineaRPCSearchPayload(opts AdsSearchOptions, page, perPage int) map[string]interface{} {
	sortBy := strings.TrimSpace(opts.SortBy)
	if sortBy == "" {
		sortBy = "-publication_date"
	}
	excludeBad := envBool("MINEA_EXCLUDE_BAD_DATA", true)
	if opts.ExcludeBad != nil {
		excludeBad = *opts.ExcludeBad
	}
	cpm := envFloatWithBounds("MINEA_CPM_VALUE", 9, 0, 100000)
	if opts.CPMValue > 0 {
		cpm = opts.CPMValue
	}
	collapse := strings.TrimSpace(opts.Collapse)
	pubDate := interface{}(coalesceString(opts.PublicationDate, strings.TrimSpace(os.Getenv("MINEA_AD_PUBLICATION_DATE")), "last_2_weeks"))
	if len(opts.PublicationDateRange) == 2 {
		pubDate = []string{strings.TrimSpace(opts.PublicationDateRange[0]), strings.TrimSpace(opts.PublicationDateRange[1])}
	}
	payload := map[string]interface{}{
		"json": map[string]interface{}{
			"sort_by": sortBy,
			"ad_media_type": map[string]interface{}{
				"includes": coalesceStrings(opts.MediaTypes, envCSVStrings("MINEA_AD_MEDIA_INCLUDES", []string{"video"})),
				"excludes": coalesceStrings(opts.MediaTypesExcludes, envCSVStrings("MINEA_AD_MEDIA_EXCLUDES", []string{})),
			},
			"ad_days_running": coalesceInts(opts.AdDays, envCSVInts("MINEA_AD_DAYS_RUNNING", []int{7, 30})),
			"ad_cta": map[string]interface{}{
				"includes": coalesceStrings(opts.CTAs, envCSVStrings("MINEA_AD_CTA_INCLUDES", []string{"SHOP_NOW", "BUY_NOW", "LEARN_MORE"})),
				"excludes": coalesceStrings(opts.CTAsExcludes, envCSVStrings("MINEA_AD_CTA_EXCLUDES", []string{})),
			},
			"ad_publication_date": pubDate,
			"ad_is_active": map[string]interface{}{
				"includes": coalesceStrings(opts.AdIsActive, envCSVStrings("MINEA_AD_IS_ACTIVE", []string{"active"})),
				"excludes": coalesceStrings(opts.AdIsActiveExcludes, envCSVStrings("MINEA_AD_IS_ACTIVE_EXCLUDES", []string{})),
			},
			"ad_languages": map[string]interface{}{
				"includes": coalesceStrings(opts.AdLanguages, envCSVStrings("MINEA_AD_LANGUAGES", []string{"de"})),
				"excludes": coalesceStrings(opts.AdLanguagesExcludes, envCSVStrings("MINEA_AD_LANGUAGES_EXCLUDES", []string{})),
			},
			"ad_countries": map[string]interface{}{
				"includes": coalesceStrings(opts.AdCountries, envCSVStrings("MINEA_AD_COUNTRIES", []string{"GB", "DE", "FR", "ES", "IT", "PL", "NL", "AT", "BE", "CZ", "DK", "FI", "GR", "HU", "IE", "LU", "PT", "RO", "SE", "SK"})),
				"excludes": coalesceStrings(opts.AdCountriesExcludes, envCSVStrings("MINEA_AD_COUNTRIES_EXCLUDES", []string{})),
			},
			"pagination": map[string]interface{}{"page": page, "per_page": perPage},
			"cpmValue": cpm,
			"excludeBadData": excludeBad,
		},
	}
	if collapse != "" {
		payload["json"].(map[string]interface{})["collapse"] = collapse
	}
	if opts.OnlyEU || strings.EqualFold(strings.TrimSpace(opts.Country), "EU") {
		payload["json"].(map[string]interface{})["is_eu"] = map[string]interface{}{
			"includes": []string{"eu"},
			"excludes": []string{},
		}
	}
	for k, v := range opts.ExtraFilters {
		payload["json"].(map[string]interface{})[k] = v
	}
	j := payload["json"].(map[string]interface{})
	if qt := strings.TrimSpace(opts.QSearchTargets); qt != "" {
		j["q_search_targets"] = qt
	}
	if q := strings.TrimSpace(opts.Query); q != "" {
		j["query"] = q
		if _, ok := j["q_search_targets"]; !ok {
			j["q_search_targets"] = "adCopy"
		}
	}
	return payload
}

func mineaRPCSearchPagination(opts AdsSearchOptions) (startPage int, pages int, perPage int) {
	perPage = opts.PerPage
	if perPage <= 0 {
		perPage = opts.Limit
		if perPage <= 0 {
			perPage = 20
		}
	}
	startPage = opts.StartPage
	if startPage <= 0 {
		startPage = envIntWithBounds("MINEA_PAGE", 1, 1, 9999)
	}
	pages = opts.Pages
	if pages <= 0 {
		pages = envIntWithBounds("MINEA_PAGES", 3, 1, 20)
	}
	return startPage, pages, perPage
}

func coalesceStrings(preferred, fallback []string) []string {
	if len(preferred) > 0 {
		return preferred
	}
	return fallback
}

func coalesceInts(preferred, fallback []int) []int {
	if len(preferred) > 0 {
		return preferred
	}
	return fallback
}

func coalesceString(preferred, fallback, hardDefault string) string {
	if strings.TrimSpace(preferred) != "" {
		return strings.TrimSpace(preferred)
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return hardDefault
}

func envIntWithBounds(name string, fallback, min, max int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func envFloatWithBounds(name string, fallback, min, max float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func envCSVStrings(name string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func envCSVInts(name string, fallback []int) []int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func mineaUserAgent() string {
	v := strings.TrimSpace(os.Getenv("MINEA_USER_AGENT"))
	if v == "" {
		return defaultMineaGraphQLUserAgent
	}
	return v
}

func mineaOrigin() string {
	v := strings.TrimSpace(os.Getenv("MINEA_ORIGIN"))
	if v == "" {
		return defaultMineaGraphQLOrigin
	}
	return v
}

func mineaCreditCostPerSearch() float64 {
	return envFloatWithBounds("MINEA_CREDIT_COST_PER_SEARCH", 20, 0, 10000)
}

func mineaMinSafeCredits() float64 {
	return envFloatWithBounds("MINEA_MIN_SAFE_CREDITS", 100, 0, 1000000)
}

func mineaAlertCredits() float64 {
	return envFloatWithBounds("MINEA_ALERT_CREDITS", 200, 0, 1000000)
}

func mineaMaxCreditsPerSession() float64 {
	return envFloatWithBounds("MINEA_MAX_CREDITS_PER_SESSION", 200, 0, 1000000)
}

func extractItemsRecursive(v interface{}) []map[string]interface{} {
	var candidates [][]map[string]interface{}
	collectItemArrays(v, &candidates)
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	bestScore := scoreItemArray(best)
	for _, c := range candidates[1:] {
		if score := scoreItemArray(c); score > bestScore {
			best = c
			bestScore = score
		}
	}
	return best
}

func collectItemArrays(v interface{}, out *[][]map[string]interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		if arr := asArray(t["items"]); len(arr) > 0 {
			*out = append(*out, arr)
		}
		for _, val := range t {
			collectItemArrays(val, out)
		}
	case []interface{}:
		for _, item := range t {
			collectItemArrays(item, out)
		}
	}
}

func scoreItemArray(items []map[string]interface{}) int {
	if len(items) == 0 {
		return 0
	}
	score := 0
	for _, item := range items {
		score += len(item)
		if getString(item, "name", "product_name", "title", "ad_title", "productName") != "" {
			score += 100
		}
	}
	return score
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
		if pc.AdURL == "" {
			pc.AdURL = firstHTTPURL(getString(item, "ad_link", "creative_link", "link"))
		}
		if pc.ShopURL == "" {
			pc.ShopURL = shopURLFromShopField(getString(item, "shop_url", "shop_domain", "domain"))
		}
		if pc.LandingURL == "" {
			pc.LandingURL = firstHTTPURL(getString(item, "landing_page", "destination_url", "product_url"))
		}
		enrichCandidateFromRPCItem(item, &pc)
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

func firstHTTPURL(values ...string) string {
	for _, s := range values {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		low := strings.ToLower(s)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			return s
		}
	}
	return ""
}

// shopURLFromShopField turns a bare hostname (e.g. myshop.myshopify.com) into https URL; returns "" if not URL-like.
func shopURLFromShopField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	low := strings.ToLower(raw)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return raw
	}
	if !strings.Contains(raw, ".") {
		return ""
	}
	host := strings.TrimPrefix(strings.TrimPrefix(raw, "http://"), "https://")
	return "https://" + host
}

func enrichCandidateFromRPCItem(item map[string]interface{}, pc *ProductCandidate) {
	if pc == nil {
		return
	}
	card := firstArrayObject(item, "ad_cards")
	brand := nestedMap(item, "brand")
	ad := nestedMap(item, "ad")
	preview := nestedMap(item, "preview")
	shop := nestedMap(item, "shop")

	if pc.Name == "" {
		pc.Name = getString(card, "title", "description")
		if pc.Name == "" {
			pc.Name = getString(brand, "name")
		}
	}
	if pc.ImageURL == "" {
		pc.ImageURL = getString(card, "image_url", "video_url")
		if pc.ImageURL == "" {
			pc.ImageURL = getString(brand, "logo_url")
		}
	}
	if pc.SupplierID == "" {
		pc.SupplierID = getString(shop, "id", "domain")
	}
	if pc.ActiveAdCount == 0 {
		pc.ActiveAdCount = getInt(brand, "active_ads")
	}
	if pc.EngagementScore == 0 {
		pc.EngagementScore = getFloat(ad, "reach", "spend")
	}
	if pc.FirstSeenDate.IsZero() {
		if t, ok := parseAnyTime(getString(preview, "published_date"), getString(ad, "start_date")); ok {
			pc.FirstSeenDate = t
		}
	}
	if len(pc.Platforms) == 0 {
		link := strings.ToLower(getString(ad, "link"))
		switch {
		case strings.Contains(link, "facebook.com"):
			pc.Platforms = []string{"meta"}
		case strings.Contains(link, "tiktok.com"):
			pc.Platforms = []string{"tiktok"}
		}
	}
	if pc.AdURL == "" {
		pc.AdURL = firstHTTPURL(
			getString(ad, "link", "permalink", "creative_url", "url"),
			getString(item, "link", "ad_link", "creative_link"),
			getString(card, "link", "cta_url"),
		)
	}
	if pc.ShopURL == "" {
		pc.ShopURL = shopURLFromShopField(getString(shop, "url", "domain", "shop_url", "myshopify_domain", "shopify_domain"))
	}
	if pc.LandingURL == "" {
		pc.LandingURL = firstHTTPURL(
			getString(item, "landing_page", "destination_url", "product_url", "page_url"),
			getString(ad, "destination_url", "landing_page"),
		)
	}
}

func nestedMap(m map[string]interface{}, key string) map[string]interface{} {
	out, _ := m[key].(map[string]interface{})
	return out
}

func firstArrayObject(m map[string]interface{}, key string) map[string]interface{} {
	raw, _ := m[key].([]interface{})
	if len(raw) == 0 {
		return nil
	}
	out, _ := raw[0].(map[string]interface{})
	return out
}

func parseAnyTime(values ...string) (time.Time, bool) {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return t, true
		}
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
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
