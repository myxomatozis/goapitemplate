package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"goapitemplate/pkg/models"

	"github.com/sirupsen/logrus"
)

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

type Handler func(ctx context.Context, event Event) error

type Manager struct {
	handlers map[string][]Handler
	store    EventStore
	mu       sync.RWMutex
	logger   *logrus.Logger
}

func NewManager(store EventStore) *Manager {
	return &Manager{
		handlers: make(map[string][]Handler),
		store:    store,
		logger:   logrus.New(),
	}
}

func (m *Manager) Subscribe(eventType string, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
	m.logger.WithField("event_type", eventType).Info("Handler subscribed")
}

func (m *Manager) Publish(ctx context.Context, event Event) error {
	event.Timestamp = time.Now()
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Save event to persistent store
	if m.store != nil {
		if err := m.store.SaveEvent(ctx, event); err != nil {
			m.logger.WithError(err).Error("Failed to save event to store")
		}
	}

	m.mu.RLock()
	handlers, exists := m.handlers[event.Type]
	m.mu.RUnlock()

	if !exists {
		m.logger.WithField("event_type", event.Type).Debug("No handlers found for event")
		return nil
	}

	m.logger.WithFields(logrus.Fields{
		"event_id":   event.ID,
		"event_type": event.Type,
		"source":     event.Source,
	}).Info("Publishing event")

	var wg sync.WaitGroup
	errors := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				errors <- fmt.Errorf("handler error for event %s: %w", event.ID, err)
			}
		}(handler)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		m.logger.WithError(err).Error("Event handler failed")
	}

	return nil
}

func (m *Manager) PublishAsync(ctx context.Context, event Event) {
	go func() {
		if err := m.Publish(ctx, event); err != nil {
			m.logger.WithError(err).Error("Failed to publish event asynchronously")
		}
	}()
}

func (m *Manager) GetStore() EventStore {
	return m.store
}

type StepConfig struct {
	MaxRetries    int                    `json:"max_retries,omitempty"`     // Number of retry attempts (default: 0)
	TimeoutMs     int                    `json:"timeout_ms,omitempty"`      // Step timeout in milliseconds (default: 30000)
	RetryDelayMs  int                    `json:"retry_delay_ms,omitempty"`  // Delay between retries in ms (default: 1000)
	ContinueOnError bool                 `json:"continue_on_error,omitempty"` // Continue workflow even if step fails
	Condition     string                 `json:"condition,omitempty"`       // Conditional execution (e.g., "input.status == 'active'")
	Parallel      bool                   `json:"parallel,omitempty"`        // Execute this step in parallel with next steps
	Parameters    map[string]interface{} `json:"parameters,omitempty"`      // Step-specific configuration
}

type WorkflowStep struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Config      StepConfig             `json:"config"`
	OnSuccess   string                 `json:"on_success,omitempty"`
	OnError     string                 `json:"on_error,omitempty"`
	OnTimeout   string                 `json:"on_timeout,omitempty"`
	Description string                 `json:"description,omitempty"`
}


type Workflow struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Steps       map[string]WorkflowStep `json:"steps"`
	StartStep   string                  `json:"start_step"`
	Variables   map[string]interface{}  `json:"variables"`
}

type WorkflowManager struct {
	workflows map[string]Workflow
	events    *Manager
	store     EventStore
	mu        sync.RWMutex
	logger    *logrus.Logger
}

func NewWorkflowManager(eventManager *Manager, store EventStore) *WorkflowManager {
	wm := &WorkflowManager{
		workflows: make(map[string]Workflow),
		events:    eventManager,
		store:     store,
		logger:    logrus.New(),
	}

	eventManager.Subscribe("workflow.execute", wm.handleWorkflowExecution)
	return wm
}

func (wm *WorkflowManager) RegisterWorkflow(workflow Workflow) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if err := wm.validateWorkflow(workflow); err != nil {
		return fmt.Errorf("invalid workflow: %w", err)
	}

	wm.workflows[workflow.ID] = workflow
	wm.logger.WithField("workflow_id", workflow.ID).Info("Workflow registered")
	return nil
}

func (wm *WorkflowManager) ExecuteWorkflow(ctx context.Context, workflowID string, input map[string]interface{}) error {
	wm.mu.RLock()
	_, exists := wm.workflows[workflowID]
	wm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Create workflow execution record
	executionID := fmt.Sprintf("exec_%d_%s", time.Now().Unix(), randomString(8))
	execution := models.WorkflowExecution{
		ID:         executionID,
		WorkflowID: workflowID,
		Status:     "pending",
		Variables:  models.JSON(input),
		StartedAt:  time.Now(),
	}

	if wm.store != nil {
		if err := wm.store.SaveWorkflowExecution(ctx, execution); err != nil {
			wm.logger.WithError(err).Error("Failed to save workflow execution")
		}
	}

	event := Event{
		Type:   "workflow.execute",
		Source: "workflow_manager",
		Data: map[string]interface{}{
			"workflow_id":  workflowID,
			"execution_id": executionID,
			"input":        input,
		},
	}

	return wm.events.Publish(ctx, event)
}

