package events

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"goapitemplate/internal/database"
	"goapitemplate/pkg/models"

	"github.com/sirupsen/logrus"
)

type Handler func(ctx context.Context, event models.Event) error

type Manager struct {
	handlers        map[string][]Handler
	store           EventStore
	webhookDelivery *WebhookDeliveryService
	mu              sync.RWMutex
	logger          *logrus.Logger
}

func NewManager(store EventStore, db *database.DB) *Manager {
	return &Manager{
		handlers:        make(map[string][]Handler),
		store:           store,
		webhookDelivery: NewWebhookDeliveryService(db),
		logger:          logrus.New(),
	}
}

func (m *Manager) Subscribe(eventType string, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
	m.logger.WithField("event_type", eventType).Info("Handler subscribed")
}

func (m *Manager) Publish(ctx context.Context, streamID, eventType, source string, data map[string]interface{}) error {
	event := models.Event{
		ID:        generateEventID(),
		Type:      eventType,
		StreamID:  streamID,
		Source:    source,
		Data:      models.JSON(data),
		Timestamp: time.Now(),
	}

	// Store event in database with proper sequence number
	if err := m.store.SaveEvent(ctx, event); err != nil {
		m.logger.WithError(err).Error("Failed to save event")
		return err
	}

	// Process handlers asynchronously
	go m.processHandlers(ctx, event)

	// Deliver to webhooks asynchronously
	go m.deliverWebhooks(ctx, event)

	return nil
}

func (m *Manager) PublishAsync(ctx context.Context, streamID, eventType, source string, data map[string]interface{}) {
	go func() {
		if err := m.Publish(ctx, streamID, eventType, source, data); err != nil {
			m.logger.WithError(err).Error("Failed to publish event")
		}
	}()
}

func (m *Manager) processHandlers(ctx context.Context, event models.Event) {
	m.mu.RLock()
	handlers := m.handlers[event.Type]
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h Handler) {
			if err := h(ctx, event); err != nil {
				m.logger.WithFields(logrus.Fields{
					"event_type": event.Type,
					"event_id":   event.ID,
					"error":      err,
				}).Error("Handler failed")
			}
		}(handler)
	}
}

func (m *Manager) deliverWebhooks(ctx context.Context, event models.Event) {
	if err := m.webhookDelivery.DeliverEvent(ctx, event); err != nil {
		m.logger.WithFields(logrus.Fields{
			"event_type": event.Type,
			"event_id":   event.ID,
			"error":      err,
		}).Error("Failed to deliver webhooks")
	}
}

func (m *Manager) GetStore() EventStore {
	return m.store
}

func (m *Manager) GetWebhookDeliveryService() *WebhookDeliveryService {
	return m.webhookDelivery
}

func generateEventID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}