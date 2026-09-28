package repository

import (
	"context"
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

const roundColumns = `id, room_id, story_title, status, created_at`

func scanRound(s scanner) (*model.VotingRound, error) {
	round := &model.VotingRound{}
	if err := s.Scan(&round.ID, &round.RoomID, &round.StoryTitle, &round.Status, &round.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return round, nil
}

// CreateRound inserts a new voting round and populates its ID and CreatedAt.
func (r *VoteRepository) CreateRound(ctx context.Context, round *model.VotingRound) error {
	return mapErr(r.db.QueryRowContext(ctx,
		`INSERT INTO voting_rounds (room_id, story_title, status)
		 VALUES ($1, $2, $3) RETURNING id, created_at`,
		round.RoomID, round.StoryTitle, round.Status,
	).Scan(&round.ID, &round.CreatedAt))
}

// GetRound retrieves a voting round by ID.
func (r *VoteRepository) GetRound(ctx context.Context, roundID string) (*model.VotingRound, error) {
	return scanRound(r.db.QueryRowContext(ctx,
		`SELECT `+roundColumns+` FROM voting_rounds WHERE id = $1`, roundID))
}

// GetLatestRound returns the most recently created round of a room (any status).
func (r *VoteRepository) GetLatestRound(ctx context.Context, roomID string) (*model.VotingRound, error) {
	return scanRound(r.db.QueryRowContext(ctx,
		`SELECT `+roundColumns+` FROM voting_rounds
		 WHERE room_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		roomID))
}

// RevealRound atomically moves a round from voting to revealed. It reports
// false when the round was not in voting status (e.g. already revealed).
func (r *VoteRepository) RevealRound(ctx context.Context, roundID string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE voting_rounds SET status = $1 WHERE id = $2 AND status = $3`,
		model.RoundStatusRevealed, roundID, model.RoundStatusVoting)
	if err != nil {
		return false, mapErr(err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ResetRound deletes every vote of the round and sets it back to voting, in
// a single transaction.
func (r *VoteRepository) ResetRound(ctx context.Context, roundID string) error {
	return mapErr(withTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM votes WHERE round_id = $1`, roundID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE voting_rounds SET status = $1 WHERE id = $2`, model.RoundStatusVoting, roundID)
		return err
	}))
}

// UpsertVote inserts or updates a vote (unique on round_id + participant_id)
// and populates its ID and VotedAt.
func (r *VoteRepository) UpsertVote(ctx context.Context, vote *model.Vote) error {
	return mapErr(r.db.QueryRowContext(ctx,
		`INSERT INTO votes (round_id, participant_id, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (round_id, participant_id)
		 DO UPDATE SET value = EXCLUDED.value, voted_at = NOW()
		 RETURNING id, voted_at`,
		vote.RoundID, vote.ParticipantID, vote.Value,
	).Scan(&vote.ID, &vote.VotedAt))
}

// ListVotes returns all votes of a round, oldest first. It never returns a nil slice.
func (r *VoteRepository) ListVotes(ctx context.Context, roundID string) ([]model.Vote, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, round_id, participant_id, value, voted_at
		 FROM votes WHERE round_id = $1 ORDER BY voted_at, id`,
		roundID,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	votes := []model.Vote{}
	for rows.Next() {
		var v model.Vote
		if err := rows.Scan(&v.ID, &v.RoundID, &v.ParticipantID, &v.Value, &v.VotedAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}
