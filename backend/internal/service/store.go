// Package service contains the business rules of PointPoker: room lifecycle,
// authorization of room-scoped actions and the voting state machine.
package service

import (
	"context"

	"github.com/pauloedsg/pointpoker/internal/model"
)

// RoomStore persists rooms and participants.
//
// Implementations must return repository.ErrNotFound for missing rows and
// repository.ErrDuplicate for unique-constraint violations.
type RoomStore interface {
	CreateRoomWithHost(ctx context.Context, room *model.Room, host *model.Participant) error
	GetRoomByCode(ctx context.Context, code string) (*model.Room, error)
	UpdateRoomStatus(ctx context.Context, roomID string, status model.RoomStatus) error
	AddParticipant(ctx context.Context, p *model.Participant) error
	GetParticipantByToken(ctx context.Context, token string) (*model.Participant, error)
	ListParticipants(ctx context.Context, roomID string) ([]model.Participant, error)
	RemoveParticipant(ctx context.Context, roomID, participantID string) (newHostID string, err error)
}

// RoundStore persists voting rounds and votes.
//
// Implementations must return repository.ErrNotFound for missing rows.
type RoundStore interface {
	CreateRound(ctx context.Context, round *model.VotingRound) error
	GetRound(ctx context.Context, roundID string) (*model.VotingRound, error)
	GetLatestRound(ctx context.Context, roomID string) (*model.VotingRound, error)
	RevealRound(ctx context.Context, roundID string) (bool, error)
	ResetRound(ctx context.Context, roundID string) error
	UpsertVote(ctx context.Context, vote *model.Vote) error
	ListVotes(ctx context.Context, roundID string) ([]model.Vote, error)
}
