package websocket

import (
	"encoding/json"
	"net/http"
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
	sendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development, configure properly in production
		return true
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
	rooms map[uuid.UUID]bool
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
		c.rooms[payload.ChatID] = true

	case "leave_chat":
		var payload struct {
			ChatID uuid.UUID `json:"chat_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		c.hub.LeaveRoom(c, payload.ChatID)
		delete(c.rooms, payload.ChatID)

	case "typing":
		var payload struct {
			ChatID uuid.UUID `json:"chat_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		// Broadcast typing indicator to room
		c.hub.BroadcastToChat(&BroadcastMessage{
			Type:          "typing",
			ChatID:        payload.ChatID,
			Payload:       map[string]interface{}{"user_id": c.UserID},
			ExcludeSender: &c.UserID,
		})

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
				"user_id":    c.UserID,
				"message_id": payload.MessageID,
			},
			ExcludeSender: &c.UserID,
		})
	}
}
