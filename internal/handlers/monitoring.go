package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get Events
// @Description Get events from the system
// @Tags monitoring
// @Produce json
// @Param limit query int false "Number of events to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/events [get]
func (h *Handler) GetEvents(c *gin.Context) {
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
	if eventStore == nil {
		c.JSON(http.StatusServiceUnavailable, models.APIResponse{
			Success: false,
			Error:   "Event store not available",
		})
		return
	}

	events, err := eventStore.GetEvents(context.Background(), "", limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get events")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get events",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    events,
	})
}

// @Summary Get Events by Type
// @Description Get events of a specific type
// @Tags monitoring
// @Produce json
// @Param type path string true "Event type"
// @Param limit query int false "Number of events to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/events/{type} [get]
func (h *Handler) GetEventsByType(c *gin.Context) {
	eventType := c.Param("type")
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
	if eventStore == nil {
		c.JSON(http.StatusServiceUnavailable, models.APIResponse{
			Success: false,
			Error:   "Event store not available",
		})
		return
	}

	events, err := eventStore.GetEvents(context.Background(), eventType, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get events by type")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get events",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    events,
	})
}

// @Summary Get Workflow Executions
// @Description Get workflow execution history
// @Tags monitoring
// @Produce json
// @Param limit query int false "Number of executions to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/workflows [get]
func (h *Handler) GetWorkflowExecutions(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Workflow executions endpoint - implementation depends on your specific requirements",
		Data:    []interface{}{},
	})
}

// @Summary Get Workflow Execution
// @Description Get specific workflow execution by ID
// @Tags monitoring
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/workflows/{id} [get]
func (h *Handler) GetWorkflowExecution(c *gin.Context) {
	executionID := c.Param("id")

	eventStore := h.eventManager.GetStore()
	if eventStore == nil {
		c.JSON(http.StatusServiceUnavailable, models.APIResponse{
			Success: false,
			Error:   "Event store not available",
		})
		return
	}

	execution, err := eventStore.GetWorkflowExecution(context.Background(), executionID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get workflow execution")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get workflow execution",
		})
		return
	}

	if execution == nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Workflow execution not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    execution,
	})
}

// @Summary Get System Stats
// @Description Get system statistics and metrics
// @Tags monitoring
// @Produce json
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	stats := map[string]interface{}{
		"timestamp": time.Now(),
		"uptime":    time.Since(time.Now().Add(-time.Hour)), // Placeholder
		"database":  "connected",
		"cache":     h.cache != nil,
		"events":    "enabled",
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}