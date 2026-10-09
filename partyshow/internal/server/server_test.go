package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ralpheichelberger/partyshow/internal/generator"
	"github.com/ralpheichelberger/partyshow/internal/store"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

type fakeFetcher struct {
	page webtext.Page
	err  error
}

func (f fakeFetcher) Fetch(context.Context, string, int) (webtext.Page, error) { return f.page, f.err }

func newTest(t *testing.T, fetcher SiteFetcher) (*httptest.Server, *Server) {
	t.Helper()
	st, err := store.New(t.TempDir())
	require.NoError(t, err)
	static := fstest.MapFS{"index.html": {Data: []byte("<h1>home</h1>")}, "css/app.css": {Data: []byte("body{}")}}
	s := New(Config{BaseURL: "https://party.example", GenerationsPerHour: 2, GenerationsPerDay: 10},
		st, generator.Stub{}, fetcher, static, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func doJSON(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, _ := http.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Edit-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestStaticAndHeaders(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{})
	resp, err := http.Get(ts.URL + "/")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Contains(t, string(body), "home")
	assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "default-src 'self'")

	resp, err = http.Get(ts.URL + "/static/css/app.css")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCreateShowFlow(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{page: webtext.Page{Title: "Acme", Text: "Gegründet 1999"}})

	code, out := doJSON(t, "POST", ts.URL+"/api/shows", "", map[string]any{
		"company_name": "Acme", "website_url": "acme.example", "facts": []string{"Wir haben einen Bürohund"}, "safe_mode": true,
	})
	require.Equal(t, http.StatusCreated, code, out)
	id, token := out["id"].(string), out["edit_token"].(string)

	code, _ = doJSON(t, "GET", ts.URL+"/api/shows/"+id, "wrong", nil)
	assert.Equal(t, http.StatusForbidden, code)
	code, got := doJSON(t, "GET", ts.URL+"/api/shows/"+id, token, nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(15), got["max_players"])

	// Edit: change the title, keep the rounds.
	got["title"] = "Neue Show"
	code, upd := doJSON(t, "PUT", ts.URL+"/api/shows/"+id, token, got)
	require.Equal(t, http.StatusOK, code, upd)
	assert.Equal(t, "Neue Show", upd["title"])

	// Invalid edit is rejected.
	got["rounds"] = []any{}
	code, _ = doJSON(t, "PUT", ts.URL+"/api/shows/"+id, token, got)
	assert.Equal(t, http.StatusBadRequest, code)

	code, room := doJSON(t, "POST", ts.URL+"/api/shows/"+id+"/open", token, nil)
	require.Equal(t, http.StatusOK, code)
	assert.True(t, strings.HasPrefix(room["join_url"].(string), "https://party.example/play/"))

	resp, err := http.Get(ts.URL + "/qr/" + room["code"].(string) + ".png")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
}

func TestCreateValidationAndLimits(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{err: assert.AnError})
	code, _ := doJSON(t, "POST", ts.URL+"/api/shows", "", map[string]any{"company_name": ""})
	assert.Equal(t, http.StatusBadRequest, code)

	// Website unreadable and no facts.
	code, _ = doJSON(t, "POST", ts.URL+"/api/shows", "", map[string]any{"company_name": "Acme", "website_url": "acme.example"})
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	// The 422 above used one of the two generations this hour; the next one
	// succeeds and the one after is over the limit.
	body := map[string]any{"company_name": "Acme", "facts": []string{"a"}}
	code, _ = doJSON(t, "POST", ts.URL+"/api/shows", "", body)
	assert.Equal(t, http.StatusCreated, code)
	code, _ = doJSON(t, "POST", ts.URL+"/api/shows", "", body)
	assert.Equal(t, http.StatusTooManyRequests, code)
}

func TestCreateWarnsOnUnreadableSite(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{err: assert.AnError})
	code, out := doJSON(t, "POST", ts.URL+"/api/shows", "", map[string]any{"company_name": "Acme", "website_url": "acme.example", "facts": []string{"a"}})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, []any{"website_unreadable"}, out["warnings"])
}

// ---- live game over WebSockets ----

type wsClient struct {
	t  *testing.T
	ws *websocket.Conn
}

func dial(t *testing.T, ts *httptest.Server, path string) *wsClient {
	t.Helper()
	u := "ws" + strings.TrimPrefix(ts.URL, "http") + path
	ws, _, err := websocket.DefaultDialer.Dial(u, http.Header{"Origin": {ts.URL}})
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return &wsClient{t: t, ws: ws}
}

func (c *wsClient) send(v any) { require.NoError(c.t, c.ws.WriteJSON(v)) }

// until reads messages until one matches.
func (c *wsClient) until(match func(map[string]any) bool) map[string]any {
	c.t.Helper()
	_ = c.ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var m map[string]any
		require.NoError(c.t, c.ws.ReadJSON(&m))
		if match(m) {
			return m
		}
	}
}

func phaseIs(p string) func(map[string]any) bool {
	return func(m map[string]any) bool {
		st, ok := m["state"].(map[string]any)
		return m["type"] == "state" && ok && st["phase"] == p
	}
}

