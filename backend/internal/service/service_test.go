package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pauloedsg/pointpoker/internal/memstore"
	"github.com/pauloedsg/pointpoker/internal/model"
)

type fixture struct {
	store  *memstore.Store
	rooms  *RoomService
	voting *VotingService
}

func newFixture() *fixture {
	st := memstore.New()
	return &fixture{store: st, rooms: NewRoomService(st), voting: NewVotingService(st, st)}
}

type session struct {
	room *model.Room
	p    *model.Participant
}

func (f *fixture) create(t *testing.T, name, host string) session {
	t.Helper()
	room, p, err := f.rooms.CreateRoom(context.Background(), name, host)
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	return session{room, p}
}

func (f *fixture) join(t *testing.T, code, name string) session {
	t.Helper()
	room, p, err := f.rooms.JoinRoom(context.Background(), code, name)
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	return session{room, p}
}

func wantKind(t *testing.T, err, kind error) {
	t.Helper()
	if kind == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if !errors.Is(err, kind) {
		t.Fatalf("error = %v, want kind %v", err, kind)
	}
}

func TestGenerateRoomCode(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := generateRoomCode(roomCodeLength)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != roomCodeLength {
			t.Fatalf("len(%q) = %d", code, len(code))
		}
		for _, r := range code {
			if !strings.ContainsRune(roomCodeCharset, r) {
				t.Fatalf("code %q contains %q outside charset", code, r)
			}
		}
		seen[code] = true
	}
	if len(seen) < 190 {
		t.Errorf("only %d distinct codes out of 200", len(seen))
	}
}

func TestNormalizeCode(t *testing.T) {
	if got := NormalizeCode("  abc12x "); got != "ABC12X" {
		t.Errorf("NormalizeCode = %q", got)
	}
}

func TestCreateRoomValidation(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n) }
	tests := []struct {
		name, room, display string
		wantErr             error
	}{
		{"ok", "Sprint 1", "Alice", nil},
		{"trimmed ok", "  Sprint  ", "  Bob ", nil},
		{"max lengths in runes", long(100), long(50), nil},
		{"empty name", "", "Alice", ErrInvalid},
		{"blank name", "   ", "Alice", ErrInvalid},
		{"name too long", long(101), "Alice", ErrInvalid},
		{"empty display", "Sprint", " ", ErrInvalid},
		{"display too long", "Sprint", long(51), ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			room, host, err := f.rooms.CreateRoom(context.Background(), tt.room, tt.display)
			wantKind(t, err, tt.wantErr)
			if err != nil {
				return
			}
			if room.Name != strings.TrimSpace(tt.room) || host.DisplayName != strings.TrimSpace(tt.display) {
				t.Errorf("not trimmed: %q / %q", room.Name, host.DisplayName)
			}
			if !host.IsHost || host.RoomID != room.ID || host.SessionToken == "" {
				t.Errorf("bad host: %+v", host)
			}
			if room.Status != model.RoomStatusWaiting {
				t.Errorf("status = %q", room.Status)
			}
		})
	}
}

func TestCreateRoomRetriesOnCodeCollision(t *testing.T) {
	f := newFixture()
	f.store.Codes = []string{"AAAAAA"}
	f.create(t, "first", "A")

	f.store.Codes = []string{"AAAAAA", "AAAAAA", "BBBBBB"}
	s := f.create(t, "second", "B")
	if s.room.Code != "BBBBBB" {
		t.Errorf("code = %q, want BBBBBB", s.room.Code)
	}

	f.store.Codes = []string{"AAAAAA", "AAAAAA", "AAAAAA", "AAAAAA", "AAAAAA"}
	if _, _, err := f.rooms.CreateRoom(context.Background(), "third", "C"); err == nil {
		t.Error("expected error after exhausting attempts")
	}
}

