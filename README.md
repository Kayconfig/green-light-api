# Green Light API

A production-ready REST API for managing movie information built with Go.

## Features

- **CRUD Operations** - Create, read, update, and delete movies
- **User Management** - User registration, activation, and authentication
- **Role-Based Permissions** - Fine-grained access control with permissions
- **Rate Limiting** - Configurable request rate limiting
- **JWT Authentication** - Secure token-based authentication
- **Database Migrations** - Versioned database schema management
- **Email Support** - SMTP integration for user activation emails
- **Metrics** - Exposed application metrics via expvar
- **CORS** - Configurable cross-origin resource sharing
- **Graceful Shutdown** - Clean server shutdown handling

## Tech Stack

- **Go** - Backend language
- **PostgreSQL** - Database
- **Chi Router** - HTTP routing
- **goose** - Database migrations
- **go-mail** - Email sending

## Prerequisites

- Go 1.25.3 or later
- PostgreSQL
- SMTP server (for email functionality)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/kayconfig/green-light-api.git
cd green-light-api
```

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables by creating a `.env` file:
```env
GOOSE_DBSTRING=postgres://user:password@localhost:5432/greenlight?sslmode=disable
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your-email@example.com
SMTP_PASSWORD=your-password
SMTP_SENDER=Green Light <no-reply@example.com>
```

4. Run the application:
```bash
go run ./cmd/api
```

## API Endpoints

### Health Check
- `GET /v1/healthcheck` - Check API status

### Movies (Requires Authentication)
- `GET /v1/movies` - List all movies with pagination, filtering, and sorting
- `GET /v1/movies/{id}` - Get a specific movie
- `POST /v1/movies` - Create a new movie (requires `movies:write` permission)
- `PATCH /v1/movies/{id}` - Update a movie (requires `movies:write` permission)
- `DELETE /v1/movies/{id}` - Delete a movie (requires `movies:write` permission)

### Users
- `POST /v1/users` - Register a new user
- `POST /v1/users/verification` - Resend activation token
- `PUT /v1/users/activated` - Activate user account

### Authentication
- `POST /v1/tokens/authentication` - Authenticate and get access token

### Metrics
- `GET /v1/metrics` - Application metrics (requires authentication)

## Configuration

The API can be configured via command-line flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 4000 | API server port |
| `-env` | development | Environment (development/staging/production) |
| `-db-dsn` | - | PostgreSQL DSN |
| `-db-max-open-conns` | 25 | Max open database connections |
| `-db-max-idle-conns` | 25 | Max idle database connections |
| `-db-max-idle-time` | 15m | Max connection idle time |
| `-limiter-rps` | 2 | Rate limiter requests per second |
| `-limiter-burst` | 4 | Rate limiter burst size |
| `-limiter-enabled` | true | Enable rate limiting |
| `-cors-trusted-origins` | - | Trusted CORS origins |

## Project Structure

```
.
├── cmd/api/            # Application entry point and handlers
├── internal/
│   ├── common/         # Shared utilities and responses
│   ├── data/           # Database models and queries
│   ├── mailer/         # Email sending functionality
│   └── validator/      # Input validation
├── migrations/         # Database migration files
└── bin/                # Compiled binaries
```

## Running Migrations

Migrations run automatically in development mode. To run manually:

```bash
go run ./cmd/api -env=development
```

## Example Usage

### Register a User
```bash
curl -X POST http://localhost:4000/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com","password":"securepassword123"}'
```

### Authenticate
```bash
curl -X POST http://localhost:4000/v1/tokens/authentication \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"securepassword123"}'
```

### Create a Movie
```bash
curl -X POST http://localhost:4000/v1/movies \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title":"Inception","year":2010,"runtime":148,"genres":["sci-fi","action"]}'
```

## Author

**Kayode Odole**
- GitHub: [@kayconfig](https://github.com/kayconfig)

## License

MIT
