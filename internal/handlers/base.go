package handlers

import (
	"goapitemplate/internal/cache"
	"goapitemplate/internal/database"
	"goapitemplate/internal/events"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	db              *database.DB
	cache           cache.Client
	eventManager    *events.Manager
	workflowManager *events.WorkflowManager
	logger          *logrus.Logger
}

func New(db *database.DB, cache cache.Client, eventManager *events.Manager, workflowManager *events.WorkflowManager) *Handler {
	return &Handler{
		db:              db,
		cache:           cache,
		eventManager:    eventManager,
		workflowManager: workflowManager,
		logger:          logrus.New(),
	}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Health check
		api.GET("/health", h.HealthCheck)

		// Monitoring routes
		monitoring := api.Group("/monitoring")
		{
			monitoring.GET("/events", h.GetEvents)
			monitoring.GET("/events/:type", h.GetEventsByType)
			monitoring.GET("/workflows", h.GetWorkflowExecutions)
			monitoring.GET("/workflows/:id", h.GetWorkflowExecution)
			monitoring.GET("/stats", h.GetStats)
		}

		// Workflow management routes
		workflows := api.Group("/workflows")
		{
			workflows.POST("/", h.CreateWorkflow)
			workflows.POST("/:id/execute", h.ExecuteWorkflow)
			workflows.GET("/:id", h.GetWorkflow)
		}
	}

	// API documentation
	router.GET("/docs/*any", h.SwaggerDocs)
}

func (h *Handler) SwaggerDocs(c *gin.Context) {
	ginSwagger.WrapHandler(swaggerFiles.Handler)(c)
}
