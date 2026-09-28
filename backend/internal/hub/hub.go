// Package hub fans server-sent events out to the WebSocket clients connected
// to each room and tracks which participants are online.
package hub

import (
	"encoding/json"
	"log"
	"slices"
	"sync"
)

// Event types sent to clients.
const (
	EventParticipantJoined = "participant_joined"
	EventParticipantLeft   = "participant_left"
	EventPresence          = "presence"
	EventRoundStarted      = "round_started"
	EventVoteCast          = "vote_cast"
	EventVotesRevealed     = "votes_revealed"
	EventRoundReset        = "round_reset"
)

// WSMessage represents a WebSocket message with a type and payload.
type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// PresencePayload is the payload of an EventPresence message.
type PresencePayload struct {
	Online []string `json:"online"`
}

// roomHub is the set of clients connected to a single room.
type roomHub struct {
	clients map[*Client]struct{}
}

// HubManager tracks the WebSocket clients of every room.
//
// All state is guarded by a single mutex; sends to clients are non-blocking
// (a client whose buffer is full is dropped), so no method ever blocks on a
// slow connection. A room's hub is created on its first client and removed
// when its last client leaves; both happen under the same lock, so a
// concurrent Register can never add a client to a hub that was just removed.
type HubManager struct {
	mu     sync.Mutex
	hubs   map[string]*roomHub
	closed bool
}

// NewHubManager creates a new HubManager.
func NewHubManager() *HubManager {
	return &HubManager{hubs: make(map[string]*roomHub)}
}

// Register adds a client to its room and broadcasts the updated presence.
// If the manager is closed, the client's send channel is closed immediately.
func (m *HubManager) Register(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		c.closeSend()
		return
	}
	h, ok := m.hubs[c.roomCode]
	if !ok {
		h = &roomHub{clients: make(map[*Client]struct{})}
		m.hubs[c.roomCode] = h
	}
	h.clients[c] = struct{}{}
	m.broadcastPresenceLocked(c.roomCode)
}

// Unregister removes a client from its room, closes its send channel and
// broadcasts the updated presence. It is safe to call more than once and
// for clients that were already dropped.
func (m *HubManager) Unregister(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.removeLocked(c) {
		m.broadcastPresenceLocked(c.roomCode)
	}
}

// DisconnectParticipant closes every connection of a participant in a room
// (e.g. after they left it) and broadcasts the updated presence.
func (m *HubManager) DisconnectParticipant(roomCode, participantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.hubs[roomCode]
	if !ok {
		return
	}
	removed := false
	for c := range h.clients {
		if c.participantID == participantID {
			removed = m.removeLocked(c) || removed
		}
	}
	if removed {
		m.broadcastPresenceLocked(roomCode)
	}
}

// Broadcast sends msg to every client connected to the room. It never blocks.
func (m *HubManager) Broadcast(roomCode string, msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[hub %s] marshal %s: %v", roomCode, msg.Type, err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendLocked(roomCode, data)
}

// Online returns the sorted, de-duplicated IDs of the participants with at
// least one open connection to the room. It never returns nil.
func (m *HubManager) Online(roomCode string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onlineLocked(roomCode)
}

// RoomCount returns the number of rooms with at least one connected client.
func (m *HubManager) RoomCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.hubs)
}

// ClientCount returns the number of clients connected to a room.
func (m *HubManager) ClientCount(roomCode string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if h, ok := m.hubs[roomCode]; ok {
		return len(h.clients)
	}
	return 0
}

// Close disconnects every client and rejects future registrations. It is
// used during graceful shutdown.
func (m *HubManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	for code, h := range m.hubs {
		for c := range h.clients {
			c.closeSend()
		}
		delete(m.hubs, code)
	}
}

// removeLocked removes c from its hub (deleting the hub if it became empty)
// and reports whether c was registered.
func (m *HubManager) removeLocked(c *Client) bool {
	h, ok := m.hubs[c.roomCode]
	if !ok {
		return false
	}
	if _, ok := h.clients[c]; !ok {
		return false
	}
	delete(h.clients, c)
	c.closeSend()
	if len(h.clients) == 0 {
		delete(m.hubs, c.roomCode)
	}
	return true
}

// sendLocked delivers data to every client of the room without blocking.
// Clients whose buffer is full are dropped, which in turn triggers a
// presence broadcast to the remaining clients.
func (m *HubManager) sendLocked(roomCode string, data []byte) {
	h, ok := m.hubs[roomCode]
	if !ok {
		return
	}
	dropped := false
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			log.Printf("[hub %s] dropping slow client %s", roomCode, c.participantID)
			m.removeLocked(c)
			dropped = true
		}
	}
	if dropped {
		// Terminates: every recursion level removes at least one client.
		m.broadcastPresenceLocked(roomCode)
	}
}

func (m *HubManager) broadcastPresenceLocked(roomCode string) {
	if _, ok := m.hubs[roomCode]; !ok {
		return
	}
	data, err := json.Marshal(WSMessage{
		Type:    EventPresence,
		Payload: PresencePayload{Online: m.onlineLocked(roomCode)},
	})
	if err != nil {
		log.Printf("[hub %s] marshal presence: %v", roomCode, err)
		return
	}
	m.sendLocked(roomCode, data)
}

func (m *HubManager) onlineLocked(roomCode string) []string {
	ids := []string{}
	if h, ok := m.hubs[roomCode]; ok {
		for c := range h.clients {
			ids = append(ids, c.participantID)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
