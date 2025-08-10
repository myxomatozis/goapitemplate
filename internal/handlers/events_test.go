package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goapitemplate/internal/database"
	"goapitemplate/internal/events"
	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestHandler(t *testing.T) (*Handler, *database.DB) {
	// Create in-memory database
	gormDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	db := &database.DB{DB: gormDB}
	err = db.AutoMigrate()
	require.NoError(t, err)

	// Create event manager
	eventStore := events.NewDBEventStore(db)
	eventManager := events.NewManager(eventStore, db)

	// Create handler
	handler := &Handler{
		db:           db,
		cache:        &MockCacheClient{}, // Mock cache client
		eventManager: eventManager,
		logger:       logrus.New(),
	}

	return handler, db
}

func TestCreateEvent(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Setup Gin in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/events", handler.CreateEvent)

	tests := []struct {
		name         string
		payload      map[string]interface{}
		expectedCode int
		expectError  bool
	}{
		{
			name: "valid event creation",
			payload: map[string]interface{}{
				"type":      "user.created",
				"stream_id": "user-123",
				"source":    "user-service",
				"data": map[string]interface{}{
					"user_id": 123,
					"email":   "test@example.com",
				},
			},
			expectedCode: http.StatusCreated,
			expectError:  false,
		},
		{
			name: "missing required fields",
			payload: map[string]interface{}{
				"type": "user.created",
				// missing stream_id and source
			},
			expectedCode: http.StatusBadRequest,
			expectError:  true,
		},
		{
			name:         "empty payload",
			payload:      map[string]interface{}{},
			expectedCode: http.StatusBadRequest,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			
			req, _ := http.NewRequest("POST", "/events", bytes.NewBuffer(payloadBytes))
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

				// Verify event was saved to database
				var savedEvents []models.Event
				err := db.Find(&savedEvents).Error
				require.NoError(t, err)
				assert.Len(t, savedEvents, 1)

				event := savedEvents[0]
				assert.Equal(t, tt.payload["type"], event.Type)
				assert.Equal(t, tt.payload["stream_id"], event.StreamID)
				assert.Equal(t, tt.payload["source"], event.Source)
			}

			// Clean up for next test
			db.Exec("DELETE FROM events")
		})
	}
}

func TestGetEvents(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test events
	events := []models.Event{
		{
			ID:             "event-1",
			Type:           "user.created",
			StreamID:       "user-1",
			Source:         "test",
			Data:           models.JSON{"test": "data1"},
			SequenceNumber: 1,
		},
		{
			ID:             "event-2",
			Type:           "user.updated",
			StreamID:       "user-1",
			Source:         "test",
			Data:           models.JSON{"test": "data2"},
			SequenceNumber: 2,
		},
	}

	for _, event := range events {
		err := db.CreateEventWithSequence(&event)
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events", handler.GetEvents)

	tests := []struct {
		name         string
		queryParams  string
		expectedCode int
		expectedLen  int
	}{
		{
			name:         "get all events",
			queryParams:  "",
			expectedCode: http.StatusOK,
			expectedLen:  2,
		},
		{
			name:         "get events with limit",
			queryParams:  "?limit=1",
			expectedCode: http.StatusOK,
			expectedLen:  1,
		},
		{
			name:         "get events with invalid limit",
			queryParams:  "?limit=invalid",
			expectedCode: http.StatusOK,
			expectedLen:  2, // Should use default limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/events"+tt.queryParams, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			
			// Verify response data
			dataBytes, _ := json.Marshal(response.Data)
			var responseEvents []models.Event
			err = json.Unmarshal(dataBytes, &responseEvents)
			require.NoError(t, err)
			
			assert.Len(t, responseEvents, tt.expectedLen)
		})
	}
}

