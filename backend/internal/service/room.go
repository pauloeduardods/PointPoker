package service

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/pauloedsg/pointpoker/internal/model"
	"github.com/pauloedsg/pointpoker/internal/repository"
)

// RoomService contains business logic for room operations.
type RoomService struct {
	roomRepo *repository.RoomRepository
}

// NewRoomService creates a new RoomService.
func NewRoomService(roomRepo *repository.RoomRepository) *RoomService {
	return &RoomService{roomRepo: roomRepo}
}

// CreateRoom creates a new room and adds the creator as host.
func (s *RoomService) CreateRoom(name, hostDisplayName string) (*model.Room, *model.Participant, error) {
	code, err := generateRoomCode(6)
	if err != nil {
		return nil, nil, fmt.Errorf("generate room code: %w", err)
	}

	room := &model.Room{
		Code:   code,
		Name:   name,
		Status: model.RoomStatusWaiting,
	}
	if err := s.roomRepo.Create(room); err != nil {
		return nil, nil, fmt.Errorf("create room: %w", err)
	}

	host := &model.Participant{
		RoomID:       room.ID,
		DisplayName:  hostDisplayName,
		SessionToken: uuid.New().String(),
		IsHost:       true,
	}
	if err := s.roomRepo.AddParticipant(host); err != nil {
		return nil, nil, fmt.Errorf("add host: %w", err)
	}

	return room, host, nil
}

// JoinRoom adds a participant to an existing room.
func (s *RoomService) JoinRoom(code, displayName string) (*model.Room, *model.Participant, error) {
	room, err := s.roomRepo.GetByCode(code)
	if err != nil {
		return nil, nil, fmt.Errorf("room not found: %w", err)
	}

	participant := &model.Participant{
		RoomID:       room.ID,
		DisplayName:  displayName,
		SessionToken: uuid.New().String(),
		IsHost:       false,
	}
	if err := s.roomRepo.AddParticipant(participant); err != nil {
		return nil, nil, fmt.Errorf("add participant: %w", err)
	}

	return room, participant, nil
}

// GetRoom retrieves a room by code.
func (s *RoomService) GetRoom(code string) (*model.Room, error) {
	return s.roomRepo.GetByCode(code)
}

// GetParticipants returns all participants in a room.
func (s *RoomService) GetParticipants(roomID string) ([]model.Participant, error) {
	return s.roomRepo.GetParticipantsByRoomID(roomID)
}

// GetParticipantByToken retrieves a participant by session token.
func (s *RoomService) GetParticipantByToken(token string) (*model.Participant, error) {
	return s.roomRepo.GetParticipantByToken(token)
}

// RemoveParticipant removes a participant from a room.
func (s *RoomService) RemoveParticipant(participantID string) error {
	return s.roomRepo.RemoveParticipant(participantID)
}

// UpdateRoomStatus changes the room status.
func (s *RoomService) UpdateRoomStatus(roomID string, status model.RoomStatus) error {
	return s.roomRepo.UpdateStatus(roomID, status)
}

// generateRoomCode creates a random alphanumeric code, excluding ambiguous characters.
func generateRoomCode(length int) (string, error) {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code[i] = charset[n.Int64()]
	}
	return string(code), nil
}
