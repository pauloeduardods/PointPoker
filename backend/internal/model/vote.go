package model

import "time"

// RoundStatus represents the current state of a voting round.
type RoundStatus string

const (
	RoundStatusVoting   RoundStatus = "voting"
	RoundStatusRevealed RoundStatus = "revealed"
)

// VotingRound represents a single estimation round within a room.
type VotingRound struct {
	ID         string      `json:"id"`
	RoomID     string      `json:"room_id"`
	StoryTitle string      `json:"story_title"`
	Status     RoundStatus `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
}

// Vote represents a single participant's vote in a round.
type Vote struct {
	ID            string    `json:"id"`
	RoundID       string    `json:"round_id"`
	ParticipantID string    `json:"participant_id"`
	Value         string    `json:"value"`
	VotedAt       time.Time `json:"voted_at"`
}

// FibonacciDeck contains the allowed vote values.
var FibonacciDeck = []string{"0", "1", "2", "3", "5", "8", "13", "21", "?"}
