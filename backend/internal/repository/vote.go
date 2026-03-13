package repository

import (
	"database/sql"

	"github.com/pauloedsg/pointpoker/internal/model"
)

// VoteRepository handles database operations for voting rounds and votes.
type VoteRepository struct {
	db *sql.DB
}

// NewVoteRepository creates a new VoteRepository.
func NewVoteRepository(db *sql.DB) *VoteRepository {
	return &VoteRepository{db: db}
}

// CreateRound inserts a new voting round.
func (r *VoteRepository) CreateRound(round *model.VotingRound) error {
	return r.db.QueryRow(
		`INSERT INTO voting_rounds (room_id, story_title, status)
		 VALUES ($1, $2, $3) RETURNING id, created_at`,
		round.RoomID, round.StoryTitle, round.Status,
	).Scan(&round.ID, &round.CreatedAt)
}

// GetRound retrieves a voting round by ID.
func (r *VoteRepository) GetRound(roundID string) (*model.VotingRound, error) {
	round := &model.VotingRound{}
	err := r.db.QueryRow(
		`SELECT id, room_id, story_title, status, created_at
		 FROM voting_rounds WHERE id = $1`,
		roundID,
	).Scan(&round.ID, &round.RoomID, &round.StoryTitle, &round.Status, &round.CreatedAt)
	if err != nil {
		return nil, err
	}
	return round, nil
}

// GetActiveRound returns the current active (voting) round for a room.
func (r *VoteRepository) GetActiveRound(roomID string) (*model.VotingRound, error) {
	round := &model.VotingRound{}
	err := r.db.QueryRow(
		`SELECT id, room_id, story_title, status, created_at
		 FROM voting_rounds
		 WHERE room_id = $1 AND status = 'voting'
		 ORDER BY created_at DESC LIMIT 1`,
		roomID,
	).Scan(&round.ID, &round.RoomID, &round.StoryTitle, &round.Status, &round.CreatedAt)
	if err != nil {
		return nil, err
	}
	return round, nil
}

// GetLatestRound returns the most recent round for a room (any status).
func (r *VoteRepository) GetLatestRound(roomID string) (*model.VotingRound, error) {
	round := &model.VotingRound{}
	err := r.db.QueryRow(
		`SELECT id, room_id, story_title, status, created_at
		 FROM voting_rounds
		 WHERE room_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		roomID,
	).Scan(&round.ID, &round.RoomID, &round.StoryTitle, &round.Status, &round.CreatedAt)
	if err != nil {
		return nil, err
	}
	return round, nil
}

// UpdateRoundStatus changes the status of a voting round.
func (r *VoteRepository) UpdateRoundStatus(roundID string, status model.RoundStatus) error {
	_, err := r.db.Exec(
		`UPDATE voting_rounds SET status = $1 WHERE id = $2`,
		status, roundID,
	)
	return err
}

// CastVote inserts or updates a vote (upsert on round_id + participant_id).
func (r *VoteRepository) CastVote(vote *model.Vote) error {
	return r.db.QueryRow(
		`INSERT INTO votes (round_id, participant_id, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (round_id, participant_id)
		 DO UPDATE SET value = $3, voted_at = NOW()
		 RETURNING id, voted_at`,
		vote.RoundID, vote.ParticipantID, vote.Value,
	).Scan(&vote.ID, &vote.VotedAt)
}

// GetVotesByRound returns all votes for a given round.
func (r *VoteRepository) GetVotesByRound(roundID string) ([]model.Vote, error) {
	rows, err := r.db.Query(
		`SELECT id, round_id, participant_id, value, voted_at
		 FROM votes WHERE round_id = $1`,
		roundID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []model.Vote
	for rows.Next() {
		var v model.Vote
		if err := rows.Scan(&v.ID, &v.RoundID, &v.ParticipantID, &v.Value, &v.VotedAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}

// DeleteVotesByRound removes all votes for a round.
func (r *VoteRepository) DeleteVotesByRound(roundID string) error {
	_, err := r.db.Exec(`DELETE FROM votes WHERE round_id = $1`, roundID)
	return err
}
