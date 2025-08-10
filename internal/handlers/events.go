package handlers

import (
	"context"
	"net/http"
	"strconv"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Create Event
// @Description Create a new event in a stream
// @Tags events
// @Accept json
// @Produce json
// @Param event body models.CreateEventRequest true "Event data"
// @Success 201 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/events [post]
func (h *Handler) CreateEvent(c *gin.Context) {
	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Publish event using the event manager
	err := h.eventManager.Publish(context.Background(), req.StreamID, req.Type, req.Source, req.Data)
	if err != nil {
		h.logger.WithError(err).Error("Failed to publish event")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create event",
		})
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Event created successfully",
	})
}

// @Summary Get Events
// @Description Get events from the system with pagination
// @Tags events
// @Produce json
// @Param limit query int false "Number of events to return" default(50)
// @Param offset query int false "Number of events to skip" default(0)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/events [get]
func (h *Handler) GetEvents(c *gin.Context) {
	limit := 50
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
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
// @Tags events
// @Produce json
// @Param type path string true "Event type"
// @Param limit query int false "Number of events to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/events/types/{type} [get]
func (h *Handler) GetEventsByType(c *gin.Context) {
	eventType := c.Param("type")
	limit := 50
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
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

// @Summary Get Event Streams
// @Description Get list of available event streams
// @Tags events
// @Produce json
// @Param limit query int false "Number of streams to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/events/streams [get]
func (h *Handler) GetEventStreams(c *gin.Context) {
	limit := 50
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
	streamIDs, err := eventStore.GetEventStreams(context.Background(), limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get event streams")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get event streams",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    streamIDs,
	})
}

// @Summary Get Events by Stream
// @Description Get events from a specific stream
// @Tags events
// @Produce json
// @Param stream_id path string true "Stream ID"
// @Param limit query int false "Number of events to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/events/streams/{stream_id} [get]
func (h *Handler) GetEventsByStream(c *gin.Context) {
	streamID := c.Param("stream_id")
	limit := 50
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	eventStore := h.eventManager.GetStore()
	events, err := eventStore.GetEventsByStream(context.Background(), streamID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get events by stream")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get events",
		})
		return
	}

	response := models.EventStreamResponse{
		StreamID: streamID,
		Events:   events,
		Count:    int64(len(events)),
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    response,
	})
}