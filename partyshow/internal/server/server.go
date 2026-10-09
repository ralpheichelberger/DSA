// Package server serves the web app, the JSON API and the live WebSockets.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/ralpheichelberger/partyshow/internal/game"
	"github.com/ralpheichelberger/partyshow/internal/generator"
	"github.com/ralpheichelberger/partyshow/internal/show"
	"github.com/ralpheichelberger/partyshow/internal/store"
	"github.com/ralpheichelberger/partyshow/internal/webtext"
)

// SiteFetcher reads a company website.
type SiteFetcher interface {
	Fetch(ctx context.Context, url string, maxChars int) (webtext.Page, error)
}

// Config holds the server's settings.
type Config struct {
	BaseURL            string    // public URL, used for join links and QR codes
	DefaultPlan        show.Plan // plan for new shows until payments exist
	GenerationsPerHour int       // per client IP
	GenerationsPerDay  int       // total
	TrustProxy         bool      // read client IP from X-Forwarded-For (behind Caddy)
	MaxSiteChars       int
}

// Server wires everything together.
type Server struct {
	cfg       Config
	store     *store.Store
	rooms     *game.Rooms
	hub       *hub
	gen       generator.Generator
	fetcher   SiteFetcher
	limit     *limiter
	demoLimit *limiter
	static    fs.FS
	log       *slog.Logger
	upgrader  websocket.Upgrader
	rng       *rand.Rand
}

// New creates a server. static holds the web files (index.html, js/, css/).
func New(cfg Config, st *store.Store, gen generator.Generator, fetcher SiteFetcher, static fs.FS, log *slog.Logger) *Server {
	if cfg.DefaultPlan == "" {
		cfg.DefaultPlan = show.PlanFree
	}
	if cfg.MaxSiteChars == 0 {
		cfg.MaxSiteChars = 8000
	}
	rooms := game.NewRooms(nil)
	s := &Server{
		cfg: cfg, store: st, rooms: rooms, hub: newHub(rooms), gen: gen, fetcher: fetcher,
		limit:     newLimiter(cfg.GenerationsPerHour, cfg.GenerationsPerDay, time.Now),
		demoLimit: newLimiter(20, 2000, time.Now),
		static:    static, log: log,
		upgrader: websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096},
		rng:      rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}
	return s
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, securityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Timeout(90 * time.Second))
		r.Post("/shows", s.createShow)
		r.Post("/demo", s.createDemo)
		r.Get("/shows/{id}", s.getShow)
		r.Put("/shows/{id}", s.updateShow)
		r.Post("/shows/{id}/open", s.openRoom)
		r.Post("/shows/{id}/restart", s.restartRoom)
		r.Get("/rooms/{code}", s.roomInfo)
	})
	r.Get("/qr/{code}.png", s.qr)
	r.Get("/ws/host/{id}", s.wsHost)
	r.Get("/ws/play/{code}", s.wsPlay)

	page := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { s.servePage(w, r, name) }
	}
	r.Get("/", page("index.html"))
	r.Get("/create", page("create.html"))
	r.Get("/edit/{id}", page("edit.html"))
	r.Get("/host/{id}", page("host.html"))
	r.Get("/play", page("play.html"))
	r.Get("/play/{code}", page("play.html"))
	r.Get("/impressum", page("impressum.html"))
	r.Get("/datenschutz", page("datenschutz.html"))
	fileServer := http.FileServer(http.FS(s.static))
	r.Get("/static/*", http.StripPrefix("/static/", fileServer).ServeHTTP)
	return r
}

