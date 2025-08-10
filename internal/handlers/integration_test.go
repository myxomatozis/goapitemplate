package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventToWebhookIntegration tests the full flow from event creation to webhook delivery
func TestEventToWebhookIntegration(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Register webhook routes directly for testing
	api := router.Group("/api/v1")
	{
		api.POST("/webhooks", handler.CreateWebhook)
		api.GET("/webhooks/stats", handler.GetWebhookStats)
		api.PUT("/webhooks/:id", handler.UpdateWebhook)
		api.POST("/events", handler.CreateEvent)
	}

	// Mock webhook server
	webhookCalled := false
	var receivedPayload map[string]interface{}
	
	mockWebhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		
		// Verify request
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "GoAPITemplate-Webhook/1.0", r.Header.Get("User-Agent"))
		assert.NotEmpty(t, r.Header.Get("X-Webhook-Signature"))
		
		// Parse payload
		err := json.NewDecoder(r.Body).Decode(&receivedPayload)
		assert.NoError(t, err)
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "received"}`))
	}))
	defer mockWebhookServer.Close()

	// Step 1: Create a webhook endpoint
	webhookPayload := map[string]interface{}{
		"name":        "Integration Test Webhook",
		"url":         mockWebhookServer.URL,
		"secret":      "test-secret-123",
		"event_types": []string{"user.created", "order.placed"},
	}

	webhookBytes, _ := json.Marshal(webhookPayload)
	req, _ := http.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(webhookBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var webhookResponse models.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &webhookResponse)
	require.NoError(t, err)
	assert.True(t, webhookResponse.Success)

	// Step 2: Create an event that matches the webhook
	eventPayload := map[string]interface{}{
		"type":      "user.created",
		"stream_id": "user-integration-test",
		"source":    "integration-test",
		"data": map[string]interface{}{
			"user_id": 12345,
			"email":   "test@integration.com",
			"name":    "Integration Test User",
		},
	}

	eventBytes, _ := json.Marshal(eventPayload)
	req, _ = http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(eventBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var eventResponse models.APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &eventResponse)
	require.NoError(t, err)
	assert.True(t, eventResponse.Success)

	// Step 3: Wait for async webhook delivery
	time.Sleep(200 * time.Millisecond)

	// Verify webhook was called
	assert.True(t, webhookCalled, "Webhook should have been called")
	
	// Verify webhook payload
	if webhookCalled {
		assert.Equal(t, "user.created", receivedPayload["event_type"])
		assert.Equal(t, "user-integration-test", receivedPayload["stream_id"])
		assert.Equal(t, "integration-test", receivedPayload["source"])
		
		// Verify event data was passed through
		eventData := receivedPayload["data"].(map[string]interface{})
		assert.Equal(t, float64(12345), eventData["user_id"]) // JSON numbers are float64
		assert.Equal(t, "test@integration.com", eventData["email"])
		assert.Equal(t, "Integration Test User", eventData["name"])
	}

	// Step 4: Verify event was saved to database
	var savedEvents []models.Event
	err = db.Find(&savedEvents).Error
	require.NoError(t, err)
	assert.Len(t, savedEvents, 1)

	savedEvent := savedEvents[0]
	assert.Equal(t, "user.created", savedEvent.Type)
	assert.Equal(t, "user-integration-test", savedEvent.StreamID)
	assert.Equal(t, int64(1), savedEvent.SequenceNumber) // First event in stream

	// Step 5: Verify webhook delivery was recorded
	var deliveries []models.WebhookDelivery
	err = db.Find(&deliveries).Error
	require.NoError(t, err)
	assert.Len(t, deliveries, 1)

	delivery := deliveries[0]
	assert.Equal(t, "success", delivery.Status)
	assert.Equal(t, 1, delivery.AttemptCount)
	assert.Contains(t, delivery.Response, "received")

	// Step 6: Get webhook statistics
	req, _ = http.NewRequest("GET", "/api/v1/webhooks/stats", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var statsResponse models.APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &statsResponse)
	require.NoError(t, err)
	assert.True(t, statsResponse.Success)

	statsBytes, _ := json.Marshal(statsResponse.Data)
	var stats struct {
		TotalDeliveries      int64   `json:"total_deliveries"`
		SuccessfulDeliveries int64   `json:"successful_deliveries"`
		PendingDeliveries    int64   `json:"pending_deliveries"`
		FailedDeliveries     int64   `json:"failed_deliveries"`
		SuccessRate          float64 `json:"success_rate"`
	}
	err = json.Unmarshal(statsBytes, &stats)
	require.NoError(t, err)

	assert.Equal(t, int64(1), stats.TotalDeliveries)
	assert.Equal(t, int64(1), stats.SuccessfulDeliveries)
	assert.Equal(t, int64(0), stats.PendingDeliveries)
	assert.Equal(t, int64(0), stats.FailedDeliveries)
	assert.Equal(t, float64(100), stats.SuccessRate)
}

// TestNonMatchingEventType tests that events not matching webhook event_types are not delivered
func TestNonMatchingEventType(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Register webhook routes directly for testing
	api := router.Group("/api/v1")
	{
		api.POST("/webhooks", handler.CreateWebhook)
		api.POST("/events", handler.CreateEvent)
	}

	// Mock webhook server
	webhookCalled := false
	mockWebhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockWebhookServer.Close()

	// Create webhook that only listens to "payment.processed"
	webhookPayload := map[string]interface{}{
		"name":        "Payment Webhook",
		"url":         mockWebhookServer.URL,
		"secret":      "test-secret",
		"event_types": []string{"payment.processed"},
	}

	webhookBytes, _ := json.Marshal(webhookPayload)
	req, _ := http.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(webhookBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Create "user.created" event (should not trigger webhook)
	eventPayload := map[string]interface{}{
		"type":      "user.created",
		"stream_id": "user-test",
		"source":    "test",
		"data":      map[string]interface{}{"user_id": 123},
	}

	eventBytes, _ := json.Marshal(eventPayload)
	req, _ = http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(eventBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Wait for potential delivery
	time.Sleep(200 * time.Millisecond)

	// Verify webhook was NOT called
	assert.False(t, webhookCalled, "Webhook should not have been called for non-matching event type")

	// Verify no delivery records were created
	var deliveries []models.WebhookDelivery
	err := db.Find(&deliveries).Error
	require.NoError(t, err)
	assert.Len(t, deliveries, 0)
}

// TestDisabledWebhookIntegration tests that disabled webhooks don't receive events
func TestDisabledWebhookIntegration(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Register webhook routes directly for testing
	api := router.Group("/api/v1")
	{
		api.POST("/webhooks", handler.CreateWebhook)
		api.PUT("/webhooks/:id", handler.UpdateWebhook)
		api.POST("/events", handler.CreateEvent)
	}

	// Mock webhook server
	webhookCalled := false
	mockWebhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockWebhookServer.Close()

	// Create webhook and then disable it
	webhookPayload := map[string]interface{}{
		"name":        "Disabled Webhook",
		"url":         mockWebhookServer.URL,
		"secret":      "test-secret",
		"event_types": []string{"test.event"},
	}

	webhookBytes, _ := json.Marshal(webhookPayload)
	req, _ := http.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(webhookBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Get the created webhook ID
	var webhookResponse models.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &webhookResponse)
	require.NoError(t, err)
	
	webhookData, _ := json.Marshal(webhookResponse.Data)
	var createdWebhook models.WebhookEndpoint
	err = json.Unmarshal(webhookData, &createdWebhook)
	require.NoError(t, err)

	// Disable the webhook
	updatePayload := map[string]interface{}{
		"enabled": false,
	}

	updateBytes, _ := json.Marshal(updatePayload)
	req, _ = http.NewRequest("PUT", "/api/v1/webhooks/"+createdWebhook.ID, bytes.NewBuffer(updateBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Create event
	eventPayload := map[string]interface{}{
		"type":      "test.event",
		"stream_id": "test-stream",
		"source":    "test",
		"data":      map[string]interface{}{"test": "data"},
	}

	eventBytes, _ := json.Marshal(eventPayload)
	req, _ = http.NewRequest("POST", "/api/v1/events", bytes.NewBuffer(eventBytes))
	req.Header.Set("Content-Type", "application/json")
	
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Wait for potential delivery
	time.Sleep(200 * time.Millisecond)

	// Verify webhook was NOT called (because it's disabled)
	assert.False(t, webhookCalled, "Disabled webhook should not receive events")

	// Verify no delivery records were created
	var deliveries []models.WebhookDelivery
	err = db.Find(&deliveries).Error
	require.NoError(t, err)
	assert.Len(t, deliveries, 0)
}