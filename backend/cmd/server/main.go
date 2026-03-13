package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/pauloedsg/pointpoker/internal/config"
	"github.com/pauloedsg/pointpoker/internal/handler"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/repository"
	"github.com/pauloedsg/pointpoker/internal/service"
)

func main() {
	cfg := config.Load()

	// Connect to database
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Run migrations
	if err := runMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Repositories
	roomRepo := repository.NewRoomRepository(db)
	voteRepo := repository.NewVoteRepository(db)

	// Services
	roomService := service.NewRoomService(roomRepo)
	votingService := service.NewVotingService(voteRepo, roomRepo)

	// WebSocket Hub Manager
	hubManager := hub.NewHubManager()

	// Handlers
	roomHandler := handler.NewRoomHandler(roomService, hubManager)
	voteHandler := handler.NewVoteHandler(votingService, roomService, hubManager)
	wsHandler := handler.NewWSHandler(roomService, hubManager)

	// Gin Router
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "X-Session-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API routes
	api := r.Group("/api")
	{
		rooms := api.Group("/rooms")
		{
			rooms.POST("", roomHandler.CreateRoom)
			rooms.GET("/:code", roomHandler.GetRoom)
			rooms.POST("/:code/join", roomHandler.JoinRoom)
			rooms.POST("/:code/rounds", voteHandler.StartRound)
			rooms.GET("/:code/rounds/current", voteHandler.GetRoundState)
			rooms.POST("/:code/rounds/:roundId/vote", voteHandler.CastVote)
			rooms.POST("/:code/rounds/:roundId/reveal", voteHandler.RevealVotes)
			rooms.POST("/:code/rounds/:roundId/reset", voteHandler.ResetRound)
			rooms.GET("/:code/ws", wsHandler.HandleWebSocket)
		}
	}

	port := cfg.ServerPort
	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runMigrations(db *sql.DB) error {
	migration, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	_, err = db.Exec(string(migration))
	if err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	log.Println("Migrations completed successfully")
	return nil
}
