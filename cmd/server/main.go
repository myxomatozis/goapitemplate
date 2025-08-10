// @title Go API Template
// @version 1.0
// @description A modern Go API service template with database support, caching, event streaming, and webhook delivery
// @contact.name API Support
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "goapitemplate/docs" // Import generated docs
	"goapitemplate/internal/cache"
	"goapitemplate/internal/config"
	"goapitemplate/internal/database"
	"goapitemplate/internal/events"
	"goapitemplate/internal/handlers"
	"goapitemplate/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.AutoMigrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	var cacheClient cache.Client
	if cfg.Cache.Enabled {
		cacheClient, err = cache.New(cfg.Cache)
		if err != nil {
			log.Fatalf("Failed to connect to cache: %v", err)
		}
		defer cacheClient.Close()
	}

	eventStore := events.NewDBEventStore(db)
	eventManager := events.NewManager(eventStore, db)

	router := gin.New()
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS(cfg.CORS))
	
	if cfg.RateLimit.Enabled {
		router.Use(middleware.RateLimit(cfg.RateLimit.MaxRequests, time.Duration(cfg.RateLimit.WindowMinutes)*time.Minute))
	}

	handler := handlers.New(db, cacheClient, eventManager)
	handler.RegisterRoutes(router)

	// Start webhook retry scheduler
	go startWebhookRetryScheduler(eventManager)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func startWebhookRetryScheduler(eventManager *events.Manager) {
	ticker := time.NewTicker(1 * time.Minute) // Check for retries every minute
	defer ticker.Stop()

	for range ticker.C {
		if err := eventManager.GetWebhookDeliveryService().RetryFailedDeliveries(context.Background()); err != nil {
			log.Printf("Error retrying webhook deliveries: %v", err)
		}
	}
}
