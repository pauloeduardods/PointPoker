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
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// WSHandler handles WebSocket upgrade requests.
type WSHandler struct {
	roomService *service.RoomService
	hubManager  *hub.HubManager
}

// NewWSHandler creates a new WSHandler.
func NewWSHandler(roomService *service.RoomService, hubManager *hub.HubManager) *WSHandler {
	return &WSHandler{
		roomService: roomService,
		hubManager:  hubManager,
	}
}

// HandleWebSocket handles GET /api/rooms/:code/ws?token=<session_token>
func (h *WSHandler) HandleWebSocket(c *gin.Context) {
	code := c.Param("code")
	token := c.Query("token")

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing session token"})
		return
	}

	participant, err := h.roomService.GetParticipantByToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	roomHub := h.hubManager.GetOrCreateHub(code)
	client := hub.NewClient(roomHub, conn, participant.ID, participant.DisplayName)

	roomHub.Register(client)

	go client.WritePump()
	go client.ReadPump(func(participantID, displayName string) {
		roomHub.BroadcastMessage(hub.WSMessage{
			Type: "user_left",
			Payload: gin.H{
				"participant_id": participantID,
				"display_name":   displayName,
			},
		})
	})
}
