# SSO — Single Sign-On (gRPC Auth Service)

A gRPC-based authentication service that provides user registration, login, and admin checks. It uses JWT for tokens, SQLite for storage, and is designed to be consumed by other services (e.g. **URL Shortener**) as a shared auth backend.

## Features

- **User Registration**: Create new users with email and password (bcrypt hashing)
- **Login**: Authenticate by email and password; receive a JWT scoped to an application
- **IsAdmin**: Check whether a user has admin privileges for authorization
- **Multi-app support**: Multiple applications (apps) with separate secrets; tokens are bound to an app ID
- **SQLite storage**: Lightweight file-based database with migrations
- **Structured logging**: Comprehensive logging with different formats for different environments (pretty local, JSON dev/prod)
- **gRPC API**: Clean gRPC API for service-to-service auth (used by URL Shortener and others)

## Requirements

- Go 1.25.0 or higher
- SQLite3 (CGO required for the Go SQLite driver; standard Go toolchain is sufficient for building)

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd sso
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o grpc-auth ./cmd/sso
```

## Configuration

The application uses YAML configuration files. Configuration path can be set via the `-config` command-line flag or the `CONFIG_PATH` environment variable.

### Configuration Files

- **`config/local.yaml`**: Local development configuration

### Configuration Structure

```yaml
env: "local"  # Environment: local, dev, or prod
storage_path: "./storage/sso.db"  # Path to SQLite database
token_ttl: 30m  # JWT token lifetime
grpc:
  port: 8084  # gRPC server listen port
  timeout: 5s  # gRPC request timeout
```

### Environment Variables

- `CONFIG_PATH`: Path to configuration file (used if `-config` is not provided)

## Usage

### Apply Migrations

Before first run, create the storage directory and apply migrations:

```bash
mkdir -p storage
task migrate:up
```

Or manually:
```bash
go run ./cmd/migrator --storage-path=./storage/sso.db --migrations-path=./migrations up
```

### Start the Server

```bash
./grpc-auth --config=./config/local.yaml
```

Or with Task:
```bash
task run
```

Or with custom config via environment:
```bash
CONFIG_PATH=config/prod.yaml ./grpc-auth
```

The gRPC server listens on the port specified in config (default `8084`).

### Taskfile Commands

| Task | Description |
|------|-------------|
| `task run` | Run the SSO application with local config |
| `task migrate:up` | Apply database migrations |
| `task migrate:down` | Rollback last migration |

## API (gRPC)

The service implements the Auth service from `github.com/bolatl/protos` (SSO package). Clients (e.g. URL Shortener) connect to the gRPC server address (host + port from config) and use the same protobuf definitions.

### 1. Register

**RPC:** `Register`

Create a new user with email and password.

**Request:**
- `email` (string): User email (required)
- `password` (string): User password (required)

**Response:**
- `user_id` (int64): ID of the created user

**Errors:** `AlreadyExists` if user with that email already exists; `InvalidArgument` if email or password is empty.

### 2. Login

**RPC:** `Login`

Authenticate with email and password for a specific application; returns a JWT.

**Request:**
- `email` (string): User email (required)
- `password` (string): User password (required)
- `app_id` (int32): Application ID (required; must be a registered app)

**Response:**
- `token` (string): JWT token for the user and app

**Errors:** `InvalidArgument` if credentials are wrong or app_id is missing; `NotFound` if app not found.

### 3. IsAdmin

**RPC:** `IsAdmin`

Check whether a user has admin privileges.

**Request:**
- `user_id` (int64): User ID (required)

**Response:**
- `is_admin` (bool): Whether the user is an admin

**Errors:** `InvalidArgument` if user_id is empty; `NotFound` if user not found.

## Project Structure

```
sso/
├── cmd/
│   ├── sso/                    # Application entry point
│   │   └── main.go
│   └── migrator/               # Database migrations runner
│       └── main.go
├── internal/
│   ├── app/                    # Application wiring (storage, auth, gRPC server)
│   │   ├── app.go
│   │   └── grpc/
│   ├── config/                 # Configuration management (cleanenv)
│   ├── domain/
│   │   └── models/             # User, App domain models
│   ├── grpc/
│   │   └── auth/               # gRPC Auth server implementation
│   ├── lib/
│   │   ├── jwt/                # JWT token generation/validation
│   │   └── logger/             # Structured logging (slog, pretty/JSON)
│   ├── services/
│   │   └── auth/               # Auth business logic (Register, Login, IsAdmin)
│   └── storage/                # Storage layer
│       ├── storage.go          # Storage interface
│       └── sqlite/             # SQLite implementation
├── config/                     # Configuration files
│   └── local.yaml
├── migrations/                 # SQL migrations (golang-migrate)
├── tests/                      # Integration tests (suite + auth tests)
│   ├── suite/                  # Test suite (starts server, test app)
│   └── auth_register_login_test.go
├── deployment/                 # Deployment files
│   └── grpc-auth.service      # Systemd service file
└── .github/workflows/          # CI/CD workflows
    └── deploy.yaml
```

## Testing

### Run Unit Tests

```bash
go test ./...
```

### Run Integration Tests

```bash
go test ./tests/...
```

Integration tests in `tests/` use a test suite that starts the gRPC server with a test database and a test app (ID and secret), then exercise Register, Login, and JWT validation. No need to run the server manually; the suite manages lifecycle.

## Logging

The application uses structured logging (slog) with different formats based on the environment:

- **Local**: Pretty-printed colored logs with debug level
- **Dev**: JSON format with debug level
- **Prod**: JSON format with info level

## Deployment

### Systemd Service

The project includes a systemd service file for deployment. See `deployment/grpc-auth.service` for configuration. Update `WorkingDirectory`, `ExecStart` path, and config path (e.g. `config/prod.yaml`) for your server.

### GitHub Actions

The project includes a GitHub Actions workflow for automated deployment. Configure the following in GitHub:

- **Secrets:** `DEPLOY_SSH_KEY` — Private SSH key for server access
- **Workflow:** Update the `HOST` environment variable in `.github/workflows/deploy.yaml` with your server IP. Optionally set `DEPLOY_DIRECTORY` and `CONFIG_PATH` to match your server layout.

The workflow builds the app and migrator, syncs files to the VM, runs migrations, and restarts the systemd service. Ensure a production config file exists on the server at the path specified by `CONFIG_PATH`.

