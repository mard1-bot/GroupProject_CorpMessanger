package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// ExpoPushClient sends push notifications using Expo Push API
// This is the recommended way for React Native apps built with Expo
type ExpoPushClient struct {
	httpClient *http.Client
	apiKey     string
	logger     *slog.Logger
}

// NewExpoPushClient creates a new Expo Push API client
func NewExpoPushClient(apiKey string, logger *slog.Logger) *ExpoPushClient {
	return &ExpoPushClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
		logger:     logger,
	}
}

// IsConfigured returns true if Expo Push API is configured
func (c *ExpoPushClient) IsConfigured() bool {
	// Expo Push API allows anonymous requests (without API key)
	return true
}

// ExpoPushMessage represents a push notification message for Expo
type ExpoPushMessage struct {
	To               string                 `json:"to"`                         // Expo push token
	Title            string                 `json:"title,omitempty"`            // Notification title
	Body             string                 `json:"body,omitempty"`             // Notification body
	Data             map[string]interface{} `json:"data,omitempty"`             // Custom data
	Sound            string                 `json:"sound,omitempty"`            // Sound to play
	Priority         string                 `json:"priority,omitempty"`         // "default", "normal", "high"
	Badge            int                    `json:"badge,omitempty"`            // Badge count
	ChannelID        string                 `json:"channelId,omitempty"`        // Android channel ID
	TTL              *int                   `json:"ttl,omitempty"`              // Time to live in seconds
	Expiration       *int                   `json:"expiration,omitempty"`       // Expiration time in seconds
	Subtitle         string                 `json:"subtitle,omitempty"`         // Subtitle (iOS)
	Category         string                 `json:"category,omitempty"`         // Category identifier
	ContentAvailable bool                   `json:"contentAvailable,omitempty"` // Silent notification
	MutableContent   bool                   `json:"mutableContent,omitempty"`   // Allow modification (iOS)
}

// ExpoPushResponse represents the response from Expo Push API
type ExpoPushResponse struct {
	Data []ExpoPushReceipt `json:"data"`
}

// ExpoPushReceipt represents the receipt for a single push notification
type ExpoPushReceipt struct {
	Status  string `json:"status"` // "ok", "error"
	Message string `json:"message,omitempty"`
	Details *struct {
		Error string `json:"error"` // "DeviceNotRegistered", "MessageTooBig", etc.
	} `json:"details,omitempty"`
}

// Send sends a single push notification via Expo Push API
func (c *ExpoPushClient) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	if !c.IsConfigured() {
		return fmt.Errorf("Expo Push API not configured")
	}

	// Validate Expo push token format
	if !isValidExpoToken(token) {
		return fmt.Errorf("invalid Expo push token format: %s", token)
	}

	// Convert data map to interface map
	dataInterface := make(map[string]interface{})
	for k, v := range data {
		dataInterface[k] = v
	}

	message := ExpoPushMessage{
		To:        token,
		Title:     title,
		Body:      body,
		Data:      dataInterface,
		Sound:     "default",
		Priority:  "high",
		Badge:     1,
		ChannelID: "messages",
		TTL:       intPtr(3600), // 1 hour
	}

	payload, err := json.Marshal([]ExpoPushMessage{message})
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://exp.host/--/api/v2/push/send", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Expo Push API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var expoResp ExpoPushResponse
	if err := json.Unmarshal(bodyBytes, &expoResp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	// Check for errors in response
	for _, receipt := range expoResp.Data {
		if receipt.Status == "error" {
			if receipt.Details != nil && receipt.Details.Error == "DeviceNotRegistered" {
				return fmt.Errorf("DEVICE_NOT_REGISTERED")
			}
			return fmt.Errorf("Expo Push error: %s", receipt.Message)
		}
	}

	c.logger.Debug("Expo Push sent successfully", "token", token[:20])
	return nil
}

// SendMulticast sends push notifications to multiple tokens
func (c *ExpoPushClient) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	if len(tokens) == 0 {
		return nil
	}

	if !c.IsConfigured() {
		return fmt.Errorf("Expo Push API not configured")
	}

	// Convert data map to interface map
	dataInterface := make(map[string]interface{})
	for k, v := range data {
		dataInterface[k] = v
	}

	// Create messages for all tokens
	messages := make([]ExpoPushMessage, 0, len(tokens))
	for _, token := range tokens {
		if !isValidExpoToken(token) {
			c.logger.Warn("skipping invalid Expo token", "token", token)
			continue
		}

		messages = append(messages, ExpoPushMessage{
			To:        token,
			Title:     title,
			Body:      body,
			Data:      dataInterface,
			Sound:     "default",
			Priority:  "high",
			Badge:     1,
			ChannelID: "messages",
			TTL:       intPtr(3600),
		})
	}

	if len(messages) == 0 {
		return fmt.Errorf("no valid tokens to send")
	}

	// Send in batches (Expo allows up to 100 messages per request)
	batchSize := 100
	var (
		failedTokens []string
		lastErr      error
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	for i := 0; i < len(messages); i += batchSize {
		end := i + batchSize
		if end > len(messages) {
			end = len(messages)
		}

		batch := messages[i:end]
		wg.Add(1)
		go func(msgs []ExpoPushMessage) {
			defer wg.Done()

			if err := c.sendBatch(ctx, msgs); err != nil {
				mu.Lock()
				for _, msg := range msgs {
					if len(msg.To) > 20 {
						failedTokens = append(failedTokens, msg.To[:20])
					} else {
						failedTokens = append(failedTokens, msg.To)
					}
				}
				lastErr = err
				mu.Unlock()
			}
		}(batch)

		// Small delay between batches to avoid rate limiting
		if end < len(messages) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	wg.Wait()

	if len(failedTokens) > 0 {
		return fmt.Errorf("failed to send to %d/%d tokens (last error: %w)", len(failedTokens), len(tokens), lastErr)
	}

	return nil
}

// sendBatch sends a batch of messages to Expo Push API
func (c *ExpoPushClient) sendBatch(ctx context.Context, messages []ExpoPushMessage) error {
	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal messages: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://exp.host/--/api/v2/push/send", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Expo Push API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var expoResp ExpoPushResponse
	if err := json.Unmarshal(bodyBytes, &expoResp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	// Check for errors in response
	for _, receipt := range expoResp.Data {
		if receipt.Status == "error" {
			if receipt.Details != nil && receipt.Details.Error == "DeviceNotRegistered" {
				c.logger.Warn("device not registered", "token", receipt.Message)
			} else {
				c.logger.Warn("Expo Push error", "error", receipt.Message)
			}
		}
	}

	return nil
}

// isValidExpoToken checks if a token is a valid Expo push token
func isValidExpoToken(token string) bool {
	// Expo push tokens start with "ExponentPushToken[" or "ExpoPushToken["
	if len(token) < 20 {
		return false
	}
	return true
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}
