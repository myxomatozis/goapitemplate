package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Create Webhook
// @Description Create a new webhook endpoint
// @Tags webhooks
// @Accept json
// @Produce json
// @Param webhook body models.CreateWebhookRequest true "Webhook data"
// @Success 201 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks [post]
func (h *Handler) CreateWebhook(c *gin.Context) {
	var req models.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	webhook := models.WebhookEndpoint{
		ID:             generateID(),
		Name:           req.Name,
		URL:            req.URL,
		Secret:         req.Secret,
		EventTypes:     req.EventTypes,
		Enabled:        true,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	// Set defaults
	if webhook.MaxRetries == 0 {
		webhook.MaxRetries = 3
	}
	if webhook.TimeoutSeconds == 0 {
		webhook.TimeoutSeconds = 30
	}

	if err := h.db.Create(&webhook).Error; err != nil {
		h.logger.WithError(err).Error("Failed to create webhook")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create webhook",
		})
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{
		Success: true,
		Data:    webhook,
	})
}

// @Summary Get Webhooks
// @Description Get all webhook endpoints
// @Tags webhooks
// @Produce json
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks [get]
func (h *Handler) GetWebhooks(c *gin.Context) {
	var webhooks []models.WebhookEndpoint
	if err := h.db.Order("created_at DESC").Find(&webhooks).Error; err != nil {
		h.logger.WithError(err).Error("Failed to get webhooks")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get webhooks",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    webhooks,
	})
}

// @Summary Get Webhook
// @Description Get webhook by ID
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/{id} [get]
func (h *Handler) GetWebhook(c *gin.Context) {
	webhookID := c.Param("id")

	var webhook models.WebhookEndpoint
	err := h.db.First(&webhook, "id = ?", webhookID).Error
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusNotFound, models.APIResponse{
				Success: false,
				Error:   "Webhook not found",
			})
			return
		}
		h.logger.WithError(err).Error("Failed to get webhook")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get webhook",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    webhook,
	})
}

// @Summary Update Webhook
// @Description Update webhook by ID
// @Tags webhooks
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Param webhook body models.UpdateWebhookRequest true "Webhook data"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/{id} [put]
func (h *Handler) UpdateWebhook(c *gin.Context) {
	webhookID := c.Param("id")

	var req models.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.URL != "" {
		updates["url"] = req.URL
	}
	if req.Secret != "" {
		updates["secret"] = req.Secret
	}
	if len(req.EventTypes) > 0 {
		updates["event_types"] = req.EventTypes
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.MaxRetries > 0 {
		updates["max_retries"] = req.MaxRetries
	}
	if req.TimeoutSeconds > 0 {
		updates["timeout_seconds"] = req.TimeoutSeconds
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "No fields to update",
		})
		return
	}

	result := h.db.Model(&models.WebhookEndpoint{}).Where("id = ?", webhookID).Updates(updates)
	if result.Error != nil {
		h.logger.WithError(result.Error).Error("Failed to update webhook")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to update webhook",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Webhook not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Webhook updated successfully",
	})
}

// @Summary Delete Webhook
// @Description Delete webhook by ID
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/{id} [delete]
func (h *Handler) DeleteWebhook(c *gin.Context) {
	webhookID := c.Param("id")

	result := h.db.Delete(&models.WebhookEndpoint{}, "id = ?", webhookID)
	if result.Error != nil {
		h.logger.WithError(result.Error).Error("Failed to delete webhook")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to delete webhook",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Webhook not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Webhook deleted successfully",
	})
}

// @Summary Get Webhook Deliveries
// @Description Get delivery history for a webhook
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Param limit query int false "Number of deliveries to return" default(50)
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/{id}/deliveries [get]
func (h *Handler) GetWebhookDeliveries(c *gin.Context) {
	webhookID := c.Param("id")
	limit := 50
	
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	deliveries, err := h.db.GetWebhookDeliveriesWithRelations(webhookID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get webhook deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get webhook deliveries",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    deliveries,
	})
}

// @Summary Retry Webhook Deliveries
// @Description Manually retry failed webhook deliveries
// @Tags webhooks
// @Produce json
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/retry [post]
func (h *Handler) RetryWebhookDeliveries(c *gin.Context) {
	deliveryService := h.eventManager.GetWebhookDeliveryService()
	
	err := deliveryService.RetryFailedDeliveries(context.Background())
	if err != nil {
		h.logger.WithError(err).Error("Failed to retry webhook deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retry deliveries",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Delivery retry initiated",
	})
}

// @Summary Get Webhook Delivery Statistics
// @Description Get statistics about webhook deliveries
// @Tags webhooks
// @Produce json
// @Success 200 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/webhooks/stats [get]
func (h *Handler) GetWebhookStats(c *gin.Context) {
	var stats struct {
		TotalDeliveries      int64   `json:"total_deliveries"`
		SuccessfulDeliveries int64   `json:"successful_deliveries"`
		FailedDeliveries     int64   `json:"failed_deliveries"`
		PendingDeliveries    int64   `json:"pending_deliveries"`
		SuccessRate          float64 `json:"success_rate"`
	}

	// Get total deliveries
	err := h.db.Model(&models.WebhookDelivery{}).Count(&stats.TotalDeliveries).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to get total deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get statistics",
		})
		return
	}

	// Get successful deliveries
	err = h.db.Model(&models.WebhookDelivery{}).Where("status = ?", "success").Count(&stats.SuccessfulDeliveries).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to get successful deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get statistics",
		})
		return
	}

	// Get failed deliveries
	err = h.db.Model(&models.WebhookDelivery{}).Where("status = ?", "failed").Count(&stats.FailedDeliveries).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to get failed deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get statistics",
		})
		return
	}

	// Get pending deliveries
	err = h.db.Model(&models.WebhookDelivery{}).Where("status = ?", "pending").Count(&stats.PendingDeliveries).Error
	if err != nil {
		h.logger.WithError(err).Error("Failed to get pending deliveries")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to get statistics",
		})
		return
	}

	// Calculate success rate
	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessfulDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// Helper function to generate IDs
func generateID() string {
	// Generate a simple unique ID using timestamp and random component
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	return timestamp[:8] + "x" // Simple but reasonably unique for webhook IDs
}