// Janitor removes idle rooms and old shows until ctx ends.
func (s *Server) Janitor(ctx context.Context, retention time.Duration) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n := s.rooms.Sweep(12 * time.Hour); n > 0 {
			s.log.Info("closed idle rooms", "count", n)
		}
		if n, err := s.store.DeleteOlderThan(time.Now().Add(-retention)); err != nil {
			s.log.Error("cleanup failed", "err", err)
		} else if n > 0 {
			s.log.Info("deleted old shows", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) servePage(w http.ResponseWriter, _ *http.Request, name string) {
	data, err := fs.ReadFile(s.static, name)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// ---- API ----

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		// The last entry is the one our own proxy (Caddy) added; earlier entries
		// come from the client and could be faked to dodge the rate limit.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type createResponse struct {
	ID        string   `json:"id"`
	EditToken string   `json:"edit_token"`
	Warnings  []string `json:"warnings,omitempty"`
}

func (s *Server) createShow(w http.ResponseWriter, r *http.Request) {
	var in show.Intake
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	in.Normalize()
	if err := in.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.limit.allow(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many shows created, please try again later")
		return
	}

	var warnings []string
	var site webtext.Page
	if in.WebsiteURL != "" {
		p, err := s.fetcher.Fetch(r.Context(), in.WebsiteURL, s.cfg.MaxSiteChars)
		if err != nil {
			s.log.Warn("website fetch failed", "url", in.WebsiteURL, "err", err)
			warnings = append(warnings, "website_unreadable")
		} else {
			site = p
		}
	}
	if site.Text == "" && len(in.Facts) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "website could not be read; please add some facts")
		return
	}

	sh, err := s.gen.Generate(r.Context(), in, site)
	if err == nil {
		err = generator.Finalize(sh, in, s.rng)
	}
	if err != nil {
		s.log.Error("generation failed", "err", err)
		writeErr(w, http.StatusBadGateway, "the show could not be generated, please try again")
		return
	}
	sh.Intake = in
	sh.Plan = s.cfg.DefaultPlan
	if err := s.store.Create(sh); err != nil {
		s.log.Error("store failed", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not save the show")
		return
	}
	s.log.Info("show created", "id", sh.ID, "questions", sh.QuestionCount(), "lang", in.Language)
	writeJSON(w, http.StatusCreated, createResponse{ID: sh.ID, EditToken: sh.EditToken, Warnings: warnings})
}

// createDemo makes a show for a fictional company without AI, for the "try it" button.
func (s *Server) createDemo(w http.ResponseWriter, r *http.Request) {
	if !s.demoLimit.allow(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "too many demos, please try again later")
		return
	}
	lang := r.URL.Query().Get("lang")
	in := DemoIntake(lang)
	in.Normalize()
	sh, err := generator.Stub{}.Generate(r.Context(), in, webtext.Page{})
	if err == nil {
		err = generator.Finalize(sh, in, s.rng)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "demo failed")
		return
	}
	sh.Intake = in
	sh.Plan = s.cfg.DefaultPlan
	if err := s.store.Create(sh); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save the show")
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{ID: sh.ID, EditToken: sh.EditToken})
}

// DemoIntake is the fictional company used by the demo.
func DemoIntake(lang string) show.Intake {
	if lang == "en" {
		return show.Intake{CompanyName: "Sample Ltd", Language: "en", Occasion: "christmas", SafeMode: true, Facts: []string{
			"Our office dog Bruno has his own email address",
			"The coffee machine broke four times this year",
			"We moved into the new office in March",
			"Lisa from accounting brings cake every Friday",
		}}
	}
	return show.Intake{CompanyName: "Musterfirma GmbH", Language: "de", Occasion: "christmas", SafeMode: true, Facts: []string{
		"Unser Bürohund Bruno hat eine eigene E-Mail-Adresse",
		"Die Kaffeemaschine ist dieses Jahr viermal kaputt gegangen",
		"Wir sind im März ins neue Büro umgezogen",
		"Lisa aus der Buchhaltung bringt jeden Freitag Kuchen mit",
	}}
}

func editToken(r *http.Request) string { return r.Header.Get("X-Edit-Token") }

func (s *Server) authorized(w http.ResponseWriter, r *http.Request) (*show.Show, bool) {
	sh, err := s.store.Authorize(chi.URLParam(r, "id"), editToken(r))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "show not found")
	case errors.Is(err, store.ErrForbidden):
		writeErr(w, http.StatusForbidden, "this link is not valid")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "could not load the show")
	default:
		return sh, true
	}
	return nil, false
}

