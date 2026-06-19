package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LiveKitRoomInfo contains LiveKit room information
type LiveKitRoomInfo struct {
	RoomName    string
	URL         string
	CallerToken string
	CalleeToken string
}

// WebRTC signaling message types
const (
	EventCallOffer   = "call_offer"
	EventCallAnswer  = "call_answer"
	EventCallIce     = "call_ice"
	EventCallEnd     = "call_end"
	EventCallReject  = "call_reject"
	EventCallAccept  = "call_accept"
	EventCallBusy    = "call_busy"
	EventCallRinging = "call_ringing"
	EventCallError   = "call_error"
)

// CallType represents the type of call
type CallType string

const (
	CallTypeAudio CallType = "audio"
	CallTypeVideo CallType = "video"
)

// CallOfferPayload represents a WebRTC offer
type CallOfferPayload struct {
	CallID   uuid.UUID `json:"call_id"`
	ChatID   uuid.UUID `json:"chat_id"`
	CallerID uuid.UUID `json:"caller_id"`
	CalleeID uuid.UUID `json:"callee_id"`
	Type     CallType  `json:"type"`
	SDP      string    `json:"sdp"`
}

// CallAnswerPayload represents a WebRTC answer
type CallAnswerPayload struct {
	CallID uuid.UUID `json:"call_id"`
	SDP    string    `json:"sdp"`
}

// CallIcePayload represents an ICE candidate
type CallIcePayload struct {
	CallID        uuid.UUID `json:"call_id"`
	Candidate     string    `json:"candidate"`
	SDPMLineIndex int       `json:"sdp_mline_index"`
	SDPMid        string    `json:"sdp_mid"`
}

// ActiveCall represents an ongoing call
type ActiveCall struct {
	ID           uuid.UUID
	ChatID       uuid.UUID
	CallerID     uuid.UUID
	Participants map[uuid.UUID]bool // participant_id -> true
	Type         CallType
	StartTime    time.Time
	Answered     bool
	mu           sync.RWMutex
}

// CallManager manages active calls
type CallManager struct {
	mu    sync.RWMutex
	calls map[uuid.UUID]*ActiveCall // call_id -> ActiveCall
}

// NewCallManager creates a new CallManager
func NewCallManager() *CallManager {
	return &CallManager{
		calls: make(map[uuid.UUID]*ActiveCall),
	}
}

// StartCall starts a new call
func (cm *CallManager) StartCall(chatID, callerID, calleeID uuid.UUID, callType CallType) *ActiveCall {
	return cm.StartGroupCall(chatID, callerID, []uuid.UUID{calleeID}, callType)
}

// StartGroupCall starts a new group call with multiple participants
func (cm *CallManager) StartGroupCall(chatID, callerID uuid.UUID, participantIDs []uuid.UUID, callType CallType) *ActiveCall {
	participants := make(map[uuid.UUID]bool)
	participants[callerID] = true
	for _, pid := range participantIDs {
		participants[pid] = true
	}

	call := &ActiveCall{
		ID:           uuid.New(),
		ChatID:       chatID,
		CallerID:     callerID,
		Participants: participants,
		Type:         callType,
		StartTime:    time.Now(),
	}

	cm.mu.Lock()
	cm.calls[call.ID] = call
	cm.mu.Unlock()

	log.Printf("[WebRTC] Group call started: %s (type: %s, caller: %s, participants: %v)",
		call.ID, callType, callerID, participants)

	return call
}

// GetCall retrieves an active call by ID
func (cm *CallManager) GetCall(callID uuid.UUID) *ActiveCall {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.calls[callID]
}

// EndCall ends an active call
func (cm *CallManager) EndCall(callID uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if call, exists := cm.calls[callID]; exists {
		log.Printf("[WebRTC] Call ended: %s (duration: %s)",
			callID, time.Since(call.StartTime))
		delete(cm.calls, callID)
	}
}

// IsUserInCall checks if a user is currently in any call
func (cm *CallManager) IsUserInCall(userID uuid.UUID) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, call := range cm.calls {
		if call.Participants[userID] {
			return true
		}
	}
	return false
}

// CleanupAbandonedCalls removes calls that haven't been answered after timeout
func (cm *CallManager) CleanupAbandonedCalls(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for callID, call := range cm.calls {
		// Remove calls that haven't been answered and have exceeded timeout
		if !call.Answered && now.Sub(call.StartTime) > timeout {
			log.Printf("[WebRTC] Cleaning up abandoned call: %s (age: %s)", callID, now.Sub(call.StartTime))
			delete(cm.calls, callID)
		}
	}
}

