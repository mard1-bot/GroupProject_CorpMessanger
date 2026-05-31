package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// FCMClient sends push notifications using Firebase HTTP API
type FCMClient struct {
	serverKey string
	client    *http.Client
}

// NewFCMClient creates FCM client from environment
func NewFCMClient() *FCMClient {
	return &FCMClient{
		serverKey: os.Getenv("FIREBASE_SERVER_KEY"),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// IsConfigured returns true if FCM is configured
func (c *FCMClient) IsConfigured() bool {
	return c.serverKey != ""
}

type FCMMessage struct {
	To           string            `json:"to,omitempty"`
	Token        string            `json:"token,omitempty"`
	Notification *FCMNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *FCMAndroidConfig `json:"android,omitempty"`
	APNS         *FCMAPNSConfig    `json:"apns,omitempty"`
}

type FCMNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type FCMAndroidConfig struct {
	Priority     string           `json:"priority"`
	Notification *FCMAndroidNotif `json:"notification,omitempty"`
}

type FCMAndroidNotif struct {
	ChannelID string `json:"channel_id"`
	Sound     string `json:"sound"`
}

type FCMAPNSConfig struct {
	Payload *FCMAPNSPayload `json:"payload,omitempty"`
}

type FCMAPNSPayload struct {
	Aps *FCMAps `json:"aps,omitempty"`
}

type FCMAps struct {
	Sound string `json:"sound"`
	Badge int    `json:"badge"`
}

type FCMResponse struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
	Results []struct {
		MessageID string `json:"message_id"`
		Error     string `json:"error"`
	} `json:"results"`
}

// Send sends push notification to a device using legacy FCM API
// Note: FCM HTTP v1 API is available in fcm_v1.go with OAuth2 support.
// This legacy client is kept for backward compatibility with existing deployments.
// To migrate to v1, use FCMClientV1 instead. See: https://firebase.google.com/docs/cloud-messaging/migrate-v1
func (c *FCMClient) Send(ctx context.Context, deviceToken, title, body string, data map[string]string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("FCM not configured: FIREBASE_SERVER_KEY not set")
	}

	// Validate device token format (must not be empty and reasonable length)
	if len(deviceToken) < 10 || len(deviceToken) > 500 {
		return fmt.Errorf("INVALID_TOKEN: invalid device token format")
	}

	message := FCMMessage{
		Token: deviceToken,
		Notification: &FCMNotification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &FCMAndroidConfig{
			Priority: "high",
			Notification: &FCMAndroidNotif{
				ChannelID: "messages",
				Sound:     "default",
			},
		},
		APNS: &FCMAPNSConfig{
			Payload: &FCMAPNSPayload{
				Aps: &FCMAps{
					Sound: "default",
					Badge: 1,
				},
			},
		},
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	// Legacy FCM API - will be deprecated by Google
	// Migration to v1 requires service account credentials and OAuth2
	req, err := http.NewRequestWithContext(ctx, "POST", "https://fcm.googleapis.com/fcm/send", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "key="+c.serverKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FCM returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var fcmResp FCMResponse
	if err := json.Unmarshal(bodyBytes, &fcmResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if fcmResp.Failure > 0 && len(fcmResp.Results) > 0 {
		// Check for invalid token error
		if fcmResp.Results[0].Error == "NotRegistered" || fcmResp.Results[0].Error == "InvalidRegistration" {
			return fmt.Errorf("INVALID_TOKEN")
		}
		return fmt.Errorf("FCM error: %s", fcmResp.Results[0].Error)
	}

	return nil
}

// SendMulticast sends to multiple tokens (using sendAll in batches)
// Returns aggregated error if any sends failed, logs individual failures
func (c *FCMClient) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	var failedTokens []string
	var lastErr error

	for _, token := range tokens {
		if err := c.Send(ctx, token, title, body, data); err != nil {
			// Log error but continue with other tokens
			log.Printf("[FCM] Failed to send to token %s...: %v", token[:min(10, len(token))], err)
			failedTokens = append(failedTokens, token[:min(10, len(token))])
			lastErr = err
		}
	}

	// Return aggregated error if any sends failed
	if len(failedTokens) > 0 {
		return fmt.Errorf("failed to send to %d/%d tokens (last error: %w)", len(failedTokens), len(tokens), lastErr)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
