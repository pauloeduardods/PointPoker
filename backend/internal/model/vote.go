package model

import (
	"slices"
	"time"
)

// RoundStatus represents the current state of a voting round.
type RoundStatus string

// Round statuses.
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

// RevealedVote is a vote joined with its participant's display name, as
// returned when a round is revealed.
type RevealedVote struct {
	ParticipantID string `json:"participant_id"`
	DisplayName   string `json:"display_name"`
	Value         string `json:"value"`
}

// FibonacciDeck contains the allowed vote values.
var FibonacciDeck = []string{"0", "1", "2", "3", "5", "8", "13", "21", "?", "☕"}

// IsValidVote reports whether value is a card of the FibonacciDeck.
func IsValidVote(value string) bool {
	return slices.Contains(FibonacciDeck, value)
}
