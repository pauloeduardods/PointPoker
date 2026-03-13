package service

import (
	"fmt"

	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
)

// VotingService contains business logic for voting operations.
type VotingService struct {
	voteRepo *repository.VoteRepository
	roomRepo *repository.RoomRepository
}

// NewVotingService creates a new VotingService.
func NewVotingService(voteRepo *repository.VoteRepository, roomRepo *repository.RoomRepository) *VotingService {
	return &VotingService{voteRepo: voteRepo, roomRepo: roomRepo}
}

// StartRound creates a new voting round and sets the room to voting status.
func (s *VotingService) StartRound(roomID, storyTitle string) (*model.VotingRound, error) {
	round := &model.VotingRound{
		RoomID:     roomID,
		StoryTitle: storyTitle,
		Status:     model.RoundStatusVoting,
	}
	if err := s.voteRepo.CreateRound(round); err != nil {
		return nil, fmt.Errorf("create round: %w", err)
	}
	if err := s.roomRepo.UpdateStatus(roomID, model.RoomStatusVoting); err != nil {
		return nil, fmt.Errorf("update room status: %w", err)
	}
	return round, nil
}

// CastVote records a participant's vote, validating the value against the Fibonacci deck.
func (s *VotingService) CastVote(roundID, participantID, value string) (*model.Vote, error) {
	valid := false
	for _, v := range model.FibonacciDeck {
		if v == value {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid vote value: %s", value)
	}

	round, err := s.voteRepo.GetRound(roundID)
	if err != nil {
		return nil, fmt.Errorf("round not found: %w", err)
	}
	if round.Status != model.RoundStatusVoting {
		return nil, fmt.Errorf("round is not in voting status")
	}

	vote := &model.Vote{
		RoundID:       roundID,
		ParticipantID: participantID,
		Value:         value,
	}
	if err := s.voteRepo.CastVote(vote); err != nil {
		return nil, fmt.Errorf("cast vote: %w", err)
	}
	return vote, nil
}

// RevealVotes marks the round as revealed and returns all votes.
func (s *VotingService) RevealVotes(roundID string) ([]model.Vote, error) {
	round, err := s.voteRepo.GetRound(roundID)
	if err != nil {
		return nil, fmt.Errorf("round not found: %w", err)
	}
	if round.Status != model.RoundStatusVoting {
		return nil, fmt.Errorf("round is not in voting status")
	}

	if err := s.voteRepo.UpdateRoundStatus(roundID, model.RoundStatusRevealed); err != nil {
		return nil, fmt.Errorf("update round status: %w", err)
	}
	if err := s.roomRepo.UpdateStatus(round.RoomID, model.RoomStatusRevealed); err != nil {
		return nil, fmt.Errorf("update room status: %w", err)
	}

	return s.voteRepo.GetVotesByRound(roundID)
}

// ResetRound clears all votes and sets the round back to voting status.
func (s *VotingService) ResetRound(roundID string) error {
	round, err := s.voteRepo.GetRound(roundID)
	if err != nil {
		return fmt.Errorf("round not found: %w", err)
	}

	if err := s.voteRepo.DeleteVotesByRound(roundID); err != nil {
		return fmt.Errorf("delete votes: %w", err)
	}
	if err := s.voteRepo.UpdateRoundStatus(roundID, model.RoundStatusVoting); err != nil {
		return fmt.Errorf("update round status: %w", err)
	}
	if err := s.roomRepo.UpdateStatus(round.RoomID, model.RoomStatusVoting); err != nil {
		return fmt.Errorf("update room status: %w", err)
	}
	return nil
}

// GetVotesByRound returns all votes for a round.
func (s *VotingService) GetVotesByRound(roundID string) ([]model.Vote, error) {
	return s.voteRepo.GetVotesByRound(roundID)
}

// GetRound retrieves a voting round by ID.
func (s *VotingService) GetRound(roundID string) (*model.VotingRound, error) {
	return s.voteRepo.GetRound(roundID)
}

// GetActiveRound returns the current active round for a room.
func (s *VotingService) GetActiveRound(roomID string) (*model.VotingRound, error) {
	return s.voteRepo.GetActiveRound(roomID)
}

// GetLatestRound returns the most recent round for a room.
func (s *VotingService) GetLatestRound(roomID string) (*model.VotingRound, error) {
	return s.voteRepo.GetLatestRound(roomID)
}
