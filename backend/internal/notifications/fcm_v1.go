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
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// FCMClientV1 sends push notifications using Firebase Cloud Messaging HTTP v1 API
// This is the modern API that uses OAuth2 instead of legacy server keys
type FCMClientV1 struct {
	projectID   string
	credentials []byte
	client      *http.Client
	tokenSource oauth2.TokenSource
	mu          sync.RWMutex
}

// NewFCMClientV1 creates FCM v1 client from service account credentials
func NewFCMClientV1() (*FCMClientV1, error) {
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		return nil, fmt.Errorf("FIREBASE_PROJECT_ID not set")
	}

	// Read service account credentials from file or environment
	var credentials []byte
	var err error

	credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credPath != "" {
		credentials, err = os.ReadFile(credPath)
		if err != nil {
			return nil, fmt.Errorf("read credentials file: %w", err)
		}
	} else {
		// Try to read from environment variable (base64 encoded JSON)
		credJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
		if credJSON == "" {
			return nil, fmt.Errorf("neither FIREBASE_CREDENTIALS_PATH nor FIREBASE_CREDENTIALS_JSON set")
		}
		credentials = []byte(credJSON)
	}

	// Create OAuth2 token source
	config, err := google.JWTConfigFromJSON(credentials, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	tokenSource := config.TokenSource(context.Background())

	return &FCMClientV1{
		projectID:   projectID,
		credentials: credentials,
		client:      &http.Client{Timeout: 10 * time.Second},
		tokenSource: tokenSource,
	}, nil
}

// IsConfigured returns true if FCM v1 is configured
func (c *FCMClientV1) IsConfigured() bool {
	return c.projectID != "" && c.credentials != nil
}

// FCMv1Message represents the FCM v1 API message format
type FCMv1Message struct {
	Message FCMv1MessageBody `json:"message"`
}

type FCMv1MessageBody struct {
	Token        string                 `json:"token,omitempty"`
	Notification *FCMv1Notification     `json:"notification,omitempty"`
	Data         map[string]string      `json:"data,omitempty"`
	Android      *FCMv1AndroidConfig    `json:"android,omitempty"`
	APNS         *FCMv1APNSConfig       `json:"apns,omitempty"`
	FCMOptions   *FCMv1Options          `json:"fcm_options,omitempty"`
}

type FCMv1Notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Image string `json:"image,omitempty"`
}

type FCMv1AndroidConfig struct {
	Priority     string                 `json:"priority"`
	Notification *FCMv1AndroidNotif     `json:"notification,omitempty"`
	Data         map[string]string      `json:"data,omitempty"`
}

type FCMv1AndroidNotif struct {
	ChannelID string `json:"channel_id"`
	Sound     string `json:"sound"`
	Priority  string `json:"priority,omitempty"`
}

type FCMv1APNSConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
	Payload *FCMv1APNSPayload `json:"payload,omitempty"`
}

type FCMv1APNSPayload struct {
	Aps *FCMv1Aps `json:"aps,omitempty"`
}

type FCMv1Aps struct {
	Alert            interface{} `json:"alert,omitempty"`
	Badge            int         `json:"badge,omitempty"`
	Sound            string      `json:"sound,omitempty"`
	ContentAvailable int         `json:"content-available,omitempty"`
}

type FCMv1Options struct {
	AnalyticsLabel string `json:"analytics_label,omitempty"`
}

type FCMv1Response struct {
	Name  string `json:"name"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			Type      string `json:"@type"`
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error,omitempty"`
}

// Send sends push notification to a device using FCM v1 API
func (c *FCMClientV1) Send(ctx context.Context, deviceToken, title, body string, data map[string]string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("FCM v1 not configured")
	}

	// Validate device token format
	if len(deviceToken) < 10 || len(deviceToken) > 500 {
		return fmt.Errorf("INVALID_TOKEN: invalid device token format")
	}

	message := FCMv1Message{
		Message: FCMv1MessageBody{
			Token: deviceToken,
			Notification: &FCMv1Notification{
				Title: title,
				Body:  body,
			},
			Data: data,
			Android: &FCMv1AndroidConfig{
				Priority: "high",
				Notification: &FCMv1AndroidNotif{
					ChannelID: "messages",
					Sound:     "default",
					Priority:  "high",
				},
			},
			APNS: &FCMv1APNSConfig{
				Headers: map[string]string{
					"apns-priority": "10",
				},
				Payload: &FCMv1APNSPayload{
					Aps: &FCMv1Aps{
						Alert: map[string]string{
							"title": title,
							"body":  body,
						},
						Badge: 1,
						Sound: "default",
					},
				},
			},
		},
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	// Get OAuth2 token
	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("get oauth2 token: %w", err)
	}

	// FCM v1 API endpoint
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", c.projectID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
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
		var fcmResp FCMv1Response
		if err := json.Unmarshal(bodyBytes, &fcmResp); err == nil && fcmResp.Error != nil {
			// Check for invalid token errors
			if fcmResp.Error.Status == "NOT_FOUND" || fcmResp.Error.Status == "INVALID_ARGUMENT" {
				for _, detail := range fcmResp.Error.Details {
					if detail.ErrorCode == "UNREGISTERED" || detail.ErrorCode == "INVALID_ARGUMENT" {
						return fmt.Errorf("INVALID_TOKEN")
					}
				}
			}
			return fmt.Errorf("FCM v1 error [%s]: %s", fcmResp.Error.Status, fcmResp.Error.Message)
		}
		return fmt.Errorf("FCM v1 returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SendMulticast sends to multiple tokens in parallel with rate limiting
func (c *FCMClientV1) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	var (
		failedTokens []string
		lastErr      error
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	// Process tokens in batches to avoid rate limiting
	batchSize := 100
	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}

		batch := tokens[i:end]
		for _, token := range batch {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()

				if err := c.Send(ctx, t, title, body, data); err != nil {
					mu.Lock()
					log.Printf("[FCM v1] Failed to send to token %s...: %v", t[:min(10, len(t))], err)
					failedTokens = append(failedTokens, t[:min(10, len(t))])
					lastErr = err
					mu.Unlock()
				}
			}(token)
		}

		// Wait for batch to complete before starting next batch
		wg.Wait()

		// Small delay between batches to avoid rate limiting
		if end < len(tokens) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if len(failedTokens) > 0 {
		return fmt.Errorf("failed to send to %d/%d tokens (last error: %w)", len(failedTokens), len(tokens), lastErr)
	}
	return nil
}
