package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)


// Event represents an event in the system with stream grouping
type Event struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Type          string    `gorm:"not null;index" json:"type"`
	StreamID      string    `gorm:"not null;index" json:"stream_id"` // For grouping related events
	Source        string    `gorm:"not null" json:"source"`
	Data          JSON      `gorm:"type:json" json:"data"`
	Timestamp     time.Time `gorm:"not null;index" json:"timestamp"`
	CreatedAt     time.Time `json:"created_at"`
	
	// Event ordering within stream
	SequenceNumber int64 `gorm:"not null;index:idx_stream_sequence" json:"sequence_number"`
}


// WebhookEndpoint represents an external webhook for event consumption
type WebhookEndpoint struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	Name           string    `gorm:"not null" json:"name"`
	URL            string    `gorm:"not null" json:"url"`
	Secret         string    `gorm:"not null" json:"secret"` // For signature verification
	EventTypes     []string  `gorm:"type:json;serializer:json" json:"event_types"` // Which events to send
	Enabled        bool      `gorm:"not null;default:true" json:"enabled"`
	MaxRetries     int       `gorm:"not null;default:3" json:"max_retries"`
	TimeoutSeconds int       `gorm:"not null;default:30" json:"timeout_seconds"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID           string     `gorm:"primaryKey" json:"id"`
	WebhookID    string     `gorm:"not null;index" json:"webhook_id"`
	EventID      string     `gorm:"not null;index" json:"event_id"`
	Status       string     `gorm:"not null" json:"status"` // pending, success, failed
	AttemptCount int        `gorm:"not null;default:0" json:"attempt_count"`
	LastAttempt  *time.Time `json:"last_attempt,omitempty"`
	NextRetry    *time.Time `json:"next_retry,omitempty"`
	Response     string     `json:"response,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	
	// Relationships
	Webhook *WebhookEndpoint `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"webhook,omitempty"`
	Event   *Event           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"event,omitempty"`
}

// JSON is a custom type for handling JSON data in GORM
type JSON map[string]interface{}

// Scan implements the Scanner interface for JSON
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSON", value)
	}

	if len(bytes) == 0 {
		*j = make(JSON)
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements the driver.Valuer interface for JSON
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Request/Response DTOs

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// Event System DTOs
type CreateEventRequest struct {
	Type     string                 `json:"type" binding:"required"`
	StreamID string                 `json:"stream_id" binding:"required"`
	Source   string                 `json:"source" binding:"required"`
	Data     map[string]interface{} `json:"data"`
}

type CreateWebhookRequest struct {
	Name           string   `json:"name" binding:"required"`
	URL            string   `json:"url" binding:"required,url"`
	Secret         string   `json:"secret" binding:"required"`
	EventTypes     []string `json:"event_types" binding:"required"`
	MaxRetries     int      `json:"max_retries"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type UpdateWebhookRequest struct {
	Name           string   `json:"name,omitempty"`
	URL            string   `json:"url,omitempty" binding:"omitempty,url"`
	Secret         string   `json:"secret,omitempty"`
	EventTypes     []string `json:"event_types,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	MaxRetries     int      `json:"max_retries,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}


type EventStreamResponse struct {
	StreamID string  `json:"stream_id"`
	Events   []Event `json:"events"`
	Count    int64   `json:"count"`
}

// TableName methods for GORM
func (Event) TableName() string {
	return "events"
}


func (WebhookEndpoint) TableName() string {
	return "webhook_endpoints"
}

func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}