// MarkCallAsAnswered marks a call as answered
func (cm *CallManager) MarkCallAsAnswered(callID uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if call, exists := cm.calls[callID]; exists {
		call.Answered = true
		log.Printf("[WebRTC] Call marked as answered: %s", callID)
	}
}

// AddParticipant adds a user to an existing call
func (cm *CallManager) AddParticipant(callID, userID uuid.UUID) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if call, exists := cm.calls[callID]; exists {
		call.Participants[userID] = true
		log.Printf("[WebRTC] Added participant %s to call %s", userID, callID)
		return true
	}
	return false
}

// RemoveParticipant removes a user from a call
func (cm *CallManager) RemoveParticipant(callID, userID uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if call, exists := cm.calls[callID]; exists {
		delete(call.Participants, userID)
		log.Printf("[WebRTC] Removed participant %s from call %s", userID, callID)
		// If no participants left, end the call
		if len(call.Participants) == 0 {
			delete(cm.calls, callID)
			log.Printf("[WebRTC] Call %s ended - no participants", callID)
		}
	}
}

// HandleCallOffer processes a call offer
func (h *Hub) HandleCallOffer(client *Client, payload []byte) {
	var offer CallOfferPayload
	if err := json.Unmarshal(payload, &offer); err != nil {
		log.Printf("[WebRTC] Invalid call offer: %v", err)
		return
	}

	// Verify caller is in the chat room
	h.mu.RLock()
	room, callerInRoom := h.rooms[offer.ChatID]
	if callerInRoom {
		_, callerInRoom = room[client]
	}
	// Verify callee is online (anywhere in the app)
	calleeClient, calleeOnline := h.userClients[offer.CalleeID]
	h.mu.RUnlock()

	if !callerInRoom {
		h.sendToClient(client, &BroadcastMessage{
			Type:   EventCallError,
			ChatID: offer.ChatID,
			Payload: map[string]interface{}{
				"call_id": "",
				"error":   "not_chat_member",
				"message": "You are not in this chat room",
			},
		})
		log.Printf("[WebRTC] Call offer rejected: caller %s is not in chat room %s", client.UserID, offer.ChatID)
		return
	}

	if !calleeOnline {
		h.sendToClient(client, &BroadcastMessage{
			Type:   EventCallError,
			ChatID: offer.ChatID,
			Payload: map[string]interface{}{
				"call_id": "",
				"error":   "callee_not_in_chat", // Keeping same error code for client compatibility, but meaning is "callee offline"
				"message": "Callee is offline or not reachable",
			},
		})
		log.Printf("[WebRTC] Call offer rejected: callee %s is offline", offer.CalleeID)
		return
	}

	// Check if callee is already in a call
	if h.callManager.IsUserInCall(offer.CalleeID) {
		// Send busy signal to caller
		h.sendToClient(client, &BroadcastMessage{
			Type:   EventCallError,
			ChatID: offer.ChatID,
			Payload: map[string]interface{}{
				"call_id":   "",
				"error":     "callee_busy",
				"message":   "User is already in a call",
				"callee_id": offer.CalleeID,
			},
		})
		log.Printf("[WebRTC] Call offer rejected: callee %s is busy", offer.CalleeID)
		return
	}

	// Create the call
	call := h.callManager.StartCall(offer.ChatID, offer.CallerID, offer.CalleeID, offer.Type)

	// Update offer with generated call ID
	offer.CallID = call.ID
	offer.CallerID = client.UserID // Ensure caller ID matches authenticated user

	// Generate LiveKit room name (using call ID)
	roomName := call.ID.String()

	// Create LiveKit room and generate tokens
	var liveKitRoom *LiveKitRoomInfo
	if h.liveKit != nil && h.liveKit.IsConfigured() {
		// Create LiveKit room
		ctx := context.Background()
		if err := h.liveKit.CreateRoom(ctx, roomName); err != nil {
			log.Printf("[WebRTC] Failed to create LiveKit room: %v", err)
			// Continue without LiveKit - fallback to peer-to-peer
		} else {
			// Generate tokens for both participants
			callerToken, err := h.liveKit.GenerateToken(roomName, offer.CallerID.String())
			if err != nil {
				log.Printf("[WebRTC] Failed to generate caller token: %v", err)
			}
			calleeToken, err := h.liveKit.GenerateToken(roomName, offer.CalleeID.String())
			if err != nil {
				log.Printf("[WebRTC] Failed to generate callee token: %v", err)
			}

			if callerToken != "" && calleeToken != "" {
				liveKitRoom = &LiveKitRoomInfo{
					RoomName:    roomName,
					URL:         h.liveKitURL,
					CallerToken: callerToken,
					CalleeToken: calleeToken,
				}
				log.Printf("[WebRTC] LiveKit room created: %s", roomName)
			} else {
				log.Printf("[WebRTC] Failed to generate valid LiveKit tokens, falling back to P2P")
			}
		}
	}

	// Find the callee client using userClients map
	h.mu.RLock()
	calleeClient = h.userClients[offer.CalleeID]
	h.mu.RUnlock()

	if calleeClient == nil {
		// Callee is not online
		h.sendToClient(client, &BroadcastMessage{
			Type:   EventCallError,
			ChatID: offer.ChatID,
			Payload: map[string]interface{}{
				"call_id":   "",
				"error":     "callee_offline",
				"message":   "User is not online",
				"callee_id": offer.CalleeID,
			},
		})
		log.Printf("[WebRTC] Call offer rejected: callee %s is not online", offer.CalleeID)
		h.callManager.EndCall(call.ID)
		return
	}

	// Send call_id back to caller only after validating callee is online
	callerPayload := map[string]interface{}{
		"call_id": call.ID,
		"chat_id": offer.ChatID,
		"type":    offer.Type,
		"sdp":     offer.SDP,
	}
	if liveKitRoom != nil {
		callerPayload["livekit"] = map[string]interface{}{
			"room_name": liveKitRoom.RoomName,
			"url":       liveKitRoom.URL,
			"token":     liveKitRoom.CallerToken,
		}
	}
	h.sendToClient(client, &BroadcastMessage{
		Type:    EventCallOffer,
		ChatID:  offer.ChatID,
		Payload: callerPayload,
	})

	// Send offer to specific callee
	calleePayload := map[string]interface{}{
		"call_id":   call.ID,
		"chat_id":   offer.ChatID,
		"caller_id": client.UserID,
		"type":      offer.Type,
		"sdp":       offer.SDP,
	}
	if liveKitRoom != nil {
		calleePayload["livekit"] = map[string]interface{}{
			"room_name": liveKitRoom.RoomName,
			"url":       liveKitRoom.URL,
			"token":     liveKitRoom.CalleeToken,
		}
	}
	h.sendToClient(calleeClient, &BroadcastMessage{
		Type:    EventCallOffer,
		ChatID:  offer.ChatID,
		Payload: calleePayload,
	})

	log.Printf("[WebRTC] Call offer sent: %s to callee %s (LiveKit: %v)", call.ID, offer.CalleeID, liveKitRoom != nil)

	// Create system message for call initiated
	go func() {
		if err := h.CreateCallSystemMessage(context.Background(), offer.ChatID, client.UserID, "call_initiated"); err != nil {
			log.Printf("[WebRTC] Failed to create call initiated message: %v", err)
		}
	}()
}

