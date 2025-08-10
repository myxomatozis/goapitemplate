package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goapitemplate/internal/database"
	"goapitemplate/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *database.DB {
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	db := &database.DB{DB: gormDB}
	
	// Run migrations
	err = db.AutoMigrate()
	require.NoError(t, err)

	return db
}

func createTestWebhook(t *testing.T, db *database.DB, eventTypes []string) models.WebhookEndpoint {
	webhook := models.WebhookEndpoint{
		ID:             "test-webhook-123",
		Name:           "Test Webhook",
		URL:            "http://example.com/webhook",
		Secret:         "test-secret",
		EventTypes:     eventTypes,
		Enabled:        true,
		MaxRetries:     3,
		TimeoutSeconds: 30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := db.Create(&webhook).Error
	require.NoError(t, err)

	return webhook
}

func createTestEvent(t *testing.T, db *database.DB, eventType string) models.Event {
	event := models.Event{
		ID:             "test-event-123",
		Type:           eventType,
		StreamID:       "test-stream",
		Source:         "test-service",
		Data:           models.JSON{"test": "data"},
		Timestamp:      time.Now(),
		SequenceNumber: 1,
		CreatedAt:      time.Now(),
	}

	err := db.CreateEventWithSequence(&event)
	require.NoError(t, err)

	return event
}

func TestWebhookDeliveryService_DeliverEvent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	tests := []struct {
		name         string
		eventType    string
		webhookTypes []string
		serverStatus int
		wantDelivery bool
	}{
		{
			name:         "successful delivery to matching webhook",
			eventType:    "user.created",
			webhookTypes: []string{"user.created", "user.updated"},
			serverStatus: http.StatusOK,
			wantDelivery: true,
		},
		{
			name:         "no delivery for non-matching webhook",
			eventType:    "payment.processed",
			webhookTypes: []string{"user.created", "user.updated"},
			serverStatus: http.StatusOK,
			wantDelivery: false,
		},
		{
			name:         "delivery recorded for failed request",
			eventType:    "user.created",
			webhookTypes: []string{"user.created"},
			serverStatus: http.StatusInternalServerError,
			wantDelivery: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "GoAPITemplate-Webhook/1.0", r.Header.Get("User-Agent"))
				assert.Equal(t, tt.eventType, r.Header.Get("X-Event-Type"))
				assert.NotEmpty(t, r.Header.Get("X-Webhook-Signature"))

				// Verify request body
				var payload map[string]interface{}
				err := json.NewDecoder(r.Body).Decode(&payload)
				assert.NoError(t, err)
				assert.Equal(t, tt.eventType, payload["event_type"])
				assert.Equal(t, "test-stream", payload["stream_id"])

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					w.Write([]byte(`{"status": "received"}`))
				} else {
					w.Write([]byte(`{"error": "internal error"}`))
				}
			}))
			defer server.Close()

			// Create webhook with server URL
			webhook := createTestWebhook(t, db, tt.webhookTypes)
			webhook.URL = server.URL
			err := db.Save(&webhook).Error
			require.NoError(t, err)

			// Create test event
			event := createTestEvent(t, db, tt.eventType)

			// Deliver event
			err = service.DeliverEvent(context.Background(), event)
			assert.NoError(t, err)

			// Allow time for async delivery
			time.Sleep(100 * time.Millisecond)

			// Check delivery record
			var deliveries []models.WebhookDelivery
			err = db.Find(&deliveries).Error
			require.NoError(t, err)

			if tt.wantDelivery {
				assert.Len(t, deliveries, 1)
				delivery := deliveries[0]
				assert.Equal(t, webhook.ID, delivery.WebhookID)
				assert.Equal(t, event.ID, delivery.EventID)
				assert.Greater(t, delivery.AttemptCount, 0)

				if tt.serverStatus == http.StatusOK {
					assert.Equal(t, "success", delivery.Status)
					assert.Contains(t, delivery.Response, "received")
				} else {
					assert.Contains(t, []string{"failed", "pending"}, delivery.Status)
				}
			} else {
				assert.Len(t, deliveries, 0)
			}

			// Clean up for next test
			db.Exec("DELETE FROM webhook_deliveries")
			db.Exec("DELETE FROM webhook_endpoints WHERE id = ?", webhook.ID)
			db.Exec("DELETE FROM events WHERE id = ?", event.ID)
		})
	}
}

