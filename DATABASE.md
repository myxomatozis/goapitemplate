# Database Migration and Management Guide

This document explains how database migrations and schema management work in this Go API template.

## Overview

This template uses **GORM AutoMigrate** for automatic database schema management. This approach:
- ✅ Automatically creates tables based on Go struct definitions
- ✅ Handles schema updates when models change
- ✅ Works consistently across PostgreSQL, MySQL, and SQLite
- ✅ Eliminates database-specific SQL migration files
- ✅ Keeps schema in sync with code models

## Migration Pattern

### Automatic Migration on Startup
The application automatically runs migrations during startup:

```go
// cmd/server/main.go
if err := db.AutoMigrate(); err != nil {
    log.Fatalf("Failed to migrate database: %v", err)
}
```

### Manual Migration Script
You can also run migrations separately using:
```bash
make setup-db
# or
go run scripts/migrate.go
```

### Migration Configuration
Migrations are configured in `internal/database/database.go`:

```go
func (db *DB) AutoMigrate() error {
    return db.DB.AutoMigrate(
        &models.Event{},
        &models.WebhookEndpoint{},
        &models.WebhookDelivery{},
        // Add new models here
    )
}
```

## Adding New Tables

### Step 1: Define Your Model
Create your model struct in `pkg/models/models.go`:

```go
// Example: Adding a User model
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Email     string    `gorm:"uniqueIndex;not null" json:"email"`
    Name      string    `gorm:"not null" json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// Optional: Custom table name
func (User) TableName() string {
    return "users"
}
```

### Step 2: Add to AutoMigrate
Update the migration function:

```go
func (db *DB) AutoMigrate() error {
    return db.DB.AutoMigrate(
        &models.Event{},
        &models.WebhookEndpoint{},
        &models.WebhookDelivery{},
        &models.User{}, // Add your new model
    )
}
```

### Step 3: Run Migration
The migration will run automatically on next application start, or run manually:
```bash
make setup-db
```

## Managing Relations

### One-to-Many Relationship
```go
type User struct {
    ID     uint    `gorm:"primaryKey"`
    Name   string  `gorm:"not null"`
    Posts  []Post  `gorm:"foreignKey:UserID"`
}

type Post struct {
    ID      uint   `gorm:"primaryKey"`
    Title   string `gorm:"not null"`
    UserID  uint   `gorm:"not null;index"`
    User    User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
```

### Many-to-Many Relationship
```go
type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"not null"`
    Roles []Role `gorm:"many2many:user_roles;"`
}

type Role struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"not null"`
    Users []User `gorm:"many2many:user_roles;"`
}
```

### Belongs-To Relationship
```go
type Profile struct {
    ID     uint `gorm:"primaryKey"`
    UserID uint `gorm:"not null;uniqueIndex"`
    User   User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
    Bio    string
}
```

## GORM Field Tags Reference

### Primary Key and Indexes
```go
ID       uint   `gorm:"primaryKey"`
Email    string `gorm:"uniqueIndex;not null"`
Status   string `gorm:"index"`
Name     string `gorm:"index:idx_name_status,composite:name_status"`
```

### Constraints
```go
Name     string    `gorm:"not null;size:100"`
Age      int       `gorm:"check:age >= 18"`
Email    string    `gorm:"uniqueIndex"`
Price    float64   `gorm:"precision:10;scale:2"`
```

### JSON and Custom Types
```go
Data       JSON      `gorm:"type:json"`           // PostgreSQL/MySQL
Settings   JSON      `gorm:"type:text"`           // SQLite fallback
Metadata   string    `gorm:"type:jsonb"`          // PostgreSQL JSONB
```

### Timestamps
```go
CreatedAt time.Time  `gorm:"autoCreateTime"`      // Auto-set on create
UpdatedAt time.Time  `gorm:"autoUpdateTime"`      // Auto-set on update
DeletedAt *time.Time `gorm:"index"`               // Soft delete
```

## Query Patterns

### Basic CRUD Operations

#### Create
```go
user := models.User{Name: "John", Email: "john@example.com"}
result := db.Create(&user)
if result.Error != nil {
    // Handle error
}
```

#### Read
```go
// Find by ID
var user models.User
err := db.First(&user, 1).Error

// Find by condition
err := db.Where("email = ?", "john@example.com").First(&user).Error

// Find multiple
var users []models.User
err := db.Where("active = ?", true).Find(&users).Error
```

#### Update
```go
// Update single field
db.Model(&user).Update("name", "John Doe")

// Update multiple fields
db.Model(&user).Updates(models.User{Name: "John Doe", Age: 25})

// Update with map
db.Model(&user).Updates(map[string]interface{}{
    "name": "John Doe",
    "age":  25,
})
```

#### Delete
```go
// Soft delete (if DeletedAt field exists)
db.Delete(&user, 1)

// Permanent delete
db.Unscoped().Delete(&user, 1)
```

### Advanced Queries

#### Joins and Preloading
```go
// Preload related data
var users []models.User
db.Preload("Posts").Find(&users)

// Join queries
var result []struct {
    UserName  string
    PostCount int64
}
db.Table("users").
    Select("users.name as user_name, count(posts.id) as post_count").
    Joins("left join posts on posts.user_id = users.id").
    Group("users.id").
    Scan(&result)
