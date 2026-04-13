package minea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

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
	_, err := s.graphql(context.Background(), "ANY", "query", map[string]interface{}{})
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
