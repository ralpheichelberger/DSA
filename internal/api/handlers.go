package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/dropshipagent/agent/internal/agent"
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
