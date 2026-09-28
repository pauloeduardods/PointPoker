package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/service"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Any origin is accepted: authentication relies on the session token in
	// the query string, which is not an ambient credential a foreign page
	// could replay (unlike cookies), so cross-site WebSocket hijacking does
	// not apply. This also keeps dev proxies that rewrite Host working.
	CheckOrigin: func(*http.Request) bool { return true },
}

// WSHandler handles WebSocket upgrade requests.
type WSHandler struct {
	rooms *service.RoomService
	hubs  *hub.HubManager
}

// Serve handles GET /api/rooms/:code/ws?token=<session_token>. The
// participant must belong to the room; the handler blocks for the lifetime
// of the connection.
func (h *WSHandler) Serve(c *gin.Context) {
	code := roomCode(c)
	_, p, err := h.rooms.Authorize(c.Request.Context(), code, c.Query("token"))
	if err != nil {
		writeError(c, err)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err) // upgrader already wrote the HTTP error
		return
	}
	hub.NewClient(conn, code, p.ID).Serve(h.hubs)
}
