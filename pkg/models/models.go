package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)


// Event represents an event in the system
type Event struct {
	ID                    string    `gorm:"primaryKey" json:"id"`
	Type                  string    `gorm:"not null;index" json:"type"`
	Source                string    `gorm:"not null" json:"source"`
	Data                  JSON      `gorm:"type:json" json:"data"`
	Timestamp             time.Time `gorm:"not null;index" json:"timestamp"`
	CreatedAt             time.Time `json:"created_at"`
	
	// Foreign key for workflow execution relationship (optional)
	WorkflowExecutionID   *string           `gorm:"index" json:"workflow_execution_id,omitempty"`
	WorkflowExecution     *WorkflowExecution `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"workflow_execution,omitempty"`
}

// StepExecution represents a single step execution in a workflow
type StepExecution struct {
	StepName     string     `json:"step_name"`
	Status       string     `json:"status"` // pending, running, completed, failed, timeout, skipped
	StartTime    time.Time  `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Attempts     int        `json:"attempts"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Output       JSON       `json:"output,omitempty"`
}

// WorkflowExecution represents a workflow execution
type WorkflowExecution struct {
	ID           string          `gorm:"primaryKey" json:"id"`
	WorkflowID   string          `gorm:"not null;index" json:"workflow_id"`
	Status       string          `gorm:"not null" json:"status"` // pending, running, completed, failed, timeout
	CurrentStep  string          `json:"current_step"`
	Variables    JSON            `gorm:"type:json" json:"variables"`
	StepHistory  []StepExecution `gorm:"type:json;serializer:json" json:"step_history"`
	StartedAt    time.Time       `gorm:"not null" json:"started_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	
	// Relationship: WorkflowExecution has many Events
	Events []Event `gorm:"foreignKey:WorkflowExecutionID" json:"events,omitempty"`
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

// Workflow DTOs
type WorkflowStepConfig struct {
	MaxRetries      int                    `json:"max_retries,omitempty"`
	TimeoutMs       int                    `json:"timeout_ms,omitempty"`
	RetryDelayMs    int                    `json:"retry_delay_ms,omitempty"`
	ContinueOnError bool                   `json:"continue_on_error,omitempty"`
	Condition       string                 `json:"condition,omitempty"`
	Parallel        bool                   `json:"parallel,omitempty"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
}

type WorkflowStep struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Config      WorkflowStepConfig `json:"config"`
	OnSuccess   string             `json:"on_success,omitempty"`
	OnError     string             `json:"on_error,omitempty"`
	OnTimeout   string             `json:"on_timeout,omitempty"`
	Description string             `json:"description,omitempty"`
}

type Workflow struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Steps       map[string]WorkflowStep `json:"steps"`
	StartStep   string                  `json:"start_step"`
	Variables   map[string]interface{}  `json:"variables"`
}

// TableName methods for GORM
func (Event) TableName() string {
	return "events"
}

func (WorkflowExecution) TableName() string {
	return "workflow_executions"
}
