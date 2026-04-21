package minea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type gqlReq struct {
	OperationName string                 `json:"operationName"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
}

func TestParseGraphQLProducts_SuccessRadarShape(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	data := map[string]interface{}{
		"getSuccessRadar": []interface{}{
			map[string]interface{}{"name": "Pet GPS Collar", "price": 49.0, "platforms": []interface{}{"facebook"}, "ad_count": float64(12)},
		},
	}
	got, err := s.parseGraphQLProducts(data, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.NotEmpty(t, got[0].Name)
}

func TestParseGraphQLProducts_SearchShape(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	data := map[string]interface{}{
		"searchProducts": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"product_name": "Wireless Charging Dock", "estimated_price": 39.9, "active_ads": float64(8), "platform": "tiktok"},
			},
		},
	}
	got, err := s.parseGraphQLProducts(data, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Wireless Charging Dock", got[0].Name)
}

func TestParseGraphQLProducts_FlatArray(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	data := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"title": "LED Desk Lamp", "selling_price": 35.0},
		},
	}
	got, err := s.parseGraphQLProducts(data, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "LED Desk Lamp", got[0].Name)
}

func TestParseGraphQLProducts_RPCShape(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	data := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"id": "rpc-1",
				"ad_cards": []interface{}{
					map[string]interface{}{
						"title":     "Chef Set - 50% off",
						"image_url": "https://cdn.example.com/img.webp",
					},
				},
				"brand": map[string]interface{}{
					"name":       "Montessori Toddlers",
					"active_ads": 87.0,
				},
				"preview": map[string]interface{}{
					"published_date": "2026-04-12T07:00:00.000Z",
				},
				"shop": map[string]interface{}{
					"id": "shop-1",
				},
				"ad": map[string]interface{}{
					"reach": 123.0,
				},
			},
		},
	}
	got, err := s.parseGraphQLProducts(data, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Chef Set - 50% off", got[0].Name)
	assert.Equal(t, "shop-1", got[0].SupplierID)
	assert.Equal(t, "https://cdn.example.com/img.webp", got[0].ImageURL)
	assert.Equal(t, 87, got[0].ActiveAdCount)
	assert.Equal(t, 123.0, got[0].EngagementScore)
}

func TestParseGraphQLProducts_EmptyResponse(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	got, err := s.parseGraphQLProducts(map[string]interface{}{}, 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestCanAffordSearch_BlocksWhenLow(t *testing.T) {
	s := scraperWithCreditsServer(t, 15.0)
	err := s.canAffordSearch(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

func TestCanAffordSearch_AllowsWhenSufficient(t *testing.T) {
	s := scraperWithCreditsServer(t, 500.0)
	require.NoError(t, s.canAffordSearch(context.Background()))
}

func TestCanAffordSearch_AlertsWhenBelowThreshold(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)
	s := scraperWithCreditsServer(t, 180.0)
	s.logger = logger

	require.NoError(t, s.canAffordSearch(context.Background()))
	found := false
	for _, e := range logs.All() {
		if e.Level == zap.WarnLevel && (contains(e.Message, "low")) {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestGetCredits_ParsesResponse(t *testing.T) {
	s := scraperWithCreditsServer(t, 93810.0)
	b, err := s.GetCredits(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 93810.0, b.Credits)
	assert.Equal(t, 6790.0, b.TotalCredits)
	assert.Equal(t, time.Date(2026, 6, 5, 8, 52, 53, 102000000, time.UTC), b.CreditsRefillAt)
}

func TestLoadSession_ReturnsFalseWhenMissing(t *testing.T) {
	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "missing.json"), zap.NewNop())
	assert.False(t, s.loadSession())
	assert.Empty(t, s.authToken)
}

func TestLoadSession_ReturnsFalseWhenStale(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	sf := sessionFile{AuthToken: "tok", UserID: "uid", SavedAt: time.Now().AddDate(0, 0, -31)}
	raw, _ := json.Marshal(sf)
	require.NoError(t, os.WriteFile(p, raw, 0o600))
	s := NewScraper("a", "b", p, zap.NewNop())
	assert.False(t, s.loadSession())
}

func TestLoadSession_RestoresToken(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	sf := sessionFile{AuthToken: "tok", UserID: "uid", SavedAt: time.Now()}
	raw, _ := json.Marshal(sf)
	require.NoError(t, os.WriteFile(p, raw, 0o600))
	s := NewScraper("a", "b", p, zap.NewNop())
	assert.True(t, s.loadSession())
	assert.Equal(t, "tok", s.authToken)
	assert.Equal(t, "uid", s.userID)
}

func TestSaveAndLoadSessionRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	s := NewScraper("a", "b", p, zap.NewNop())
	s.authToken = "token-1"
	s.userID = "user-1"
	require.NoError(t, s.saveSession())
	s.authToken, s.userID = "", ""
	assert.True(t, s.loadSession())
	assert.Equal(t, "token-1", s.authToken)
	assert.Equal(t, "user-1", s.userID)
}

func TestDeriveShopifyStore(t *testing.T) {
	testCases := map[string]string{
		"Pet GPS Collar":         "pets",
		"Smart Dog Feeder":       "pets",
		"Cat Toy Laser":          "pets",
		"Wireless Charging Dock": "tech",
		"LED Desk Lamp":          "tech",
		"Ergonomic Mouse Pad":    "tech",
	}
	for in, want := range testCases {
		assert.Equal(t, want, deriveShopifyStore(in, ""))
	}
}

func TestSetMineaGraphQLAuthHeader_AppSyncRawJWT(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://x.appsync-api.eu-west-1.amazonaws.com/graphql", strings.NewReader("{}"))
	require.NoError(t, err)
	setMineaGraphQLAuthHeader(req, req.URL.String(), "eyJ.h.p")
	assert.Equal(t, "eyJ.h.p", req.Header.Get("Authorization"))
}

func TestSetMineaGraphQLAuthHeader_LegacyBearer(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://app.minea.com/graphql", strings.NewReader("{}"))
	require.NoError(t, err)
	setMineaGraphQLAuthHeader(req, req.URL.String(), "tok")
	assert.Equal(t, "Bearer tok", req.Header.Get("Authorization"))
}

func TestSetMineaGraphQLAuthHeader_StripsBearerForAppSync(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://x.appsync-api.eu-west-1.amazonaws.com/graphql", strings.NewReader("{}"))
	require.NoError(t, err)
	setMineaGraphQLAuthHeader(req, req.URL.String(), "Bearer inner")
	assert.Equal(t, "inner", req.Header.Get("Authorization"))
}

func TestCookieAppliesToHost(t *testing.T) {
	tests := []struct {
		domain string
		host   string
		want   bool
	}{
		{".minea.com", "app.minea.com", true},
		{"minea.com", "app.minea.com", true},
		{"app.minea.com", "app.minea.com", true},
		{".app.minea.com", "app.minea.com", true},
		{".tiktok.com", "app.minea.com", false},
		{"", "app.minea.com", false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, cookieAppliesToHost(tc.domain, tc.host), "domain=%q host=%q", tc.domain, tc.host)
	}
}

func TestMineaCookieHeader(t *testing.T) {
	host := "app.minea.com"
	cookies := []savedCookie{
		{Name: "a", Value: "1", Domain: ".minea.com"},
		{Name: "b", Value: "2", Domain: ".tiktok.com"},
		{Name: "a", Value: "3", Domain: "app.minea.com"}, // last wins for name a
	}
	got := mineaCookieHeader(cookies, host)
	assert.Contains(t, got, "a=3")
	assert.NotContains(t, got, "b=")
	assert.NotContains(t, got, "a=1")
}

func TestCalculateCreditUsage(t *testing.T) {
	tests := []struct {
		name     string
		before   *float64
		after    float64
		wantUsed float64
		wantOK   bool
	}{
		{name: "nil before", before: nil, after: 100, wantUsed: 0, wantOK: false},
		{name: "no delta", before: ptrFloat(100), after: 100, wantUsed: 0, wantOK: false},
		{name: "increase", before: ptrFloat(100), after: 110, wantUsed: 0, wantOK: false},
		{name: "decrease", before: ptrFloat(100), after: 80, wantUsed: 20, wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotUsed, gotOK := calculateCreditUsage(tc.before, tc.after)
			assert.Equal(t, tc.wantOK, gotOK)
			assert.Equal(t, tc.wantUsed, gotUsed)
		})
	}
}

func TestExtractItemsRecursive_PrefersRichestItemsArray(t *testing.T) {
	raw := map[string]interface{}{
		"result": map[string]interface{}{
			"data": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "a1"},
					map[string]interface{}{"id": "a2"},
				},
				"nested": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{"id": "p1", "product_name": "Portable Blender", "active_ads": 14.0},
					},
				},
			},
		},
	}
	items := extractItemsRecursive(raw)
	require.Len(t, items, 1)
	assert.Equal(t, "Portable Blender", getString(items[0], "product_name"))
}

func TestGraphQL_DoesNotFollowRedirectToHTML(t *testing.T) {
	var htmlHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/en/register")
		w.WriteHeader(http.StatusTemporaryRedirect)
		_, _ = w.Write([]byte(`{"redirect":"/en/register","status":"307"}`))
	})
	mux.HandleFunc("/en/register", func(w http.ResponseWriter, r *http.Request) {
		htmlHits++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "sess.json"), zap.NewNop())
	s.graphqlURL = srv.URL + "/graphql"
	_, err := s.graphql(context.Background(), "T", `query { x }`, map[string]interface{}{}, false)
	require.Error(t, err)
	assert.Zero(t, htmlHits, "client must not follow redirect to an HTML page")
}

func TestGraphQL_ReturnsErrorWhenGraphQLErrorsPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer srv.Close()

	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "sess.json"), zap.NewNop())
	s.graphqlURL = srv.URL

	_, err := s.graphql(context.Background(), "GET_TRENDING_PRODUCTS", `query { x }`, map[string]interface{}{}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "graphql errors")
	assert.Contains(t, err.Error(), "forbidden")
}

func TestGetTrendingProducts_FallsBackWhenSuccessRadarUnavailable(t *testing.T) {
	t.Setenv("MINEA_PAGES", "1")
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rpc/meta-ads-graphql/searchAds" {
			seen = append(seen, "RPC_SEARCH_ADS")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]interface{}{
					"data": map[string]interface{}{
						"items": []map[string]interface{}{
							{"product_name": "Pet Grooming Glove", "estimated_price": 24.9, "active_ads": 11.0},
						},
					},
				},
			})
			return
		}
		var req gqlReq
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seen = append(seen, req.OperationName)
		switch req.OperationName {
		case "GET_PROFILE":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"getUser": map[string]interface{}{
						"id":              "user-1",
						"credits":         93810.0,
						"totalCredits":    6790.0,
						"creditsRefillAt": "2026-06-05T08:52:53.102Z",
					},
				},
			})
		case "GET_TRENDING_PRODUCTS":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{
					{"message": "Validation error of type FieldUndefined: Field 'getSuccessRadar' in type 'Query' is undefined @ 'getSuccessRadar'"},
				},
			})
		case "SEARCH_PRODUCTS":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{
					{"message": "Validation error of type UnknownArgument: Unknown field argument niche @ 'searchProducts'"},
				},
			})
		default:
			t.Fatalf("unexpected operation: %s", req.OperationName)
		}
	}))
	defer srv.Close()

	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "sess.json"), zap.NewNop())
	s.authToken = "token"
	s.userID = "user-1"
	s.graphqlURL = srv.URL
	s.cookies = []savedCookie{{Name: "sid", Value: "abc", Domain: "app.minea.com"}}

	products, err := s.GetTrendingProducts(context.Background(), "", "US", 10)
	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, "Pet Grooming Glove", products[0].Name)
	assert.Contains(t, seen, "GET_TRENDING_PRODUCTS")
	assert.Contains(t, seen, "SEARCH_PRODUCTS")
	assert.Contains(t, seen, "RPC_SEARCH_ADS")
}

func TestMineaRPCSearchPagination(t *testing.T) {
	t.Setenv("MINEA_PAGE", "2")
	t.Setenv("MINEA_PAGES", "5")
	start, pages, perPage := mineaRPCSearchPagination(AdsSearchOptions{Limit: 15})
	assert.Equal(t, 2, start)
	assert.Equal(t, 5, pages)
	assert.Equal(t, 15, perPage)
}

func TestMineaRPCSearchPayload_QueryDefaultsQSearchTargetsToAdCopy(t *testing.T) {
	p := mineaRPCSearchPayload(AdsSearchOptions{Query: "pets"}, 1, 20)
	j := p["json"].(map[string]interface{})
	assert.Equal(t, "pets", j["query"])
	assert.Equal(t, "adCopy", j["q_search_targets"])
}

func TestMineaRPCSearchPayload_QueryRespectsQSearchTargets(t *testing.T) {
	p := mineaRPCSearchPayload(AdsSearchOptions{Query: "pets", QSearchTargets: "brandName"}, 1, 20)
	j := p["json"].(map[string]interface{})
	assert.Equal(t, "pets", j["query"])
	assert.Equal(t, "brandName", j["q_search_targets"])
}

func TestMineaRPCSearchPayload_QSearchTargetsOnly(t *testing.T) {
	p := mineaRPCSearchPayload(AdsSearchOptions{QSearchTargets: "adCopy"}, 1, 20)
	j := p["json"].(map[string]interface{})
	_, hasQuery := j["query"]
	assert.False(t, hasQuery)
	assert.Equal(t, "adCopy", j["q_search_targets"])
}

func TestMineaRPCSearchPayload_NoQueryNoTargets(t *testing.T) {
	p := mineaRPCSearchPayload(AdsSearchOptions{}, 1, 20)
	j := p["json"].(map[string]interface{})
	_, hasQuery := j["query"]
	_, hasQT := j["q_search_targets"]
	assert.False(t, hasQuery)
	assert.False(t, hasQT)
}

func TestSaveAndLoadSessionRoundTrip_IncludesCookies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	s := NewScraper("a", "b", p, zap.NewNop())
	s.authToken = "token-1"
	s.userID = "user-1"
	s.cookies = []savedCookie{{Name: "sid", Value: "abc", Domain: "app.minea.com", Path: "/"}}
	require.NoError(t, s.saveSession())
	s.authToken, s.userID = "", ""
	s.cookies = nil
	assert.True(t, s.loadSession())
	assert.Equal(t, "token-1", s.authToken)
	assert.Equal(t, "user-1", s.userID)
	require.Len(t, s.cookies, 1)
	assert.Equal(t, "sid", s.cookies[0].Name)
	assert.Equal(t, "abc", s.cookies[0].Value)
}

func TestGraphQL_307RedirectClearsSessionWhenInvalidateTrue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"auth_token":"x","user_id":"u","saved_at":"2026-01-01T00:00:00Z"}`), 0o600))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/en/register")
		w.WriteHeader(http.StatusTemporaryRedirect)
		_, _ = w.Write([]byte(`{"redirect":"/en/register/quickview?from=%2Fgraphql"}`))
	}))
	defer srv.Close()

	s := NewScraper("a", "b", p, zap.NewNop())
	s.authToken = "tok"
	s.userID = "u"
	s.graphqlURL = srv.URL
	_, err := s.graphql(context.Background(), "X", "query {}", map[string]interface{}{}, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthExpired)
	_, statErr := os.Stat(p)
	assert.Error(t, statErr)
}