// HandleCallAnswer processes a call answer
func (h *Hub) HandleCallAnswer(client *Client, payload []byte) {
	var answer CallAnswerPayload
	if err := json.Unmarshal(payload, &answer); err != nil {
		log.Printf("[WebRTC] Invalid call answer: %v", err)
		return
	}

	call := h.callManager.GetCall(answer.CallID)
	if call == nil {
		log.Printf("[WebRTC] Call not found: %s", answer.CallID)
		return
	}

	// Mark call as answered
	h.callManager.MarkCallAsAnswered(answer.CallID)

	// Find the caller client
	h.mu.RLock()
	callerClient := h.userClients[call.CallerID]
	h.mu.RUnlock()

	if callerClient != nil {
		// Send answer to specific caller
		h.sendToClient(callerClient, &BroadcastMessage{
			Type:   EventCallAnswer,
			ChatID: call.ChatID,
			Payload: map[string]interface{}{
				"call_id": answer.CallID,
				"sdp":     answer.SDP,
			},
		})
	} else {
		log.Printf("[WebRTC] Caller not online for answer: %s", call.CallerID)
	}

	log.Printf("[WebRTC] Call answer sent: %s", answer.CallID)

	// Create system message for call accepted
	go func() {
		if err := h.CreateCallSystemMessage(context.Background(), call.ChatID, client.UserID, "call_accepted"); err != nil {
			log.Printf("[WebRTC] Failed to create call accepted message: %v", err)
		}
	}()
}

