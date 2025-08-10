# Go API Template

A modern, production-ready Go API service template featuring multiple database engine support, caching, event stream management with webhook delivery, and comprehensive tooling.

## Features

- **Multi-Database Support**: PostgreSQL, MySQL, SQLite with automatic configuration
- **Caching Layer**: Redis and Memcache support with configurable TTL
- **Event Streaming**: Event stream management with external webhook delivery  
- **API Documentation**: Automatic OpenAPI/Swagger documentation generation
- **Middleware Stack**: Logging, recovery, CORS, rate limiting, request ID tracking
- **Configuration Management**: Environment-based configuration with validation
- **Docker Support**: Complete containerization with docker-compose for development
- **Structured Logging**: JSON-formatted logging with logrus
- **Health Checks**: Built-in health endpoints for monitoring
- **Graceful Shutdown**: Proper resource cleanup on termination

## Quick Start

### Prerequisites

- Go 1.24+ installed
- Docker and Docker Compose (optional)
- Database server (PostgreSQL/MySQL) or use SQLite

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd goapitemplate
```

2. Install dependencies:
```bash
make install-deps
```

3. Copy and configure environment variables:
```bash
cp configs/config.example.env .env
# Edit .env with your configuration
```

4. Generate API documentation:
```bash
make docs
```

5. Run the application:
```bash
make run
```

The API will be available at `http://localhost:8080`

## Configuration

Configuration is managed through environment variables. See `configs/config.example.env` for all available options.

### Database Configuration

```bash
# Database type: postgres, mysql, sqlite
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=goapitemplate
DB_USER=postgres
DB_PASSWORD=yourpassword
```

### Cache Configuration

```bash
# Enable caching
CACHE_ENABLED=true
# Cache type: redis, memcache
CACHE_TYPE=redis
CACHE_HOST=localhost
CACHE_PORT=6379
```

## Development

### Using Make Commands

```bash
make help          # Show available commands
make build         # Build the application
make run           # Build and run
make test          # Run tests
make docs          # Generate API documentation
make dev           # Development mode with docs
make clean         # Clean build artifacts
```

### Using Docker

#### Development with Docker Compose

```bash
# Start with PostgreSQL and Redis
docker-compose -f deployments/docker-compose.yml up

# Start with MySQL instead
docker-compose -f deployments/docker-compose.yml --profile mysql up

# Include Adminer for database management
docker-compose -f deployments/docker-compose.yml --profile admin up
```

#### Production Docker Build

```bash
make docker-build
make docker-run
```

## API Endpoints

### Health Check
- `GET /api/v1/health` - Service health status

### Event Streaming
- `POST /api/v1/events` - Create event in a stream
- `GET /api/v1/events` - Get events with pagination
- `GET /api/v1/events/types/:type` - Get events by type
- `GET /api/v1/events/streams` - Get available event streams
- `GET /api/v1/events/streams/:stream_id` - Get events from specific stream

### Webhook Management
- `POST /api/v1/webhooks` - Create webhook endpoint
- `GET /api/v1/webhooks` - List webhook endpoints
- `GET /api/v1/webhooks/:id` - Get webhook by ID
- `PUT /api/v1/webhooks/:id` - Update webhook
- `DELETE /api/v1/webhooks/:id` - Delete webhook
- `GET /api/v1/webhooks/:id/deliveries` - Get webhook delivery history

### Monitoring
- `GET /api/v1/monitoring/stats` - System statistics

### Documentation
- `GET /docs/` - Swagger UI documentation

## Project Structure

```
.
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── database/        # Database layer
│   ├── cache/           # Cache layer
│   ├── handlers/        # HTTP handlers
│   ├── middleware/      # HTTP middleware
│   └── events/          # Event stream manager
├── pkg/
│   ├── models/          # Data models
│   └── utils/           # Utility functions
├── configs/             # Configuration files
├── deployments/         # Docker and deployment files
├── docs/                # Generated API documentation
└── Makefile            # Build automation
```

