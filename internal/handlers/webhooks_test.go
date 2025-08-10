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

func TestCreateWebhook(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhooks", handler.CreateWebhook)

	tests := []struct {
		name         string
		payload      map[string]interface{}
		expectedCode int
		expectError  bool
	}{
		{
			name: "valid webhook creation",
			payload: map[string]interface{}{
				"name":            "Test Webhook",
				"url":             "https://example.com/webhook",
				"secret":          "secret123",
				"event_types":     []string{"user.created", "user.updated"},
				"max_retries":     5,
				"timeout_seconds": 45,
			},
			expectedCode: http.StatusCreated,
			expectError:  false,
		},
		{
			name: "webhook with defaults",
			payload: map[string]interface{}{
				"name":        "Simple Webhook",
				"url":         "https://example.com/webhook",
				"secret":      "secret123",
				"event_types": []string{"payment.processed"},
			},
			expectedCode: http.StatusCreated,
			expectError:  false,
		},
		{
			name: "missing required fields",
			payload: map[string]interface{}{
				"name": "Incomplete Webhook",
				// missing url, secret, event_types
			},
			expectedCode: http.StatusBadRequest,
			expectError:  true,
		},
		{
			name: "invalid URL format",
			payload: map[string]interface{}{
				"name":        "Bad URL Webhook",
				"url":         "not-a-valid-url",
				"secret":      "secret123",
				"event_types": []string{"test.event"},
			},
			expectedCode: http.StatusBadRequest,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			
			req, _ := http.NewRequest("POST", "/webhooks", bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.False(t, response.Success)
				assert.NotEmpty(t, response.Error)
			} else {
				assert.True(t, response.Success)
				assert.Empty(t, response.Error)

				// Verify webhook was saved to database
				var savedWebhooks []models.WebhookEndpoint
				err := db.Find(&savedWebhooks).Error
				require.NoError(t, err)
				assert.Len(t, savedWebhooks, 1)

				webhook := savedWebhooks[0]
				assert.Equal(t, tt.payload["name"], webhook.Name)
				assert.Equal(t, tt.payload["url"], webhook.URL)
				assert.True(t, webhook.Enabled)

				// Check defaults were applied
				if tt.payload["max_retries"] == nil {
					assert.Equal(t, 3, webhook.MaxRetries)
				}
				if tt.payload["timeout_seconds"] == nil {
					assert.Equal(t, 30, webhook.TimeoutSeconds)
				}
			}

			// Clean up for next test
			db.Exec("DELETE FROM webhook_endpoints")
		})
	}
}

