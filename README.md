# Green Light API

A production-ready REST API for managing movie information, built with Go.

## Tech Stack

- **Go 1.25.3** — Language
- **PostgreSQL** — Database
- **Chi Router** — HTTP routing
- **goose** — Database migrations
- **go-mail** — Email sending
- **Swagger** — API docs

## Prerequisites

- Go 1.25.3+
- PostgreSQL
- SMTP server

## Setup

1. Clone and install dependencies:
```bash
git clone https://github.com/kayconfig/green-light-api.git
cd green-light-api
go mod download
```

2. Create a `.env` file:
```env
GOOSE_DBSTRING=postgres://user:password@localhost:5432/greenlight?sslmode=disable
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your-email@example.com
SMTP_PASSWORD=your-password
SMTP_SENDER=Green Light <no-reply@example.com>
api-url=http://localhost:4000
```

3. Run:
```bash
make run/api
```

Migrations run automatically in development mode.

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/v1/healthcheck` | — | API status |
| `GET` | `/swagger/*` | — | Swagger UI |
| `POST` | `/v1/users` | — | Register user |
| `POST` | `/v1/users/verification` | — | Resend activation token |
| `PUT` | `/v1/users/activated` | — | Activate account |
| `PUT` | `/v1/users/password` | — | Update password |
| `POST` | `/v1/tokens/authentication` | — | Get access token |
| `POST` | `/v1/tokens/password-reset` | — | Request password reset |
| `GET` | `/v1/movies` | `movies:read` | List movies |
| `GET` | `/v1/movies/{id}` | `movies:read` | Get movie |
| `POST` | `/v1/movies` | `movies:write` | Create movie |
| `PATCH` | `/v1/movies/{id}` | `movies:write` | Update movie |
| `DELETE` | `/v1/movies/{id}` | `movies:write` | Delete movie |
| `GET` | `/v1/metrics` | — | App metrics |

Movie endpoints require an activated user with the appropriate permission. Pass the token as `Authorization: Bearer <token>`.

## Configuration

Flags (defaults read from `.env`):

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `4000` | Server port |
| `-env` | `development` | Environment |
| `-db-dsn` | `$GOOSE_DBSTRING` | PostgreSQL DSN |
| `-db-max-open-conns` | `25` | Max open DB connections |
| `-db-max-idle-conns` | `25` | Max idle DB connections |
| `-db-max-idle-time` | `15m` | Max connection idle time |
| `-limiter-rps` | `2` | Rate limit (req/sec) |
| `-limiter-burst` | `4` | Rate limit burst |
| `-limiter-enabled` | `true` | Enable rate limiting |
| `-cors-trusted-origins` | — | Space-separated trusted origins |
| `-api-url` | `$api-url` | Base URL (used for Swagger) |

## Project Structure

```
.
├── cmd/api/        # Entry point, handlers, routes
├── internal/
│   ├── common/     # Shared utilities
│   ├── data/       # DB models and queries
│   ├── mailer/     # Email sending
│   └── validator/  # Input validation
├── migrations/     # SQL migration files
└── bin/            # Compiled binaries
```

## Makefile Commands

```bash
make run/api            # Run the API
make build/api          # Build binary to ./bin/api
make audit              # Vet, lint, and test
make tidy               # Tidy and format Go files
make db/migration/new   # Create a new migration (name=<name>)
make db/migration/up    # Run pending migrations
```

## Author

**Kayode Odole** — [@kayconfig](https://github.com/kayconfig)

## License

MIT
