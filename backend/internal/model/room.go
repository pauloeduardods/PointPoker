package model

import "time"

// RoomStatus represents the current state of a room.
type RoomStatus string

const (
	RoomStatusWaiting  RoomStatus = "waiting"
	RoomStatusVoting   RoomStatus = "voting"
	RoomStatusRevealed RoomStatus = "revealed"
)

// Room represents a planning poker room.
type Room struct {
	ID        string     `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Status    RoomStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

// Participant represents a user in a room.
type Participant struct {
	ID           string    `json:"id"`
	RoomID       string    `json:"room_id"`
	DisplayName  string    `json:"display_name"`
	SessionToken string    `json:"session_token,omitempty"`
	IsHost       bool      `json:"is_host"`
	JoinedAt     time.Time `json:"joined_at"`
}
