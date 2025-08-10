package handlers

import (
	"net/http"
	"time"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Health Check
// @Description Check if the API is running
// @Tags health
// @Produce json
// @Success 200 {object} models.APIResponse
// @Router /api/v1/health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "API is running",
		Data: map[string]interface{}{
			"timestamp": time.Now(),
			"version":   "1.0.0",
		},
	})
}
