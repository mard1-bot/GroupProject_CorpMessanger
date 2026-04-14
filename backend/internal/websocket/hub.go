package websocket

import (
	"sync"

	"github.com/google/uuid"
)

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

			for client := range room {
				// Skip excluded sender
				if message.ExcludeSender != nil && client.UserID == *message.ExcludeSender {
					continue
				}

				select {
				case client.send <- message:
				default:
					// Client's send buffer is full, close it
					h.mu.Lock()
					delete(h.clients, client)
					delete(room, client)
					h.mu.Unlock()
					// Close channel outside the lock to prevent race condition
					close(client.send)
				}
			}
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
	default:
		// Broadcast channel full, drop message (shouldn't happen with buffer)
	}
}
