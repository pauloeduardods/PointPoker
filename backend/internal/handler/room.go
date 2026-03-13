package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// RoomHandler handles HTTP requests for room operations.
type RoomHandler struct {
	roomService *service.RoomService
	hubManager  *hub.HubManager
}

// NewRoomHandler creates a new RoomHandler.
func NewRoomHandler(roomService *service.RoomService, hubManager *hub.HubManager) *RoomHandler {
	return &RoomHandler{roomService: roomService, hubManager: hubManager}
}

// CreateRoomRequest is the request body for creating a room.
type CreateRoomRequest struct {
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
}

// JoinRoomRequest is the request body for joining a room.
type JoinRoomRequest struct {
	DisplayName string `json:"display_name" binding:"required"`
}

// CreateRoom handles POST /api/rooms
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room, host, err := h.roomService.CreateRoom(req.Name, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create room"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"room":          room,
		"session_token": host.SessionToken,
		"participant":   host,
	})
}

// GetRoom handles GET /api/rooms/:code
func (h *RoomHandler) GetRoom(c *gin.Context) {
	code := c.Param("code")

	room, err := h.roomService.GetRoom(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	participants, err := h.roomService.GetParticipants(room.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get participants"})
		return
	}

	// Strip session tokens from response
	for i := range participants {
		participants[i].SessionToken = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"room":         room,
		"participants": participants,
	})
}

// JoinRoom handles POST /api/rooms/:code/join
func (h *RoomHandler) JoinRoom(c *gin.Context) {
	code := c.Param("code")

	var req JoinRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	room, participant, err := h.roomService.JoinRoom(code, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	// Notify existing participants via WebSocket
	roomHub := h.hubManager.GetOrCreateHub(code)
	roomHub.BroadcastMessage(hub.WSMessage{
		Type: "user_joined",
		Payload: gin.H{
			"participant_id": participant.ID,
			"display_name":   participant.DisplayName,
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"room":          room,
		"session_token": participant.SessionToken,
		"participant":   participant,
	})
}
