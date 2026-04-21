package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dropshipagent/agent/config"
	"github.com/dropshipagent/agent/internal/agent"
	"github.com/dropshipagent/agent/internal/integrations/openai"
	"github.com/dropshipagent/agent/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const serverVersion = "0.1.0"

// StoreReader is the subset of store operations used by the HTTP API.
// *store.Store satisfies this interface.
type StoreReader interface {
	GetAllProducts() ([]store.ProductTest, error)
	GetActiveCampaigns() ([]store.CampaignResult, error)
	GetAllLessons() ([]store.LearnedLesson, error)
	BuildMemoryContext() (string, error)
	GetProductTest(id string) (*store.ProductTest, error)
	SaveProductTest(pt store.ProductTest) error
}

// AgentConn is the subset of agent operations used by the HTTP and WebSocket layer.
// *agent.Agent satisfies this interface.
type AgentConn interface {
	ApprovalChan() chan<- agent.ApprovalResponse
	Outbox() <-chan agent.Message
}

// Compile-time checks.
var (
	_ StoreReader = (*store.Store)(nil)
	_ AgentConn   = (*agent.Agent)(nil)
)

type Server struct {
	agent    AgentConn
	store    StoreReader
	hub      *Hub
	router   *chi.Mux
	logger   *zap.Logger
	cfg      *config.Config
	reasoner openai.Reasoner
}

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				_ = c.SetWriteDeadline(time.Now().Add(8 * time.Second))
				if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
					_ = c.Close()
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
		// Drop if broadcast queue is full to avoid blocking the agent.
	}
}

// New constructs the API server. store is typically *store.Store; StoreReader allows mocks in tests.
// reasoner may be nil; POST /api/chat will return an error in that case.
func New(cfg *config.Config, agent AgentConn, store StoreReader, reasoner openai.Reasoner, logger *zap.Logger) *Server {
	if cfg == nil {
		cfg = &config.Config{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	s := &Server{
		agent:    agent,
		store:    store,
		hub:      NewHub(),
		router:   chi.NewRouter(),
		logger:   logger,
		cfg:      cfg,
		reasoner: reasoner,
	}
	s.mountRoutes()
	go s.hub.Run()
	return s
}

func (s *Server) mountRoutes() {
	s.router.Get("/health", s.handleHealth)
	s.router.Get("/api/products", s.handleGetProducts)
	s.router.Get("/api/campaigns", s.handleGetCampaigns)
	s.router.Get("/api/lessons", s.handleGetLessons)
	s.router.Get("/api/minea/scraped", s.handleGetMineaScraped)
	s.router.Post("/api/minea/search", s.handleMineaSearch)
	s.router.Post("/api/approve", s.handleApprove)
	s.router.Post("/api/chat", s.handleChat)
	s.router.Get("/ws", s.handleWebSocket)
}

// Start listens on port (e.g. ":8080" or "127.0.0.1:0") and serves until ctx is cancelled.
func (s *Server) Start(ctx context.Context, port string) error {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	return s.Serve(ctx, ln)
}

// Serve accepts connections on ln until ctx is cancelled (used by smoke tests with :0 listeners).
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go s.runOutboxPump(ctx)

	srv := &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func (s *Server) runOutboxPump(ctx context.Context) {
	if s.agent == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-s.agent.Outbox():
			if !ok {
				return
			}
			b, err := json.Marshal(msg)
			if err != nil {
				s.logger.Warn("outbox marshal failed", zap.Error(err))
				continue
			}
			s.hub.Broadcast(b)
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
