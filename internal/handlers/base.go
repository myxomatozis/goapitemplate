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
	db           *database.DB
	cache        cache.Client
	eventManager *events.Manager
	logger       *logrus.Logger
}

func New(db *database.DB, cache cache.Client, eventManager *events.Manager) *Handler {
	return &Handler{
		db:           db,
		cache:        cache,
		eventManager: eventManager,
		logger:       logrus.New(),
	}
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")
	{
		// Health check
		api.GET("/health", h.HealthCheck)

		// Event routes
		events := api.Group("/events")
		{
			events.POST("/", h.CreateEvent)
			events.GET("/", h.GetEvents)
			events.GET("/types/:type", h.GetEventsByType)
			events.GET("/streams", h.GetEventStreams)
			events.GET("/streams/:stream_id", h.GetEventsByStream)
		}

		// Webhook management routes
		webhooks := api.Group("/webhooks")
		{
			webhooks.POST("/", h.CreateWebhook)
			webhooks.GET("/", h.GetWebhooks)
			webhooks.GET("/:id", h.GetWebhook)
			webhooks.PUT("/:id", h.UpdateWebhook)
			webhooks.DELETE("/:id", h.DeleteWebhook)
			webhooks.GET("/:id/deliveries", h.GetWebhookDeliveries)
			webhooks.POST("/retry", h.RetryWebhookDeliveries)
			webhooks.GET("/stats", h.GetWebhookStats)
		}


		// Monitoring routes
		monitoring := api.Group("/monitoring")
		{
			monitoring.GET("/stats", h.GetStats)
		}
	}

	// Root redirect to documentation
	router.GET("/", h.RootRedirect)
	
	// API documentation
	router.GET("/docs/*any", h.SwaggerDocs)
}

func (h *Handler) RootRedirect(c *gin.Context) {
	c.Redirect(302, "/docs/index.html")
}

func (h *Handler) SwaggerDocs(c *gin.Context) {
	ginSwagger.WrapHandler(swaggerFiles.Handler)(c)
}
