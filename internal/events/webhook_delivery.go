package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"goapitemplate/internal/database"
	"goapitemplate/pkg/models"

	"github.com/sirupsen/logrus"
)

type WebhookDeliveryService struct {
	db     *database.DB
	client *http.Client
	logger *logrus.Logger
}

func NewWebhookDeliveryService(db *database.DB) *WebhookDeliveryService {
	return &WebhookDeliveryService{
		db: db,
		client: &http.Client{
			Timeout: time.Second * 30,
		},
		logger: logrus.New(),
	}
}

// DeliverEvent finds all applicable webhooks and delivers the event to them
func (w *WebhookDeliveryService) DeliverEvent(ctx context.Context, event models.Event) error {
	// Find all active webhooks - we'll filter by event type in Go for SQLite compatibility
	var allWebhooks []models.WebhookEndpoint
	err := w.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&allWebhooks).Error
	if err != nil {
		w.logger.WithError(err).Error("Failed to find webhooks")
		return err
	}

	// Filter webhooks that should receive this event type
	var webhooks []models.WebhookEndpoint
	for _, webhook := range allWebhooks {
		for _, eventType := range webhook.EventTypes {
			if eventType == event.Type {
				webhooks = append(webhooks, webhook)
				break
			}
		}
	}

	// Create delivery records and attempt delivery for each webhook
	for _, webhook := range webhooks {
		delivery := models.WebhookDelivery{
			ID:           generateDeliveryID(),
			WebhookID:    webhook.ID,
			EventID:      event.ID,
			Status:       "pending",
			AttemptCount: 0,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		// Save initial delivery record
		if err := w.db.WithContext(ctx).Create(&delivery).Error; err != nil {
			w.logger.WithError(err).WithFields(logrus.Fields{
				"webhook_id": webhook.ID,
				"event_id":   event.ID,
			}).Error("Failed to create delivery record")
			continue
		}

		// Attempt delivery asynchronously
		go w.attemptDelivery(context.Background(), webhook, event, &delivery)
	}

	return nil
}

// attemptDelivery attempts to deliver an event to a webhook endpoint
func (w *WebhookDeliveryService) attemptDelivery(ctx context.Context, webhook models.WebhookEndpoint, event models.Event, delivery *models.WebhookDelivery) {
	maxRetries := webhook.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	timeout := time.Duration(webhook.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// Create a client with the webhook-specific timeout
	client := &http.Client{Timeout: timeout}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		delivery.AttemptCount = attempt
		delivery.LastAttempt = &time.Time{}
		*delivery.LastAttempt = time.Now()
		delivery.UpdatedAt = time.Now()

		success, response, err := w.deliverToEndpoint(ctx, client, webhook, event)
		
		if success {
			delivery.Status = "success"
			delivery.Response = response
			delivery.ErrorMessage = ""
			delivery.NextRetry = nil
		} else {
			if attempt < maxRetries {
				delivery.Status = "pending"
				nextRetry := time.Now().Add(w.calculateRetryDelay(attempt))
				delivery.NextRetry = &nextRetry
			} else {
				delivery.Status = "failed"
				delivery.NextRetry = nil
			}

			if err != nil {
				delivery.ErrorMessage = err.Error()
			}
			delivery.Response = response
		}

		// Update delivery record
		if updateErr := w.db.WithContext(ctx).Save(delivery).Error; updateErr != nil {
			w.logger.WithError(updateErr).WithFields(logrus.Fields{
				"delivery_id": delivery.ID,
				"webhook_id":  webhook.ID,
				"event_id":    event.ID,
			}).Error("Failed to update delivery record")
		}

		if success {
			w.logger.WithFields(logrus.Fields{
				"delivery_id": delivery.ID,
				"webhook_id":  webhook.ID,
				"event_id":    event.ID,
				"attempt":     attempt,
			}).Info("Webhook delivered successfully")
			break
		}

		w.logger.WithFields(logrus.Fields{
			"delivery_id": delivery.ID,
			"webhook_id":  webhook.ID,
			"event_id":    event.ID,
			"attempt":     attempt,
			"error":       err,
		}).Warn("Webhook delivery failed")

		// Wait before retry (except on last attempt)
		if attempt < maxRetries {
			time.Sleep(w.calculateRetryDelay(attempt))
		}
	}
}

// deliverToEndpoint performs the actual HTTP request to the webhook endpoint
func (w *WebhookDeliveryService) deliverToEndpoint(ctx context.Context, client *http.Client, webhook models.WebhookEndpoint, event models.Event) (bool, string, error) {
	// Prepare webhook payload
	payload := map[string]interface{}{
		"event_id":        event.ID,
		"event_type":      event.Type,
		"stream_id":       event.StreamID,
		"source":          event.Source,
		"data":            event.Data,
		"timestamp":       event.Timestamp.Format(time.RFC3339),
		"sequence_number": event.SequenceNumber,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoAPITemplate-Webhook/1.0")
	
	// Add signature header for verification
	if webhook.Secret != "" {
		signature := w.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Add event metadata headers
	req.Header.Set("X-Event-Type", event.Type)
	req.Header.Set("X-Event-Stream", event.StreamID)
	req.Header.Set("X-Event-ID", event.ID)

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, _ := io.ReadAll(resp.Body)
	responseStr := string(body)
	if len(responseStr) > 1000 {
		responseStr = responseStr[:1000] + "..."
	}

	// Check if delivery was successful (2xx status codes)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		return false, responseStr, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return true, responseStr, nil
}

// generateSignature creates HMAC-SHA256 signature for webhook verification
func (w *WebhookDeliveryService) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// calculateRetryDelay calculates exponential backoff delay
func (w *WebhookDeliveryService) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, etc., up to max 30s
	delay := time.Duration(1<<uint(attempt-1)) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

// RetryFailedDeliveries finds and retries failed deliveries that are ready for retry
func (w *WebhookDeliveryService) RetryFailedDeliveries(ctx context.Context) error {
	var deliveries []models.WebhookDelivery
	
	// Find pending deliveries that are ready for retry
	err := w.db.WithContext(ctx).
		Preload("Webhook").
		Preload("Event").
		Where("status = ? AND next_retry <= ?", "pending", time.Now()).
		Find(&deliveries).Error
	
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		if delivery.Webhook != nil && delivery.Event != nil {
			w.attemptDelivery(ctx, *delivery.Webhook, *delivery.Event, &delivery)
		}
	}

	return nil
}

func generateDeliveryID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("del_%x", bytes)
}