```

#### Pagination
```go
var users []models.User
var total int64

db.Model(&models.User{}).Count(&total)
db.Offset(offset).Limit(limit).Find(&users)
```

#### Transactions
```go
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user).Error; err != nil {
        return err
    }
    if err := tx.Create(&profile).Error; err != nil {
        return err
    }
    return nil
})
```

## Database-Specific Features

### PostgreSQL
```go
// JSONB support
Data JSON `gorm:"type:jsonb"`

// Arrays
Tags pq.StringArray `gorm:"type:text[]"`

// UUID
ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
```

### MySQL
```go
// JSON support
Data JSON `gorm:"type:json"`

// Full-text search
Content string `gorm:"type:text;index:,class:FULLTEXT"`
```

### SQLite
```go
// JSON stored as TEXT
Data JSON `gorm:"type:text"`

// Note: SQLite limitations with concurrent writes
```

## Best Practices

### 1. Model Design
- Use consistent naming conventions
- Always include `CreatedAt` and `UpdatedAt` for audit trails
- Consider soft deletes with `DeletedAt *gorm.DeletedAt`
- Use appropriate data types and constraints

### 2. Migration Safety
- Test migrations on development/staging first
- Backup production data before major schema changes
- Use transactions for complex migrations
- Monitor migration performance on large tables

### 3. Performance
- Add appropriate indexes for frequently queried fields
- Use composite indexes for multi-column queries
- Consider database-specific optimizations
- Monitor query performance

### 4. Error Handling
```go
result := db.Create(&user)
if result.Error != nil {
    if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
        // Handle duplicate key error
    }
    return result.Error
}
```

## Migration Rollback

GORM AutoMigrate only adds columns and indexes - it doesn't drop them. For rollbacks:

1. **Manual rollback**: Connect to database and run SQL manually
2. **Version-controlled migrations**: Consider using a migration tool like [golang-migrate](https://github.com/golang-migrate/migrate) for complex scenarios
3. **Backup-restore**: Keep database backups for major changes

## Monitoring Migrations

Enable GORM logging to monitor migration activity:

```go
gormConfig := &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info), // Shows SQL statements
}
```

## Troubleshooting

### Common Issues

1. **Column type mismatch**: GORM may not change existing column types
   - Solution: Manual ALTER TABLE statements

2. **Index conflicts**: Naming conflicts with existing indexes
   - Solution: Use explicit index names with `index:idx_name`

3. **Foreign key constraints**: Dependency order matters
   - Solution: Order models properly in AutoMigrate()

4. **Large table migrations**: May cause downtime
   - Solution: Use online schema change tools for production

### Debug Mode
Enable debug mode to see generated SQL:
```go
db.Debug().AutoMigrate(&models.User{})
```

## Example Usage in Handlers

The template includes example database methods in `internal/database/database.go` that demonstrate various GORM patterns:

### Using Preloading (Relationships)
```go
// Get webhook deliveries with related webhook and event data
deliveries, err := h.db.GetWebhookDeliveriesWithRelations("webhook-123", 10)
if err != nil {
    // Handle error
}
// deliveries will contain related webhook and event data
```

### Pagination with Filtering
```go
// Get paginated events with optional filtering by type
events, total, err := h.db.GetEventsByTypeWithPagination("user.created", 0, 10)
if err != nil {
    // Handle error
}
log.Printf("Found %d events, showing first 10", total)
```

### Transaction Usage
```go
// Create event with proper sequence numbering (atomic)
event := &models.Event{
    ID: "event-123",
    Type: "user.created",
    StreamID: "user-stream-456",
    Source: "user-service",
    Data: models.JSON{"user_id": 789},
}

err := h.db.CreateEventWithSequence(event)
if err != nil {
    // Transaction will be rolled back
}
```

### Aggregation Queries
```go
// Get event statistics by type
stats, err := h.db.GetEventStatsByType()
if err != nil {
    // Handle error
}
// stats map contains: {"user.created": 150, "payment.processed": 25, "order.placed": 10}
```

## Current Schema

The template includes these models with relationships:

### Event Streaming Models

#### Event
- **Core event model** with stream grouping and sequence numbering
- Fields: `ID`, `Type`, `StreamID`, `Source`, `Data`, `Timestamp`, `SequenceNumber`
- Events are grouped by `StreamID` and ordered by `SequenceNumber`

#### WebhookEndpoint ↔ WebhookDelivery (One-to-Many)
- **WebhookEndpoint** has many **WebhookDelivery** records
- **WebhookDelivery** belongs to **WebhookEndpoint** and **Event**
- Uses foreign keys with CASCADE constraints for data integrity

#### WebhookDelivery ↔ Event (Many-to-One)
- **WebhookDelivery** tracks each delivery attempt to a webhook
- Links to both **WebhookEndpoint** and **Event** for complete audit trail
- Stores delivery status, attempts, responses, and retry information

### JSON Support
- Custom `JSON` type works across all supported databases
- PostgreSQL: Uses native `json` type
- MySQL: Uses native `json` type  
- SQLite: Stores as `text` with JSON serialization

This migration pattern provides a robust, maintainable approach to database schema management that scales with your application.