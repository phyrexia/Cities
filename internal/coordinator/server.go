package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cities/game/internal/ai"
	"github.com/cities/game/internal/city"
	"github.com/cities/game/internal/trade"
	"github.com/gorilla/websocket"
)

// Server is the main coordinator server.
type Server struct {
	registry    *Registry
	ticker      *Ticker
	broadcaster *Broadcaster
	tradeEngine *trade.Engine
	upgrader    websocket.Upgrader
}

// NewServer creates a fully wired coordinator server.
func NewServer() *Server {
	broadcaster := NewBroadcaster()
	registry := NewRegistry()
	tradeEngine := trade.New()
	claudeClient := ai.NewClient()
	generator := ai.NewGenerator(claudeClient)

	interval := DefaultHeartbeatInterval
	if envInterval := os.Getenv("HEARTBEAT_INTERVAL"); envInterval != "" {
		if d, err := time.ParseDuration(envInterval); err == nil {
			interval = d
		}
	}

	ticker := NewTicker(interval, registry, generator, tradeEngine, broadcaster)

	return &Server{
		registry:    registry,
		ticker:      ticker,
		broadcaster: broadcaster,
		tradeEngine: tradeEngine,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start begins the game loop and HTTP server.
func (s *Server) Start(ctx context.Context, addr string) error {
	s.ticker.Start(ctx)

	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWebSocket)

	// REST API
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/game/state", s.handleGameState)
	mux.HandleFunc("/api/decision", s.handleDecision)
	mux.HandleFunc("/api/trade", s.handleTrade)
	mux.HandleFunc("/api/trade/orders", s.handleTradeOrders)
	mux.HandleFunc("/api/cities", s.handleCities)
	mux.HandleFunc("/api/debug/heartbeat", s.handleDebugHeartbeat)
	mux.HandleFunc("/api/event/decision", s.handleEventDecision)

	// Static web client
	mux.Handle("/", http.FileServer(http.Dir("./web")))

	log.Printf("[Server] Listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// ─── WebSocket Handler ──────────────────────────────────────────────────────

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	cityID := r.URL.Query().Get("city_id")
	if playerID == "" || cityID == "" {
		http.Error(w, "player_id and city_id required", http.StatusBadRequest)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &WSClient{
		PlayerID: playerID,
		CityID:   cityID,
		Send:     make(chan []byte, 256),
	}
	s.broadcaster.Register(client)
	defer s.broadcaster.Unregister(playerID)

	// Send initial state
	if c, err := s.registry.GetCity(cityID); err == nil {
		result := HeartbeatResult{
			CityID: cityID,
			City:   c,
		}
		msg := WSMessage{Type: MsgCityUpdate, Payload: result}
		if data, err := json.Marshal(msg); err == nil {
			client.Send <- data
		}
	}

	// Send active heartbeat if exists
	if hb := s.ticker.GetLatestHeartbeat(cityID); hb != nil {
		if time.Now().Before(hb.Deadline) {
			msg := WSMessage{Type: MsgHeartbeat, Payload: hb}
			if data, err := json.Marshal(msg); err == nil {
				client.Send <- data
			}
		}
	}

	// Write pump
	go func() {
		defer conn.Close()
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[WS] Write error for %s: %v", playerID, err)
				return
			}
		}
	}()

	// Read pump (handle pings and incoming messages)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.registry.UpdatePlayerActivity(playerID)
		s.handleWSMessage(playerID, cityID, data)
	}
}

func (s *Server) handleWSMessage(playerID, cityID string, data []byte) {
	var msg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "decision":
		var dec struct {
			HeartbeatID  string `json:"heartbeat_id"`
			InitiativeID string `json:"initiative_id"`
		}
		if err := json.Unmarshal(msg.Payload, &dec); err != nil {
			return
		}
		_ = s.ticker.RecordDecision(Decision{
			HeartbeatID:  dec.HeartbeatID,
			CityID:       cityID,
			PlayerID:     playerID,
			InitiativeID: dec.InitiativeID,
			DecidedAt:    time.Now(),
		})
	}
}

// ─── REST Handlers ──────────────────────────────────────────────────────────

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PlayerName string `json:"player_name"`
		CityName   string `json:"city_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlayerName == "" || req.CityName == "" {
		http.Error(w, "player_name and city_name required", http.StatusBadRequest)
		return
	}

	player, city, err := s.registry.RegisterPlayer(req.PlayerName, req.CityName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"player_id":     player.ID,
		"city_id":       city.ID,
		"session_token": player.Token,
		"city":          city,
	})
}

func (s *Server) handleGameState(w http.ResponseWriter, r *http.Request) {
	cities := s.registry.AllCities()
	writeJSON(w, map[string]interface{}{
		"round":          s.ticker.CurrentRound(),
		"cities":         cities,
		"total_players":  s.registry.PlayerCount(),
		"connected":      s.broadcaster.ConnectedCount(),
		"next_heartbeat": s.ticker.NextHeartbeatAt(),
	})
}

func (s *Server) handleCities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.registry.AllCities())
}

func (s *Server) handleDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req Decision
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.DecidedAt = time.Now()
	if err := s.ticker.RecordDecision(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"accepted": true})
}

func (s *Server) handleTrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FromCityID   string `json:"from_city_id"`
		ToCityID     string `json:"to_city_id"`
		ProductID    string `json:"product_id"`
		Quantity     int    `json:"quantity"`
		PricePerUnit int64  `json:"price_per_unit"`
		Direction    string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order, err := s.tradeEngine.PlaceOrder(
		req.FromCityID, req.ToCityID, req.ProductID,
		req.Quantity, req.PricePerUnit, trade.Direction(req.Direction),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"accepted": true,
		"order_id": order.ID,
	})
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orders := s.tradeEngine.ListPendingOrders()

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(orders)
	if err != nil {
		log.Printf("[Trade] Error encoding orders: %v", err)
		return
	}
	w.Write(data)
}

func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CityID   string `json:"city_id"`
		Product  string `json:"product"`
		Side     string `json:"side"` // "buy" or "sell"
		Quantity int    `json:"quantity"`
		Price    int    `json:"price"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid request body"}`))
		return
	}

	// Place order in engine
	placedOrder, err := s.tradeEngine.PlaceOrder(
		req.CityID, "", req.Product,
		req.Quantity, int64(req.Price), trade.Direction(req.Side),
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data, _ := json.Marshal(placedOrder)
	w.Write(data)
}

func (s *Server) handleTradeOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleListOrders(w, r)
	} else if r.Method == http.MethodPost {
		s.handlePlaceOrder(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDebugHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	go s.ticker.TriggerManual(r.Context())
	writeJSON(w, map[string]string{"status": "heartbeat triggered"})
}

func (s *Server) handleEventDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		CityID   string `json:"city_id"`
		EventID  string `json:"event_id"`
		OptionID string `json:"option_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var result *city.EventOption
	round := s.ticker.CurrentRound()
	err := s.registry.UpdateCity(req.CityID, func(c *city.City) {
		for _, ae := range c.ActiveEvents {
			if ae.ID == req.EventID {
				result = ApplyEventOption(c, ae, req.OptionID, round)
				break
			}
		}
	})
	if err != nil || result == nil {
		http.Error(w, "event or option not found", 404)
		return
	}

	s.broadcaster.BroadcastEventResolved(req.CityID, req.EventID, result.Title)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "choice": result.Title})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[REST] encode error: %v", err)
		return
	}
	w.Write(data)
}

// corsMiddleware adds CORS headers for browser access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

var _ = fmt.Sprintf
var _ = strconv.Itoa