func TestLiveGame(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{})
	code, out := doJSON(t, "POST", ts.URL+"/api/demo?lang=en", "", nil)
	require.Equal(t, http.StatusCreated, code)
	id, token := out["id"].(string), out["edit_token"].(string)

	// Wrong host token is refused.
	bad := dial(t, ts, "/ws/host/"+id)
	bad.send(map[string]string{"type": "auth", "token": "nope"})
	m := bad.until(func(m map[string]any) bool { return true })
	assert.Equal(t, "error", m["type"])

	host := dial(t, ts, "/ws/host/"+id)
	host.send(map[string]string{"type": "auth", "token": token})
	room := host.until(func(m map[string]any) bool { return m["type"] == "room" })
	roomCode := room["code"].(string)
	host.until(phaseIs("lobby"))

	anna := dial(t, ts, "/ws/play/"+roomCode)
	anna.send(map[string]string{"type": "join", "name": "Anna"})
	joined := anna.until(func(m map[string]any) bool { return m["type"] == "joined" })
	assert.Equal(t, "Anna", joined["name"])
	anna.until(phaseIs("lobby"))

	ben := dial(t, ts, "/ws/play/"+strings.ToLower(roomCode))
	ben.send(map[string]string{"type": "join", "name": "Ben"})
	ben.until(func(m map[string]any) bool { return m["type"] == "joined" })

	host.send(map[string]string{"type": "next"}) // intro
	host.send(map[string]string{"type": "next"}) // first question
	hq := host.until(phaseIs("question"))
	q := hq["state"].(map[string]any)["question"].(map[string]any)
	assert.Equal(t, float64(-1), q["correct"], "host screen must not leak the answer early")

	pq := anna.until(phaseIs("question"))
	assert.NotEmpty(t, pq["state"].(map[string]any)["options"])
	assert.Equal(t, float64(-1), pq["state"].(map[string]any)["correct"])

	anna.send(map[string]any{"type": "answer", "option": 0})
	ben.until(phaseIs("question"))
	ben.send(map[string]any{"type": "answer", "option": 0})

	// Both answered: reveal arrives without waiting for the timer.
	rv := host.until(phaseIs("reveal"))
	counts := rv["state"].(map[string]any)["question"].(map[string]any)["counts"].([]any)
	total := 0.0
	for _, c := range counts {
		total += c.(float64)
	}
	assert.Equal(t, 2.0, total)

	// Rejoin after a dropped connection keeps the player.
	annaID, annaToken := joined["id"].(string), joined["token"].(string)
	anna.ws.Close()
	again := dial(t, ts, "/ws/play/"+roomCode)
	again.send(map[string]string{"type": "rejoin", "id": annaID, "token": annaToken})
	rj := again.until(func(m map[string]any) bool { return m["type"] == "joined" })
	assert.Equal(t, "Anna", rj["name"])

	// Unknown room.
	nobody := dial(t, ts, "/ws/play/ZZZZZ")
	m = nobody.until(func(m map[string]any) bool { return true })
	assert.Equal(t, "no_room", m["code"])
}

func TestRestartKeepsRunningShow(t *testing.T) {
	ts, _ := newTest(t, fakeFetcher{})
	_, out := doJSON(t, "POST", ts.URL+"/api/demo", "", nil)
	id, token := out["id"].(string), out["edit_token"].(string)
	_, r1 := doJSON(t, "POST", ts.URL+"/api/shows/"+id+"/open", token, nil)

	// Lobby and unchanged show: same room.
	_, r2 := doJSON(t, "POST", ts.URL+"/api/shows/"+id+"/restart", token, nil)
	assert.Equal(t, r1["code"], r2["code"])

	// Forced restart: new room.
	_, r3 := doJSON(t, "POST", ts.URL+"/api/shows/"+id+"/restart?force=1", token, nil)
	assert.NotEqual(t, r1["code"], r3["code"])
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	l := newLimiter(2, 3, func() time.Time { return now })
	assert.True(t, l.allow("a"))
	assert.True(t, l.allow("a"))
	assert.False(t, l.allow("a"), "per-IP limit")
	assert.True(t, l.allow("b"))
	assert.False(t, l.allow("c"), "daily limit")
	now = now.Add(24 * time.Hour)
	assert.True(t, l.allow("a"), "new day resets")
}

// TestManyPlayers checks a full-size party: 200 phones join, all answer, the reveal reaches everyone.
func TestManyPlayers(t *testing.T) {
	if testing.Short() {
		t.Skip("load test")
	}
	ts, s := newTest(t, fakeFetcher{})
	s.cfg.DefaultPlan = "medium" // 200 players
	_, out := doJSON(t, "POST", ts.URL+"/api/demo", "", nil)
	id, token := out["id"].(string), out["edit_token"].(string)

	host := dial(t, ts, "/ws/host/"+id)
	host.send(map[string]string{"type": "auth", "token": token})
	code := host.until(func(m map[string]any) bool { return m["type"] == "room" })["code"].(string)

	const n = 200
	players := make([]*wsClient, n)
	for i := range players {
		p := dial(t, ts, "/ws/play/"+code)
		p.send(map[string]string{"type": "join", "name": "P" + strconv.Itoa(i)})
		p.until(func(m map[string]any) bool { return m["type"] == "joined" })
		players[i] = p
	}
	host.until(func(m map[string]any) bool {
		st, ok := m["state"].(map[string]any)
		return ok && st["player_count"] == float64(n)
	})

	host.send(map[string]string{"type": "next"})
	host.send(map[string]string{"type": "next"})
	start := time.Now()
	for _, p := range players {
		p.until(phaseIs("question"))
		p.send(map[string]any{"type": "answer", "option": 0})
	}
	for _, p := range players {
		p.until(phaseIs("reveal"))
	}
	t.Logf("200 players answered and got the reveal in %v", time.Since(start))
}

func TestClientIP(t *testing.T) {
	s := &Server{cfg: Config{TrustProxy: true}}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 203.0.113.9")
	assert.Equal(t, "203.0.113.9", s.clientIP(r), "a client-supplied first entry is ignored")
	s.cfg.TrustProxy = false
	assert.Equal(t, "10.0.0.5", s.clientIP(r))
}
