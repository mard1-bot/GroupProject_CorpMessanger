package websocket

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB

	// Send buffer size for client.
	sendBufferSize = 256 // Buffer size for WebSocket send channel
)

// getClientIP extracts the real client IP from request headers
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			// Validate IP format
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if net.ParseIP(xri) != nil {
			return xri
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// AllowedOrigins is set by the HTTP handler from configuration
var AllowedOrigins = make(map[string]bool)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// SECURITY: Reject empty origin to prevent CORS bypass
		// Native apps must provide a specific origin header
		if origin == "" {
			log.Printf("[WebSocket] Rejected connection with empty origin from IP: %s", getClientIP(r))
			return false
		}

		// Check against allowed origins
		if AllowedOrigins[origin] {
			return true
		}

		// Log all rejected origins for security monitoring
		log.Printf("[WebSocket] Rejected connection from unauthorized origin: %s (IP: %s)", origin, getClientIP(r))
		return false
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan *BroadcastMessage

	// User ID associated with this connection.
	UserID uuid.UUID

	// Current chat rooms the client is subscribed to.
	rooms   map[uuid.UUID]bool
	roomsMu sync.RWMutex // Protects rooms map concurrent access
}

// WSMessage represents a message from client
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	client := &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan *BroadcastMessage, sendBufferSize),
		UserID: userID,
		rooms:  make(map[uuid.UUID]bool),
	}

	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in new goroutines.
	go client.writePump()
	go client.readPump()

	return nil
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg WSMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close
			}
			break
		}

		c.handleMessage(&msg)
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes messages from the client.
func (c *Client) handleMessage(msg *WSMessage) {
	// Skip rate limiting for certain message types
	skipRateLimit := msg.Type == "ping" || msg.Type == "pong"

	// Check rate limit for user actions
	if !skipRateLimit && !c.hub.rateLimiter.Allow(c.UserID) {
		// Send rate limit error to client
		errorMsg := &BroadcastMessage{
			Type: "error",
			Payload: map[string]interface{}{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many messages. Please slow down.",
			},
		}
		select {
		case c.send <- errorMsg:
		default:
			// Channel full, skip error message
		}
		log.Printf("[WebSocket] Rate limit exceeded for user %s (type: %s)", c.UserID, msg.Type)
		return
	}

	switch msg.Type {
	case "ping":
		// Respond with pong to keep connection alive
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.conn.WriteJSON(&WSMessage{Type: "pong"})
	case "join_chat":
		var payload struct {
			ChatID uuid.UUID `json:"chat_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		c.hub.JoinRoom(c, payload.ChatID)
		c.roomsMu.Lock()
		c.rooms[payload.ChatID] = true
		c.roomsMu.Unlock()
		log.Printf("[WebSocket] User %s joined chat %s", c.UserID, payload.ChatID)

	case "leave_chat":
		var payload struct {
			ChatID uuid.UUID `json:"chat_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		c.hub.LeaveRoom(c, payload.ChatID)
		c.roomsMu.Lock()
		delete(c.rooms, payload.ChatID)
		c.roomsMu.Unlock()
		log.Printf("[WebSocket] User %s left chat %s", c.UserID, payload.ChatID)

	case "typing":
		var payload struct {
			ChatID    uuid.UUID `json:"chat_id"`
			IsTyping  bool      `json:"is_typing"`
			FirstName string    `json:"first_name,omitempty"`
			LastName  string    `json:"last_name,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		// Update typing state and broadcast to room
		c.hub.SetTyping(payload.ChatID, c.UserID, payload.FirstName, payload.LastName, payload.IsTyping)
		log.Printf("[WebSocket] Typing from user %s in chat %s (is_typing: %v)", c.UserID, payload.ChatID, payload.IsTyping)

	case "read_receipt":
		var payload struct {
			ChatID    uuid.UUID `json:"chat_id"`
			MessageID uuid.UUID `json:"message_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		// Broadcast read receipt to room
		c.hub.BroadcastToChat(&BroadcastMessage{
			Type:   "read_receipt",
			ChatID: payload.ChatID,
			Payload: map[string]interface{}{
				"reader_id":  c.UserID, // Matches client expectation
				"message_id": payload.MessageID,
			},
			ExcludeSender: &c.UserID,
		})

	// Load chats synchronously
	case "load_chats":
		c.hub.HandleLoadChats(c)
	// Load messages synchronously
	case "load_messages":
		c.hub.HandleLoadMessages(c, msg.Payload)
	// WebRTC call events
	case EventCallOffer:
		c.hub.HandleCallOffer(c, msg.Payload)
	case EventCallAnswer:
		c.hub.HandleCallAnswer(c, msg.Payload)
	case EventCallIce:
		c.hub.HandleCallIce(c, msg.Payload)
	case EventCallEnd:
		c.hub.HandleCallEnd(c, msg.Payload)
	case EventCallReject:
		c.hub.HandleCallReject(c, msg.Payload)
	case EventCallAccept:
		c.hub.HandleCallAccept(c, msg.Payload)
	case EventCallBusy:
		c.hub.HandleCallBusy(c, msg.Payload)
	}
}
