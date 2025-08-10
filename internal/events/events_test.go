package events

import (
	"context"
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

func setupEventTestDB(t *testing.T) *database.DB {
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

func TestManager_Subscribe_and_Publish(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	// Test data
	handlerCalled := false
	var receivedEvent models.Event

	// Subscribe to events
	handler := func(ctx context.Context, event models.Event) error {
		handlerCalled = true
		receivedEvent = event
		return nil
	}

	manager.Subscribe("user.created", handler)

	// Publish an event
	testData := map[string]interface{}{
		"user_id": 123,
		"email":   "test@example.com",
	}

	err := manager.Publish(context.Background(), "user-stream-123", "user.created", "user-service", testData)
	assert.NoError(t, err)

	// Allow time for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify handler was called
	assert.True(t, handlerCalled)
	assert.Equal(t, "user.created", receivedEvent.Type)
	assert.Equal(t, "user-stream-123", receivedEvent.StreamID)
	assert.Equal(t, "user-service", receivedEvent.Source)
	assert.Equal(t, testData, map[string]interface{}(receivedEvent.Data))

	// Verify event was saved to database
	var savedEvents []models.Event
	err = db.Find(&savedEvents).Error
	require.NoError(t, err)
	assert.Len(t, savedEvents, 1)

	event := savedEvents[0]
	assert.Equal(t, "user.created", event.Type)
	assert.Equal(t, "user-stream-123", event.StreamID)
	assert.Equal(t, int64(1), event.SequenceNumber) // Should be first in stream
}

func TestManager_MultipleHandlers(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	// Test tracking
	handler1Called := false
	handler2Called := false

	// Subscribe multiple handlers to same event type
	handler1 := func(ctx context.Context, event models.Event) error {
		handler1Called = true
		return nil
	}

	handler2 := func(ctx context.Context, event models.Event) error {
		handler2Called = true
		return nil
	}

	manager.Subscribe("order.created", handler1)
	manager.Subscribe("order.created", handler2)

	// Publish event
	err := manager.Publish(context.Background(), "order-stream", "order.created", "order-service", map[string]interface{}{"order_id": 456})
	assert.NoError(t, err)

	// Allow time for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify both handlers were called
	assert.True(t, handler1Called)
	assert.True(t, handler2Called)
}

func TestManager_NoMatchingHandlers(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	handlerCalled := false

	// Subscribe to different event type
	handler := func(ctx context.Context, event models.Event) error {
		handlerCalled = true
		return nil
	}

	manager.Subscribe("user.deleted", handler)

	// Publish different event type
	err := manager.Publish(context.Background(), "user-stream", "user.created", "user-service", map[string]interface{}{"user_id": 789})
	assert.NoError(t, err)

	// Allow time for potential processing
	time.Sleep(100 * time.Millisecond)

	// Verify handler was not called
	assert.False(t, handlerCalled)

	// But event should still be saved
	var savedEvents []models.Event
	err = db.Find(&savedEvents).Error
	require.NoError(t, err)
	assert.Len(t, savedEvents, 1)
}

func TestManager_PublishAsync(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	// Publish event asynchronously
	manager.PublishAsync(context.Background(), "async-stream", "async.event", "async-service", map[string]interface{}{"async": true})

	// Allow time for async processing
	time.Sleep(200 * time.Millisecond)

	// Verify event was saved
	var savedEvents []models.Event
	err := db.Find(&savedEvents).Error
	require.NoError(t, err)
	assert.Len(t, savedEvents, 1)

	event := savedEvents[0]
	assert.Equal(t, "async.event", event.Type)
	assert.Equal(t, "async-stream", event.StreamID)
}

func TestManager_EventSequencing(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	streamID := "sequencing-test-stream"

	// Publish multiple events to same stream
	events := []string{"event1", "event2", "event3"}
	
	for _, eventType := range events {
		err := manager.Publish(context.Background(), streamID, eventType, "test-service", map[string]interface{}{"type": eventType})
		assert.NoError(t, err)
		// Small delay to ensure ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Allow time for all events to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify events have correct sequence numbers
	var savedEvents []models.Event
	err := db.Order("sequence_number ASC").Find(&savedEvents, "stream_id = ?", streamID).Error
	require.NoError(t, err)
	assert.Len(t, savedEvents, 3)

	for i, event := range savedEvents {
		assert.Equal(t, int64(i+1), event.SequenceNumber)
		assert.Equal(t, events[i], event.Type)
		assert.Equal(t, streamID, event.StreamID)
	}
}

func TestDBEventStore_SaveEvent(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)

	event := models.Event{
		ID:       "test-event-id",
		Type:     "test.event",
		StreamID: "test-stream",
		Source:   "test-source",
		Data:     models.JSON{"test": "data"},
	}

	err := store.SaveEvent(context.Background(), event)
	assert.NoError(t, err)

	// Verify event was saved with sequence number
	var savedEvent models.Event
	err = db.First(&savedEvent, "id = ?", event.ID).Error
	require.NoError(t, err)

	assert.Equal(t, event.Type, savedEvent.Type)
	assert.Equal(t, event.StreamID, savedEvent.StreamID)
	assert.Equal(t, event.Source, savedEvent.Source)
	assert.Equal(t, int64(1), savedEvent.SequenceNumber) // First event in stream
}

func TestDBEventStore_GetEvents(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)

	// Create test events
	events := []models.Event{
		{
			ID:       "event-1",
			Type:     "user.created",
			StreamID: "stream-1",
			Source:   "test",
			Data:     models.JSON{"user": "alice"},
		},
		{
			ID:       "event-2",
			Type:     "user.updated",
			StreamID: "stream-1",
			Source:   "test",
			Data:     models.JSON{"user": "alice"},
		},
		{
			ID:       "event-3",
			Type:     "user.created",
			StreamID: "stream-2",
			Source:   "test",
			Data:     models.JSON{"user": "bob"},
		},
	}

	for _, event := range events {
		err := store.SaveEvent(context.Background(), event)
		require.NoError(t, err)
	}

	tests := []struct {
		name         string
		eventType    string
		limit        int
		expectedLen  int
	}{
		{
			name:        "get all events",
			eventType:   "",
			limit:       10,
			expectedLen: 3,
		},
		{
			name:        "get events by type",
			eventType:   "user.created",
			limit:       10,
			expectedLen: 2,
		},
		{
			name:        "limit results",
			eventType:   "",
			limit:       2,
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.GetEvents(context.Background(), tt.eventType, tt.limit)
			assert.NoError(t, err)
			assert.Len(t, results, tt.expectedLen)

			// If filtering by type, verify all results match
			if tt.eventType != "" {
				for _, event := range results {
					assert.Equal(t, tt.eventType, event.Type)
				}
			}
		})
	}
}

