package hub

import (
	"encoding/json"
	"log"
	"sync"
)

// WSMessage represents a WebSocket message with a type and payload.
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// RoomHub manages WebSocket clients for a single room.
type RoomHub struct {
	roomCode   string
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// HubManager manages all room hubs.
type HubManager struct {
	hubs map[string]*RoomHub
	mu   sync.RWMutex
}

// NewHubManager creates a new HubManager.
func NewHubManager() *HubManager {
	return &HubManager{
		hubs: make(map[string]*RoomHub),
	}
}

// GetOrCreateHub returns an existing hub for the room, or creates a new one.
func (m *HubManager) GetOrCreateHub(roomCode string) *RoomHub {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hub, ok := m.hubs[roomCode]; ok {
		return hub
	}

	hub := &RoomHub{
		roomCode:   roomCode,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
	m.hubs[roomCode] = hub
	go hub.Run()
	return hub
}

// RemoveHub removes a hub for a room.
func (m *HubManager) RemoveHub(roomCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hubs, roomCode)
}

// Run starts the hub's main loop processing register, unregister, and broadcast events.
func (h *RoomHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[Hub %s] Client registered: %s", h.roomCode, client.participantID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[Hub %s] Client unregistered: %s", h.roomCode, client.participantID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastMessage serializes and sends a message to all connected clients.
func (h *RoomHub) BroadcastMessage(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Hub %s] Error marshaling message: %v", h.roomCode, err)
		return
	}
	h.broadcast <- data
}

// Register adds a client to the hub.
func (h *RoomHub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *RoomHub) Unregister(client *Client) {
	h.unregister <- client
}

// ClientCount returns the number of connected clients.
func (h *RoomHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
