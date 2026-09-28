// Package handler exposes the HTTP and WebSocket API over gin.
package handler

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// SessionTokenHeader carries the participant's session token on REST calls.
const SessionTokenHeader = "X-Session-Token"

// Pinger checks that a dependency (the database) is reachable. *sql.DB satisfies it.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Deps are the dependencies needed to build the router.
type Deps struct {
	Rooms  *service.RoomService
	Voting *service.VotingService
	Hubs   *hub.HubManager
	// DB is pinged by /api/health.
	DB Pinger
	// CORSOrigins lists allowed cross-origin callers; "*" allows any.
	CORSOrigins []string
}

// NewRouter builds the gin engine with every route of the API under /api.
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware(d.CORSOrigins))
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) { abortError(c, http.StatusNotFound, "route not found") })
	r.NoMethod(func(c *gin.Context) { abortError(c, http.StatusMethodNotAllowed, "method not allowed") })

	roomH := &RoomHandler{rooms: d.Rooms, hubs: d.Hubs}
	voteH := &VoteHandler{voting: d.Voting, hubs: d.Hubs}
	wsH := &WSHandler{rooms: d.Rooms, hubs: d.Hubs}

	api := r.Group("/api")
	api.GET("/health", healthHandler(d.DB))

	rooms := api.Group("/rooms")
	rooms.POST("", roomH.CreateRoom)
	rooms.GET("/:code", roomH.GetRoom)
	rooms.POST("/:code/join", roomH.JoinRoom)
	rooms.GET("/:code/me", roomH.Me)
	rooms.DELETE("/:code/participants/me", roomH.Leave)
	rooms.POST("/:code/rounds", voteH.StartRound)
	rooms.GET("/:code/rounds/current", voteH.CurrentRound)
	rooms.POST("/:code/rounds/:roundId/vote", voteH.CastVote)
	rooms.POST("/:code/rounds/:roundId/reveal", voteH.Reveal)
	rooms.POST("/:code/rounds/:roundId/reset", voteH.Reset)
	rooms.GET("/:code/ws", wsH.Serve)

	return r
}

func corsMiddleware(origins []string) gin.HandlerFunc {
	cfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", SessionTokenHeader},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	if len(origins) == 0 || slices.Contains(origins, "*") {
		cfg.AllowAllOrigins = true
		cfg.AllowCredentials = false
	} else {
		cfg.AllowOrigins = origins
	}
	crossOrigin := cors.New(cfg)
	return func(c *gin.Context) {
		// Browsers send Origin on same-origin POST/DELETE too (e.g. behind the
		// nginx or Vite proxy); those must never be rejected by the allow-list.
		if isSameOrigin(c.Request) {
			c.Next()
			return
		}
		crossOrigin(c)
	}
}

// isSameOrigin reports whether the request's Origin header points at the host it was sent to.
func isSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func healthHandler(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if err := db.PingContext(ctx); err != nil {
				abortError(c, http.StatusServiceUnavailable, "database unavailable")
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
