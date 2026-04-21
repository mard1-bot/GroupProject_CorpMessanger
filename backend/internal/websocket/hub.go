package websocket

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TypingState tracks who's typing in a chat
type TypingState struct {
	UserID    uuid.UUID `json:"user_id"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *BroadcastMessage

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Rooms (chatID -> set of clients)
	rooms map[uuid.UUID]map[*Client]bool

	// UserID to client mapping for presence
	userClients map[uuid.UUID]*Client

	// Typing state per chat (chatID -> map[userID]TypingState)
	typingState map[uuid.UUID]map[uuid.UUID]*TypingState

	// WebRTC call manager
	callManager *CallManager

	mu sync.RWMutex
}

// BroadcastMessage represents a message to be broadcast to specific room
type BroadcastMessage struct {
	Type    string      `json:"type"`
	ChatID  uuid.UUID   `json:"chat_id"`
	Payload interface{} `json:"payload"`
	// Exclude sender from broadcast (optional)
	ExcludeSender *uuid.UUID `json:"-"`
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		broadcast:   make(chan *BroadcastMessage, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		rooms:       make(map[uuid.UUID]map[*Client]bool),
		userClients: make(map[uuid.UUID]*Client),
		typingState: make(map[uuid.UUID]map[uuid.UUID]*TypingState),
		callManager: NewCallManager(),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.userClients[client.UserID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.userClients, client.UserID)
				// Remove from all rooms
				for chatID, room := range h.rooms {
					if _, ok := room[client]; ok {
						delete(room, client)
						if len(room) == 0 {
							delete(h.rooms, chatID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			room, ok := h.rooms[message.ChatID]
			h.mu.RUnlock()
			if !ok {
				continue
			}

			clientCount := 0
			for client := range room {
				// Skip excluded sender
				if message.ExcludeSender != nil && client.UserID == *message.ExcludeSender {
					continue
				}

				select {
				case client.send <- message:
					clientCount++
				default:
					// Client's send buffer is full, mark for removal
					// Don't close channel here - let unregister handle it
					h.mu.Lock()
					delete(h.clients, client)
					delete(room, client)
					h.mu.Unlock()
					// Signal client to close itself
					select {
					case client.send <- nil:
					default:
						// Client buffer still full, just let it be
					}
				}
			}
			log.Printf("[Hub] Broadcast %s to %d clients in chat %s", message.Type, clientCount, message.ChatID)
		}
	}
}

// JoinRoom adds a client to a chat room.
func (h *Hub) JoinRoom(client *Client, chatID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[chatID] == nil {
		h.rooms[chatID] = make(map[*Client]bool)
	}
	h.rooms[chatID][client] = true
}

// LeaveRoom removes a client from a chat room.
func (h *Hub) LeaveRoom(client *Client, chatID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[chatID]; ok {
		if _, ok := room[client]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.rooms, chatID)
			}
		}
	}
}

// GetOnlineUsers returns list of online user IDs.
func (h *Hub) GetOnlineUsers() []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := make([]uuid.UUID, 0, len(h.userClients))
	for userID := range h.userClients {
		users = append(users, userID)
	}
	return users
}

// IsUserOnline checks if a user is currently connected.
func (h *Hub) IsUserOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.userClients[userID]
	return ok
}

// BroadcastToChat sends a message to all clients in a specific chat room.
func (h *Hub) BroadcastToChat(msg *BroadcastMessage) {
	select {
	case h.broadcast <- msg:
		log.Printf("[Hub] Broadcasting %s to chat %s", msg.Type, msg.ChatID)
	default:
		// Broadcast channel full, drop message (shouldn't happen with buffer)
		log.Printf("[Hub] Broadcast channel full, dropping %s to chat %s", msg.Type, msg.ChatID)
	}
}

// SetTyping updates typing state for a user in a chat
func (h *Hub) SetTyping(chatID, userID uuid.UUID, firstName, lastName string, isTyping bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.typingState[chatID] == nil {
		h.typingState[chatID] = make(map[uuid.UUID]*TypingState)
	}

	if isTyping {
		h.typingState[chatID][userID] = &TypingState{
			UserID:    userID,
			FirstName: firstName,
			LastName:  lastName,
			ExpiresAt: time.Now().Add(5 * time.Second),
		}
	} else {
		delete(h.typingState[chatID], userID)
		if len(h.typingState[chatID]) == 0 {
			delete(h.typingState, chatID)
		}
	}

	// Broadcast typing update to room
	go h.BroadcastToChat(&BroadcastMessage{
		Type:   "typing",
		ChatID: chatID,
		Payload: map[string]interface{}{
			"user_id":    userID,
			"first_name": firstName,
			"last_name":  lastName,
			"is_typing":  isTyping,
		},
		ExcludeSender: &userID,
	})
}

// GetTypingUsers returns list of users currently typing in a chat
func (h *Hub) GetTypingUsers(chatID uuid.UUID) []TypingState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.typingState[chatID] == nil {
		return nil
	}

	now := time.Now()
	var users []TypingState
	for _, state := range h.typingState[chatID] {
		if state.ExpiresAt.After(now) {
			users = append(users, *state)
		}
	}
	return users
}

// CleanupExpiredTyping removes expired typing entries (call periodically)
func (h *Hub) CleanupExpiredTyping() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for chatID, users := range h.typingState {
		for userID, state := range users {
			if state.ExpiresAt.Before(now) {
				delete(users, userID)
			}
		}
		if len(users) == 0 {
			delete(h.typingState, chatID)
		}
	}
}

// sendToClient sends a message directly to a specific client
func (h *Hub) sendToClient(client *Client, msg *BroadcastMessage) {
	select {
	case client.send <- msg:
		log.Printf("[Hub] Sent %s to client %s", msg.Type, client.UserID)
	default:
		log.Printf("[Hub] Failed to send %s to client %s (channel full)", msg.Type, client.UserID)
	}
}