func TestGetEventsByType(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test events with different types
	events := []models.Event{
		{
			ID:             "event-1",
			Type:           "user.created",
			StreamID:       "user-1",
			Source:         "test",
			Data:           models.JSON{"test": "data1"},
			SequenceNumber: 1,
		},
		{
			ID:             "event-2",
			Type:           "user.updated",
			StreamID:       "user-1",
			Source:         "test",
			Data:           models.JSON{"test": "data2"},
			SequenceNumber: 2,
		},
		{
			ID:             "event-3",
			Type:           "user.created",
			StreamID:       "user-2",
			Source:         "test",
			Data:           models.JSON{"test": "data3"},
			SequenceNumber: 1,
		},
	}

	for _, event := range events {
		err := db.CreateEventWithSequence(&event)
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events/types/:type", handler.GetEventsByType)

	tests := []struct {
		name         string
		eventType    string
		expectedCode int
		expectedLen  int
	}{
		{
			name:         "get user.created events",
			eventType:    "user.created",
			expectedCode: http.StatusOK,
			expectedLen:  2,
		},
		{
			name:         "get user.updated events",
			eventType:    "user.updated",
			expectedCode: http.StatusOK,
			expectedLen:  1,
		},
		{
			name:         "get non-existent event type",
			eventType:    "payment.processed",
			expectedCode: http.StatusOK,
			expectedLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/events/types/"+tt.eventType, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			
			dataBytes, _ := json.Marshal(response.Data)
			var responseEvents []models.Event
			err = json.Unmarshal(dataBytes, &responseEvents)
			require.NoError(t, err)
			
			assert.Len(t, responseEvents, tt.expectedLen)

			// Verify all returned events have the correct type
			for _, event := range responseEvents {
				assert.Equal(t, tt.eventType, event.Type)
			}
		})
	}
}

func TestGetEventsByStream(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test events in different streams
	events := []models.Event{
		{
			ID:             "event-1",
			Type:           "user.created",
			StreamID:       "stream-1",
			Source:         "test",
			Data:           models.JSON{"test": "data1"},
			SequenceNumber: 1,
		},
		{
			ID:             "event-2",
			Type:           "user.updated",
			StreamID:       "stream-1",
			Source:         "test",
			Data:           models.JSON{"test": "data2"},
			SequenceNumber: 2,
		},
		{
			ID:             "event-3",
			Type:           "user.created",
			StreamID:       "stream-2",
			Source:         "test",
			Data:           models.JSON{"test": "data3"},
			SequenceNumber: 1,
		},
	}

	for _, event := range events {
		err := db.CreateEventWithSequence(&event)
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events/streams/:stream_id", handler.GetEventsByStream)

	tests := []struct {
		name         string
		streamID     string
		expectedCode int
		expectedLen  int
	}{
		{
			name:         "get events from stream-1",
			streamID:     "stream-1",
			expectedCode: http.StatusOK,
			expectedLen:  2,
		},
		{
			name:         "get events from stream-2",
			streamID:     "stream-2",
			expectedCode: http.StatusOK,
			expectedLen:  1,
		},
		{
			name:         "get events from non-existent stream",
			streamID:     "stream-999",
			expectedCode: http.StatusOK,
			expectedLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/events/streams/"+tt.streamID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			var response models.APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			
			dataBytes, _ := json.Marshal(response.Data)
			var streamResponse models.EventStreamResponse
			err = json.Unmarshal(dataBytes, &streamResponse)
			require.NoError(t, err)
			
			assert.Equal(t, tt.streamID, streamResponse.StreamID)
			assert.Len(t, streamResponse.Events, tt.expectedLen)
			assert.Equal(t, int64(tt.expectedLen), streamResponse.Count)

			// Verify all returned events have the correct stream ID
			for _, event := range streamResponse.Events {
				assert.Equal(t, tt.streamID, event.StreamID)
			}
		})
	}
}

func TestGetEventStreams(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()

	// Create test events in different streams
	events := []models.Event{
		{
			ID:             "event-1",
			Type:           "user.created",
			StreamID:       "stream-alpha",
			Source:         "test",
			Data:           models.JSON{"test": "data1"},
			SequenceNumber: 1,
		},
		{
			ID:             "event-2",
			Type:           "user.updated",
			StreamID:       "stream-beta",
			Source:         "test",
			Data:           models.JSON{"test": "data2"},
			SequenceNumber: 1,
		},
		{
			ID:             "event-3",
			Type:           "user.created",
			StreamID:       "stream-alpha", // Same stream as first
			Source:         "test",
			Data:           models.JSON{"test": "data3"},
			SequenceNumber: 2,
		},
	}

	for _, event := range events {
		err := db.CreateEventWithSequence(&event)
		require.NoError(t, err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/events/streams", handler.GetEventStreams)

	req, _ := http.NewRequest("GET", "/events/streams", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	
	dataBytes, _ := json.Marshal(response.Data)
	var streamIDs []string
	err = json.Unmarshal(dataBytes, &streamIDs)
	require.NoError(t, err)
	
	// Should return unique stream IDs
	assert.Len(t, streamIDs, 2)
	assert.Contains(t, streamIDs, "stream-alpha")
	assert.Contains(t, streamIDs, "stream-beta")
}

// MockCacheClient implements cache.Client interface for testing
type MockCacheClient struct{}

func (m *MockCacheClient) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *MockCacheClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (m *MockCacheClient) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *MockCacheClient) Close() error {
	return nil
}