func TestGetWebhooks(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test webhooks
	webhooks := []models.WebhookEndpoint{
		{
			ID:             "webhook-1",
			Name:           "Webhook 1",
			URL:            "https://example.com/webhook1",
			Secret:         "secret1",
			EventTypes:     []string{"user.created"},
			Enabled:        true,
			MaxRetries:     3,
			TimeoutSeconds: 30,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             "webhook-2",
			Name:           "Webhook 2",
			URL:            "https://example.com/webhook2",
			Secret:         "secret2",
			EventTypes:     []string{"payment.processed"},
			Enabled:        false,
			MaxRetries:     5,
			TimeoutSeconds: 60,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, webhook := range webhooks {
		err := db.Create(&webhook).Error
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/webhooks", handler.GetWebhooks)

	req, _ := http.NewRequest("GET", "/webhooks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	
	dataBytes, _ := json.Marshal(response.Data)
	var responseWebhooks []models.WebhookEndpoint
	err = json.Unmarshal(dataBytes, &responseWebhooks)
	require.NoError(t, err)
	
	assert.Len(t, responseWebhooks, 2)
	
	// Verify webhook data
	webhookNames := make(map[string]bool)
	for _, webhook := range responseWebhooks {
		webhookNames[webhook.Name] = true
	}
	assert.True(t, webhookNames["Webhook 1"])
	assert.True(t, webhookNames["Webhook 2"])
}

func TestGetWebhook(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	webhook := models.WebhookEndpoint{
		ID:             "test-webhook-123",
		Name:           "Test Webhook",
		URL:            "https://example.com/webhook",
		Secret:         "secret123",
		EventTypes:     []string{"user.created"},
		Enabled:        true,
		MaxRetries:     3,
		TimeoutSeconds: 30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := db.Create(&webhook).Error
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/webhooks/:id", handler.GetWebhook)

	tests := []struct {
		name         string
		webhookID    string
		expectedCode int
		expectError  bool
	}{
		{
			name:         "get existing webhook",
			webhookID:    "test-webhook-123",
			expectedCode: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "get non-existent webhook",
			webhookID:    "non-existent-id",
			expectedCode: http.StatusNotFound,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/webhooks/"+tt.webhookID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.False(t, response.Success)
				assert.NotEmpty(t, response.Error)
			} else {
				assert.True(t, response.Success)
				assert.Empty(t, response.Error)

				dataBytes, _ := json.Marshal(response.Data)
				var responseWebhook models.WebhookEndpoint
				err = json.Unmarshal(dataBytes, &responseWebhook)
				require.NoError(t, err)

				assert.Equal(t, webhook.ID, responseWebhook.ID)
				assert.Equal(t, webhook.Name, responseWebhook.Name)
				assert.Equal(t, webhook.URL, responseWebhook.URL)
			}
		})
	}
}

func TestUpdateWebhook(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	webhook := models.WebhookEndpoint{
		ID:             "test-webhook-123",
		Name:           "Original Webhook",
		URL:            "https://example.com/original",
		Secret:         "original-secret",
		EventTypes:     []string{"user.created"},
		Enabled:        true,
		MaxRetries:     3,
		TimeoutSeconds: 30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := db.Create(&webhook).Error
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/webhooks/:id", handler.UpdateWebhook)

	tests := []struct {
		name         string
		webhookID    string
		payload      map[string]interface{}
		expectedCode int
		expectError  bool
	}{
		{
			name:      "update webhook name",
			webhookID: "test-webhook-123",
			payload: map[string]interface{}{
				"name": "Updated Webhook",
			},
			expectedCode: http.StatusOK,
			expectError:  false,
		},
		{
			name:      "update multiple fields",
			webhookID: "test-webhook-123",
			payload: map[string]interface{}{
				"name":            "Fully Updated Webhook",
				"url":             "https://example.com/updated",
				"enabled":         false,
				"max_retries":     5,
				"timeout_seconds": 60,
			},
			expectedCode: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "update non-existent webhook",
			webhookID:    "non-existent-id",
			payload:      map[string]interface{}{"name": "New Name"},
			expectedCode: http.StatusNotFound,
			expectError:  true,
		},
		{
			name:         "empty update payload",
			webhookID:    "test-webhook-123",
			payload:      map[string]interface{}{},
			expectedCode: http.StatusBadRequest,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			
			req, _ := http.NewRequest("PUT", "/webhooks/"+tt.webhookID, bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.False(t, response.Success)
				assert.NotEmpty(t, response.Error)
			} else {
				assert.True(t, response.Success)
				assert.Empty(t, response.Error)

				// Verify changes were applied
				if tt.webhookID == "test-webhook-123" {
					var updatedWebhook models.WebhookEndpoint
					err := db.First(&updatedWebhook, "id = ?", tt.webhookID).Error
					require.NoError(t, err)

					if name, ok := tt.payload["name"]; ok {
						assert.Equal(t, name, updatedWebhook.Name)
					}
					if url, ok := tt.payload["url"]; ok {
						assert.Equal(t, url, updatedWebhook.URL)
					}
					if enabled, ok := tt.payload["enabled"]; ok {
						assert.Equal(t, enabled, updatedWebhook.Enabled)
					}
				}
			}
		})
	}
}

func TestDeleteWebhook(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	webhook := models.WebhookEndpoint{
		ID:             "test-webhook-123",
		Name:           "Test Webhook",
		URL:            "https://example.com/webhook",
		Secret:         "secret123",
		EventTypes:     []string{"user.created"},
		Enabled:        true,
		MaxRetries:     3,
		TimeoutSeconds: 30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err := db.Create(&webhook).Error
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/webhooks/:id", handler.DeleteWebhook)

	tests := []struct {
		name         string
		webhookID    string
		expectedCode int
		expectError  bool
	}{
		{
			name:         "delete existing webhook",
			webhookID:    "test-webhook-123",
			expectedCode: http.StatusOK,
			expectError:  false,
		},
		{
			name:         "delete non-existent webhook",
			webhookID:    "non-existent-id",
			expectedCode: http.StatusNotFound,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("DELETE", "/webhooks/"+tt.webhookID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError {
				assert.False(t, response.Success)
				assert.NotEmpty(t, response.Error)
			} else {
				assert.True(t, response.Success)
				assert.Empty(t, response.Error)

				// Verify webhook was deleted
				var count int64
				err := db.Model(&models.WebhookEndpoint{}).Where("id = ?", tt.webhookID).Count(&count).Error
				require.NoError(t, err)
				assert.Equal(t, int64(0), count)
			}
		})
	}
}

func TestGetWebhookStats(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test webhook
	webhook := models.WebhookEndpoint{
		ID:             "test-webhook",
		Name:           "Test Webhook",
		URL:            "https://example.com/webhook",
		Secret:         "secret",
		EventTypes:     []string{"test.event"},
		Enabled:        true,
		MaxRetries:     3,
		TimeoutSeconds: 30,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	err := db.Create(&webhook).Error
	require.NoError(t, err)

	// Create test event
	event := models.Event{
		ID:             "test-event",
		Type:           "test.event",
		StreamID:       "test-stream",
		Source:         "test",
		Data:           models.JSON{"test": "data"},
		SequenceNumber: 1,
		CreatedAt:      time.Now(),
	}
	err = db.CreateEventWithSequence(&event)
	require.NoError(t, err)

	// Create test deliveries with different statuses
	deliveries := []models.WebhookDelivery{
		{
			ID:           "delivery-1",
			WebhookID:    webhook.ID,
			EventID:      event.ID,
			Status:       "success",
			AttemptCount: 1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "delivery-2",
			WebhookID:    webhook.ID,
			EventID:      event.ID,
			Status:       "failed",
			AttemptCount: 3,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "delivery-3",
			WebhookID:    webhook.ID,
			EventID:      event.ID,
			Status:       "pending",
			AttemptCount: 1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, delivery := range deliveries {
		err := db.Create(&delivery).Error
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/webhooks/stats", handler.GetWebhookStats)

	req, _ := http.NewRequest("GET", "/webhooks/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.APIResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)

	dataBytes, _ := json.Marshal(response.Data)
	var stats struct {
		TotalDeliveries      int64   `json:"total_deliveries"`
		SuccessfulDeliveries int64   `json:"successful_deliveries"`
		FailedDeliveries     int64   `json:"failed_deliveries"`
		PendingDeliveries    int64   `json:"pending_deliveries"`
		SuccessRate          float64 `json:"success_rate"`
	}
	err = json.Unmarshal(dataBytes, &stats)
	require.NoError(t, err)

	assert.Equal(t, int64(3), stats.TotalDeliveries)
	assert.Equal(t, int64(1), stats.SuccessfulDeliveries)
	assert.Equal(t, int64(1), stats.FailedDeliveries)
	assert.Equal(t, int64(1), stats.PendingDeliveries)
	assert.InDelta(t, 33.33, stats.SuccessRate, 0.1) // 1/3 = 33.33%
}

func TestRetryWebhookDeliveries(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhooks/retry", handler.RetryWebhookDeliveries)

	req, _ := http.NewRequest("POST", "/webhooks/retry", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.Contains(t, response.Message, "retry initiated")
}