func TestErrAuthExpired_TriggersOnHTTP401(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"auth_token":"x","user_id":"u","saved_at":"2026-01-01T00:00:00Z"}`), 0o600))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	s := NewScraper("a", "b", p, zap.NewNop())
	s.authToken = "x"
	s.graphqlURL = srv.URL
	_, err := s.graphql(context.Background(), "ANY", "query", map[string]interface{}{}, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthExpired)
	_, statErr := os.Stat(p)
	assert.Error(t, statErr)
}

func scraperWithCreditsServer(t *testing.T, credits float64) *Scraper {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"getUser": map[string]interface{}{
					"id":              "user-1",
					"credits":         credits,
					"totalCredits":    6790.0,
					"creditsRefillAt": "2026-06-05T08:52:53.102Z",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	s := NewScraper("a", "b", filepath.Join(t.TempDir(), "session.json"), zap.NewNop())
	s.authToken = "token"
	s.userID = "user-1"
	s.graphqlURL = srv.URL
	return s
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (stringIndex(s, sub) >= 0) }
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func ptrFloat(v float64) *float64 { return &v }

func TestEnrichCandidateFromRPCItem_URLs(t *testing.T) {
	item := map[string]interface{}{
		"ad": map[string]interface{}{
			"link":             "https://www.facebook.com/ads/library/?id=999",
			"destination_url":  "https://shop.example/p/widget",
			"landing_page":     "",
		},
		"shop": map[string]interface{}{
			"domain": "cool-brand.myshopify.com",
		},
	}
	pc := &ProductCandidate{}
	enrichCandidateFromRPCItem(item, pc)
	assert.Equal(t, "https://www.facebook.com/ads/library/?id=999", pc.AdURL)
	assert.Equal(t, "https://cool-brand.myshopify.com", pc.ShopURL)
	assert.Equal(t, "https://shop.example/p/widget", pc.LandingURL)
}

func TestShopURLFromShopField(t *testing.T) {
	assert.Equal(t, "https://a.myshopify.com", shopURLFromShopField("a.myshopify.com"))
	assert.Equal(t, "https://x.com", shopURLFromShopField("https://x.com"))
	assert.Equal(t, "", shopURLFromShopField("uuid-without-dots"))
	assert.Equal(t, "", shopURLFromShopField(""))
}
