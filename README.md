# Go API Template

A modern, production-ready Go API service template featuring multiple database engine support, caching, event management, and comprehensive tooling.

## Features

- **Multi-Database Support**: PostgreSQL, MySQL, SQLite with automatic configuration
- **Caching Layer**: Redis and Memcache support with configurable TTL
- **Event System**: Internal workflow/event manager for decoupled architecture  
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

### User Management
- `POST /api/v1/users` - Create user
- `GET /api/v1/users` - List users
- `GET /api/v1/users/:id` - Get user by ID
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user

### Authentication
- `POST /api/v1/auth/login` - User login

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
│   └── events/          # Event/workflow manager
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

## Event System

The template includes a built-in event management system for decoupled architecture:

- **Event Publisher**: Publish events throughout the application
- **Event Subscribers**: Register handlers for specific event types
- **Workflow Manager**: Define and execute multi-step workflows
- **Async Processing**: Support for asynchronous event handling

Example usage:
```go
// Publish an event
eventManager.Publish(ctx, events.Event{
    Type: "user.created",
    Data: map[string]interface{}{"user_id": 123},
})

// Subscribe to events
eventManager.Subscribe("user.created", func(ctx context.Context, event Event) error {
    // Handle user creation
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
- Check the documentation at `/docs/` endpoint
- Review the configuration examples in `configs/`