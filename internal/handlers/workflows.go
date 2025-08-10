package handlers

import (
	"context"
	"fmt"
	"net/http"

	"goapitemplate/internal/events"
	"goapitemplate/pkg/models"

	"github.com/gin-gonic/gin"
)

// @Summary Create Workflow
// @Description Create a new workflow definition
// @Tags workflows
// @Accept json
// @Produce json
// @Param workflow body models.Workflow true "Workflow definition"
// @Success 201 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/workflows [post]
func (h *Handler) CreateWorkflow(c *gin.Context) {
	var workflow events.Workflow
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := h.workflowManager.RegisterWorkflow(workflow); err != nil {
		h.logger.WithError(err).Error("Failed to register workflow")
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Workflow created successfully",
		Data: map[string]interface{}{
			"workflow_id": workflow.ID,
		},
	})
}

// @Summary Execute Workflow
// @Description Execute a workflow with input data
// @Tags workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param input body map[string]interface{} true "Workflow input data"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /api/v1/workflows/{id}/execute [post]
func (h *Handler) ExecuteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := h.workflowManager.ExecuteWorkflow(context.Background(), workflowID, input); err != nil {
		if err.Error() == fmt.Sprintf("workflow not found: %s", workflowID) {
			c.JSON(http.StatusNotFound, models.APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		h.logger.WithError(err).Error("Failed to execute workflow")
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to execute workflow",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Workflow execution started",
	})
}

// @Summary Get Workflow
// @Description Get workflow definition by ID
// @Tags workflows
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Router /api/v1/workflows/{id} [get]
func (h *Handler) GetWorkflow(c *gin.Context) {
	workflowID := c.Param("id")

	workflow := h.workflowManager.GetWorkflow(workflowID)
	if workflow == nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Success: false,
			Error:   "Workflow not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    workflow,
	})
}