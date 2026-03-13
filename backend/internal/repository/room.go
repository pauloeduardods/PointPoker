package repository

import (
	"database/sql"

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

// Create inserts a new room and populates its ID and CreatedAt.
func (r *RoomRepository) Create(room *model.Room) error {
	return r.db.QueryRow(
		`INSERT INTO rooms (code, name, status) VALUES ($1, $2, $3) RETURNING id, created_at`,
		room.Code, room.Name, room.Status,
	).Scan(&room.ID, &room.CreatedAt)
}

// GetByCode retrieves a room by its unique code.
func (r *RoomRepository) GetByCode(code string) (*model.Room, error) {
	room := &model.Room{}
	err := r.db.QueryRow(
		`SELECT id, code, name, status, created_at FROM rooms WHERE code = $1`,
		code,
	).Scan(&room.ID, &room.Code, &room.Name, &room.Status, &room.CreatedAt)
	if err != nil {
		return nil, err
	}
	return room, nil
}

// UpdateStatus changes the status of a room.
func (r *RoomRepository) UpdateStatus(roomID string, status model.RoomStatus) error {
	_, err := r.db.Exec(
		`UPDATE rooms SET status = $1 WHERE id = $2`,
		status, roomID,
	)
	return err
}

// AddParticipant inserts a participant and populates its ID and JoinedAt.
func (r *RoomRepository) AddParticipant(p *model.Participant) error {
	return r.db.QueryRow(
		`INSERT INTO participants (room_id, display_name, session_token, is_host)
		 VALUES ($1, $2, $3, $4) RETURNING id, joined_at`,
		p.RoomID, p.DisplayName, p.SessionToken, p.IsHost,
	).Scan(&p.ID, &p.JoinedAt)
}

// GetParticipantByToken retrieves a participant by session token.
func (r *RoomRepository) GetParticipantByToken(token string) (*model.Participant, error) {
	p := &model.Participant{}
	err := r.db.QueryRow(
		`SELECT id, room_id, display_name, session_token, is_host, joined_at
		 FROM participants WHERE session_token = $1`,
		token,
	).Scan(&p.ID, &p.RoomID, &p.DisplayName, &p.SessionToken, &p.IsHost, &p.JoinedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetParticipantsByRoomID returns all participants in a room.
func (r *RoomRepository) GetParticipantsByRoomID(roomID string) ([]model.Participant, error) {
	rows, err := r.db.Query(
		`SELECT id, room_id, display_name, session_token, is_host, joined_at
		 FROM participants WHERE room_id = $1 ORDER BY joined_at`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []model.Participant
	for rows.Next() {
		var p model.Participant
		if err := rows.Scan(&p.ID, &p.RoomID, &p.DisplayName, &p.SessionToken, &p.IsHost, &p.JoinedAt); err != nil {
			return nil, err
		}
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

// RemoveParticipant deletes a participant by ID.
func (r *RoomRepository) RemoveParticipant(participantID string) error {
	_, err := r.db.Exec(`DELETE FROM participants WHERE id = $1`, participantID)
	return err
}