func (wm *WorkflowManager) handleWorkflowExecution(ctx context.Context, event Event) error {
	workflowID, ok := event.Data["workflow_id"].(string)
	if !ok {
		return fmt.Errorf("invalid workflow_id in event")
	}

	executionID, _ := event.Data["execution_id"].(string)
	input, _ := event.Data["input"].(map[string]interface{})

	// Update execution status to running
	if wm.store != nil && executionID != "" {
		execution := models.WorkflowExecution{
			ID:          executionID,
			WorkflowID:  workflowID,
			Status:      "running",
			Variables:   models.JSON(input),
			StartedAt:   time.Now(),
		}
		wm.store.SaveWorkflowExecution(ctx, execution)
	}

	wm.mu.RLock()
	workflow := wm.workflows[workflowID]
	wm.mu.RUnlock()

	wm.logger.WithFields(logrus.Fields{
		"workflow_id":  workflowID,
		"execution_id": executionID,
		"event_id":     event.ID,
	}).Info("Executing workflow")

	err := wm.executeStep(ctx, workflow, workflow.StartStep, input)

	// Update final execution status
	if wm.store != nil && executionID != "" {
		status := "completed"
		errorMessage := ""
		completedAt := time.Now()

		if err != nil {
			status = "failed"
			errorMessage = err.Error()
		}

		execution := models.WorkflowExecution{
			ID:           executionID,
			WorkflowID:   workflowID,
			Status:       status,
			Variables:    models.JSON(input),
			StartedAt:    time.Now(),
			CompletedAt:  &completedAt,
			ErrorMessage: errorMessage,
		}
		wm.store.SaveWorkflowExecution(ctx, execution)
	}

	return err
}