type showResponse struct {
	*show.Show
	MaxPlayers int `json:"max_players"`
}

func (s *Server) getShow(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.authorized(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, showResponse{Show: sh, MaxPlayers: sh.Plan.MaxPlayers()})
}

func (s *Server) updateShow(w http.ResponseWriter, r *http.Request) {
	var edited show.Show
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&edited); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	sh, err := s.store.UpdateContent(chi.URLParam(r, "id"), editToken(r), edited)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "show not found")
	case errors.Is(err, store.ErrForbidden):
		writeErr(w, http.StatusForbidden, "this link is not valid")
	case err != nil:
		writeErr(w, http.StatusBadRequest, err.Error())
	default:
		// A running room keeps its copy; restarting picks up the edits.
		writeJSON(w, http.StatusOK, showResponse{Show: sh, MaxPlayers: sh.Plan.MaxPlayers()})
	}
}

type roomResponse struct {
	Code    string `json:"code"`
	JoinURL string `json:"join_url"`
}

func (s *Server) openRoom(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.authorized(w, r)
	if !ok {
		return
	}
	room, err := s.rooms.Open(sh, s.hub.broadcast)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, roomResponse{Code: room.Code, JoinURL: s.joinURL(room.Code)})
}

func (s *Server) restartRoom(w http.ResponseWriter, r *http.Request) {
	sh, ok := s.authorized(w, r)
	if !ok {
		return
	}
	// Without force, a show in progress keeps running (the editor's "start" button
	// must not wipe a party that is already playing).
	// A lobby is only replaced when the show was edited, so joined phones stay in.
	if room, ok := s.rooms.Running(sh.ID); ok && r.URL.Query().Get("force") != "1" {
		p := room.Phase()
		inProgress := p != game.PhaseLobby && p != game.PhaseFinal
		unchanged := room.Show.UpdatedAt.Equal(sh.UpdatedAt)
		if inProgress || (p == game.PhaseLobby && unchanged) {
			writeJSON(w, http.StatusOK, roomResponse{Code: room.Code, JoinURL: s.joinURL(room.Code)})
			return
		}
	}
	s.rooms.Restart(sh.ID)
	room, err := s.rooms.Open(sh, s.hub.broadcast)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, roomResponse{Code: room.Code, JoinURL: s.joinURL(room.Code)})
}

func (s *Server) roomInfo(w http.ResponseWriter, r *http.Request) {
	room, err := s.rooms.Get(strings.ToUpper(chi.URLParam(r, "code")))
	if err != nil {
		writeErr(w, http.StatusNotFound, "no show with this code")
		return
	}
	hv := room.HostView()
	writeJSON(w, http.StatusOK, map[string]any{"title": hv.Title, "language": hv.Language, "phase": hv.Phase})
}

func (s *Server) joinURL(code string) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/play/" + code
}

func (s *Server) qr(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if _, err := s.rooms.Get(code); err != nil {
		http.NotFound(w, r)
		return
	}
	png, err := qrcode.Encode(s.joinURL(code), qrcode.Medium, 512)
	if err != nil {
		http.Error(w, "qr failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(png)
}

// ---- WebSockets ----

type wsIn struct {
	Type   string `json:"type"`
	Token  string `json:"token,omitempty"`
	Name   string `json:"name,omitempty"`
	ID     string `json:"id,omitempty"`
	Option int    `json:"option,omitempty"`
}

func readJSON(ws *websocket.Conn, v any) error {
	_, data, err := ws.ReadMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// readMsg returns the next message. Malformed JSON yields an empty message
// (ignored by callers); only connection errors are returned.
func readMsg(ws *websocket.Conn) (wsIn, error) {
	var m wsIn
	_, data, err := ws.ReadMessage()
	if err != nil {
		return m, err
	}
	if json.Unmarshal(data, &m) != nil {
		return wsIn{}, nil
	}
	return m, nil
}

func prepare(ws *websocket.Conn) {
	ws.SetReadLimit(maxMsgSize)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(pongWait)) })
}

