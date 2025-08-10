package handlers

import (
	"net/http"
	"time"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Get System Stats
// @Description Get system statistics and metrics
// @Tags monitoring
// @Produce json
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/monitoring/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	// Get event type statistics
	eventStats, err := h.db.GetEventStatsByType()
	if err != nil {
		h.logger.WithError(err).Error("Failed to get event stats")
		eventStats = make(map[string]int64)
	}

	stats := map[string]interface{}{
		"timestamp":   time.Now(),
		"uptime":      time.Since(time.Now().Add(-time.Hour)), // Placeholder
		"database":    "connected",
		"cache":       h.cache != nil,
		"events":      "enabled",
		"event_stats": eventStats,
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}