func (wm *WorkflowManager) executeStep(ctx context.Context, workflow Workflow, stepName string, variables map[string]interface{}) error {
	step, exists := workflow.Steps[stepName]
	if !exists {
		return fmt.Errorf("step not found: %s", stepName)
	}

	// Check if step should be executed based on condition
	if step.Config.Condition != "" {
		if !wm.evaluateCondition(step.Config.Condition, variables) {
			wm.logger.WithFields(logrus.Fields{
				"workflow_id": workflow.ID,
				"step_name":   stepName,
				"condition":   step.Config.Condition,
			}).Info("Step skipped due to condition")
			
			// Continue to next step on success path
			if step.OnSuccess != "" {
				return wm.executeStep(ctx, workflow, step.OnSuccess, variables)
			}
			return nil
		}
	}

	wm.logger.WithFields(logrus.Fields{
		"workflow_id": workflow.ID,
		"step_name":   stepName,
		"step_type":   step.Type,
	}).Info("Executing workflow step")

	// Create step execution record
	stepExec := models.StepExecution{
		StepName:  stepName,
		Status:    "running",
		StartTime: time.Now(),
		Attempts:  0,
	}

	var err error
	maxRetries := step.Config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	// Execute step with retry logic
	for attempt := 1; attempt <= maxRetries; attempt++ {
		stepExec.Attempts = attempt
		
		err = wm.executeStepWithTimeout(ctx, step, variables)
		
		if err == nil {
			// Success
			endTime := time.Now()
			stepExec.Status = "completed"
			stepExec.EndTime = &endTime
			break
		}

		// Log attempt failure
		wm.logger.WithFields(logrus.Fields{
			"workflow_id": workflow.ID,
			"step_name":   stepName,
			"attempt":     attempt,
			"max_retries": maxRetries,
			"error":       err.Error(),
		}).Warn("Step execution attempt failed")

		// If not the last attempt, wait before retry
		if attempt < maxRetries {
			retryDelay := step.Config.RetryDelayMs
			if retryDelay == 0 {
				retryDelay = 1000 // Default 1 second
			}
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	// Update final status
	if err != nil {
		endTime := time.Now()
		stepExec.Status = "failed"
		stepExec.EndTime = &endTime
		stepExec.ErrorMessage = err.Error()
	}

	// TODO: Save step execution to database
	wm.logger.WithFields(logrus.Fields{
		"step_name": stepName,
		"status":    stepExec.Status,
		"attempts":  stepExec.Attempts,
	}).Info("Step execution completed")

	// Determine next step
	var nextStep string
	if err != nil {
		if step.Config.ContinueOnError {
			nextStep = step.OnSuccess // Continue on success path even with error
		} else {
			nextStep = step.OnError
		}
	} else {
		nextStep = step.OnSuccess
	}

	if nextStep != "" {
		return wm.executeStep(ctx, workflow, nextStep, variables)
	}

	return err
}

func (wm *WorkflowManager) executeStepWithTimeout(ctx context.Context, step WorkflowStep, variables map[string]interface{}) error {
	timeoutMs := step.Config.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000 // Default 30 seconds
	}

	// Create context with timeout
	stepCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Execute step logic
	resultChan := make(chan error, 1)
	go func() {
		resultChan <- wm.executeStepLogic(stepCtx, step, variables)
	}()

	select {
	case err := <-resultChan:
		return err
	case <-stepCtx.Done():
		if stepCtx.Err() == context.DeadlineExceeded {
			wm.logger.WithFields(logrus.Fields{
				"step_name":  step.Name,
				"timeout_ms": timeoutMs,
			}).Warn("Step execution timed out")
			return fmt.Errorf("step execution timed out after %dms", timeoutMs)
		}
		return stepCtx.Err()
	}
}

func (wm *WorkflowManager) executeStepLogic(ctx context.Context, step WorkflowStep, variables map[string]interface{}) error {
	switch step.Type {
	case "http_request":
		return wm.executeHTTPRequest(ctx, step.Config.Parameters, variables)
	case "database_query":
		return wm.executeDatabaseQuery(ctx, step.Config.Parameters, variables)
	case "event_publish":
		return wm.executeEventPublish(ctx, step.Config.Parameters, variables)
	case "delay":
		return wm.executeDelay(ctx, step.Config.Parameters, variables)
	case "condition":
		return wm.executeCondition(ctx, step.Config.Parameters, variables)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (wm *WorkflowManager) evaluateCondition(condition string, variables map[string]interface{}) bool {
	// Simple condition evaluation - in production, you might want to use a proper expression evaluator
	// For now, implement basic conditions like "input.status == 'active'"
	
	// This is a simplified implementation
	// In production, consider using libraries like:
	// - github.com/antonmedv/expr
	// - github.com/Knetic/govaluate
	
	wm.logger.WithFields(logrus.Fields{
		"condition": condition,
		"variables": fmt.Sprintf("%+v", variables),
	}).Debug("Evaluating condition (simplified implementation)")
	
	// For demo purposes, return true for non-empty conditions
	// Implement proper condition evaluation based on your needs
	return true
}

func (wm *WorkflowManager) executeHTTPRequest(ctx context.Context, config map[string]interface{}, variables map[string]interface{}) error {
	wm.logger.WithFields(logrus.Fields{
		"config":    config,
		"variables": variables,
	}).Info("Executing HTTP request step")
	
	// TODO: Implement actual HTTP request logic
	// Example parameters: url, method, headers, body, etc.
	// url, _ := config["url"].(string)
	// method, _ := config["method"].(string)
	
	return nil
}

func (wm *WorkflowManager) executeDatabaseQuery(ctx context.Context, config map[string]interface{}, variables map[string]interface{}) error {
	wm.logger.WithFields(logrus.Fields{
		"config":    config,
		"variables": variables,
	}).Info("Executing database query step")
	
	// TODO: Implement actual database query logic
	// Example parameters: query, params, etc.
	// query, _ := config["query"].(string)
	
	return nil
}

func (wm *WorkflowManager) executeEventPublish(ctx context.Context, config map[string]interface{}, variables map[string]interface{}) error {
	eventType, ok := config["event_type"].(string)
	if !ok {
		return fmt.Errorf("event_type not specified in config")
	}

	eventData := variables
	if data, exists := config["data"]; exists {
		if dataMap, ok := data.(map[string]interface{}); ok {
			eventData = dataMap
		}
	}

	event := Event{
		Type:   eventType,
		Source: "workflow_manager",
		Data:   eventData,
	}

	wm.logger.WithFields(logrus.Fields{
		"event_type": eventType,
		"event_data": eventData,
	}).Info("Publishing event from workflow step")

	return wm.events.Publish(ctx, event)
}

func (wm *WorkflowManager) executeDelay(ctx context.Context, config map[string]interface{}, variables map[string]interface{}) error {
	delayMs, ok := config["delay_ms"].(float64)
	if !ok {
		delayMs = 1000 // Default 1 second
	}

	wm.logger.WithField("delay_ms", delayMs).Info("Executing delay step")

	select {
	case <-time.After(time.Duration(delayMs) * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wm *WorkflowManager) executeCondition(ctx context.Context, config map[string]interface{}, variables map[string]interface{}) error {
	condition, ok := config["condition"].(string)
	if !ok {
		return fmt.Errorf("condition not specified in config")
	}

	result := wm.evaluateCondition(condition, variables)
	
	wm.logger.WithFields(logrus.Fields{
		"condition": condition,
		"result":    result,
	}).Info("Condition step executed")

	if !result {
		return fmt.Errorf("condition evaluation failed: %s", condition)
	}

	return nil
}

func (wm *WorkflowManager) GetWorkflow(workflowID string) *Workflow {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if workflow, exists := wm.workflows[workflowID]; exists {
		return &workflow
	}
	return nil
}

func (wm *WorkflowManager) validateWorkflow(workflow Workflow) error {
	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}

	if workflow.StartStep == "" {
		return fmt.Errorf("start step is required")
	}

	if _, exists := workflow.Steps[workflow.StartStep]; !exists {
		return fmt.Errorf("start step %s not found in steps", workflow.StartStep)
	}

	return nil
}

func generateEventID() string {
	return fmt.Sprintf("event_%d_%s", time.Now().Unix(), randomString(8))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[i%len(charset)]
	}
	return string(result)
}