func sendNow(ws *websocket.Conn, v any) {
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	_ = ws.WriteJSON(v)
}

// wsHost: the first message must be {"type":"auth","token":...}; then "next", "kick".
func (s *Server) wsHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	prepare(ws)
	var auth wsIn
	if err := readJSON(ws, &auth); err != nil || auth.Type != "auth" {
		_ = ws.Close()
		return
	}
	sh, err := s.store.Authorize(id, auth.Token)
	if err != nil {
		sendNow(ws, map[string]string{"type": "error", "message": "invalid link"})
		_ = ws.Close()
		return
	}
	room, err := s.rooms.Open(sh, s.hub.broadcast)
	if err != nil {
		sendNow(ws, map[string]string{"type": "error", "message": err.Error()})
		_ = ws.Close()
		return
	}
	c := newConn(ws)
	c.code, c.host = room.Code, true
	s.hub.add(c)
	go c.writeLoop()
	c.queue(mustJSON(map[string]any{"type": "room", "code": room.Code, "join_url": s.joinURL(room.Code)}))
	s.hub.broadcast(room.Code)
	defer func() {
		s.hub.remove(c)
		c.close()
	}()
	for {
		m, err := readMsg(ws)
		if err != nil {
			return
		}
		switch m.Type {
		case "next":
			room.Next()
		case "kick":
			room.Kick(m.ID)
		default:
			continue
		}
		s.hub.broadcast(room.Code)
	}
}

// wsPlay: the first message is {"type":"join","name":...} or {"type":"rejoin","id":...,"token":...};
// then {"type":"answer","option":n}.
func (s *Server) wsPlay(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	prepare(ws)
	room, err := s.rooms.Get(code)
	if err != nil {
		sendNow(ws, map[string]string{"type": "error", "code": "no_room", "message": err.Error()})
		_ = ws.Close()
		return
	}
	var hello wsIn
	if err := readJSON(ws, &hello); err != nil {
		_ = ws.Close()
		return
	}
	var p *game.Player
	switch hello.Type {
	case "join":
		p, err = room.Join(hello.Name)
	case "rejoin":
		p, err = room.Rejoin(hello.ID, hello.Token)
	default:
		err = errors.New("expected join")
	}
	if err != nil {
		sendNow(ws, map[string]string{"type": "error", "code": errCode(err), "message": err.Error()})
		_ = ws.Close()
		return
	}
	c := newConn(ws)
	c.code, c.playerID = code, p.ID
	s.hub.add(c)
	go c.writeLoop()
	c.queue(mustJSON(map[string]any{"type": "joined", "id": p.ID, "token": p.Token, "name": p.Name}))
	s.hub.broadcast(code)
	defer func() {
		s.hub.remove(c)
		c.close()
		room.SetConnected(p.ID, false)
		s.hub.broadcast(code)
	}()
	for {
		m, err := readMsg(ws)
		if err != nil {
			return
		}
		if m.Type != "answer" {
			continue
		}
		if err := room.Answer(p.ID, m.Option); err != nil {
			c.queue(mustJSON(map[string]string{"type": "error", "code": errCode(err), "message": err.Error()}))
			continue
		}
		s.hub.broadcast(code)
	}
}

func errCode(err error) string {
	switch {
	case errors.Is(err, game.ErrFull):
		return "full"
	case errors.Is(err, game.ErrBadName):
		return "bad_name"
	case errors.Is(err, game.ErrShowEnded):
		return "ended"
	case errors.Is(err, game.ErrBadRejoin):
		return "bad_rejoin"
	case errors.Is(err, game.ErrAnswered):
		return "answered"
	case errors.Is(err, game.ErrNotAnswer):
		return "not_now"
	default:
		return "error"
	}
}