func TestJoinRoom(t *testing.T) {
	f := newFixture()
	host := f.create(t, "Room", "Host")
	ctx := context.Background()

	_, p, err := f.rooms.JoinRoom(ctx, strings.ToLower(host.room.Code), " Guest ")
	wantKind(t, err, nil)
	if p.IsHost || p.DisplayName != "Guest" || p.RoomID != host.room.ID {
		t.Errorf("bad participant: %+v", p)
	}

	_, _, err = f.rooms.JoinRoom(ctx, "NOPE00", "Guest")
	wantKind(t, err, ErrNotFound)
	_, _, err = f.rooms.JoinRoom(ctx, host.room.Code, "")
	wantKind(t, err, ErrInvalid)

	_, participants, err := f.rooms.GetRoom(ctx, host.room.Code)
	wantKind(t, err, nil)
	if len(participants) != 2 {
		t.Errorf("participants = %d, want 2", len(participants))
	}
}

func TestAuthorize(t *testing.T) {
	f := newFixture()
	a := f.create(t, "A", "HostA")
	b := f.create(t, "B", "HostB")
	tests := []struct {
		name, code, token string
		want              error
	}{
		{"member", a.room.Code, a.p.SessionToken, nil},
		{"lowercase code", strings.ToLower(a.room.Code), a.p.SessionToken, nil},
		{"unknown room", "ZZZZZZ", a.p.SessionToken, ErrNotFound},
		{"missing token", a.room.Code, "", ErrUnauthorized},
		{"malformed token", a.room.Code, "not-a-uuid", ErrUnauthorized},
		{"unknown token", a.room.Code, "00000000-0000-0000-0000-000000000000", ErrUnauthorized},
		{"other room's token", a.room.Code, b.p.SessionToken, ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, p, err := f.rooms.Authorize(context.Background(), tt.code, tt.token)
			wantKind(t, err, tt.want)
			if err == nil && p.ID != a.p.ID {
				t.Errorf("participant = %s, want %s", p.ID, a.p.ID)
			}
		})
	}
}

func TestLeaveTransfersHost(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	host := f.create(t, "Room", "Host")
	second := f.join(t, host.room.Code, "Second")
	third := f.join(t, host.room.Code, "Third")

	// A non-host leaving does not change the host.
	res, err := f.rooms.Leave(ctx, host.room.Code, third.p.SessionToken)
	wantKind(t, err, nil)
	if res.ParticipantID != third.p.ID || res.NewHostID != "" {
		t.Errorf("non-host leave result = %+v", res)
	}

	// The host leaving promotes the earliest-joined remaining participant.
	res, err = f.rooms.Leave(ctx, host.room.Code, host.p.SessionToken)
	wantKind(t, err, nil)
	if res.NewHostID != second.p.ID {
		t.Errorf("NewHostID = %q, want %q", res.NewHostID, second.p.ID)
	}
	_, me, err := f.rooms.Authorize(ctx, host.room.Code, second.p.SessionToken)
	wantKind(t, err, nil)
	if !me.IsHost {
		t.Error("second participant was not promoted")
	}

	// The old token no longer works.
	_, err = f.rooms.Leave(ctx, host.room.Code, host.p.SessionToken)
	wantKind(t, err, ErrUnauthorized)

	// The last participant leaving leaves an empty room and no new host.
	res, err = f.rooms.Leave(ctx, host.room.Code, second.p.SessionToken)
	wantKind(t, err, nil)
	if res.NewHostID != "" {
		t.Errorf("NewHostID = %q, want empty", res.NewHostID)
	}
}

func TestLeaveOtherRoomForbidden(t *testing.T) {
	f := newFixture()
	a := f.create(t, "A", "HostA")
	b := f.create(t, "B", "HostB")
	_, err := f.rooms.Leave(context.Background(), a.room.Code, b.p.SessionToken)
	wantKind(t, err, ErrForbidden)
}

