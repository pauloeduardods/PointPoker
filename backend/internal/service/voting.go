package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
)

// VotingService contains business logic for rounds and votes.
type VotingService struct {
	rounds RoundStore
	rooms  RoomStore
}

// NewVotingService creates a new VotingService.
func NewVotingService(rounds RoundStore, rooms RoomStore) *VotingService {
	return &VotingService{rounds: rounds, rooms: rooms}
}

// StartRound creates a new voting round, which becomes the room's current
// round. Only the host may start a round.
func (s *VotingService) StartRound(ctx context.Context, code, token, storyTitle string) (*model.VotingRound, error) {
	room, _, err := s.authorizeHost(ctx, code, token, "start a round")
	if err != nil {
		return nil, err
	}
	storyTitle, err = cleanText("story_title", storyTitle, MaxStoryTitleLen)
	if err != nil {
		return nil, err
	}

	round := &model.VotingRound{RoomID: room.ID, StoryTitle: storyTitle, Status: model.RoundStatusVoting}
	if err := s.rounds.CreateRound(ctx, round); err != nil {
		return nil, fmt.Errorf("create round: %w", err)
	}
	if err := s.rooms.UpdateRoomStatus(ctx, room.ID, model.RoomStatusVoting); err != nil {
		return nil, fmt.Errorf("update room status: %w", err)
	}
	return round, nil
}

// CastVote records (or replaces) the caller's vote in a round that is still
// open for voting.
func (s *VotingService) CastVote(ctx context.Context, code, token, roundID, value string) (*model.Vote, error) {
	room, p, err := authorize(ctx, s.rooms, code, token)
	if err != nil {
		return nil, err
	}
	value = strings.TrimSpace(value)
	if !model.IsValidVote(value) {
		return nil, newError(ErrInvalid, "value must be one of %s", strings.Join(model.FibonacciDeck, ", "))
	}
	round, err := s.roundInRoom(ctx, room, roundID)
	if err != nil {
		return nil, err
	}
	if round.Status != model.RoundStatusVoting {
		return nil, newError(ErrConflict, "round is not open for voting")
	}

	vote := &model.Vote{RoundID: round.ID, ParticipantID: p.ID, Value: value}
	if err := s.rounds.UpsertVote(ctx, vote); err != nil {
		return nil, fmt.Errorf("cast vote: %w", err)
	}
	return vote, nil
}

// Reveal closes voting on a round and returns every vote with the voter's
// display name. Only the host may reveal; revealing twice is a conflict.
func (s *VotingService) Reveal(ctx context.Context, code, token, roundID string) ([]model.RevealedVote, error) {
	room, _, err := s.authorizeHost(ctx, code, token, "reveal votes")
	if err != nil {
		return nil, err
	}
	round, err := s.roundInRoom(ctx, room, roundID)
	if err != nil {
		return nil, err
	}
	ok, err := s.rounds.RevealRound(ctx, round.ID)
	if err != nil {
		return nil, fmt.Errorf("reveal round: %w", err)
	}
	if !ok {
		return nil, newError(ErrConflict, "round is already revealed")
	}
	if err := s.rooms.UpdateRoomStatus(ctx, room.ID, model.RoomStatusRevealed); err != nil {
		return nil, fmt.Errorf("update room status: %w", err)
	}

	votes, err := s.rounds.ListVotes(ctx, round.ID)
	if err != nil {
		return nil, fmt.Errorf("list votes: %w", err)
	}
	participants, err := s.rooms.ListParticipants(ctx, room.ID)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	names := make(map[string]string, len(participants))
	for _, p := range participants {
		names[p.ID] = p.DisplayName
	}
	results := make([]model.RevealedVote, 0, len(votes))
	for _, v := range votes {
		results = append(results, model.RevealedVote{
			ParticipantID: v.ParticipantID,
			DisplayName:   names[v.ParticipantID],
			Value:         v.Value,
		})
	}
	return results, nil
}

// Reset re-opens a round for a revote: all its votes are deleted and its
// status goes back to voting. Only the host may reset.
func (s *VotingService) Reset(ctx context.Context, code, token, roundID string) (*model.VotingRound, error) {
	room, _, err := s.authorizeHost(ctx, code, token, "reset a round")
	if err != nil {
		return nil, err
	}
	round, err := s.roundInRoom(ctx, room, roundID)
	if err != nil {
		return nil, err
	}
	if err := s.rounds.ResetRound(ctx, round.ID); err != nil {
		return nil, fmt.Errorf("reset round: %w", err)
	}
	if err := s.rooms.UpdateRoomStatus(ctx, room.ID, model.RoomStatusVoting); err != nil {
		return nil, fmt.Errorf("update room status: %w", err)
	}
	round.Status = model.RoundStatusVoting
	return round, nil
}

// CurrentRound returns the room's latest round (nil if none) and its votes.
// The votes slice is never nil. While the round is voting, every vote value
// is blanked except the one cast by the participant owning token (if token
// is empty or does not identify a member of the room, all values are hidden).
func (s *VotingService) CurrentRound(ctx context.Context, code, token string) (*model.VotingRound, []model.Vote, error) {
	room, err := getRoom(ctx, s.rooms, code)
	if err != nil {
		return nil, nil, err
	}
	round, err := s.rounds.GetLatestRound(ctx, room.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, []model.Vote{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get latest round: %w", err)
	}
	votes, err := s.rounds.ListVotes(ctx, round.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list votes: %w", err)
	}
	if votes == nil {
		votes = []model.Vote{}
	}

	if round.Status == model.RoundStatusVoting {
		requesterID := ""
		if token != "" {
			// The token is optional here: an invalid or foreign token simply
			// means the caller sees no values.
			if p, err := participantByToken(ctx, s.rooms, token); err == nil && p.RoomID == room.ID {
				requesterID = p.ID
			}
		}
		for i := range votes {
			if votes[i].ParticipantID != requesterID {
				votes[i].Value = ""
			}
		}
	}
	return round, votes, nil
}

func (s *VotingService) authorizeHost(ctx context.Context, code, token, action string) (*model.Room, *model.Participant, error) {
	room, p, err := authorize(ctx, s.rooms, code, token)
	if err != nil {
		return nil, nil, err
	}
	if !p.IsHost {
		return nil, nil, newError(ErrForbidden, "only the host can %s", action)
	}
	return room, p, nil
}

// roundInRoom loads a round and checks that it belongs to room.
func (s *VotingService) roundInRoom(ctx context.Context, room *model.Room, roundID string) (*model.VotingRound, error) {
	notFound := newError(ErrNotFound, "round not found")
	if !isUUID(roundID) {
		return nil, notFound
	}
	round, err := s.rounds.GetRound(ctx, roundID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, fmt.Errorf("get round: %w", err)
	}
	if round.RoomID != room.ID {
		return nil, notFound
	}
	return round, nil
}
