package database

import (
	"fmt"
	"time"

	"goapitemplate/internal/config"
	"goapitemplate/pkg/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
	dbType string
}

func New(cfg config.DatabaseConfig) (*DB, error) {
	var (
		gormDB *gorm.DB
		err    error
	)

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	switch cfg.Type {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, cfg.SSLMode)
		gormDB, err = gorm.Open(postgres.Open(dsn), gormConfig)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		gormDB, err = gorm.Open(mysql.Open(dsn), gormConfig)
	case "sqlite":
		gormDB, err = gorm.Open(sqlite.Open(cfg.Database), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{
		DB:     gormDB,
		dbType: cfg.Type,
	}, nil
}

func (db *DB) GetDBType() string {
	return db.dbType
}

func (db *DB) AutoMigrate() error {
	return db.DB.AutoMigrate(
		&models.Event{},
		&models.WebhookEndpoint{},
		&models.WebhookDelivery{},
	)
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Example query methods demonstrating GORM patterns for event streams

// GetEventsByStreamWithPagination gets events from a specific stream with pagination
func (db *DB) GetEventsByStreamWithPagination(streamID string, offset, limit int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64
	
	query := db.DB.Model(&models.Event{}).Where("stream_id = ?", streamID)
	
	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Get paginated results ordered by sequence
	if err := query.Order("sequence_number ASC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	
	return events, total, nil
}

// GetEventsByTypeWithPagination demonstrates pagination and filtering by type
func (db *DB) GetEventsByTypeWithPagination(eventType string, offset, limit int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64
	
	query := db.DB.Model(&models.Event{})
	if eventType != "" {
		query = query.Where("type = ?", eventType)
	}
	
	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// Get paginated results
	if err := query.Order("timestamp DESC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	
	return events, total, nil
}

// CreateEventWithSequence creates an event with proper sequence number
func (db *DB) CreateEventWithSequence(event *models.Event) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		// Get next sequence number for this stream
		var maxSeq int64
		err := tx.Model(&models.Event{}).
			Where("stream_id = ?", event.StreamID).
			Select("COALESCE(MAX(sequence_number), 0)").
			Scan(&maxSeq).Error
		if err != nil {
			return err
		}
		
		event.SequenceNumber = maxSeq + 1
		
		// Create event
		return tx.Create(event).Error
	})
}

// GetWebhookDeliveriesWithRelations demonstrates complex relationships
func (db *DB) GetWebhookDeliveriesWithRelations(webhookID string, limit int) ([]models.WebhookDelivery, error) {
	var deliveries []models.WebhookDelivery
	
	err := db.DB.Preload("Webhook").Preload("Event").
		Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&deliveries).Error
	
	return deliveries, err
}

// GetEventStatsByType demonstrates aggregation queries for events
func (db *DB) GetEventStatsByType() (map[string]int64, error) {
	var results []struct {
		Type  string
		Count int64
	}
	
	err := db.DB.Model(&models.Event{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&results).Error
	
	if err != nil {
		return nil, err
	}
	
	stats := make(map[string]int64)
	for _, result := range results {
		stats[result.Type] = result.Count
	}
	
	return stats, nil
}

