package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

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
	ID        uuid.UUID
	ChatID    uuid.UUID
	CallerID  uuid.UUID
	CalleeID  uuid.UUID
	Type      CallType
	StartTime time.Time
	mu        sync.RWMutex
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
	call := &ActiveCall{
		ID:        uuid.New(),
		ChatID:    chatID,
		CallerID:  callerID,
		CalleeID:  calleeID,
		Type:      callType,
		StartTime: time.Now(),
	}

	cm.mu.Lock()
	cm.calls[call.ID] = call
	cm.mu.Unlock()

	log.Printf("[WebRTC] Call started: %s (type: %s, caller: %s, callee: %s)",
		call.ID, callType, callerID, calleeID)

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
		if call.CallerID == userID || call.CalleeID == userID {
			return true
		}
	}
	return false
}

// HandleCallOffer processes a call offer
func (h *Hub) HandleCallOffer(client *Client, payload []byte) {
	var offer CallOfferPayload
	if err := json.Unmarshal(payload, &offer); err != nil {
		log.Printf("[WebRTC] Invalid call offer: %v", err)
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

	// Broadcast to callee
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallOffer,
		ChatID: offer.ChatID,
		Payload: map[string]interface{}{
			"call_id":   call.ID,
			"chat_id":   offer.ChatID,
			"caller_id": client.UserID,
			"type":      offer.Type,
			"sdp":       offer.SDP,
		},
		ExcludeSender: &client.UserID,
	})

	log.Printf("[WebRTC] Call offer sent: %s to callee %s", call.ID, offer.CalleeID)
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

	// Broadcast answer to caller
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallAnswer,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id": answer.CallID,
			"sdp":     answer.SDP,
		},
		ExcludeSender: &client.UserID,
	})

	log.Printf("[WebRTC] Call answer sent: %s", answer.CallID)
}

// HandleCallIce processes ICE candidates
func (h *Hub) HandleCallIce(client *Client, payload []byte) {
	var ice CallIcePayload
	if err := json.Unmarshal(payload, &ice); err != nil {
		log.Printf("[WebRTC] Invalid ICE candidate: %v", err)
		return
	}

	call := h.callManager.GetCall(ice.CallID)
	if call == nil {
		log.Printf("[WebRTC] Call not found for ICE: %s", ice.CallID)
		return
	}

	// Broadcast ICE candidate to the other party
	h.BroadcastToChat(&BroadcastMessage{
		Type:   EventCallIce,
		ChatID: call.ChatID,
		Payload: map[string]interface{}{
			"call_id":         ice.CallID,
			"candidate":       ice.Candidate,
			"sdp_mline_index": ice.SDPMLineIndex,
			"sdp_mid":         ice.SDPMid,
		},
		ExcludeSender: &client.UserID,
	})
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