// HandleCallIce processes ICE candidates
func (h *Hub) HandleCallIce(client *Client, payload []byte) {
	var ice CallIcePayload
	if err := json.Unmarshal(payload, &ice); err != nil {
		log.Printf("[WebRTC] Invalid ICE candidate: %v", err)
		return
	}

	// Validate call_id
	if ice.CallID == uuid.Nil {
		log.Printf("[WebRTC] ICE candidate with empty call_id from user %s", client.UserID)
		return
	}

	call := h.callManager.GetCall(ice.CallID)
	if call == nil {
		log.Printf("[WebRTC] Call not found for ICE: %s", ice.CallID)
		return
	}

	// Determine target user (the other party in the call)
	var targetUserID uuid.UUID
	for participantID := range call.Participants {
		if participantID != client.UserID {
			targetUserID = participantID
			break
		}
	}

	// Find the target client
	h.mu.RLock()
	targetClient := h.userClients[targetUserID]
	h.mu.RUnlock()

	if targetClient != nil {
		// Send ICE candidate to specific target
		h.sendToClient(targetClient, &BroadcastMessage{
			Type:   EventCallIce,
			ChatID: call.ChatID,
			Payload: map[string]interface{}{
				"call_id":         ice.CallID,
				"candidate":       ice.Candidate,
				"sdp_mline_index": ice.SDPMLineIndex,
				"sdp_mid":         ice.SDPMid,
			},
		})
	} else {
		log.Printf("[WebRTC] Target user not online for ICE: %s", targetUserID)
	}
}

// HandleCallEnd ends a call
func (h *Hub) HandleCallEnd(client *Client, payload []byte) {
	var data struct {
		CallID uuid.UUID `json:"call_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("[WebRTC] Invalid call end: %v", err)
		return
	}

	call := h.callManager.GetCall(data.CallID)
	if call == nil {
		return
	}

	// Broadcast end to all participants
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallEnd,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id": data.CallID,
		},
	})

	h.callManager.EndCall(data.CallID)

	// Create system message for call ended
	go func() {
		if err := h.CreateCallSystemMessage(context.Background(), call.ChatID, client.UserID, "call_ended"); err != nil {
			log.Printf("[WebRTC] Failed to create call ended message: %v", err)
		}
	}()
}

// HandleCallReject rejects a call
func (h *Hub) HandleCallReject(client *Client, payload []byte) {
	var data struct {
		CallID uuid.UUID `json:"call_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("[WebRTC] Invalid call reject: %v", err)
		return
	}

	call := h.callManager.GetCall(data.CallID)
	if call == nil {
		return
	}

	// Broadcast reject to caller
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallReject,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id": data.CallID,
		},
		ExcludeSender: &client.UserID,
	})

	h.callManager.EndCall(data.CallID)

	// Create system message for call rejected
	go func() {
		if err := h.CreateCallSystemMessage(context.Background(), call.ChatID, client.UserID, "call_rejected"); err != nil {
			log.Printf("[WebRTC] Failed to create call rejected message: %v", err)
		}
	}()
}

// HandleCallAccept accepts a call (sends ringing)
func (h *Hub) HandleCallAccept(client *Client, payload []byte) {
	var data struct {
		CallID uuid.UUID `json:"call_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("[WebRTC] Invalid call accept: %v", err)
		return
	}

	call := h.callManager.GetCall(data.CallID)
	if call == nil {
		return
	}

	// Notify caller that callee is ringing
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallRinging,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id": data.CallID,
		},
		ExcludeSender: &client.UserID,
	})
}

// HandleCallBusy sends busy signal
func (h *Hub) HandleCallBusy(client *Client, payload []byte) {
	var data struct {
		CallID uuid.UUID `json:"call_id"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("[WebRTC] Invalid call busy: %v", err)
		return
	}

	call := h.callManager.GetCall(data.CallID)
	if call == nil {
		return
	}

	// Broadcast busy to caller
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallBusy,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id": data.CallID,
		},
		ExcludeSender: &client.UserID,
	})

	h.callManager.EndCall(data.CallID)
}
