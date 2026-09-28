package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// VoteHandler handles HTTP requests for rounds and votes.
type VoteHandler struct {
	voting *service.VotingService
	hubs   *hub.HubManager
}

// StartRoundRequest is the request body for starting a new round.
type StartRoundRequest struct {
	StoryTitle string `json:"story_title"`
}

// CastVoteRequest is the request body for casting a vote.
type CastVoteRequest struct {
	Value string `json:"value"`
}

// CurrentRoundResponse is returned by GET /rooms/:code/rounds/current.
type CurrentRoundResponse struct {
	Round *model.VotingRound `json:"round"`
	Votes []model.Vote       `json:"votes"`
}

// StartRound handles POST /api/rooms/:code/rounds.
func (h *VoteHandler) StartRound(c *gin.Context) {
	code := roomCode(c)
	var req StartRoundRequest
	if !bindJSON(c, &req) {
		return
	}
	round, err := h.voting.StartRound(c.Request.Context(), code, sessionToken(c), req.StoryTitle)
	if err != nil {
		writeError(c, err)
		return
	}
	h.hubs.Broadcast(code, hub.WSMessage{Type: hub.EventRoundStarted, Payload: gin.H{"round": round}})
	c.JSON(http.StatusCreated, gin.H{"round": round})
}

// CastVote handles POST /api/rooms/:code/rounds/:roundId/vote.
func (h *VoteHandler) CastVote(c *gin.Context) {
	code := roomCode(c)
	var req CastVoteRequest
	if !bindJSON(c, &req) {
		return
	}
	vote, err := h.voting.CastVote(c.Request.Context(), code, sessionToken(c), c.Param("roundId"), req.Value)
	if err != nil {
		writeError(c, err)
		return
	}
	h.hubs.Broadcast(code, hub.WSMessage{
		Type:    hub.EventVoteCast,
		Payload: gin.H{"participant_id": vote.ParticipantID},
	})
	c.JSON(http.StatusOK, gin.H{"vote": vote})
}

// Reveal handles POST /api/rooms/:code/rounds/:roundId/reveal.
func (h *VoteHandler) Reveal(c *gin.Context) {
	code := roomCode(c)
	roundID := c.Param("roundId")
	votes, err := h.voting.Reveal(c.Request.Context(), code, sessionToken(c), roundID)
	if err != nil {
		writeError(c, err)
		return
	}
	h.hubs.Broadcast(code, hub.WSMessage{
		Type:    hub.EventVotesRevealed,
		Payload: gin.H{"round_id": roundID, "votes": votes},
	})
	c.JSON(http.StatusOK, gin.H{"votes": votes})
}

// Reset handles POST /api/rooms/:code/rounds/:roundId/reset.
func (h *VoteHandler) Reset(c *gin.Context) {
	code := roomCode(c)
	round, err := h.voting.Reset(c.Request.Context(), code, sessionToken(c), c.Param("roundId"))
	if err != nil {
		writeError(c, err)
		return
	}
	h.hubs.Broadcast(code, hub.WSMessage{Type: hub.EventRoundReset, Payload: gin.H{"round": round}})
	c.JSON(http.StatusOK, gin.H{"round": round})
}

// CurrentRound handles GET /api/rooms/:code/rounds/current. The session
// token is optional and only used to reveal the caller's own vote.
func (h *VoteHandler) CurrentRound(c *gin.Context) {
	round, votes, err := h.voting.CurrentRound(c.Request.Context(), roomCode(c), sessionToken(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, CurrentRoundResponse{Round: round, Votes: votes})
}
