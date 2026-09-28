package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// RoomHandler handles HTTP requests for room and participant operations.
type RoomHandler struct {
	rooms *service.RoomService
	hubs  *hub.HubManager
}

// CreateRoomRequest is the request body for creating a room.
type CreateRoomRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

// JoinRoomRequest is the request body for joining a room.
type JoinRoomRequest struct {
	DisplayName string `json:"display_name"`
}

// SessionResponse is returned by create and join: the only responses that
// carry the participant's session token.
type SessionResponse struct {
	Room         *model.Room        `json:"room"`
	SessionToken string             `json:"session_token"`
	Participant  *model.Participant `json:"participant"`
}

// ParticipantView is a participant enriched with its WebSocket presence.
type ParticipantView struct {
	model.Participant
	Online bool `json:"online"`
}

// RoomResponse is returned by GET /rooms/:code.
type RoomResponse struct {
	Room         *model.Room       `json:"room"`
	Participants []ParticipantView `json:"participants"`
}

// CreateRoom handles POST /api/rooms.
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if !bindJSON(c, &req) {
		return
	}
	room, host, err := h.rooms.CreateRoom(c.Request.Context(), req.Name, req.DisplayName)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, SessionResponse{Room: room, SessionToken: host.SessionToken, Participant: host})
}

// GetRoom handles GET /api/rooms/:code.
func (h *RoomHandler) GetRoom(c *gin.Context) {
	code := roomCode(c)
	room, participants, err := h.rooms.GetRoom(c.Request.Context(), code)
	if err != nil {
		writeError(c, err)
		return
	}
	online := make(map[string]bool)
	for _, id := range h.hubs.Online(code) {
		online[id] = true
	}
	views := make([]ParticipantView, 0, len(participants))
	for _, p := range participants {
		views = append(views, ParticipantView{Participant: p, Online: online[p.ID]})
	}
	c.JSON(http.StatusOK, RoomResponse{Room: room, Participants: views})
}

// JoinRoom handles POST /api/rooms/:code/join.
func (h *RoomHandler) JoinRoom(c *gin.Context) {
	code := roomCode(c)
	var req JoinRoomRequest
	if !bindJSON(c, &req) {
		return
	}
	room, participant, err := h.rooms.JoinRoom(c.Request.Context(), code, req.DisplayName)
	if err != nil {
		writeError(c, err)
		return
	}
	h.hubs.Broadcast(code, hub.WSMessage{
		Type:    hub.EventParticipantJoined,
		Payload: gin.H{"participant": participant},
	})
	c.JSON(http.StatusOK, SessionResponse{Room: room, SessionToken: participant.SessionToken, Participant: participant})
}

// Me handles GET /api/rooms/:code/me, used to restore a session after reload.
func (h *RoomHandler) Me(c *gin.Context) {
	_, p, err := h.rooms.Authorize(c.Request.Context(), roomCode(c), sessionToken(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"participant": p})
}

// Leave handles DELETE /api/rooms/:code/participants/me.
func (h *RoomHandler) Leave(c *gin.Context) {
	code := roomCode(c)
	res, err := h.rooms.Leave(c.Request.Context(), code, sessionToken(c))
	if err != nil {
		writeError(c, err)
		return
	}
	var newHostID *string
	if res.NewHostID != "" {
		newHostID = &res.NewHostID
	}
	h.hubs.DisconnectParticipant(code, res.ParticipantID)
	h.hubs.Broadcast(code, hub.WSMessage{
		Type:    hub.EventParticipantLeft,
		Payload: gin.H{"participant_id": res.ParticipantID, "new_host_id": newHostID},
	})
	c.Status(http.StatusNoContent)
}
