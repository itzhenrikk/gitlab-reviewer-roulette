# GitLab Reviewer Roulette Bot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/ci.yml/badge.svg)](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/ci.yml)
[![Security](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/security.yml/badge.svg)](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aimd54/gitlab-reviewer-roulette)](https://goreportcard.com/report/github.com/aimd54/gitlab-reviewer-roulette)

An intelligent code review assignment system for GitLab that automatically
selects reviewers based on availability, workload, expertise, and team
distribution. Includes gamification features to motivate teams and comprehensive
metrics for continuous improvement.

## Features

- 🎲 **Smart Reviewer Selection**: Automatically assigns 3 reviewers (code
  owner, team member, external) via `/roulette` command
- 📊 **Intelligent Weighting**: Considers current workload, recent activity, and
  expertise
- 🚦 **Availability Management**: Checks GitLab status and OOO entries
- 🌐 **Multilingual Support**: Bot responses in English and French
  (configurable)
- 📈 **Comprehensive Metrics**: Track TTFR, Time to Approval, Review
  Thoroughness
- 🏆 **Gamification**: Badges, leaderboards, and personal statistics with REST API
- 💬 **Daily Notifications**: Mattermost reminders for pending reviews
- 🎖️ **Badge System**: Automatic badge evaluation with 4 default badges (Speed Demon, Thorough Reviewer, Team Player, Mentor)
- 📊 **Leaderboards**: Global and team rankings with multiple criteria
- 📉 **Prometheus Integration**: Export metrics to Prometheus/Grafana

## Requirements

- Go 1.25+
- PostgreSQL 15+ client tools (`psql`) - for database operations and seeding
- Docker & Docker Compose (for development)
- GitLab 16.11+ (self-hosted)
- Make (optional but recommended)

## Installation

### Prerequisites

Install PostgreSQL client tools:

```bash
# macOS
brew install postgresql@15

# Ubuntu/Debian
sudo apt-get install postgresql-client-15

# Arch Linux
sudo pacman -S postgresql-libs

# Fedora/RHEL
sudo dnf install postgresql
```

Verify installation:

```bash
psql --version  # Should show PostgreSQL 15.x or higher
```

## Quick Start (Development)

**For development and testing only.** For production, see [Production Deployment](#production-deployment).

**Recommended:** Run everything in Docker for the complete local development experience.

```bash
# 1. Check prerequisites
make check-env

# 2. Setup local Docker environment (automated)
make setup-complete
```

This single command will:

- Build the Docker image
- Start all services (PostgreSQL, Redis, GitLab, App)
- Wait for GitLab to be ready (5-10 minutes)
- Configure GitLab with test users, projects, and webhooks
- Run database migrations and initialize data

**Access:**

- **App**: <http://localhost:8080>
- **GitLab**: <http://localhost:8000> (login: root/admin123)
- **Health**: <http://localhost:8080/health>
- **Metrics**: <http://localhost:9090/metrics>

**Test the bot:** Create a merge request in GitLab and comment `/roulette`.

**Useful commands:**

```bash
make logs      # Follow all logs
make logs-app  # Follow app logs only
make restart   # Restart app container
make down      # Stop everything
make up        # Start everything (after setup-complete)
```

<details>
<summary><b>Alternative: Binary Development (Advanced)</b></summary>

For Go development without running the app in Docker:

```bash
# Build and test
make build
make test

# Start minimal services (postgres + redis only)
docker compose up -d

# Run app binary on host
cp config.example.yaml config.yaml
# Edit config.yaml with GitLab URL and credentials
make run
```

Note: Database operations (`make migrate`, `make seed`) auto-detect whether app
is running in Docker and adjust accordingly.

</details>

---

## Configuration

The application supports two configuration methods following 12-factor app
principles:

1. **Config file** (`config.yaml`) - For local development
2. **Environment variables** - For production/Docker/Kubernetes (takes
   precedence)

### Environment Variables (Production/Docker)

All configuration values can be overridden via environment variables:

**GitLab Configuration:**

- `GITLAB_URL` - GitLab instance URL (e.g., `https://gitlab.example.com`)
- `GITLAB_TOKEN` or `GITLAB_BOT_TOKEN` - Personal access token with `api` scope
- `GITLAB_BOT_USERNAME` - Bot username (default: `reviewer-roulette-bot`)
- `GITLAB_WEBHOOK_SECRET` - Secret for webhook validation

**Database Configuration:**

- `POSTGRES_HOST` - PostgreSQL host (default: `localhost`)
- `POSTGRES_PORT` - PostgreSQL port (default: `5432`)
- `POSTGRES_DB` - Database name (default: `reviewer_roulette`)
- `POSTGRES_USER` - Database user (default: `postgres`)
- `POSTGRES_PASSWORD` - Database password
- `POSTGRES_SSL_MODE` - SSL mode (default: `disable`)
- `REDIS_HOST` - Redis host (default: `localhost`)
- `REDIS_PORT` - Redis port (default: `6379`)
- `REDIS_PASSWORD` - Redis password (optional)
- `REDIS_DB` - Redis database number (default: `0`)

**Server Configuration:**

- `SERVER_PORT` - HTTP port (default: `8080`)
- `SERVER_ENVIRONMENT` - Environment name (default: `development`)
- `SERVER_LANGUAGE` - Bot response language: `en` or `fr` (default: `en`)

**Logging Configuration:**

- `LOG_LEVEL` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `LOG_FORMAT` - Format: `json` or `text` (default: `json`)
- `LOG_OUTPUT` - Output: `stdout` or file path (default: `stdout`)

**Mattermost Configuration:**

- `MATTERMOST_WEBHOOK_URL` - Incoming webhook URL (optional)
- `MATTERMOST_CHANNEL` - Channel for notifications (default: `#reviews`)
- `MATTERMOST_ENABLED` - Enable notifications: `true` or `false` (default:
  `false`)

### Teams Configuration

Edit the `teams` section in `config.yaml`:

```yaml
teams:
  - name: team-frontend
    members:
      - username: alice
        role: dev
      - username: bob
        role: dev
```

### Customizing Configuration

The `config.yaml` file is required and contains team structure, badge thresholds, and other settings. Sensitive values (tokens, passwords) should be provided via environment variables.

**Key customization areas:**

1. **Team Structure** - Define teams, members, and roles matching your GitLab usernames
2. **Badge Thresholds** - Adjust criteria based on team size (small: 10 reviews/month, large: 30+)
3. **Timezone & Language** - Set scheduler timezone and bot language (en/fr)
4. **File Expertise** - Configure file patterns for dev/ops role matching
5. **GitLab URL** - Your GitLab instance URL (GitLab.com or self-hosted)

**Complete template:** See [config.example.yaml](./config.example.yaml) for all available options with inline documentation.

**Production tip:** Use environment variables for sensitive values (tokens, passwords) instead of storing them in config.yaml.

---

## Production Deployment

### Kubernetes with Helm (Recommended)

The **recommended** production deployment method using Helm charts:

```bash
# Install with Helm
helm install reviewer-roulette ./helm/reviewer-roulette \
  -f helm/reviewer-roulette/values-production.yaml \
  --set config.gitlab.url=https://gitlab.example.com \
  --set config.gitlab.token=$GITLAB_TOKEN \
  --set config.gitlab.webhookSecret=$WEBHOOK_SECRET \
  --set config.postgres.host=your-postgres-host \
  --set config.postgres.password=$DB_PASSWORD \
  --set config.redis.host=your-redis-host \
  --namespace reviewer-roulette \
  --create-namespace

# Verify deployment
kubectl get pods -n reviewer-roulette

# Initialize users
kubectl exec -n reviewer-roulette -it deployment/reviewer-roulette -- \
  /app/init --config /app/config/config.yaml
```

**Features:**

- ✅ High availability (2+ replicas with anti-affinity)
- ✅ Horizontal Pod Autoscaler (HPA)
- ✅ PodDisruptionBudget for resilience
- ✅ Ingress with TLS support
- ✅ Prometheus ServiceMonitor
- ✅ Init container for automatic migrations
- ✅ ConfigMap and Secret management
- ✅ Production-ready resource limits

See [helm/reviewer-roulette/README.md](helm/reviewer-roulette/README.md) for complete Helm documentation.

### Docker Image (Pre-built - Recommended)

The application is available as a pre-built multi-architecture Docker image on GitHub Container Registry:

```bash
# Pull the latest version
docker pull ghcr.io/aimd54/gitlab-reviewer-roulette:latest

# Pull a specific version
docker pull ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0

# Quick start (requires config.yaml)
docker run -d \
  --name reviewer-roulette \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0
```

**Available tags:**

- `latest` - Latest stable release
- `v1.8.0`, `1.8`, `1` - Semantic version tags
- **Multi-arch**: `linux/amd64`, `linux/arm64` (automatically detected)

### Docker (Building from Source)

For single-host deployments or custom builds:

```bash
# Option 1: Pull pre-built image (recommended)
docker pull ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0

# Option 2: Build from source
git clone https://github.com/aimd54/gitlab-reviewer-roulette.git
cd gitlab-reviewer-roulette
docker build -t reviewer-roulette:v1.8.0 .

# Run migrations (using pre-built or custom image)
docker run --rm \
  --network host \
  -v $(pwd)/config.yaml:/app/config.yaml \
  ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0 \
  /app/migrate up

# Run application
docker run -d \
  --name reviewer-roulette \
  --network host \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -e GITLAB_TOKEN=$GITLAB_TOKEN \
  -e GITLAB_WEBHOOK_SECRET=$WEBHOOK_SECRET \
  -e POSTGRES_PASSWORD=$DB_PASSWORD \
  ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0
```

### Docker Compose (Development/Small Deployments)

For development or small-scale deployments:

```bash
# Start all services (PostgreSQL, Redis, app)
make start

# Run migrations
make docker-migrate

# Initialize users from GitLab
make docker-init

# View logs
make logs-app
```

## Usage

### GitLab Webhook Setup

1. Go to your GitLab project/group → Settings → Webhooks
2. URL: `https://your-server.com/webhook/gitlab`
3. Secret Token: (use value from `GITLAB_WEBHOOK_SECRET`)
4. Trigger events:
   - ✅ Comments
   - ✅ Merge request events
5. Enable SSL verification
6. Click "Add webhook"

### Trigger Reviewer Selection

In any Merge Request, post a comment:

```shell
/roulette
```

The bot will respond with:

```md
🎲 Reviewer Roulette Results:
• Code Owner: @alice (3 active reviews)
• Team Member: @bob (1 active review)
• External Reviewer: @charlie from Team-Platform (2 active reviews)
```

### Command Variations

```bash
/roulette                              # Standard selection
/roulette --force                      # Override recent review penalties
/roulette --include @user1 @user2      # Force include specific users
/roulette --exclude @user3             # Exclude specific users
/roulette --no-codeowner               # Skip codeowner selection
```

## Checking Your Environment

Before starting, you can verify your environment has all prerequisites:

```bash
make check-env  # Checks Go, Docker, psql, config.yaml
make status     # Shows status of all running services
```

## Development

### Project Structure

```md
.
├── cmd/
│   ├── server/         # Main API server
│   └── migrate/        # Database migration tool
├── internal/
│   ├── api/            # HTTP handlers
│   ├── service/        # Business logic
│   ├── repository/     # Data access layer
│   ├── models/         # Domain models
│   ├── config/         # Configuration
│   ├── gitlab/         # GitLab client wrapper
│   ├── mattermost/     # Mattermost client
│   ├── i18n/           # Internationalization
│   └── cache/          # Redis wrapper
├── migrations/         # SQL migrations
├── scripts/            # Helper scripts
└── docs/               # Documentation
```

### Make Commands

**Setup:**

```bash
make check-env       # Check prerequisites (Go, Docker, psql, etc.)
make setup-gitlab    # Setup GitLab only (minimal - creates bot user and token)
make setup-complete  # Complete automated setup (GitLab + test data + app)
```

**Running Services:**

```bash
make start    # Start Docker stack (postgres, redis, app)
make down     # Stop all services
make restart  # Restart app container
make logs     # Follow all logs
make logs-app # Follow app logs only
```

**GitLab Management:**

```bash
make gitlab-up     # Start GitLab infrastructure
make gitlab-down   # Stop GitLab
make gitlab-seed   # Populate GitLab with test data
make gitlab-logs   # View GitLab logs
```

**Database:**

```bash
make migrate            # Run migrations (auto-detects Docker vs host)
make migrate-up         # Apply migrations (host)
make migrate-down       # Rollback migrations (host)
make docker-migrate     # Run migrations in Docker
make migrate-create name=xxx # Create new migration
make seed               # Seed test data
```

**Development:**

```bash
make run            # Run the server (host binary)
make build          # Build all binaries
make test           # Run tests
make test-coverage  # Generate coverage report
make fmt            # Format code
make lint           # Run linters
make check          # Run all quality checks (fmt, vet, lint, test)
```

**Quality & Security:**

```bash
make install-tools       # Install golangci-lint, gosec, govulncheck
make pre-commit-install  # Install Git pre-commit hooks
make security            # Run security scan
make vuln-check          # Check dependency vulnerabilities
```

**Environment:**

```bash
make status       # Show status of all services
make reset        # Complete reset (removes containers, volumes, .env, binaries)
make help         # Show all available commands with descriptions
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test -v ./internal/service/roulette/...
```

### Adding a New Migration

```bash
make migrate-create name=add_user_preferences
# Edit migrations/YYYYMMDDHHMMSS_add_user_preferences.up.sql
# Edit migrations/YYYYMMDDHHMMSS_add_user_preferences.down.sql
make migrate-up
```

## API Endpoints

### Webhook

- `POST /webhook/gitlab` - Receive GitLab webhooks

### Health & Metrics

- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics (port 9090)

### Dashboard API (Read-Only, No Authentication Required)

These endpoints provide public access to leaderboards, statistics, and badge information:

- `GET /api/v1/leaderboard` - Global leaderboard (supports query params: `period`, `metric`, `limit`)
- `GET /api/v1/leaderboard/:team` - Team-specific leaderboard
- `GET /api/v1/users/:id/stats` - User statistics for a period
- `GET /api/v1/users/:id/badges` - User's earned badges
- `GET /api/v1/badges` - Complete badge catalog with holder counts
- `GET /api/v1/badges/:id` - Specific badge details
- `GET /api/v1/badges/:id/holders` - Users who earned a badge

## Troubleshooting

### Webhook not received

- Check GitLab webhook logs (Settings → Webhooks → Recent Deliveries)
- Verify webhook URL is accessible
- Check webhook secret matches
- Review application logs

### No reviewers selected

- Verify team configuration in `config.yaml`
- Check user availability in GitLab
- Ensure CODEOWNERS file exists (if required)
- Review application logs for errors

### Database connection failed

- Verify PostgreSQL is running: `docker compose ps`
- Check connection parameters in config
- Ensure migrations are applied: `make migrate-up`

### Redis connection failed

- Verify Redis is running: `docker compose ps`
- Check Redis connection parameters
- Test connection: `redis-cli -h localhost ping`

### GitLab container not starting

- Check logs: `docker logs gitlab`
- GitLab takes 5-10 minutes to fully initialize
- Ensure you have enough memory (minimum 4GB recommended)
- Check disk space

## Documentation

### Core Documentation

- **[README.md](README.md)** (this file) - Project overview and quick start
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contributor guidelines and quality standards
- **[SECURITY.md](SECURITY.md)** - Security policy and vulnerability reporting

### Features & Operations

- **[METRICS.md](METRICS.md)** - Metrics collection, Prometheus setup, and Grafana dashboards

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for
detailed guidelines on:

- Development setup and workflow
- Code quality standards and pre-commit hooks
- Commit message conventions (Conventional Commits)
- Testing requirements
- Pull request process

### Quick Start for Contributors

```bash
# 1. Install development tools
make install-tools

# 2. Install pre-commit hooks (runs quality checks automatically)
make pre-commit-install

# 3. Before committing, run quality checks
make check

# 4. Follow conventional commit format
# Example: feat(roulette): add expertise matching
git commit -m "type(scope): description"
```

### Code Quality Standards

- **Formatting**: Enforced by pre-commit hooks (`gofmt`)
- **Linting**: Comprehensive checks with `golangci-lint` (see `.golangci.yml`)
- **Testing**: Maintain coverage >80% for business logic
- **Security**: Run `make security` and `make vuln-check`
- **Commit Messages**: Follow [Conventional
  Commits](https://www.conventionalcommits.org/)

For complete details, see [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT License
