package handler

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// VoteHandler handles HTTP requests for voting operations.
type VoteHandler struct {
	votingService *service.VotingService
	roomService   *service.RoomService
	hubManager    *hub.HubManager
}

// NewVoteHandler creates a new VoteHandler.
func NewVoteHandler(votingService *service.VotingService, roomService *service.RoomService, hubManager *hub.HubManager) *VoteHandler {
	return &VoteHandler{
		votingService: votingService,
		roomService:   roomService,
		hubManager:    hubManager,
	}
}

// StartRoundRequest is the request body for starting a new round.
type StartRoundRequest struct {
	StoryTitle string `json:"story_title" binding:"required"`
}

// CastVoteRequest is the request body for casting a vote.
type CastVoteRequest struct {
	Value string `json:"value" binding:"required"`
}

// StartRound handles POST /api/rooms/:code/rounds
func (h *VoteHandler) StartRound(c *gin.Context) {
	code := c.Param("code")
	token := c.GetHeader("X-Session-Token")

	participant, err := h.roomService.GetParticipantByToken(token)
	if err != nil || !participant.IsHost {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the host can start a round"})
		return
	}

	room, err := h.roomService.GetRoom(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	var req StartRoundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	round, err := h.votingService.StartRound(room.ID, req.StoryTitle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start round"})
		return
	}

	roomHub := h.hubManager.GetOrCreateHub(code)
	roomHub.BroadcastMessage(hub.WSMessage{
		Type: "round_started",
		Payload: gin.H{
			"round_id":    round.ID,
			"story_title": round.StoryTitle,
		},
	})

	c.JSON(http.StatusCreated, gin.H{"round": round})
}

// CastVote handles POST /api/rooms/:code/rounds/:roundId/vote
func (h *VoteHandler) CastVote(c *gin.Context) {
	code := c.Param("code")
	roundID := c.Param("roundId")
	token := c.GetHeader("X-Session-Token")

	participant, err := h.roomService.GetParticipantByToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
		return
	}

	var req CastVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vote, err := h.votingService.CastVote(roundID, participant.ID, req.Value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roomHub := h.hubManager.GetOrCreateHub(code)
	roomHub.BroadcastMessage(hub.WSMessage{
		Type: "vote_cast",
		Payload: gin.H{
			"participant_id": participant.ID,
			"display_name":   participant.DisplayName,
			"has_voted":      true,
		},
	})

	c.JSON(http.StatusOK, gin.H{"vote": vote})
}

// RevealVotes handles POST /api/rooms/:code/rounds/:roundId/reveal
func (h *VoteHandler) RevealVotes(c *gin.Context) {
	code := c.Param("code")
	roundID := c.Param("roundId")
	token := c.GetHeader("X-Session-Token")

	participant, err := h.roomService.GetParticipantByToken(token)
	if err != nil || !participant.IsHost {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the host can reveal votes"})
		return
	}

	votes, err := h.votingService.RevealVotes(roundID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build results with display names
	participants, _ := h.roomService.GetParticipants(participant.RoomID)
	nameMap := make(map[string]string)
	for _, p := range participants {
		nameMap[p.ID] = p.DisplayName
	}

	results := make([]gin.H, 0, len(votes))
	for _, v := range votes {
		results = append(results, gin.H{
			"participant_id": v.ParticipantID,
			"display_name":   nameMap[v.ParticipantID],
			"value":          v.Value,
		})
	}

	roomHub := h.hubManager.GetOrCreateHub(code)
	roomHub.BroadcastMessage(hub.WSMessage{
		Type: "votes_revealed",
		Payload: gin.H{
			"round_id": roundID,
			"votes":    results,
		},
	})

	c.JSON(http.StatusOK, gin.H{"votes": results})
}

// ResetRound handles POST /api/rooms/:code/rounds/:roundId/reset
func (h *VoteHandler) ResetRound(c *gin.Context) {
	code := c.Param("code")
	roundID := c.Param("roundId")
	token := c.GetHeader("X-Session-Token")

	participant, err := h.roomService.GetParticipantByToken(token)
	if err != nil || !participant.IsHost {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the host can reset a round"})
		return
	}

	if err := h.votingService.ResetRound(roundID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roomHub := h.hubManager.GetOrCreateHub(code)
	roomHub.BroadcastMessage(hub.WSMessage{
		Type: "round_reset",
		Payload: gin.H{
			"round_id": roundID,
		},
	})

	c.JSON(http.StatusOK, gin.H{"message": "round reset"})
}

// GetRoundState handles GET /api/rooms/:code/rounds/current
func (h *VoteHandler) GetRoundState(c *gin.Context) {
	code := c.Param("code")

	room, err := h.roomService.GetRoom(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "room not found"})
		return
	}

	round, err := h.votingService.GetLatestRound(room.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{"round": nil, "votes": []model.Vote{}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get round"})
		return
	}

	votes, _ := h.votingService.GetVotesByRound(round.ID)

	// If round is not revealed, hide vote values
	if round.Status == model.RoundStatusVoting {
		for i := range votes {
			votes[i].Value = ""
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"round": round,
		"votes": votes,
	})
}
