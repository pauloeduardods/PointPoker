package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/pauloedsg/pointpoker/internal/model"
)

// RoomRepository handles database operations for rooms and participants.
type RoomRepository struct {
	db *sql.DB
}

// NewRoomRepository creates a new RoomRepository.
func NewRoomRepository(db *sql.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

const participantColumns = `id, room_id, display_name, session_token, is_host, joined_at`

type scanner interface{ Scan(dest ...any) error }

func scanParticipant(s scanner) (*model.Participant, error) {
	p := &model.Participant{}
	if err := s.Scan(&p.ID, &p.RoomID, &p.DisplayName, &p.SessionToken, &p.IsHost, &p.JoinedAt); err != nil {
		return nil, err
	}
	return p, nil
}

// CreateRoomWithHost inserts room and its host participant in a single
// transaction, populating the generated IDs and timestamps. It returns
// ErrDuplicate when the room code is already taken.
func (r *RoomRepository) CreateRoomWithHost(ctx context.Context, room *model.Room, host *model.Participant) error {
	return mapErr(withTx(ctx, r.db, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO rooms (code, name, status) VALUES ($1, $2, $3) RETURNING id, created_at`,
			room.Code, room.Name, room.Status,
		).Scan(&room.ID, &room.CreatedAt); err != nil {
			return err
		}
		host.RoomID = room.ID
		return insertParticipant(ctx, tx, host)
	}))
}

// GetRoomByCode retrieves a room by its unique code.
func (r *RoomRepository) GetRoomByCode(ctx context.Context, code string) (*model.Room, error) {
	room := &model.Room{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, code, name, status, created_at FROM rooms WHERE code = $1`,
		code,
	).Scan(&room.ID, &room.Code, &room.Name, &room.Status, &room.CreatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return room, nil
}

// UpdateRoomStatus changes the status of a room.
func (r *RoomRepository) UpdateRoomStatus(ctx context.Context, roomID string, status model.RoomStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE rooms SET status = $1 WHERE id = $2`, status, roomID)
	return mapErr(err)
}

// AddParticipant inserts a participant and populates its ID and JoinedAt.
func (r *RoomRepository) AddParticipant(ctx context.Context, p *model.Participant) error {
	return mapErr(insertParticipant(ctx, r.db, p))
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertParticipant(ctx context.Context, q queryRower, p *model.Participant) error {
	return q.QueryRowContext(ctx,
		`INSERT INTO participants (room_id, display_name, session_token, is_host)
		 VALUES ($1, $2, $3, $4) RETURNING id, joined_at`,
		p.RoomID, p.DisplayName, p.SessionToken, p.IsHost,
	).Scan(&p.ID, &p.JoinedAt)
}

// GetParticipantByToken retrieves a participant by session token.
func (r *RoomRepository) GetParticipantByToken(ctx context.Context, token string) (*model.Participant, error) {
	p, err := scanParticipant(r.db.QueryRowContext(ctx,
		`SELECT `+participantColumns+` FROM participants WHERE session_token = $1`, token))
	if err != nil {
		return nil, mapErr(err)
	}
	return p, nil
}

// ListParticipants returns all participants in a room, earliest joined first.
func (r *RoomRepository) ListParticipants(ctx context.Context, roomID string) ([]model.Participant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+participantColumns+` FROM participants WHERE room_id = $1 ORDER BY joined_at, id`,
		roomID,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()

	participants := []model.Participant{}
	for rows.Next() {
		p, err := scanParticipant(rows)
		if err != nil {
			return nil, err
		}
		participants = append(participants, *p)
	}
	return participants, rows.Err()
}

// RemoveParticipant deletes a participant from a room. If the participant was
// the host and others remain, the earliest-joined remaining participant is
// promoted to host within the same transaction and its ID is returned;
// otherwise newHostID is empty.
func (r *RoomRepository) RemoveParticipant(ctx context.Context, roomID, participantID string) (newHostID string, err error) {
	err = withTx(ctx, r.db, func(tx *sql.Tx) error {
		// Lock the room row so concurrent leaves in the same room serialize and
		// can never leave the room with zero or two hosts.
		var id string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM rooms WHERE id = $1 FOR UPDATE`, roomID).Scan(&id); err != nil {
			return err
		}

		var wasHost bool
		if err := tx.QueryRowContext(ctx,
			`DELETE FROM participants WHERE id = $1 AND room_id = $2 RETURNING is_host`,
			participantID, roomID,
		).Scan(&wasHost); err != nil {
			return err
		}
		if !wasHost {
			return nil
		}

		err := tx.QueryRowContext(ctx,
			`UPDATE participants SET is_host = TRUE
			 WHERE id = (SELECT id FROM participants WHERE room_id = $1 ORDER BY joined_at, id LIMIT 1)
			 RETURNING id`,
			roomID,
		).Scan(&newHostID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // room is now empty
		}
		return err
	})
	if err != nil {
		return "", mapErr(err)
	}
	return newHostID, nil
}