## Database Support

### PostgreSQL
```bash
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
```

### MySQL
```bash
DB_TYPE=mysql
DB_HOST=localhost
DB_PORT=3306
```

### SQLite
```bash
DB_TYPE=sqlite
DB_NAME=./data.db  # File path
```

## Caching

### Redis
```bash
CACHE_ENABLED=true
CACHE_TYPE=redis
CACHE_HOST=localhost
CACHE_PORT=6379
```

### Memcache
```bash
CACHE_ENABLED=true
CACHE_TYPE=memcache
CACHE_HOST=localhost
CACHE_PORT=11211
```

## Event Streaming System

The template features a comprehensive event streaming system designed for external consumption and real-time data distribution:

### Core Concepts

- **Event Streams**: Groups of related events identified by `stream_id` for logical organization
- **Sequence Numbers**: Events within each stream are ordered sequentially for guaranteed ordering
- **External Consumption**: Webhook-based delivery system for external applications
- **Event Types**: Categorized events with type-specific processing and filtering

### Event Structure

Events are structured with the following key properties:
- `id`: Unique event identifier
- `type`: Event category (e.g., "user.created", "payment.processed")
- `stream_id`: Logical grouping for related events
- `source`: Event origin/producer
- `data`: Event payload as JSON
- `timestamp`: Event creation time
- `sequence_number`: Ordering within stream

### Webhook Delivery

Configure webhooks to receive events automatically:

```bash
# Create a webhook endpoint
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Service Webhook",
    "url": "https://myservice.com/webhooks/events",
    "secret": "webhook-secret-key",
    "event_types": ["user.created", "payment.processed"],
    "max_retries": 3,
    "timeout_seconds": 30
  }'
```

### Publishing Events

```bash
# Create an event
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -d '{
    "type": "user.created",
    "stream_id": "user-123",
    "source": "user-service",
    "data": {
      "user_id": 123,
      "email": "user@example.com"
    }
  }'
```

### Event Querying

```bash
# Get events from specific stream
curl "http://localhost:8080/api/v1/events/streams/user-123?limit=10"

# Get events by type
curl "http://localhost:8080/api/v1/events/types/user.created?limit=50"

# Get all event streams
curl "http://localhost:8080/api/v1/events/streams"
```

### Internal Event Handlers

Event handlers can be registered internally for processing events within the application. These are separate from the public API and are designed for internal business logic:

```go
// Register internal event handler (not exposed via API)
eventManager.RegisterHandler("user.created", func(ctx context.Context, event models.Event) error {
    // Process user creation event internally
    log.Printf("User created: %s", event.Data)
    return nil
})
```

## Middleware

- **Logger**: Structured request logging
- **Recovery**: Panic recovery with stack traces
- **CORS**: Cross-origin resource sharing
- **Rate Limiting**: IP-based rate limiting
- **Request ID**: Request tracing

## Monitoring

### Health Checks
- HTTP health endpoint at `/api/v1/health`
- Docker health checks configured
- Database connection validation

### Logging
- Structured JSON logging
- Configurable log levels (debug, info, warn, error)
- Request/response logging with correlation IDs

## Production Deployment

### Environment Variables
Set all required environment variables in your production environment.

### Database Migration
Run database setup:
```bash
make setup-db
```

### Docker Production
```bash
docker build -t goapitemplate .
docker run -p 8080:8080 \
  -e DB_TYPE=postgres \
  -e DB_HOST=your-db-host \
  -e DB_PASSWORD=your-password \
  goapitemplate
```

### Kubernetes
Deployment manifests can be added to `deployments/k8s/` directory.

## Testing

```bash
# Run all tests
make test

# Run tests with coverage
go test -v -cover ./...

# Run specific package tests
go test -v ./internal/handlers
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Run `make test` and `make docs`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:
- Create an issue in the repository
- Check the documentation in `*.md` files
- Review the configuration examples in `configs/`