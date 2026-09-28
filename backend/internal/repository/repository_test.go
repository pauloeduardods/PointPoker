package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
	"github.com/pauloedsg/pointpoker/migrations"
)

// openTestDB connects to TEST_DATABASE_URL, applies migrations and empties
// every table. Tests are skipped when the variable is unset.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping repository integration tests")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	// Applying twice proves the migrations are idempotent.
	for range 2 {
		if err := migrations.Apply(ctx, db); err != nil {
			t.Fatalf("apply migrations: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE rooms, participants, voting_rounds, votes CASCADE`); err != nil {
		t.Fatal(err)
	}
	return db
}

type repos struct {
	rooms *repository.RoomRepository
	votes *repository.VoteRepository
}

func newRepos(t *testing.T) (repos, *sql.DB) {
	db := openTestDB(t)
	return repos{repository.NewRoomRepository(db), repository.NewVoteRepository(db)}, db
}

func mustRoom(t *testing.T, r repos, code string) (*model.Room, *model.Participant) {
	t.Helper()
	room := &model.Room{Code: code, Name: "Room " + code, Status: model.RoomStatusWaiting}
	host := &model.Participant{DisplayName: "Host", SessionToken: uuid.NewString(), IsHost: true}
	if err := r.rooms.CreateRoomWithHost(context.Background(), room, host); err != nil {
		t.Fatalf("CreateRoomWithHost: %v", err)
	}
	return room, host
}

func mustJoin(t *testing.T, r repos, roomID, name string) *model.Participant {
	t.Helper()
	p := &model.Participant{RoomID: roomID, DisplayName: name, SessionToken: uuid.NewString()}
	if err := r.rooms.AddParticipant(context.Background(), p); err != nil {
		t.Fatalf("AddParticipant: %v", err)
	}
	return p
}

func TestMigrationNames(t *testing.T) {
	names, err := migrations.Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 || names[0] != "001_init.sql" {
		t.Errorf("names = %v", names)
	}
}

func TestRoomRepository(t *testing.T) {
	r, _ := newRepos(t)
	ctx := context.Background()

	room, host := mustRoom(t, r, "AAAAAA")
	if room.ID == "" || room.CreatedAt.IsZero() || host.ID == "" || host.RoomID != room.ID {
		t.Fatalf("ids not populated: %+v %+v", room, host)
	}

	// Duplicate codes are reported and the transaction leaves no orphan host.
	dup := &model.Room{Code: "AAAAAA", Name: "dup", Status: model.RoomStatusWaiting}
	orphan := &model.Participant{DisplayName: "X", SessionToken: uuid.NewString(), IsHost: true}
	if err := r.rooms.CreateRoomWithHost(ctx, dup, orphan); !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("duplicate code err = %v", err)
	}
	if _, err := r.rooms.GetParticipantByToken(ctx, orphan.SessionToken); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("orphan host persisted: %v", err)
	}

	got, err := r.rooms.GetRoomByCode(ctx, "AAAAAA")
	if err != nil || got.ID != room.ID || got.Status != model.RoomStatusWaiting {
		t.Fatalf("GetRoomByCode = %+v, %v", got, err)
	}
	if _, err := r.rooms.GetRoomByCode(ctx, "NOPE00"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("missing room err = %v", err)
	}
	if err := r.rooms.UpdateRoomStatus(ctx, room.ID, model.RoomStatusRevealed); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.rooms.GetRoomByCode(ctx, "AAAAAA"); got.Status != model.RoomStatusRevealed {
		t.Errorf("status = %q", got.Status)
	}

	p, err := r.rooms.GetParticipantByToken(ctx, host.SessionToken)
	if err != nil || p.ID != host.ID || !p.IsHost || p.SessionToken != host.SessionToken {
		t.Errorf("GetParticipantByToken = %+v, %v", p, err)
	}
	for _, tok := range []string{uuid.NewString(), "not-a-uuid"} {
		if _, err := r.rooms.GetParticipantByToken(ctx, tok); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("token %q err = %v", tok, err)
		}
	}

	g1 := mustJoin(t, r, room.ID, "G1")
	g2 := mustJoin(t, r, room.ID, "G2")
	list, err := r.rooms.ListParticipants(ctx, room.ID)
	if err != nil || len(list) != 3 || list[0].ID != host.ID || list[1].ID != g1.ID || list[2].ID != g2.ID {
		t.Errorf("ListParticipants = %+v, %v", list, err)
	}
	empty, err := r.rooms.ListParticipants(ctx, uuid.NewString())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("empty list = %#v, %v", empty, err)
	}
}

func TestRemoveParticipantHostTransfer(t *testing.T) {
	r, _ := newRepos(t)
	ctx := context.Background()
	room, host := mustRoom(t, r, "BBBBBB")
	g1 := mustJoin(t, r, room.ID, "G1")
	g2 := mustJoin(t, r, room.ID, "G2")
	otherRoom, _ := mustRoom(t, r, "CCCCCC")

	// Removing from the wrong room is not found.
	if _, err := r.rooms.RemoveParticipant(ctx, otherRoom.ID, g1.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("wrong room err = %v", err)
	}
	// Non-host leaves: no transfer.
	if newHost, err := r.rooms.RemoveParticipant(ctx, room.ID, g2.ID); err != nil || newHost != "" {
		t.Errorf("non-host remove = %q, %v", newHost, err)
	}
	// Host leaves: earliest remaining is promoted.
	newHost, err := r.rooms.RemoveParticipant(ctx, room.ID, host.ID)
	if err != nil || newHost != g1.ID {
		t.Fatalf("host remove = %q, %v; want %q", newHost, err, g1.ID)
	}
	p, _ := r.rooms.GetParticipantByToken(ctx, g1.SessionToken)
	if !p.IsHost {
		t.Error("g1 not promoted")
	}
	// Removing twice is not found.
	if _, err := r.rooms.RemoveParticipant(ctx, room.ID, host.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("double remove err = %v", err)
	}
	// Last one leaves: nobody to promote.
	if newHost, err := r.rooms.RemoveParticipant(ctx, room.ID, g1.ID); err != nil || newHost != "" {
		t.Errorf("last remove = %q, %v", newHost, err)
	}
}

