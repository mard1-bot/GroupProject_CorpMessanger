package livekit

import (
	"context"
	"fmt"
	"log"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

type Service struct {
	apiKey    string
	apiSecret string
	url       string
}

func NewService(apiKey, apiSecret, url string) *Service {
	return &Service{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		url:       url,
	}
}

// CreateRoom creates a new LiveKit room for a call
func (s *Service) CreateRoom(ctx context.Context, roomName string) error {
	if s.apiKey == "" || s.apiSecret == "" || s.url == "" {
		log.Printf("[LiveKit] Not configured - skipping room creation")
		return nil
	}

	roomClient := lksdk.NewRoomServiceClient(s.url, s.apiKey, s.apiSecret)
	room := &livekit.CreateRoomRequest{
		Name:         roomName,
		EmptyTimeout: 300, // 5 minutes
	}

	_, err := roomClient.CreateRoom(ctx, room)
	if err != nil {
		return fmt.Errorf("failed to create LiveKit room: %w", err)
	}

	log.Printf("[LiveKit] Room created: %s", roomName)
	return nil
}

// DeleteRoom deletes a LiveKit room
func (s *Service) DeleteRoom(ctx context.Context, roomName string) error {
	if s.apiKey == "" || s.apiSecret == "" || s.url == "" {
		log.Printf("[LiveKit] Not configured - skipping room deletion")
		return nil
	}

	roomClient := lksdk.NewRoomServiceClient(s.url, s.apiKey, s.apiSecret)
	deleteReq := &livekit.DeleteRoomRequest{
		Room: roomName,
	}
	_, err := roomClient.DeleteRoom(ctx, deleteReq)
	if err != nil {
		return fmt.Errorf("failed to delete LiveKit room: %w", err)
	}

	log.Printf("[LiveKit] Room deleted: %s", roomName)
	return nil
}

// GenerateToken generates an access token for a participant
func (s *Service) GenerateToken(roomName, participantName string) (string, error) {
	if s.apiKey == "" || s.apiSecret == "" {
		return "", fmt.Errorf("LiveKit not configured")
	}

	at := auth.NewAccessToken(s.apiKey, s.apiSecret)
	at.AddGrant(&auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	})

	at.SetIdentity(participantName)
	at.SetMetadata(fmt.Sprintf(`{"name": "%s"}`, participantName))

	token, err := at.ToJWT()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

// IsConfigured returns true if LiveKit is properly configured
func (s *Service) IsConfigured() bool {
	return s.apiKey != "" && s.apiSecret != "" && s.url != ""
}
