package hub

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
	sendBufferSize = 64
)

// Client represents a single WebSocket connection of a participant to a room.
type Client struct {
	conn          *websocket.Conn
	send          chan []byte
	closeOnce     sync.Once
	roomCode      string
	participantID string
}

// NewClient creates a new Client for a participant connected to a room.
func NewClient(conn *websocket.Conn, roomCode, participantID string) *Client {
	return &Client{
		conn:          conn,
		send:          make(chan []byte, sendBufferSize),
		roomCode:      roomCode,
		participantID: participantID,
	}
}

// closeSend closes the send channel exactly once. Callers hold the manager lock.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

// Serve registers the client with m and pumps messages until the connection
// closes, then unregisters it. It blocks for the lifetime of the connection.
func (c *Client) Serve(m *HubManager) {
	m.Register(c)
	go c.writePump()
	c.readPump()
	m.Unregister(c)
}

// readPump discards incoming messages (clients never need to send anything)
// and keeps the read deadline fresh via pongs. It returns when the
// connection fails or is closed.
func (c *Client) readPump() {
	defer c.conn.Close()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[hub %s] websocket error: %v", c.roomCode, err)
			}
			return
		}
	}
}

// writePump sends queued messages and periodic pings. It returns (closing
// the connection, which also stops readPump) when the send channel is
// closed by the manager or a write fails.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
