// Package memstore provides an in-memory implementation of the service
// store interfaces. It mirrors the semantics of the PostgreSQL repositories
// (including sentinel errors and host transfer) and is intended for tests.
package memstore

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
)

// Store is a concurrency-safe, in-memory RoomStore and RoundStore.
// The zero value is not usable; call New.
type Store struct {
	mu           sync.Mutex
	rooms        []model.Room
	participants []model.Participant // in join order
	rounds       []model.VotingRound // in creation order
	votes        []model.Vote

	// Err, when non-nil, is returned by every method (to simulate outages).
	Err error
	// Codes, when non-empty, forces the codes assigned to newly created
	// rooms to be taken from this queue instead of the caller's value.
	Codes []string
}

// New returns an empty Store.
func New() *Store { return &Store{} }

// CreateRoomWithHost stores room and host, failing with ErrDuplicate if the code is taken.
func (s *Store) CreateRoomWithHost(_ context.Context, room *model.Room, host *model.Participant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if len(s.Codes) > 0 {
		room.Code, s.Codes = s.Codes[0], s.Codes[1:]
	}
	for _, r := range s.rooms {
		if r.Code == room.Code {
			return repository.ErrDuplicate
		}
	}
	room.ID = uuid.NewString()
	room.CreatedAt = time.Now()
	s.rooms = append(s.rooms, *room)
	host.RoomID = room.ID
	s.addParticipantLocked(host)
	return nil
}

// GetRoomByCode returns the room with the given code.
func (s *Store) GetRoomByCode(_ context.Context, code string) (*model.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	for _, r := range s.rooms {
		if r.Code == code {
			return &r, nil
		}
	}
	return nil, repository.ErrNotFound
}

// UpdateRoomStatus sets the status of a room.
func (s *Store) UpdateRoomStatus(_ context.Context, roomID string, status model.RoomStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for i := range s.rooms {
		if s.rooms[i].ID == roomID {
			s.rooms[i].Status = status
		}
	}
	return nil
}

// AddParticipant stores a participant.
func (s *Store) AddParticipant(_ context.Context, p *model.Participant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.addParticipantLocked(p)
	return nil
}

func (s *Store) addParticipantLocked(p *model.Participant) {
	p.ID = uuid.NewString()
	p.JoinedAt = time.Now()
	s.participants = append(s.participants, *p)
}

// GetParticipantByToken returns the participant owning token.
func (s *Store) GetParticipantByToken(_ context.Context, token string) (*model.Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	for _, p := range s.participants {
		if p.SessionToken == token {
			return &p, nil
		}
	}
	return nil, repository.ErrNotFound
}

// ListParticipants returns a room's participants in join order.
func (s *Store) ListParticipants(_ context.Context, roomID string) ([]model.Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	out := []model.Participant{}
	for _, p := range s.participants {
		if p.RoomID == roomID {
			out = append(out, p)
		}
	}
	return out, nil
}

// RemoveParticipant deletes a participant (and their votes) and promotes the
// earliest-joined remaining participant if the host left.
func (s *Store) RemoveParticipant(_ context.Context, roomID, participantID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return "", s.Err
	}
	idx := slices.IndexFunc(s.participants, func(p model.Participant) bool {
		return p.ID == participantID && p.RoomID == roomID
	})
	if idx < 0 {
		return "", repository.ErrNotFound
	}
	wasHost := s.participants[idx].IsHost
	s.participants = slices.Delete(s.participants, idx, idx+1)
	s.votes = slices.DeleteFunc(s.votes, func(v model.Vote) bool { return v.ParticipantID == participantID })
	if !wasHost {
		return "", nil
	}
	for i := range s.participants {
		if s.participants[i].RoomID == roomID {
			s.participants[i].IsHost = true
			return s.participants[i].ID, nil
		}
	}
	return "", nil
}

// CreateRound stores a round.
func (s *Store) CreateRound(_ context.Context, round *model.VotingRound) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	round.ID = uuid.NewString()
	round.CreatedAt = time.Now()
	s.rounds = append(s.rounds, *round)
	return nil
}

// GetRound returns a round by ID.
func (s *Store) GetRound(_ context.Context, roundID string) (*model.VotingRound, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	for _, r := range s.rounds {
		if r.ID == roundID {
			return &r, nil
		}
	}
	return nil, repository.ErrNotFound
}

// GetLatestRound returns the most recently created round of a room.
func (s *Store) GetLatestRound(_ context.Context, roomID string) (*model.VotingRound, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	for i := len(s.rounds) - 1; i >= 0; i-- {
		if s.rounds[i].RoomID == roomID {
			r := s.rounds[i]
			return &r, nil
		}
	}
	return nil, repository.ErrNotFound
}

// RevealRound moves a voting round to revealed, reporting false otherwise.
func (s *Store) RevealRound(_ context.Context, roundID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return false, s.Err
	}
	for i := range s.rounds {
		if s.rounds[i].ID == roundID && s.rounds[i].Status == model.RoundStatusVoting {
			s.rounds[i].Status = model.RoundStatusRevealed
			return true, nil
		}
	}
	return false, nil
}

// ResetRound deletes the round's votes and sets it back to voting.
func (s *Store) ResetRound(_ context.Context, roundID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	s.votes = slices.DeleteFunc(s.votes, func(v model.Vote) bool { return v.RoundID == roundID })
	for i := range s.rounds {
		if s.rounds[i].ID == roundID {
			s.rounds[i].Status = model.RoundStatusVoting
		}
	}
	return nil
}

// UpsertVote inserts or replaces the participant's vote in a round.
func (s *Store) UpsertVote(_ context.Context, vote *model.Vote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	vote.VotedAt = time.Now()
	for i, v := range s.votes {
		if v.RoundID == vote.RoundID && v.ParticipantID == vote.ParticipantID {
			vote.ID = v.ID
			s.votes[i] = *vote
			return nil
		}
	}
	vote.ID = uuid.NewString()
	s.votes = append(s.votes, *vote)
	return nil
}

// ListVotes returns a round's votes in cast order.
func (s *Store) ListVotes(_ context.Context, roundID string) ([]model.Vote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	out := []model.Vote{}
	for _, v := range s.votes {
		if v.RoundID == roundID {
			out = append(out, v)
		}
	}
	return out, nil
}

// Room returns a copy of the room with the given ID (test helper).
func (s *Store) Room(roomID string) (model.Room, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rooms {
		if r.ID == roomID {
			return r, true
		}
	}
	return model.Room{}, false
}
