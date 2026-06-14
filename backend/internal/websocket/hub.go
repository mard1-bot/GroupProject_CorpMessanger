package websocket

import (
	"context"
	"corp-messenger/backend/internal/livekit"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"
	"encoding/json"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

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

	// Storage for creating system messages
	storage storage.Storage

	// Structured logger
	logger *slog.Logger

	// LiveKit service for group calls
	liveKit *livekit.Service

	// LiveKit URL
	liveKitURL string

	// Rate limiter for WebSocket messages
	rateLimiter *RateLimiter
	redisClient *redis.Client

	mu         sync.RWMutex
	shutdownMu sync.Mutex
	shutdown   bool
}

// BroadcastMessage represents a message to be broadcast to specific room
type BroadcastMessage struct {
	Type    string      `json:"type"`
	ChatID  uuid.UUID   `json:"chat_id"`
	Payload interface{} `json:"payload"`
	// Exclude sender from broadcast (optional)
	ExcludeSender *uuid.UUID `json:"exclude_sender,omitempty"`
	TargetUserID  *uuid.UUID `json:"target_user_id,omitempty"`
}

// NewHub creates a new Hub instance.
func NewHub(storage storage.Storage, liveKit *livekit.Service, liveKitURL string, redisURL string) *Hub {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		opts = &redis.Options{Addr: "redis:6379"} // fallback
	}
	rdb := redis.NewClient(opts)
	// Rate limit: 60 messages per minute with burst of 10
	rateLimiter := NewRateLimiter(60, 10)

	return &Hub{
		broadcast:   make(chan *BroadcastMessage, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		rooms:       make(map[uuid.UUID]map[*Client]bool),
		userClients: make(map[uuid.UUID]*Client),
		typingState: make(map[uuid.UUID]map[uuid.UUID]*TypingState),
		callManager: NewCallManager(),
		storage:     storage,
		logger:      slog.Default(),
		liveKit:     liveKit,
		liveKitURL:  liveKitURL,
		rateLimiter: rateLimiter,
		redisClient: rdb,
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run(ctx context.Context) {
	go h.listenRedis(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Println("[Hub] Shutting down hub")
			h.Shutdown()
			return
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
			if message.TargetUserID != nil {
				h.mu.RLock()
				client, ok := h.userClients[*message.TargetUserID]
				h.mu.RUnlock()
				if ok {
					select {
					case client.send <- message:
					default:
					}
				}
				continue
			}

			h.mu.RLock()
			room, ok := h.rooms[message.ChatID]
			h.mu.RUnlock()
			if !ok {
				continue
			}

			clientCount := 0
			var clientsToRemove []*Client
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
					// Collect for removal after loop to avoid deadlock
					clientsToRemove = append(clientsToRemove, client)
					// Signal client to close itself
					select {
					case client.send <- nil:
					default:
						// Client buffer still full, just let it be
					}
				}
			}
			// Remove marked clients outside the loop to avoid deadlock
			if len(clientsToRemove) > 0 {
				h.mu.Lock()
				for _, client := range clientsToRemove {
					delete(h.clients, client)
					delete(h.userClients, client.UserID)
					if room, ok := h.rooms[message.ChatID]; ok {
						delete(room, client)
						if len(room) == 0 {
							delete(h.rooms, message.ChatID)
						}
					}
				}
				h.mu.Unlock()
			}
			log.Printf("[Hub] Broadcast %s to %d clients in chat %s", message.Type, clientCount, message.ChatID)
		}
	}
}

// Shutdown gracefully closes all client connections
func (h *Hub) Shutdown() {
	h.shutdownMu.Lock()
	defer h.shutdownMu.Unlock()

	if h.shutdown {
		return // Already shut down
	}
	h.shutdown = true

	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all client connections
	for client := range h.clients {
		close(client.send)
	}
	// Clear all maps
	h.clients = make(map[*Client]bool)
	h.userClients = make(map[uuid.UUID]*Client)
	h.rooms = make(map[uuid.UUID]map[*Client]bool)
	h.typingState = make(map[uuid.UUID]map[uuid.UUID]*TypingState)
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
	}
}

// BroadcastToAll sends a message to all connected clients.
func (h *Hub) BroadcastToAll(msg *BroadcastMessage) {
	h.broadcast <- msg
}