func TestDBEventStore_GetEventsByStream(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)

	streamID := "test-stream"

	// Create events in specific order
	events := []models.Event{
		{
			ID:       "event-1",
			Type:     "first",
			StreamID: streamID,
			Source:   "test",
			Data:     models.JSON{"order": 1},
		},
		{
			ID:       "event-2",
			Type:     "second",
			StreamID: streamID,
			Source:   "test",
			Data:     models.JSON{"order": 2},
		},
		{
			ID:       "event-3",
			Type:     "third",
			StreamID: "different-stream", // Different stream
			Source:   "test",
			Data:     models.JSON{"order": 3},
		},
	}

	for _, event := range events {
		err := store.SaveEvent(context.Background(), event)
		require.NoError(t, err)
	}

	// Get events from specific stream
	results, err := store.GetEventsByStream(context.Background(), streamID, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 2) // Only 2 events in the target stream

	// Verify ordering by sequence number
	assert.Equal(t, int64(1), results[0].SequenceNumber)
	assert.Equal(t, "first", results[0].Type)
	assert.Equal(t, int64(2), results[1].SequenceNumber)
	assert.Equal(t, "second", results[1].Type)

	// All events should belong to the requested stream
	for _, event := range results {
		assert.Equal(t, streamID, event.StreamID)
	}
}

func TestDBEventStore_GetEventStreams(t *testing.T) {
	db := setupEventTestDB(t)
	defer db.Close()

	store := NewDBEventStore(db)

	// Create events in different streams
	events := []models.Event{
		{
			ID:       "event-1",
			Type:     "test",
			StreamID: "stream-alpha",
			Source:   "test",
			Data:     models.JSON{"test": "data1"},
		},
		{
			ID:       "event-2",
			Type:     "test",
			StreamID: "stream-beta",
			Source:   "test",
			Data:     models.JSON{"test": "data2"},
		},
		{
			ID:       "event-3",
			Type:     "test",
			StreamID: "stream-alpha", // Duplicate stream
			Source:   "test",
			Data:     models.JSON{"test": "data3"},
		},
		{
			ID:       "event-4",
			Type:     "test",
			StreamID: "stream-gamma",
			Source:   "test",
			Data:     models.JSON{"test": "data4"},
		},
	}

	for _, event := range events {
		err := store.SaveEvent(context.Background(), event)
		require.NoError(t, err)
	}

	// Get unique stream IDs
	streamIDs, err := store.GetEventStreams(context.Background(), 10)
	assert.NoError(t, err)
	assert.Len(t, streamIDs, 3) // Should return 3 unique streams

	// Verify streams are returned (order may vary)
	streamSet := make(map[string]bool)
	for _, id := range streamIDs {
		streamSet[id] = true
	}

	assert.True(t, streamSet["stream-alpha"])
	assert.True(t, streamSet["stream-beta"])
	assert.True(t, streamSet["stream-gamma"])
}

func BenchmarkEventPublish(b *testing.B) {
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

	store := NewDBEventStore(db)
	manager := NewManager(store, db)

	testData := map[string]interface{}{
		"benchmark": true,
		"iteration": 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testData["iteration"] = i
		err := manager.Publish(context.Background(), "bench-stream", "bench.event", "benchmark", testData)
		if err != nil {
			b.Error(err)
		}
	}
}