func TestVotingStateMachine(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	host := f.create(t, "Room", "Host")
	guest := f.join(t, host.room.Code, "Guest")
	code := host.room.Code

	// Only the host can start a round.
	_, err := f.voting.StartRound(ctx, code, guest.p.SessionToken, "Story")
	wantKind(t, err, ErrForbidden)
	_, err = f.voting.StartRound(ctx, code, host.p.SessionToken, "  ")
	wantKind(t, err, ErrInvalid)
	_, err = f.voting.StartRound(ctx, code, host.p.SessionToken, strings.Repeat("x", 201))
	wantKind(t, err, ErrInvalid)

	round, err := f.voting.StartRound(ctx, code, host.p.SessionToken, " Story 1 ")
	wantKind(t, err, nil)
	if round.StoryTitle != "Story 1" || round.Status != model.RoundStatusVoting {
		t.Fatalf("round = %+v", round)
	}
	if r, _ := f.store.Room(host.room.ID); r.Status != model.RoomStatusVoting {
		t.Errorf("room status = %q, want voting", r.Status)
	}

	// Vote validation.
	for _, v := range model.FibonacciDeck {
		_, err := f.voting.CastVote(ctx, code, guest.p.SessionToken, round.ID, v)
		wantKind(t, err, nil)
	}
	for _, v := range []string{"", "4", "100", "coffee", "??"} {
		_, err := f.voting.CastVote(ctx, code, guest.p.SessionToken, round.ID, v)
		wantKind(t, err, ErrInvalid)
	}
	_, err = f.voting.CastVote(ctx, code, guest.p.SessionToken, "not-a-uuid", "5")
	wantKind(t, err, ErrNotFound)
	_, err = f.voting.CastVote(ctx, code, guest.p.SessionToken, "00000000-0000-0000-0000-000000000000", "5")
	wantKind(t, err, ErrNotFound)

	// Re-voting replaces the previous vote.
	_, err = f.voting.CastVote(ctx, code, guest.p.SessionToken, round.ID, "8")
	wantKind(t, err, nil)
	_, err = f.voting.CastVote(ctx, code, host.p.SessionToken, round.ID, "☕")
	wantKind(t, err, nil)

	// Votes are hidden while voting, except the requester's own.
	_, votes, err := f.voting.CurrentRound(ctx, code, guest.p.SessionToken)
	wantKind(t, err, nil)
	if len(votes) != 2 {
		t.Fatalf("votes = %d, want 2", len(votes))
	}
	for _, v := range votes {
		want := ""
		if v.ParticipantID == guest.p.ID {
			want = "8"
		}
		if v.Value != want {
			t.Errorf("vote of %s = %q, want %q", v.ParticipantID, v.Value, want)
		}
	}
	_, votes, _ = f.voting.CurrentRound(ctx, code, "")
	for _, v := range votes {
		if v.Value != "" {
			t.Errorf("anonymous caller sees value %q", v.Value)
		}
	}

	// Reveal: host only, once.
	_, err = f.voting.Reveal(ctx, code, guest.p.SessionToken, round.ID)
	wantKind(t, err, ErrForbidden)
	revealed, err := f.voting.Reveal(ctx, code, host.p.SessionToken, round.ID)
	wantKind(t, err, nil)
	if len(revealed) != 2 {
		t.Fatalf("revealed = %+v", revealed)
	}
	for _, v := range revealed {
		if v.DisplayName == "" || v.Value == "" {
			t.Errorf("incomplete revealed vote %+v", v)
		}
	}
	_, err = f.voting.Reveal(ctx, code, host.p.SessionToken, round.ID)
	wantKind(t, err, ErrConflict)

	// No voting on a revealed round; everyone sees the values.
	_, err = f.voting.CastVote(ctx, code, guest.p.SessionToken, round.ID, "3")
	wantKind(t, err, ErrConflict)
	cur, votes, _ := f.voting.CurrentRound(ctx, code, "")
	if cur.Status != model.RoundStatusRevealed {
		t.Errorf("status = %q", cur.Status)
	}
	for _, v := range votes {
		if v.Value == "" {
			t.Error("revealed vote value hidden")
		}
	}

	// Reset: host only; clears votes and re-opens voting.
	_, err = f.voting.Reset(ctx, code, guest.p.SessionToken, round.ID)
	wantKind(t, err, ErrForbidden)
	reset, err := f.voting.Reset(ctx, code, host.p.SessionToken, round.ID)
	wantKind(t, err, nil)
	if reset.Status != model.RoundStatusVoting || reset.ID != round.ID {
		t.Errorf("reset round = %+v", reset)
	}
	_, votes, _ = f.voting.CurrentRound(ctx, code, "")
	if votes == nil || len(votes) != 0 {
		t.Errorf("votes after reset = %#v, want empty non-nil", votes)
	}
	_, err = f.voting.CastVote(ctx, code, guest.p.SessionToken, round.ID, "3")
	wantKind(t, err, nil)

	// A new round becomes the current one.
	round2, err := f.voting.StartRound(ctx, code, host.p.SessionToken, "Story 2")
	wantKind(t, err, nil)
	cur, votes, _ = f.voting.CurrentRound(ctx, code, guest.p.SessionToken)
	if cur.ID != round2.ID || len(votes) != 0 {
		t.Errorf("current = %s with %d votes, want %s with 0", cur.ID, len(votes), round2.ID)
	}
}

