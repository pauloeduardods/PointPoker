// Command server runs the PointPoker HTTP/WebSocket API.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/pauloedsg/pointpoker/internal/config"
	"github.com/pauloedsg/pointpoker/internal/handler"
	"github.com/pauloedsg/pointpoker/internal/hub"
	"github.com/pauloedsg/pointpoker/internal/repository"
	"github.com/pauloedsg/pointpoker/internal/service"
	"github.com/pauloedsg/pointpoker/migrations"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	log.Println("Connected to database")

	if err := migrations.Apply(ctx, db); err != nil {
		return err
	}
	log.Println("Migrations completed successfully")

	roomRepo := repository.NewRoomRepository(db)
	voteRepo := repository.NewVoteRepository(db)
	hubs := hub.NewHubManager()

	router := handler.NewRouter(handler.Deps{
		Rooms:       service.NewRoomService(roomRepo),
		Voting:      service.NewVotingService(voteRepo, roomRepo),
		Hubs:        hubs,
		DB:          db,
		CORSOrigins: cfg.CORSOrigins,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Server starting on port %s", cfg.ServerPort)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// WebSocket connections are hijacked and not tracked by Shutdown, so
	// close them explicitly.
	hubs.Close()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Println("Server stopped")
	return nil
}