func TestConcurrentLeavesKeepExactlyOneHost(t *testing.T) {
	r, db := newRepos(t)
	ctx := context.Background()
	room, host := mustRoom(t, r, "DDDDDD")
	var guests []*model.Participant
	for _, n := range []string{"G1", "G2", "G3", "G4"} {
		guests = append(guests, mustJoin(t, r, room.ID, n))
	}

	// Host and the first two guests leave at once.
	var wg sync.WaitGroup
	for _, p := range []*model.Participant{host, guests[0], guests[1]} {
		wg.Go(func() {
			if _, err := r.rooms.RemoveParticipant(ctx, room.ID, p.ID); err != nil {
				t.Errorf("remove %s: %v", p.DisplayName, err)
			}
		})
	}
	wg.Wait()

	var hosts int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM participants WHERE room_id = $1 AND is_host`, room.ID).Scan(&hosts); err != nil {
		t.Fatal(err)
	}
	if hosts != 1 {
		t.Errorf("hosts = %d, want 1", hosts)
	}
}

func TestVoteRepository(t *testing.T) {
	r, _ := newRepos(t)
	ctx := context.Background()
	room, host := mustRoom(t, r, "EEEEEE")
	guest := mustJoin(t, r, room.ID, "Guest")

	if _, err := r.votes.GetLatestRound(ctx, room.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("latest round of empty room err = %v", err)
	}
	for _, id := range []string{uuid.NewString(), "not-a-uuid"} {
		if _, err := r.votes.GetRound(ctx, id); !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("GetRound(%q) err = %v", id, err)
		}
	}

	r1 := &model.VotingRound{RoomID: room.ID, StoryTitle: "One", Status: model.RoundStatusVoting}
	if err := r.votes.CreateRound(ctx, r1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	r2 := &model.VotingRound{RoomID: room.ID, StoryTitle: "Two", Status: model.RoundStatusVoting}
	if err := r.votes.CreateRound(ctx, r2); err != nil {
		t.Fatal(err)
	}
	latest, err := r.votes.GetLatestRound(ctx, room.ID)
	if err != nil || latest.ID != r2.ID {
		t.Errorf("latest = %+v, %v", latest, err)
	}
	got, err := r.votes.GetRound(ctx, r1.ID)
	if err != nil || got.StoryTitle != "One" || got.RoomID != room.ID {
		t.Errorf("GetRound = %+v, %v", got, err)
	}

	votes, err := r.votes.ListVotes(ctx, r2.ID)
	if err != nil || votes == nil || len(votes) != 0 {
		t.Errorf("empty ListVotes = %#v, %v", votes, err)
	}

	v := &model.Vote{RoundID: r2.ID, ParticipantID: guest.ID, Value: "5"}
	if err := r.votes.UpsertVote(ctx, v); err != nil {
		t.Fatal(err)
	}
	firstID := v.ID
	v2 := &model.Vote{RoundID: r2.ID, ParticipantID: guest.ID, Value: "☕"}
	if err := r.votes.UpsertVote(ctx, v2); err != nil {
		t.Fatal(err)
	}
	if v2.ID != firstID {
		t.Errorf("upsert created a new row")
	}
	if err := r.votes.UpsertVote(ctx, &model.Vote{RoundID: r2.ID, ParticipantID: host.ID, Value: "?"}); err != nil {
		t.Fatal(err)
	}
	votes, _ = r.votes.ListVotes(ctx, r2.ID)
	if len(votes) != 2 || votes[0].Value != "☕" || votes[0].ParticipantID != guest.ID {
		t.Errorf("votes = %+v", votes)
	}

	ok, err := r.votes.RevealRound(ctx, r2.ID)
	if err != nil || !ok {
		t.Fatalf("reveal = %v, %v", ok, err)
	}
	ok, err = r.votes.RevealRound(ctx, r2.ID)
	if err != nil || ok {
		t.Errorf("second reveal = %v, %v; want false", ok, err)
	}
	if got, _ := r.votes.GetRound(ctx, r2.ID); got.Status != model.RoundStatusRevealed {
		t.Errorf("status = %q", got.Status)
	}

	if err := r.votes.ResetRound(ctx, r2.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = r.votes.GetRound(ctx, r2.ID)
	votes, _ = r.votes.ListVotes(ctx, r2.ID)
	if got.Status != model.RoundStatusVoting || len(votes) != 0 {
		t.Errorf("after reset: status %q, %d votes", got.Status, len(votes))
	}

	// Removing a participant cascades to their votes.
	if err := r.votes.UpsertVote(ctx, &model.Vote{RoundID: r2.ID, ParticipantID: guest.ID, Value: "3"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.rooms.RemoveParticipant(ctx, room.ID, guest.ID); err != nil {
		t.Fatal(err)
	}
	if votes, _ = r.votes.ListVotes(ctx, r2.ID); len(votes) != 0 {
		t.Errorf("votes of removed participant remain: %+v", votes)
	}
}
