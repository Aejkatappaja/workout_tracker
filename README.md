# Workout Tracker API

A robust, production-ready REST API built in Go for tracking workout sessions, exercises, and user fitness data. This project demonstrates modern Go development practices, clean architecture principles, and enterprise-level application design.

## Project Overview

This is a comprehensive workout tracking system that allows users to create, manage, and track their fitness routines. The application features user authentication, workout management, exercise tracking, and a scalable database architecture.

## Architecture & Design Patterns

### Clean Architecture

The project follows clean architecture principles with clear separation of concerns:

- **API Layer**: HTTP handlers and request/response management
- **Business Logic**: Core application logic and domain models
- **Data Layer**: Database operations and data persistence
- **Infrastructure**: Database connections, middleware, and utilities

### Project Structure

```
├── cmd/                    # Application entry points
├── internal/               # Private application code
│   ├── api/               # HTTP handlers and API logic
│   ├── app/               # Application configuration and setup
│   ├── middleware/        # Authentication and request processing
│   ├── store/             # Data access layer and repositories
│   ├── tokens/            # JWT token management
│   ├── utils/             # Shared utilities and helpers
│   └── routes/            # Route definitions and middleware setup
├── migrations/             # Database schema migrations
├── database/               # Database configuration and setup
└── tests/                  # Test files and test utilities
```

## Key Features

### User Management

- User registration and authentication
- Secure password hashing using bcrypt
- JWT-based token authentication
- User profile management with bio information

### Workout System

- Create, read, update, and delete workouts
- Exercise entry management with sets, reps, and weights
- Duration tracking for both workouts and individual exercises
- Calorie burn estimation
- Exercise ordering and progression tracking

### Security Features

- Middleware-based authentication
- Role-based access control
- Secure token management with expiration
- Password hashing and validation

## Technical Implementation

### Technology Stack

- **Language**: Go 1.24.5
- **Web Framework**: Chi router for HTTP routing
- **Database**: PostgreSQL with pgx driver
- **Migrations**: Goose for database schema management
- **Testing**: Testify for assertions and test utilities
- **Containerization**: Docker and Docker Compose

### Database Design

The application uses a relational database with the following key tables:

- **users**: User accounts and profiles
- **workouts**: Workout sessions and metadata
- **workout_entries**: Individual exercises within workouts
- **tokens**: Authentication tokens with expiration

### API Endpoints

#### Public Endpoints

- `GET /health` - Health check endpoint
- `POST /users` - User registration
- `POST /tokens/authentication` - User authentication

#### Protected Endpoints (Require Authentication)

- `GET /workouts/{id}` - Retrieve workout by ID
- `POST /workouts` - Create new workout
- `PUT /workouts/{id}` - Update existing workout
- `DELETE /workouts/{id}` - Delete workout

### Middleware Architecture

- **Authentication Middleware**: Validates JWT tokens and sets user context
- **Authorization Middleware**: Ensures authenticated access to protected routes
- **Request Processing**: Handles CORS, logging, and error handling

## Development & Testing

### Setup Requirements

- Go 1.24.5 or later
- PostgreSQL 12.4 or later
- Docker and Docker Compose

### Local Development

```bash
# Clone the repository
git clone <repository-url>
cd workout-tracker-api

# Start the database
docker-compose up -d

# Run migrations
go run migrations/fs.go

# Start the application
go run main.go
```

### Testing

The project includes comprehensive unit tests for the data layer:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Database Migrations

Database schema changes are managed through Goose migrations:

```bash
# Apply migrations
goose up

# Rollback migrations
goose down
```

## Code Quality & Best Practices

### Go Conventions

- Follows Go coding standards and idioms
- Proper error handling and logging
- Interface-based design for testability
- Consistent naming conventions

### Security Considerations

- Input validation and sanitization
- SQL injection prevention through parameterized queries
- Secure password storage with bcrypt
- JWT token validation and expiration

### Performance Optimizations

- Database connection pooling
- Efficient SQL queries with proper indexing
- Transaction management for data consistency
- Optimized HTTP response handling

## Scalability & Production Readiness

### Database Design

- Normalized schema for data integrity
- Proper foreign key relationships
- Indexing strategy for query performance
- Transaction support for data consistency

### Application Architecture

- Stateless design for horizontal scaling
- Middleware-based request processing
- Configurable timeouts and connection limits
- Health check endpoints for monitoring

### Deployment Considerations

- Docker containerization
- Environment-based configuration
- Database migration management
- Graceful shutdown handling

## Future Enhancements

The application is designed with extensibility in mind:

- User roles and permissions
- Workout templates and sharing
- Progress tracking and analytics
- Mobile API support
- Real-time notifications
- Integration with fitness devices

## Contributing

This project demonstrates enterprise-level Go development practices suitable for:

- Backend developers
- API developers
- DevOps engineers
- Full-stack developers with Go experience

---

_This project showcases modern Go development practices, clean architecture principles, and production-ready application design. It serves as an excellent example of building scalable, maintainable APIs in Go._

