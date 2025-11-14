# Technical Context

This document provides implementation details, code organization, conventions, and practical guidance for contributors to the GitLab Reviewer Roulette Bot.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Key Components](#key-components)
- [Coding Conventions](#coding-conventions)
- [Development Workflow](#development-workflow)
- [Testing Strategy](#testing-strategy)
- [Configuration](#configuration)
- [Database Migrations](#database-migrations)
- [Troubleshooting](#troubleshooting)
- [Quick Reference](#quick-reference)

---

## Getting Started

### Prerequisites

- Go 1.25+
- PostgreSQL 15+ client tools (`psql`)
- Docker & Docker Compose
- Make (optional but recommended)

### Quick Setup

```bash
# 1. Check prerequisites
make check-env

# 2. Setup local Docker environment (fully automated)
make setup-complete

# 3. View logs
make logs

# 4. Test the bot
# Create a merge request in GitLab (http://localhost:8000)
# Comment: /roulette
```

See [README.md](README.md) for detailed installation instructions.

---

## Project Structure

### Directory Layout

```
gitlab-reviewer-roulette/
├── cmd/                          # Application entry points
│   ├── server/main.go            # Main web server
│   ├── migrate/main.go           # Database migration tool
│   └── init/main.go              # User synchronization utility
│
├── internal/                     # Private application code
│   ├── api/                      # HTTP handlers
│   │   ├── health/               # Health check endpoints
│   │   ├── webhook/              # GitLab webhook handling
│   │   └── dashboard/            # Dashboard API (leaderboard, badges)
│   │
│   ├── service/                  # Business logic
│   │   ├── roulette/             # Reviewer selection algorithm
│   │   ├── metrics/              # Metrics recording
│   │   ├── aggregator/           # Daily batch aggregation
│   │   ├── scheduler/            # Cron jobs (notifications, badges)
│   │   ├── badges/               # Badge evaluation logic
│   │   └── leaderboard/          # Leaderboard calculations
│   │
│   ├── repository/               # Data access layer
│   │   ├── user_repository.go
│   │   ├── review_repository.go
│   │   ├── badge_repository.go
│   │   └── metrics_repository.go
│   │
│   ├── models/                   # Domain models
│   │   ├── user.go
│   │   ├── review.go
│   │   ├── badge.go
│   │   └── metrics.go
│   │
│   ├── config/                   # Configuration management
│   ├── gitlab/                   # GitLab API client
│   ├── mattermost/               # Mattermost webhook client
│   ├── i18n/                     # Internationalization (en/fr)
│   ├── cache/                    # Redis cache wrapper
│   └── metrics/                  # Prometheus metrics exporter
│
├── migrations/                   # SQL database migrations
│   ├── 000001_initial_schema.up.sql
│   ├── 000001_initial_schema.down.sql
│   └── ...
│
├── scripts/                      # Automation scripts
│   ├── setup-gitlab.sh           # Unified GitLab setup (--minimal, --realistic)
│   └── README.md
│
├── helm/                         # Kubernetes Helm charts
│   └── reviewer-roulette/
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│
├── config.example.yaml           # Configuration template
├── Makefile                      # Build and development tasks
├── Dockerfile                    # Multi-stage production image
├── docker-compose.yml            # Local development environment
└── go.mod                        # Go module dependencies
```

### Package Organization

**`cmd/`** - Application entry points

- Each subdirectory has a `main.go` with `package main`
- Minimal logic - primarily wiring and initialization
- Examples: server, migrate, init

**`internal/`** - Private application code (not importable by external projects)

- All business logic lives here
- Organized by layer (api, service, repository, models)

**`internal/api/`** - HTTP handlers and request/response DTOs

- Thin layer - delegates to services
- Handles HTTP concerns (parsing, validation, response formatting)
- No business logic

**`internal/service/`** - Business logic and domain operations

- Core algorithms (reviewer selection, badge evaluation)
- Coordinates between repositories, external APIs, cache
- No direct HTTP concerns (receives plain structs, returns errors)

**`internal/repository/`** - Data access layer

- GORM-based database operations
- One repository per aggregate root (User, Review, Badge)
- Hides SQL details from services

**`internal/models/`** - Domain models and DTOs

- GORM models (struct tags for database mapping)
- Validation tags (`binding:"required"`)
- Business logic methods (e.g., `User.IsAvailable()`)

---

## Key Components

### 1. Reviewer Selection (`internal/service/roulette/service.go`)

**Main Function**: `SelectReviewers()`

**Workflow:**

1. Parse MR context (team label, modified files)
2. Fetch CODEOWNERS and parse patterns
3. Build candidate pools:
   - Code owners (from CODEOWNERS matching modified files)
   - Team members (from config matching team label)
   - External reviewers (other teams)
4. Filter by availability (GitLab status + OOO + Redis cache)
5. Calculate scores for each candidate
6. Select 3 unique reviewers (code owner, team, external)

**Key Functions:**

```go
// Main entry point
func (s *Service) SelectReviewers(projectID int, mrIID int) ([]Reviewer, error)

// Scoring algorithm
func (s *Service) calculateScore(user User, ctx SelectionContext) int

// CODEOWNERS parsing
func (s *Service) ParseCodeowners(content string) (map[string][]string, error)

// Availability checking
func (s *Service) IsAvailable(userID int) (bool, error)
```

**Algorithm Detail:**

```go
score := 100                                    // Base score
score -= activeReviews * 10                     // Workload penalty
if hasRecentReview { score -= 5 }               // Recent activity penalty
if hasFileExpertise { score += 2 }              // Expertise bonus
score = max(score, 0)                           // Minimum: 0
```

### 2. Webhook Handling (`internal/api/webhook/handler.go`)

**Main Function**: `HandleGitLabWebhook()`

**Workflow:**

1. Validate webhook signature (`X-Gitlab-Token` header)
2. Parse JSON payload (note events, MR events)
3. Detect `/roulette` command in note body
4. Extract flags (`--force`, `--include`, `--exclude`, `--no-codeowner`)
5. Call Roulette Service for selection
6. Format response message (multilingual)
7. Post comment back to GitLab MR
8. Record metrics (Prometheus counters)

**Security:**

```go
// Signature validation
func validateSignature(r *http.Request, secret string) bool {
    token := r.Header.Get("X-Gitlab-Token")
    return token != "" && token == secret
}
```

### 3. CODEOWNERS Parsing (`internal/gitlab/codeowners.go`)

**Supports**:

- Basic patterns: `/docs/` (directory), `*.md` (extension)
- Glob patterns: `/lib/**/*.go` (recursive), `/src/**/test/*.go`
- Negation: `!*.test.go` (exclude test files)
- Multiple owners: `@user1 @user2 @team`

**Pattern Matching:**

```go
// Returns true if file matches CODEOWNERS pattern
func matchPattern(pattern string, filePath string) bool
```

**Example CODEOWNERS:**

```
# Default owner for everything
* @default-owner

# Backend code
/backend/**/*.go @backend-team @alice

# Frontend code
/frontend/**/*.{ts,tsx} @frontend-team @bob

# Documentation (multiple owners)
*.md @docs-team @charlie

# Tests (exclude from default)
!*.test.go
```

### 4. Metrics Collection (`internal/service/metrics/service.go`)

**Prometheus Metrics:**

```go
// Counters
roulette_triggers_total{team, status}
reviews_completed_total{team, user, role}
reviews_abandoned_total{team}
badge_awarded_total{badge_type, team}

// Gauges
active_reviews{team, user}
available_reviewers{team, role}

// Histograms
review_ttfr_seconds{team}                   // Time to First Review
review_time_to_approval_seconds{team}       // Time to Approval
review_comment_count{team}
review_comment_length{team}

// Summary
reviewer_engagement_score{team, user}
```

**Recording Events:**

```go
// Record roulette trigger
func (s *Service) RecordReviewTriggered(team string)

// Record completion
func (s *Service) RecordReviewCompleted(team string, user string, ttfr time.Duration, approvalTime time.Duration)
```

### 5. Internationalization (`internal/i18n/translator.go`)

**Supported Languages**: English (en), French (fr)

**Usage:**

```go
// Simple translation
title := translator.Get("roulette.title")

// With variables
msg := translator.Get("roulette.selected_reviewers", map[string]interface{}{
    "Count": len(reviewers),
})

// Pluralization
active := translator.GetPlural("roulette.active_reviews", activeCount)
```

**Translation Files**: `internal/i18n/locales/en.yaml`, `fr.yaml`

**Embedded at Compile Time**: Uses `//go:embed` for bundling translations in binary.

### 6. Configuration (`internal/config/config.go`)

**Loading Priority** (highest to lowest):

1. Environment variables
2. Config file (`config.yaml`)
3. Default values (hardcoded)

**Environment Variable Expansion** in config file:

```yaml
database:
  host: ${POSTGRES_HOST:localhost}    # Uses env var or default "localhost"
  port: ${POSTGRES_PORT:5432}
```

**Accessing Config:**

```go
cfg := config.Get()
dbHost := cfg.Database.Host
redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
```

---

## Coding Conventions

### Go Style Guidelines

Follow [Effective Go](https://golang.org/doc/effective_go.html) and project-specific conventions:

**1. Naming**

```go
// Interfaces: -er suffix
type UserRepository interface { ... }

// Structs: PascalCase
type ReviewService struct { ... }

// Functions: Exported = PascalCase, private = camelCase
func SelectReviewers() { ... }
func calculateScore() { ... }

// Files: snake_case.go
user_repository.go
roulette_service.go
```

**2. Error Handling**

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to select reviewers: %w", err)
}

// Log before returning
if err := db.Save(&user).Error; err != nil {
    log.Error().Err(err).Int("user_id", user.ID).Msg("Failed to save user")
    return err
}
```

**3. Logging (Zerolog)**

```go
// Structured logging
log.Info().
    Str("username", user.Username).
    Int("active_reviews", count).
    Msg("Reviewer selected")

// Log levels
log.Debug().Msg("Verbose details")         // Development only
log.Info().Msg("Normal operations")        // Default
log.Warn().Msg("Expected errors")          // Recoverable
log.Error().Msg("Unexpected errors")       // Needs investigation
log.Fatal().Msg("Unrecoverable errors")    // Exit process
```

**4. Function Length**

- Keep functions under ~50 lines
- Extract helper functions for clarity
- Single responsibility per function

**5. Comments**

```go
// Public functions: Godoc format
// SelectReviewers selects 3 optimal reviewers based on availability, workload, and expertise.
// It returns a slice of Reviewer structs or an error if selection fails.
func SelectReviewers() ([]Reviewer, error) { ... }

// Inline comments: Explain "why", not "what"
// Cache availability for 5 minutes to reduce GitLab API load
cacheKey := fmt.Sprintf("user:availability:%d", userID)
```

### Code Quality Tools

**Pre-commit Hooks** (automatically installed via `make pre-commit-install`):

- `gofmt` - Format code
- `go vet` - Lint for common mistakes
- `golangci-lint` - Comprehensive linting (25 linters enabled)
- Fast tests - Run unit tests

**Linters Enabled**:

- Core: errcheck, gosimple, govet, staticcheck, unused
- Format: gofmt, goimports, godot (comments end with period)
- Quality: revive, gocritic, unparam, misspell
- Security: gosec, bodyclose, noctx
- Performance: prealloc, unconvert
- Style: gocyclo (complexity), whitespace

**Running Quality Checks:**

```bash
make check          # Run all checks (fmt, lint, test, security)
make fmt            # Format code
make lint           # Run linters
make test           # Run tests
make security       # Run gosec + govulncheck
```

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types**: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`

**Examples:**

```
feat(roulette): add --no-codeowner flag for skipping code owner selection
fix(metrics): correct TTFR calculation for multi-day reviews
refactor(badges): extract badge criteria evaluation to separate function
test(codeowners): add test cases for glob pattern matching
docs(readme): update installation instructions for Helm deployment
```

---

## Development Workflow

### Local Development

**1. Setup**

```bash
make setup-complete        # Automated full setup (GitLab + PostgreSQL + Redis + App)
```

This command:

- Builds Docker images
- Starts all services (GitLab takes 5-10 min to initialize)
- Waits for GitLab readiness
- Creates admin token and bot user
- Configures webhooks
- Runs database migrations
- Seeds test data (12 users across 5 teams)

**2. Development Cycle**

```bash
make run                # Run the server

# In another terminal
make logs               # Follow all logs
make logs-app           # Follow app logs only
```

**3. Make Changes**

- Edit code
- Tests run automatically (if using pre-commit hooks)
- Restart app: `make restart`

**4. Testing**

```bash
make test               # Run all tests
make test-coverage      # Run with coverage report
make test-verbose       # Run with verbose output
```

**5. Cleanup**

```bash
make down               # Stop all containers
make docker-clean       # Remove volumes and images
```

### Common Development Tasks

**Database Migrations:**

```bash
# Create new migration
make migrate-create NAME=add_user_preferences

# Apply migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check migration status
make migrate-version
```

**Seed Data:**

```bash
make seed               # Add test users and teams
```

**Database Inspection:**

```bash
# Connect to PostgreSQL
docker exec -it reviewer-roulette-postgres psql -U postgres -d reviewer_roulette

# Common queries
SELECT * FROM users;
SELECT * FROM mr_reviews ORDER BY created_at DESC LIMIT 10;
SELECT * FROM user_badges JOIN badges ON badges.id = user_badges.badge_id;
```

**GitLab Access:**

- URL: <http://localhost:8000>
- Username: `root`
- Password: `admin123`

**Application Endpoints:**

- App: <http://localhost:8080>
- Health: <http://localhost:8080/health>
- Metrics: <http://localhost:9090/metrics>

---

## Testing Strategy

### Test Organization

```
internal/
  service/
    roulette/
      service.go              # Implementation
      service_test.go         # Unit tests
      mocks/                  # Generated mocks (mockery)
        mock_repository.go
```

### Unit Tests

**Coverage Goals:**

- Business logic: 80%+ (roulette algorithm, badge evaluation, CODEOWNERS parsing)
- Handlers: 60%+ (happy path + error cases)
- Overall: 70%+

**Current Coverage:**

- CODEOWNERS parsing: 100%
- Availability checking: 100%
- Scoring algorithm: 100%
- Metrics calculations: 100%
- Redis cache: 100%
- i18n: 100%
- Overall: ~13% (focused on critical paths)

**Writing Tests:**

```go
func TestSelectReviewers(t *testing.T) {
    // Arrange
    mockRepo := &mocks.MockUserRepository{}
    mockRepo.On("GetAvailableUsers", mock.Anything).Return(users, nil)

    service := NewService(mockRepo, mockCache, mockGitLab)

    // Act
    reviewers, err := service.SelectReviewers(123, 456)

    // Assert
    assert.NoError(t, err)
    assert.Len(t, reviewers, 3)
    assert.NotEqual(t, reviewers[0].ID, reviewers[1].ID) // Uniqueness
    mockRepo.AssertExpectations(t)
}
```

**Mocking:**

- Use `mockery` for generating mocks: `make mocks`
- Mock interfaces (Repository, Cache, GitLab client)
- Don't mock value objects or simple structs

**Running Tests:**

```bash
make test                    # All tests
make test-coverage           # With coverage report (opens in browser)
make test-verbose            # With verbose output
go test ./internal/service/roulette -v  # Specific package
```

### Integration Tests

Currently minimal. Planned for Phase 7:

- End-to-end webhook flow with real GitLab instance
- Database integration tests (real PostgreSQL, not mocks)
- Redis integration tests

---

## Configuration

### Configuration File Structure

**`config.example.yaml`** (template):

```yaml
server:
  port: 8080
  environment: development
  language: en

gitlab:
  url: ${GITLAB_URL:http://localhost:8000}
  token: ${GITLAB_TOKEN}
  webhook_secret: ${GITLAB_WEBHOOK_SECRET}

database:
  host: ${POSTGRES_HOST:localhost}
  port: ${POSTGRES_PORT:5432}
  database: ${POSTGRES_DB:reviewer_roulette}
  user: ${POSTGRES_USER:postgres}
  password: ${POSTGRES_PASSWORD}

redis:
  host: ${REDIS_HOST:localhost}
  port: ${REDIS_PORT:6379}
  password: ${REDIS_PASSWORD:}
  db: ${REDIS_DB:0}

teams:
  - name: backend
    members: ["alice", "bob"]
    file_patterns: ["**.go", "backend/**"]

  - name: frontend
    members: ["charlie", "dave"]
    file_patterns: ["**.ts", "**.tsx", "frontend/**"]
```

### Environment Variables

**Server:**

- `SERVER_PORT` - HTTP port (default: 8080)
- `SERVER_ENVIRONMENT` - `development` or `production`
- `SERVER_LANGUAGE` - `en` or `fr`

**GitLab:**

- `GITLAB_URL` - GitLab instance URL
- `GITLAB_TOKEN` - GitLab bot token (scope: `api`)
- `GITLAB_WEBHOOK_SECRET` - Webhook signature secret

**Database:**

- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_DB`
- `POSTGRES_USER`, `POSTGRES_PASSWORD`

**Redis:**

- `REDIS_HOST`, `REDIS_PORT`, `REDIS_DB`
- `REDIS_PASSWORD` (optional)

**Mattermost:**

- `MATTERMOST_WEBHOOK_URL` - Webhook URL for notifications
- `MATTERMOST_ENABLED` - Enable/disable notifications (default: `false`)

**Metrics:**

- `METRICS_PORT` - Prometheus metrics port (default: 9090)

### Deployment Configuration

**Local Development:**

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with local values
make run
```

**Docker Compose:**

```yaml
# docker-compose.yml
environment:
  - GITLAB_TOKEN=${GITLAB_TOKEN}
  - POSTGRES_PASSWORD=secure_password
```

**Kubernetes:**

```yaml
# ConfigMap for non-sensitive
apiVersion: v1
kind: ConfigMap
metadata:
  name: reviewer-roulette-config
data:
  GITLAB_URL: "https://gitlab.example.com"
  SERVER_PORT: "8080"

---
# Secret for sensitive
apiVersion: v1
kind: Secret
metadata:
  name: reviewer-roulette-secrets
type: Opaque
data:
  GITLAB_TOKEN: <base64-encoded>
  POSTGRES_PASSWORD: <base64-encoded>
```

---

## Database Migrations

### Migration Files

**Location**: `migrations/`

**Naming**: `{version}_{description}.{up|down}.sql`

**Example**:

```
migrations/
  000001_initial_schema.up.sql
  000001_initial_schema.down.sql
  000002_add_badges.up.sql
  000002_add_badges.down.sql
```

### Creating Migrations

```bash
make migrate-create NAME=add_user_preferences
```

This creates:

- `migrations/000003_add_user_preferences.up.sql`
- `migrations/000003_add_user_preferences.down.sql`

**Write SQL:**

```sql
-- 000003_add_user_preferences.up.sql
ALTER TABLE users ADD COLUMN preferences JSONB DEFAULT '{}';
CREATE INDEX idx_users_preferences ON users USING gin(preferences);

-- 000003_add_user_preferences.down.sql
DROP INDEX IF EXISTS idx_users_preferences;
ALTER TABLE users DROP COLUMN IF EXISTS preferences;
```

### Applying Migrations

```bash
# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check current version
make migrate-version

# Force version (dangerous!)
make migrate-force VERSION=2
```

### Best Practices

1. **Always create both up and down migrations**
2. **Use IF EXISTS/IF NOT EXISTS for idempotency**
3. **Add indexes for foreign keys and frequently queried columns**
4. **Test rollback (down migration) before deploying**
5. **Never modify existing migrations** (create new ones instead)

---

## Troubleshooting

### Common Issues

**1. Docker won't start**

```bash
make docker-down && make docker-up
```

**2. Database connection failed**

```bash
docker compose ps postgres      # Check if PostgreSQL is running
docker compose logs postgres    # Check logs
```

Verify config.yaml has correct credentials:

```yaml
database:
  host: localhost
  port: 5432
  user: postgres
  password: your_password
```

**3. Migrations fail**

```bash
make migrate-down    # Rollback
make migrate-up      # Reapply
```

Check migration SQL syntax and database logs.

**4. Webhook not received**

- Check GitLab webhook logs: Project Settings → Webhooks → Recent Deliveries
- Verify webhook URL is accessible from GitLab container
- Verify `X-Gitlab-Token` matches `GITLAB_WEBHOOK_SECRET`

**5. No reviewers selected**

- Verify team config in `config.yaml` matches team labels
- Check users are synced to database: `SELECT * FROM users;`
- Verify users are marked as available (not OOO)

**6. Redis connection refused**

```bash
docker compose ps redis          # Check if Redis is running
docker compose logs redis        # Check logs
redis-cli -h localhost -p 6379 ping  # Test connection
```

### Debugging

**Enable debug logging:**

```yaml
# config.yaml
logging:
  level: debug
```

**Check application status:**

```bash
curl http://localhost:8080/health
curl http://localhost:9090/metrics | grep roulette
docker compose ps
docker compose logs -f app
```

**Inspect database:**

```bash
docker exec -it reviewer-roulette-postgres psql -U postgres -d reviewer_roulette

# Useful queries
\dt                                                      # List tables
SELECT * FROM users;                                     # All users
SELECT * FROM mr_reviews ORDER BY created_at DESC;       # Recent reviews
SELECT COUNT(*) FROM reviewer_assignments;               # Total assignments
```

**Inspect Redis cache:**

```bash
docker exec -it reviewer-roulette-redis redis-cli

> KEYS user:availability:*         # List all availability keys
> GET user:availability:123          # Get specific user availability
> TTL user:availability:123          # Check TTL (seconds remaining)
```

---

## Quick Reference

### Important Files

```
cmd/server/main.go                       # Application entry point
internal/service/roulette/service.go     # Reviewer selection algorithm
internal/api/webhook/handler.go          # Webhook request handling
internal/gitlab/client.go                # GitLab API client
internal/gitlab/codeowners.go            # CODEOWNERS parser
config.yaml                              # Configuration
migrations/                              # Database schema versions
```

### Key Functions

**Reviewer Selection:**

- `SelectReviewers()` - Main entry point for selection algorithm
- `calculateScore()` - Weighted scoring function
- `IsAvailable()` - Availability checking (status + OOO + cache)

**CODEOWNERS:**

- `ParseCodeowners()` - Parse CODEOWNERS file into map
- `matchPattern()` - Check if file matches CODEOWNERS pattern

**Webhooks:**

- `HandleGitLabWebhook()` - Main webhook handler
- `validateSignature()` - Webhook signature validation
- `parseRouletteCommand()` - Extract flags from `/roulette` command

**Metrics:**

- `RecordReviewTriggered()` - Record `/roulette` invocation
- `RecordReviewCompleted()` - Record review completion with timings
- `RecordBadgeAwarded()` - Record badge award event

### Useful Commands

```bash
# Development
make run                # Run the server
make logs               # Follow all logs
make restart            # Restart app container

# Testing
make test               # Run all tests
make test-coverage      # Coverage report
make check              # Run all quality checks (fmt, lint, test, security)

# Database
make migrate-up         # Apply migrations
make migrate-down       # Rollback last migration
make seed               # Add test data

# Docker
make docker-up          # Start all containers
make docker-down        # Stop all containers
make docker-clean       # Remove volumes and images

# Quality
make fmt                # Format code
make lint               # Run linters
make security           # Security scanning
```

### Environment Variables Cheat Sheet

```bash
# GitLab
export GITLAB_URL="http://localhost:8000"
export GITLAB_TOKEN="glpat-xxxxxxxxxxxxxxxxxxxx"
export GITLAB_WEBHOOK_SECRET="your-secret"

# Database
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_DB="reviewer_roulette"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="secure_password"

# Redis
export REDIS_HOST="localhost"
export REDIS_PORT="6379"
export REDIS_PASSWORD=""
export REDIS_DB="0"

# Mattermost
export MATTERMOST_WEBHOOK_URL="https://mattermost.example.com/hooks/xxx"
export MATTERMOST_ENABLED="true"
```

### Configuration Template

```yaml
# Minimal config.yaml for local development
server:
  port: 8080
  environment: development
  language: en

gitlab:
  url: http://localhost:8000
  token: ${GITLAB_TOKEN}
  webhook_secret: ${GITLAB_WEBHOOK_SECRET}

database:
  host: localhost
  port: 5432
  database: reviewer_roulette
  user: postgres
  password: ${POSTGRES_PASSWORD}

redis:
  host: localhost
  port: 6379

teams:
  - name: backend
    members: ["alice", "bob"]
    file_patterns: ["**.go"]

  - name: frontend
    members: ["charlie"]
    file_patterns: ["**.ts", "**.tsx"]
```

---

## Additional Resources

**Project Documentation:**

- [README.md](README.md) - Quick start and overview
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [ARCHITECTURE.md](ARCHITECTURE.md) - System design and architecture
- [METRICS.md](METRICS.md) - Metrics and observability

**External Resources:**

- [Effective Go](https://golang.org/doc/effective_go.html)
- [GitLab API v4 Documentation](https://docs.gitlab.com/ee/api/)
- [GitLab Webhooks Guide](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html)
- [CODEOWNERS Syntax](https://docs.gitlab.com/ee/user/project/code_owners.html)
- [Gin Framework Docs](https://gin-gonic.com/docs/)
- [GORM Guide](https://gorm.io/docs/)
- [Zerolog Documentation](https://github.com/rs/zerolog)