func TestWebhookDeliveryService_RetryFailedDeliveries(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	// Create test server that succeeds on retry
	retryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retryCount++
		if retryCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "temporary failure"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success on retry"}`))
		}
	}))
	defer server.Close()

	// Create webhook and event
	webhook := createTestWebhook(t, db, []string{"test.event"})
	webhook.URL = server.URL
	err := db.Save(&webhook).Error
	require.NoError(t, err)

	event := createTestEvent(t, db, "test.event")

	// Create a failed delivery that's ready for retry
	delivery := models.WebhookDelivery{
		ID:           "test-delivery",
		WebhookID:    webhook.ID,
		EventID:      event.ID,
		Status:       "pending",
		AttemptCount: 1,
		NextRetry:    &time.Time{}, // Set to past time to make it ready
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	*delivery.NextRetry = time.Now().Add(-1 * time.Minute)

	err = db.Create(&delivery).Error
	require.NoError(t, err)

	// Retry failed deliveries
	err = service.RetryFailedDeliveries(context.Background())
	assert.NoError(t, err)

	// Allow more time for async retry since it involves HTTP call
	time.Sleep(500 * time.Millisecond)

	// Check delivery was updated
	var updatedDelivery models.WebhookDelivery
	err = db.First(&updatedDelivery, "id = ?", delivery.ID).Error
	require.NoError(t, err)

	// Should eventually succeed on the second attempt
	if updatedDelivery.Status == "pending" {
		// Allow more time for async processing
		time.Sleep(500 * time.Millisecond)
		err = db.First(&updatedDelivery, "id = ?", delivery.ID).Error
		require.NoError(t, err)
	}

	assert.Equal(t, "success", updatedDelivery.Status)
	assert.Contains(t, updatedDelivery.Response, "success on retry")
	assert.GreaterOrEqual(t, updatedDelivery.AttemptCount, 2)
}

func TestWebhookDeliveryService_GenerateSignature(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	payload := []byte(`{"test": "data"}`)
	secret := "test-secret"

	signature := service.generateSignature(payload, secret)
	
	// Verify signature format
	assert.True(t, len(signature) > 7) // "sha256=" + hex
	assert.Contains(t, signature, "sha256=")

	// Verify signature is consistent
	signature2 := service.generateSignature(payload, secret)
	assert.Equal(t, signature, signature2)

	// Verify different payloads produce different signatures
	signature3 := service.generateSignature([]byte(`{"different": "data"}`), secret)
	assert.NotEqual(t, signature, signature3)
}

func TestWebhookDeliveryService_CalculateRetryDelay(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	tests := []struct {
		attempt      int
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{1, 1 * time.Second, 1 * time.Second},
		{2, 2 * time.Second, 2 * time.Second},
		{3, 4 * time.Second, 4 * time.Second},
		{4, 8 * time.Second, 8 * time.Second},
		{10, 30 * time.Second, 30 * time.Second}, // Should cap at 30s
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := service.calculateRetryDelay(tt.attempt)
			assert.True(t, delay >= tt.expectedMin, "delay should be at least %v, got %v", tt.expectedMin, delay)
			assert.True(t, delay <= tt.expectedMax, "delay should be at most %v, got %v", tt.expectedMax, delay)
		})
	}
}

func TestWebhookDeliveryService_DisabledWebhook(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	// Create disabled webhook
	webhook := createTestWebhook(t, db, []string{"test.event"})
	webhook.Enabled = false
	err := db.Save(&webhook).Error
	require.NoError(t, err)

	event := createTestEvent(t, db, "test.event")

	// Deliver event
	err = service.DeliverEvent(context.Background(), event)
	assert.NoError(t, err)

	// Allow time for potential delivery
	time.Sleep(100 * time.Millisecond)

	// Verify no delivery was created for disabled webhook
	var deliveries []models.WebhookDelivery
	err = db.Find(&deliveries).Error
	require.NoError(t, err)
	assert.Len(t, deliveries, 0)
}

func BenchmarkWebhookDelivery(b *testing.B) {
	// Helper functions that work with both *testing.T and *testing.B
	setupBenchDB := func() *database.DB {
		gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			b.Fatal(err)
		}

		db := &database.DB{DB: gormDB}
		
		err = db.AutoMigrate()
		if err != nil {
			b.Fatal(err)
		}

		return db
	}

	createBenchWebhook := func(db *database.DB, eventTypes []string) models.WebhookEndpoint {
		webhook := models.WebhookEndpoint{
			ID:             "bench-webhook-123",
			Name:           "Bench Webhook",
			URL:            "http://example.com/webhook",
			Secret:         "bench-secret",
			EventTypes:     eventTypes,
			Enabled:        true,
			MaxRetries:     3,
			TimeoutSeconds: 30,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err := db.Create(&webhook).Error
		if err != nil {
			b.Fatal(err)
		}

		return webhook
	}

	createBenchEvent := func(db *database.DB, eventType string) models.Event {
		event := models.Event{
			ID:             "bench-event-123",
			Type:           eventType,
			StreamID:       "bench-stream",
			Source:         "bench-service",
			Data:           models.JSON{"bench": "data"},
			Timestamp:      time.Now(),
			SequenceNumber: 1,
			CreatedAt:      time.Now(),
		}

		err := db.CreateEventWithSequence(&event)
		if err != nil {
			b.Fatal(err)
		}

		return event
	}

	db := setupBenchDB()
	defer db.Close()

	service := NewWebhookDeliveryService(db)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	webhook := createBenchWebhook(db, []string{"bench.event"})
	webhook.URL = server.URL
	err := db.Save(&webhook).Error
	if err != nil {
		b.Fatal(err)
	}

	event := createBenchEvent(db, "bench.event")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := service.DeliverEvent(context.Background(), event)
		if err != nil {
			b.Error(err)
		}
	}
}