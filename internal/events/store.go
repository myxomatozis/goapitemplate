package events

import (
	"context"
	"errors"

	"goapitemplate/internal/database"
	"goapitemplate/pkg/models"

	"gorm.io/gorm"
)

type EventStore interface {
	SaveEvent(ctx context.Context, event Event) error
	GetEvents(ctx context.Context, eventType string, limit int) ([]Event, error)
	SaveWorkflowExecution(ctx context.Context, execution models.WorkflowExecution) error
	GetWorkflowExecution(ctx context.Context, executionID string) (*models.WorkflowExecution, error)
}

type DBEventStore struct {
	db *database.DB
}

func NewDBEventStore(db *database.DB) *DBEventStore {
	return &DBEventStore{db: db}
}

func (s *DBEventStore) SaveEvent(ctx context.Context, event Event) error {
	dbEvent := models.Event{
		ID:        event.ID,
		Type:      event.Type,
		Source:    event.Source,
		Data:      models.JSON(event.Data),
		Timestamp: event.Timestamp,
	}
	
	return s.db.WithContext(ctx).Create(&dbEvent).Error
}

func (s *DBEventStore) GetEvents(ctx context.Context, eventType string, limit int) ([]Event, error) {
	var dbEvents []models.Event
	
	query := s.db.WithContext(ctx).Order("timestamp DESC").Limit(limit)
	if eventType != "" {
		query = query.Where("type = ?", eventType)
	}
	
	if err := query.Find(&dbEvents).Error; err != nil {
		return nil, err
	}

	events := make([]Event, len(dbEvents))
	for i, dbEvent := range dbEvents {
		events[i] = Event{
			ID:        dbEvent.ID,
			Type:      dbEvent.Type,
			Source:    dbEvent.Source,
			Data:      map[string]interface{}(dbEvent.Data),
			Timestamp: dbEvent.Timestamp,
		}
	}

	return events, nil
}

func (s *DBEventStore) SaveWorkflowExecution(ctx context.Context, execution models.WorkflowExecution) error {
	return s.db.WithContext(ctx).Save(&execution).Error
}

func (s *DBEventStore) GetWorkflowExecution(ctx context.Context, executionID string) (*models.WorkflowExecution, error) {
	var execution models.WorkflowExecution
	
	err := s.db.WithContext(ctx).Where("id = ?", executionID).First(&execution).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &execution, nil
}

