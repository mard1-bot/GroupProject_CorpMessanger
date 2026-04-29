package ejabberd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// XMPPClient is a real implementation of the ejabberd client
type XMPPClient struct {
	host       string
	port       int
	apiSecret  string
	httpClient *http.Client
}

// NewXMPPClient creates a new XMPP client for ejabberd
func NewXMPPClient(host string, port int, apiSecret string) *XMPPClient {
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 5280
	}
	return &XMPPClient{
		host:      host,
		port:      port,
		apiSecret: apiSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetHost returns the XMPP server host
func (c *XMPPClient) GetHost() string {
	return c.host
}

// Ready checks if ejabberd is available
func (c *XMPPClient) Ready(ctx context.Context) error {
	url := fmt.Sprintf("http://%s:%d/api/status", c.host, c.port)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", c.apiSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[XMPP] Ejabberd not ready: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ejabberd returned status %d", resp.StatusCode)
	}

	return nil
}

// Close closes the client
func (c *XMPPClient) Close() error {
	return nil
}

// CreateUser creates a new XMPP user
func (c *XMPPClient) CreateUser(userID uuid.UUID, password string) error {
	payload := map[string]interface{}{
		"user":     userID.String(),
		"host":     c.host,
		"password": password,
	}

	return c.apiRequest("POST", "/api/register", payload)
}

// DeleteUser deletes an XMPP user
func (c *XMPPClient) DeleteUser(userID uuid.UUID) error {
	payload := map[string]interface{}{
		"user": userID.String(),
		"host": c.host,
	}

	return c.apiRequest("POST", "/api/unregister", payload)
}

// UpdateUserPassword updates an XMPP user's password
func (c *XMPPClient) UpdateUserPassword(userID uuid.UUID, password string) error {
	userJID := fmt.Sprintf("%s@%s", userID.String(), c.host)

	payload := map[string]interface{}{
		"user":     userJID,
		"password": password,
	}

	return c.apiRequest("POST", "/api/change_password", payload)
}

// CreateChatRoom creates a new XMPP chat room (MUC)
func (c *XMPPClient) CreateChatRoom(roomID uuid.UUID, title string, ownerID uuid.UUID) error {
	roomJID := fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)

	// Create room
	payload := map[string]interface{}{
		"name":    roomID.String(),
		"service": fmt.Sprintf("conference.%s", c.host),
		"host":    c.host,
	}

	if err := c.apiRequest("POST", "/api/create_room", payload); err != nil {
		return err
	}

	// Set room title
	configPayload := map[string]interface{}{
		"room":    roomJID,
		"title":   title,
		"options": map[string]string{"title": title},
	}

	return c.apiRequest("POST", "/api/change_room_option", configPayload)
}

// DestroyChatRoom destroys an XMPP chat room
func (c *XMPPClient) DestroyChatRoom(roomID uuid.UUID) error {
	payload := map[string]interface{}{
		"name":    roomID.String(),
		"service": fmt.Sprintf("conference.%s", c.host),
	}

	return c.apiRequest("POST", "/api/destroy_room", payload)
}

// UpdateChatRoomTitle updates a chat room's title
func (c *XMPPClient) UpdateChatRoomTitle(roomID uuid.UUID, title string) error {
	roomJID := fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)

	configPayload := map[string]interface{}{
		"room":    roomJID,
		"title":   title,
		"options": map[string]string{"title": title},
	}

	return c.apiRequest("POST", "/api/change_room_option", configPayload)
}

// AddMemberToRoom adds a member to a chat room
func (c *XMPPClient) AddMemberToRoom(roomID uuid.UUID, userID uuid.UUID, role string) error {
	roomJID := fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)
	userJID := fmt.Sprintf("%s@%s", userID.String(), c.host)

	payload := map[string]interface{}{
		"room": roomJID,
		"nick": userID.String(),
		"jid":  userJID,
		"role": role,
	}

	return c.apiRequest("POST", "/api/set_room_affiliation", payload)
}

// RemoveMemberFromRoom removes a member from a chat room
func (c *XMPPClient) RemoveMemberFromRoom(roomID uuid.UUID, userID uuid.UUID) error {
	roomJID := fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)

	payload := map[string]interface{}{
		"room": roomJID,
		"jid":  fmt.Sprintf("%s@%s", userID.String(), c.host),
		"role": "none",
	}

	return c.apiRequest("POST", "/api/set_room_affiliation", payload)
}

// SendMessage sends a message to a user or room
func (c *XMPPClient) SendMessage(from, to, body string) error {
	payload := map[string]interface{}{
		"from": from,
		"to":   to,
		"body": body,
		"type": "chat",
	}

	return c.apiRequest("POST", "/api/send_message", payload)
}

// SendRoomMessage sends a message to a chat room (MUC)
func (c *XMPPClient) SendRoomMessage(roomID uuid.UUID, from, body string) error {
	roomJID := fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)

	payload := map[string]interface{}{
		"type": "groupchat",
		"from": from,
		"to":   roomJID,
		"body": body,
	}

	return c.apiRequest("POST", "/api/send_message", payload)
}

// GetRoomMessages retrieves messages from a chat room
func (c *XMPPClient) GetRoomMessages(roomID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	// Note: ejabberd API doesn't have a direct endpoint for retrieving room messages
	// This would require either:
	// 1. Using mod_mam for message archiving
	// 2. Querying the database directly
	// 3. Using a custom ejabberd module
	// For now, return empty as this requires additional ejabberd configuration
	log.Printf("[XMPP] GetRoomMessages called for room %s (not yet implemented)", roomID)
	return []map[string]interface{}{}, nil
}

// GetOnlineUsers returns a list of online users
func (c *XMPPClient) GetOnlineUsers() ([]string, error) {
	url := fmt.Sprintf("http://%s:%d/api/connected_users", c.host, c.port)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.apiSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ejabberd returned status %d", resp.StatusCode)
	}

	var users []string
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	return users, nil
}

// apiRequest makes an API request to ejabberd
func (c *XMPPClient) apiRequest(method, endpoint string, payload interface{}) error {
	url := fmt.Sprintf("http://%s:%d%s", c.host, c.port, endpoint)

	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", c.apiSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[XMPP] API request failed: %s %s - %v", method, endpoint, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[XMPP] API error: %s %s - %d: %s", method, endpoint, resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("ejabberd API error: %d", resp.StatusCode)
	}

	return nil
}

// Helper function to build JID
func (c *XMPPClient) buildJID(userID uuid.UUID) string {
	return fmt.Sprintf("%s@%s", userID.String(), c.host)
}

// Helper function to build room JID
func (c *XMPPClient) buildRoomJID(roomID uuid.UUID) string {
	return fmt.Sprintf("%s@conference.%s", roomID.String(), c.host)
}
