package coordinator

import (
	"encoding/json"
	"log"
	"sync"
)

// Message types sent to clients over WebSocket.
const (
	MsgHeartbeat  = "heartbeat"
	MsgCityUpdate = "city_update"
	MsgWorldEvent = "world_event"
)

// WSMessage is a generic WebSocket envelope.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	PlayerID string
	CityID   string
	Send     chan []byte
}

// Broadcaster manages fan-out of game events to all connected clients.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[string]*WSClient // playerID → client
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[string]*WSClient),
	}
}

// Register adds a WebSocket client.
func (b *Broadcaster) Register(client *WSClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[client.PlayerID] = client
	log.Printf("[Broadcaster] Client registered: player=%s city=%s", client.PlayerID, client.CityID)
}

// Unregister removes a client.
func (b *Broadcaster) Unregister(playerID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, playerID)
	log.Printf("[Broadcaster] Client unregistered: player=%s", playerID)
}

// BroadcastHeartbeat sends heartbeat proposals to the specific city's mayor.
func (b *Broadcaster) BroadcastHeartbeat(hb *Heartbeat) {
	msg := WSMessage{Type: MsgHeartbeat, Payload: hb}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling heartbeat: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		if client.CityID == hb.CityID {
			select {
			case client.Send <- data:
			default:
				log.Printf("[Broadcaster] Client %s send buffer full — dropping heartbeat", client.PlayerID)
			}
		}
	}
}

// BroadcastCityUpdate sends city state updates to the city's mayor.
func (b *Broadcaster) BroadcastCityUpdate(result HeartbeatResult) {
	msg := WSMessage{Type: MsgCityUpdate, Payload: result}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling city update: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		if client.CityID == result.CityID {
			select {
			case client.Send <- data:
			default:
				log.Printf("[Broadcaster] Client %s send buffer full", client.PlayerID)
			}
		}
	}
}

// BroadcastWorldEvent sends an event to ALL connected clients.
func (b *Broadcaster) BroadcastWorldEvent(event WorldEvent) {
	msg := WSMessage{Type: MsgWorldEvent, Payload: event}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling world event: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

// ConnectedCount returns the number of connected clients.
func (b *Broadcaster) ConnectedCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
