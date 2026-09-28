package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
)

const (
	roomCodeLength = 6
	// roomCodeCharset excludes ambiguous characters (0/O, 1/I/L).
	roomCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	// maxCodeAttempts bounds retries when a generated code collides.
	maxCodeAttempts = 5
)

// RoomService contains business logic for room operations.
type RoomService struct {
	rooms RoomStore
}

// NewRoomService creates a new RoomService.
func NewRoomService(rooms RoomStore) *RoomService {
	return &RoomService{rooms: rooms}
}

// CreateRoom creates a new room and adds the creator as host.
func (s *RoomService) CreateRoom(ctx context.Context, name, hostDisplayName string) (*model.Room, *model.Participant, error) {
	name, err := cleanText("name", name, MaxRoomNameLen)
	if err != nil {
		return nil, nil, err
	}
	hostDisplayName, err = cleanText("display_name", hostDisplayName, MaxDisplayNameLen)
	if err != nil {
		return nil, nil, err
	}

	for range maxCodeAttempts {
		code, err := generateRoomCode(roomCodeLength)
		if err != nil {
			return nil, nil, fmt.Errorf("generate room code: %w", err)
		}
		room := &model.Room{Code: code, Name: name, Status: model.RoomStatusWaiting}
		host := &model.Participant{
			DisplayName:  hostDisplayName,
			SessionToken: uuid.NewString(),
			IsHost:       true,
		}
		err = s.rooms.CreateRoomWithHost(ctx, room, host)
		if errors.Is(err, repository.ErrDuplicate) {
			continue // code collision: try another one
		}
		if err != nil {
			return nil, nil, fmt.Errorf("create room: %w", err)
		}
		return room, host, nil
	}
	return nil, nil, errors.New("create room: could not generate a unique room code")
}

// JoinRoom adds a new participant to an existing room.
func (s *RoomService) JoinRoom(ctx context.Context, code, displayName string) (*model.Room, *model.Participant, error) {
	displayName, err := cleanText("display_name", displayName, MaxDisplayNameLen)
	if err != nil {
		return nil, nil, err
	}
	room, err := s.getRoom(ctx, code)
	if err != nil {
		return nil, nil, err
	}

	participant := &model.Participant{
		RoomID:       room.ID,
		DisplayName:  displayName,
		SessionToken: uuid.NewString(),
	}
	if err := s.rooms.AddParticipant(ctx, participant); err != nil {
		return nil, nil, fmt.Errorf("add participant: %w", err)
	}
	return room, participant, nil
}

// GetRoom returns a room and its participants (earliest joined first).
func (s *RoomService) GetRoom(ctx context.Context, code string) (*model.Room, []model.Participant, error) {
	room, err := s.getRoom(ctx, code)
	if err != nil {
		return nil, nil, err
	}
	participants, err := s.rooms.ListParticipants(ctx, room.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list participants: %w", err)
	}
	if participants == nil {
		participants = []model.Participant{}
	}
	return room, participants, nil
}

// Authorize resolves the room identified by code and the participant owning
// token, enforcing that the participant belongs to that room.
//
// It fails with ErrNotFound if the room does not exist, ErrUnauthorized if
// the token is missing or unknown, and ErrForbidden if the participant
// belongs to another room.
func (s *RoomService) Authorize(ctx context.Context, code, token string) (*model.Room, *model.Participant, error) {
	return authorize(ctx, s.rooms, code, token)
}

// LeaveResult describes the outcome of a participant leaving a room.
type LeaveResult struct {
	Room          *model.Room
	ParticipantID string
	// NewHostID is set when the leaving participant was the host and another
	// participant was promoted.
	NewHostID string
}

// Leave removes the participant owning token from the room. If they were the
// host, the earliest-joined remaining participant becomes host.
func (s *RoomService) Leave(ctx context.Context, code, token string) (*LeaveResult, error) {
	room, p, err := s.Authorize(ctx, code, token)
	if err != nil {
		return nil, err
	}
	newHostID, err := s.rooms.RemoveParticipant(ctx, room.ID, p.ID)
	if errors.Is(err, repository.ErrNotFound) {
		// Removed concurrently (e.g. a double-submitted leave).
		return nil, newError(ErrUnauthorized, "invalid session token")
	}
	if err != nil {
		return nil, fmt.Errorf("remove participant: %w", err)
	}
	return &LeaveResult{Room: room, ParticipantID: p.ID, NewHostID: newHostID}, nil
}

func (s *RoomService) getRoom(ctx context.Context, code string) (*model.Room, error) {
	return getRoom(ctx, s.rooms, code)
}

func getRoom(ctx context.Context, rooms RoomStore, code string) (*model.Room, error) {
	code = NormalizeCode(code)
	if code == "" {
		return nil, newError(ErrNotFound, "room not found")
	}
	room, err := rooms.GetRoomByCode(ctx, code)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, newError(ErrNotFound, "room not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return room, nil
}

// authorize implements the room-scoped auth rule shared by all services.
func authorize(ctx context.Context, rooms RoomStore, code, token string) (*model.Room, *model.Participant, error) {
	room, err := getRoom(ctx, rooms, code)
	if err != nil {
		return nil, nil, err
	}
	p, err := participantByToken(ctx, rooms, token)
	if err != nil {
		return nil, nil, err
	}
	if p.RoomID != room.ID {
		return nil, nil, newError(ErrForbidden, "you are not a participant of this room")
	}
	return room, p, nil
}

func participantByToken(ctx context.Context, rooms RoomStore, token string) (*model.Participant, error) {
	if token == "" {
		return nil, newError(ErrUnauthorized, "missing session token")
	}
	if !isUUID(token) {
		return nil, newError(ErrUnauthorized, "invalid session token")
	}
	p, err := rooms.GetParticipantByToken(ctx, token)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, newError(ErrUnauthorized, "invalid session token")
	}
	if err != nil {
		return nil, fmt.Errorf("get participant: %w", err)
	}
	return p, nil
}

// generateRoomCode creates a random code of the given length from roomCodeCharset.
func generateRoomCode(length int) (string, error) {
	max := big.NewInt(int64(len(roomCodeCharset)))
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		code[i] = roomCodeCharset[n.Int64()]
	}
	return string(code), nil
}