// BroadcastToUser sends a message to all clients of a specific user.
func (h *Hub) BroadcastToUser(userID uuid.UUID, msg *BroadcastMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	client, ok := h.userClients[userID]
	if !ok {
		return
	}

	select {
	case client.send <- msg:
	default:
		log.Printf("[Hub] Send channel full for user %s, dropping message", userID)
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

	// Broadcast typing update synchronously to avoid goroutine accumulation
	// This is safe because typing events are infrequent per user
	h.BroadcastToChat(&BroadcastMessage{
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

// CallManager returns the call manager instance
func (h *Hub) CallManager() *CallManager {
	return h.callManager
}

// HandleLoadChats handles synchronous loading of user's chats
func (h *Hub) HandleLoadChats(client *Client) {
	if h.storage == nil {
		log.Printf("[Hub] Storage not available for load_chats")
		return
	}

	ctx := context.Background()
	chats, err := h.storage.GetUserChats(ctx, client.UserID)
	if err != nil {
		log.Printf("[Hub] Failed to load chats for user %s: %v", client.UserID, err)
		h.sendToClient(client, &BroadcastMessage{
			Type:    "error",
			Payload: map[string]interface{}{"error": "Failed to load chats"},
		})
		return
	}

	// Send chats directly to the requesting client
	h.sendToClient(client, &BroadcastMessage{
		Type:    "chats_loaded",
		Payload: map[string]interface{}{"chats": chats},
	})

	log.Printf("[Hub] Loaded %d chats for user %s", len(chats), client.UserID)
}

// HandleLoadMessages handles synchronous loading of messages for a specific chat
func (h *Hub) HandleLoadMessages(client *Client, payload json.RawMessage) {
	if h.storage == nil {
		log.Printf("[Hub] Storage not available for load_messages")
		return
	}

	var params struct {
		ChatID string `json:"chat_id"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	if err := json.Unmarshal(payload, &params); err != nil {
		log.Printf("[Hub] Invalid load_messages payload: %v", err)
		h.sendToClient(client, &BroadcastMessage{
			Type:    "error",
			Payload: map[string]interface{}{"error": "Invalid payload"},
		})
		return
	}

	chatID, err := uuid.Parse(params.ChatID)
	if err != nil {
		log.Printf("[Hub] Invalid chat ID in load_messages: %v", err)
		h.sendToClient(client, &BroadcastMessage{
			Type:    "error",
			Payload: map[string]interface{}{"error": "Invalid chat ID"},
		})
		return
	}

	// Set defaults
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	ctx := context.Background()
	messages, err := h.storage.GetMessagesByChat(ctx, chatID, params.Limit, params.Offset)
	if err != nil {
		log.Printf("[Hub] Failed to load messages for chat %s: %v", chatID, err)
		h.sendToClient(client, &BroadcastMessage{
			Type:    "error",
			Payload: map[string]interface{}{"error": "Failed to load messages"},
		})
		return
	}

	// Load reactions for each message and create response structure
	type MessageWithReactions struct {
		*models.Message
		Reactions []*models.Reaction `json:"reactions,omitempty"`
	}

	messagesWithReactions := make([]*MessageWithReactions, len(messages))
	for i, msg := range messages {
		reactions, err := h.storage.GetMessageReactions(ctx, msg.ID)
		if err != nil {
			log.Printf("[Hub] Failed to load reactions for message %s: %v", msg.ID, err)
			reactions = []*models.Reaction{}
		}
		messagesWithReactions[i] = &MessageWithReactions{
			Message:   msg,
			Reactions: reactions,
		}
	}

	// Send messages directly to the requesting client
	h.sendToClient(client, &BroadcastMessage{
		Type:   "messages_loaded",
		ChatID: chatID,
		Payload: map[string]interface{}{
			"messages": messagesWithReactions,
			"chat_id":  chatID,
			"limit":    params.Limit,
			"offset":   params.Offset,
		},
	})

	log.Printf("[Hub] Loaded %d messages for chat %s (user %s)", len(messagesWithReactions), chatID, client.UserID)
}

// CreateCallSystemMessage creates a system message for call events
func (h *Hub) CreateCallSystemMessage(ctx context.Context, chatID, userID uuid.UUID, eventType string) error {
	if h.storage == nil {
		return nil
	}

	var content string
	switch eventType {
	case "call_initiated":
		content = "📞 Звонок начат"
	case "call_accepted":
		content = "✅ Звонок принят"
	case "call_rejected":
		content = "❌ Звонок отклонен"
	case "call_ended":
		content = "📞 Звонок завершен"
	default:
		return nil
	}

	// Create message in database
	msg := &models.Message{
		ChatID:   chatID,
		SenderID: userID,
		Type:     "system",
		Content:  content,
	}

	if err := h.storage.CreateMessage(ctx, msg); err != nil {
		log.Printf("[Hub] Failed to create call system message: %v", err)
		return err
	}

	// Broadcast the system message via WebSocket
	h.BroadcastToChat(&BroadcastMessage{
		Type:   "new_message",
		ChatID: chatID,
		Payload: map[string]interface{}{
			"id":         msg.ID,
			"chat_id":    msg.ChatID,
			"sender_id":  msg.SenderID,
			"type":       msg.Type,
			"content":    msg.Content,
			"created_at": msg.CreatedAt,
		},
	})

	log.Printf("[Hub] Created call system message: %s in chat %s", eventType, chatID)
	return nil
}

// listenRedis subscribes to Redis pub/sub for chat events and forwards them to the local broadcast channel
func (h *Hub) listenRedis(ctx context.Context) {
	pubsub := h.redisClient.Subscribe(ctx, "websocket_broadcast")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			var broadcastMsg BroadcastMessage
			if err := json.Unmarshal([]byte(msg.Payload), &broadcastMsg); err == nil {
				h.broadcast <- &broadcastMsg
			} else {
				h.logger.Error("failed to unmarshal redis broadcast message", "error", err)
			}
		}
	}
}