func TestCurrentRoundEmptyRoom(t *testing.T) {
	f := newFixture()
	host := f.create(t, "Room", "Host")
	round, votes, err := f.voting.CurrentRound(context.Background(), host.room.Code, "")
	wantKind(t, err, nil)
	if round != nil || votes == nil || len(votes) != 0 {
		t.Errorf("round = %v, votes = %#v", round, votes)
	}
	_, _, err = f.voting.CurrentRound(context.Background(), "NOPE00", "")
	wantKind(t, err, ErrNotFound)
}

func TestCurrentRoundForeignTokenSeesNothing(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	a := f.create(t, "A", "HostA")
	b := f.create(t, "B", "HostB")
	round, _ := f.voting.StartRound(ctx, a.room.Code, a.p.SessionToken, "S")
	_, _ = f.voting.CastVote(ctx, a.room.Code, a.p.SessionToken, round.ID, "5")
	_, votes, err := f.voting.CurrentRound(ctx, a.room.Code, b.p.SessionToken)
	wantKind(t, err, nil)
	if len(votes) != 1 || votes[0].Value != "" {
		t.Errorf("votes = %+v", votes)
	}
}

func TestCrossRoomRejection(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	a := f.create(t, "A", "HostA")
	b := f.create(t, "B", "HostB")
	roundA, err := f.voting.StartRound(ctx, a.room.Code, a.p.SessionToken, "A story")
	wantKind(t, err, nil)
	roundB, err := f.voting.StartRound(ctx, b.room.Code, b.p.SessionToken, "B story")
	wantKind(t, err, nil)

	// Host of B acting on room A: forbidden.
	_, err = f.voting.StartRound(ctx, a.room.Code, b.p.SessionToken, "x")
	wantKind(t, err, ErrForbidden)
	_, err = f.voting.CastVote(ctx, a.room.Code, b.p.SessionToken, roundA.ID, "5")
	wantKind(t, err, ErrForbidden)
	_, err = f.voting.Reveal(ctx, a.room.Code, b.p.SessionToken, roundA.ID)
	wantKind(t, err, ErrForbidden)
	_, err = f.voting.Reset(ctx, a.room.Code, b.p.SessionToken, roundA.ID)
	wantKind(t, err, ErrForbidden)

	// Host of A using room A's code but room B's round: not found.
	_, err = f.voting.CastVote(ctx, a.room.Code, a.p.SessionToken, roundB.ID, "5")
	wantKind(t, err, ErrNotFound)
	_, err = f.voting.Reveal(ctx, a.room.Code, a.p.SessionToken, roundB.ID)
	wantKind(t, err, ErrNotFound)
	_, err = f.voting.Reset(ctx, a.room.Code, a.p.SessionToken, roundB.ID)
	wantKind(t, err, ErrNotFound)

	// Room B's round is untouched.
	cur, _, _ := f.voting.CurrentRound(ctx, b.room.Code, "")
	if cur.Status != model.RoundStatusVoting {
		t.Errorf("room B round status = %q", cur.Status)
	}
}

func TestStoreFailuresAreInternal(t *testing.T) {
	f := newFixture()
	host := f.create(t, "Room", "Host")
	f.store.Err = errors.New("db down")
	_, _, err := f.rooms.GetRoom(context.Background(), host.room.Code)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, kind := range []error{ErrInvalid, ErrUnauthorized, ErrForbidden, ErrNotFound, ErrConflict} {
		if errors.Is(err, kind) {
			t.Errorf("infrastructure error classified as %v", kind)
		}
	}
}
