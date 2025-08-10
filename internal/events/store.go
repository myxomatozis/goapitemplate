package events

import (
	"context"

	"goapitemplate/internal/database"
	"goapitemplate/pkg/models"
)

type EventStore interface {
	SaveEvent(ctx context.Context, event models.Event) error
	GetEvents(ctx context.Context, eventType string, limit int) ([]models.Event, error)
	GetEventsByStream(ctx context.Context, streamID string, limit int) ([]models.Event, error)
	GetEventStreams(ctx context.Context, limit int) ([]string, error)
}

type DBEventStore struct {
	db *database.DB
}

func NewDBEventStore(db *database.DB) *DBEventStore {
	return &DBEventStore{db: db}
}

func (s *DBEventStore) SaveEvent(ctx context.Context, event models.Event) error {
	// Use the database method that handles sequence numbering
	return s.db.CreateEventWithSequence(&event)
}

func (s *DBEventStore) GetEvents(ctx context.Context, eventType string, limit int) ([]models.Event, error) {
	var events []models.Event
	
	query := s.db.WithContext(ctx).Order("timestamp DESC").Limit(limit)
	if eventType != "" {
		query = query.Where("type = ?", eventType)
	}
	
	if err := query.Find(&events).Error; err != nil {
		return nil, err
	}

	return events, nil
}

func (s *DBEventStore) GetEventsByStream(ctx context.Context, streamID string, limit int) ([]models.Event, error) {
	var events []models.Event
	
	err := s.db.WithContext(ctx).
		Where("stream_id = ?", streamID).
		Order("sequence_number ASC").
		Limit(limit).
		Find(&events).Error
	
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (s *DBEventStore) GetEventStreams(ctx context.Context, limit int) ([]string, error) {
	var streamIDs []string
	
	err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Distinct("stream_id").
		Order("stream_id").
		Limit(limit).
		Pluck("stream_id", &streamIDs).Error
	
	if err != nil {
		return nil, err
	}

	return streamIDs, nil
}