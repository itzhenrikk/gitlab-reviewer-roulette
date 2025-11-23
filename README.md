# GitLab Reviewer Roulette Bot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/ci.yml/badge.svg)](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/ci.yml)
[![Security](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/security.yml/badge.svg)](https://github.com/aimd54/gitlab-reviewer-roulette/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aimd54/gitlab-reviewer-roulette)](https://goreportcard.com/report/github.com/aimd54/gitlab-reviewer-roulette)

An intelligent code review assignment system for GitLab that automatically selects reviewers based on availability, workload, expertise, and team distribution. Includes gamification features and comprehensive metrics.

## Features

- 🎲 **Smart Reviewer Selection**: Automatically assigns 3 reviewers (code owner, team member, external) via `/roulette` command
- 📊 **Intelligent Weighting**: Considers current workload, recent activity, and expertise
- 🚦 **Availability Management**: Checks GitLab status and OOO entries
- 🌐 **Multilingual**: Bot responses in English and French
- 📈 **Metrics & Analytics**: Track TTFR, Time to Approval, Review Thoroughness
- 🏆 **Gamification**: Badges, leaderboards, and personal statistics with REST API
- 💬 **Daily Notifications**: Mattermost reminders for pending reviews
- 📉 **Prometheus Integration**: Export metrics to Prometheus/Grafana

## Requirements

- **Go** 1.25+
- **PostgreSQL** 15+
- **Redis** 6+
- **Docker & Docker Compose** (for development)
- **GitLab** 16.11+
- **Make** (recommended)

## Quick Start

**One-command setup** for local development and testing:

```bash
# 1. Check prerequisites
make check-env

# 2. Setup everything (automated)
make setup-complete
```

This command will:

- Build Docker image
- Start PostgreSQL, Redis, GitLab, and the app
- Configure GitLab with test users, projects, and webhooks
- Run database migrations and seed data
- Wait for services to be ready (5-10 minutes for GitLab)

**Access:**

- **App**: <http://localhost:8080>
- **GitLab**: <http://localhost:8000>
- **Metrics**: <http://localhost:9090/metrics>

**Test it:** Create a merge request in GitLab and comment `/roulette`

**Common commands:**

```bash
make logs       # Follow all logs
make restart    # Restart app
make down       # Stop everything
make status     # Show service status
make help       # See all commands
```

## Configuration

The app uses `config.yaml` for structure (teams, badges) and environment variables for secrets.

### Quick Setup

1. Copy template: `cp config.example.yaml config.yaml`
2. Edit teams to match your GitLab users
3. Set environment variables for secrets

### Key Environment Variables

```bash
# GitLab
GITLAB_URL=https://gitlab.example.com
GITLAB_TOKEN=glpat-your-token-here
GITLAB_WEBHOOK_SECRET=your-secret-here

# Database
POSTGRES_HOST=localhost
POSTGRES_PASSWORD=your-password

# Redis
REDIS_HOST=localhost

# Server
SERVER_LANGUAGE=en  # or 'fr'
LOG_LEVEL=info
```

### Customization

Edit `config.yaml` to configure:

- **Team Structure**: Define teams, members, and roles
- **Badge Thresholds**: Adjust based on team size
- **Timezone**: Set scheduler timezone
- **File Expertise**: Configure file patterns for dev/ops roles

**Full reference:** See [config.example.yaml](./config.example.yaml) for all options with inline documentation.

## Production Deployment

### Kubernetes (Recommended)

```bash
helm install reviewer-roulette ./helm/reviewer-roulette \
  --set config.gitlab.url=https://gitlab.example.com \
  --set config.gitlab.token=$GITLAB_TOKEN \
  --set config.gitlab.webhookSecret=$WEBHOOK_SECRET \
  --namespace reviewer-roulette \
  --create-namespace
```

**Features:**

- High availability (2+ replicas)
- Horizontal autoscaling (HPA)
- Automatic migrations (init container)
- Prometheus ServiceMonitor
- TLS ingress support

See [helm/reviewer-roulette/README.md](helm/reviewer-roulette/README.md) for complete docs.

### Docker

Pre-built multi-architecture images available on GitHub Container Registry:

```bash
# Pull image
docker pull ghcr.io/aimd54/gitlab-reviewer-roulette:1.8.0

# Run
docker run -d \
  --name reviewer-roulette \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/app/config.yaml \
  -e GITLAB_TOKEN=$GITLAB_TOKEN \
  -e POSTGRES_PASSWORD=$DB_PASSWORD \
  ghcr.io/aimd54/gitlab-reviewer-roulette:1.8.0
```

**Available tags:** `latest`, `1.8.0`, `1.8`, `1`
**Architectures:** linux/amd64, linux/arm64

### Docker Compose

For small deployments:

```bash
make start           # Start services
make docker-migrate  # Run migrations
make docker-init     # Sync users from GitLab
```

## Usage

### GitLab Webhook Setup

1. Navigate to: Project/Group → Settings → Webhooks
2. Configure:
   - **URL**: `https://your-server.com/webhook/gitlab`
   - **Secret**: Your `GITLAB_WEBHOOK_SECRET` value
   - **Triggers**: ✅ Comments, ✅ Merge request events
   - **SSL**: ✅ Enable verification
3. Click "Add webhook"

### Trigger Reviewer Selection

In any Merge Request, post:

```
/roulette
```

Bot response:

```
🎲 Reviewer Roulette Results:
• Code Owner: @alice (3 active reviews)
• Team Member: @bob (1 active review)
• External Reviewer: @charlie from Team-Platform (2 active reviews)
```

### Command Variations

```bash
/roulette                          # Standard selection
/roulette --force                  # Override recent review penalties
/roulette --include @user1         # Force include users
/roulette --exclude @user2         # Exclude users
/roulette --no-codeowner           # Skip codeowner
```

## API Endpoints

### Core

- `POST /webhook/gitlab` - Receive GitLab webhooks
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics (port 9090)

### Dashboard API (Public, Read-Only)

- `GET /api/v1/leaderboard` - Global leaderboard
- `GET /api/v1/leaderboard/:team` - Team leaderboard
- `GET /api/v1/users/:id/stats` - User statistics
- `GET /api/v1/users/:id/badges` - User badges
- `GET /api/v1/badges` - Badge catalog
- `GET /api/v1/badges/:id` - Badge details
- `GET /api/v1/badges/:id/holders` - Badge holders

## Development

### Project Structure

```
cmd/
  ├── server/      # Main API server
  ├── migrate/     # Database migrations
  └── init/        # User sync from GitLab
internal/
  ├── api/         # HTTP handlers (webhook, dashboard)
  ├── service/     # Business logic (roulette, metrics)
  ├── repository/  # Data access (GORM)
  ├── gitlab/      # GitLab API client
  └── cache/       # Redis wrapper
```

### Common Make Commands

```bash
# Setup & Running
make setup-complete    # Complete setup
make start             # Start services
make logs              # View logs
make status            # Service status

# Database
make migrate           # Run migrations (auto-detect)
make seed              # Seed test data

# Development
make build             # Build binaries
make test              # Run tests
make check             # All quality checks
make fmt               # Format code
make lint              # Run linters

# Quality & Security
make install-tools     # Install dev tools
make security          # Security scan
make vuln-check        # Check vulnerabilities

# See all commands
make help
```

For complete development guidelines, see [CONTRIBUTING.md](./CONTRIBUTING.md).

**Local Kubernetes Testing:** For testing the Helm chart in a local Kubernetes cluster (Kind/k3d), see [examples/kind/](./examples/kind/) for minimal manifests and instructions.

### Running Tests

```bash
make test               # All tests with coverage
make test-short         # Quick tests (no race detector)
go test -v ./internal/service/roulette/...  # Specific package
```

## Troubleshooting

**Webhook not received:**

- Check GitLab webhook logs (Settings → Webhooks → Recent Deliveries)
- Verify URL is accessible and secret matches
- Review app logs: `make logs-app`

**No reviewers selected:**

- Verify team config in `config.yaml`
- Check user availability in GitLab
- Ensure CODEOWNERS file exists (if needed)

**Database/Redis connection failed:**

- Check services: `docker compose ps`
- Verify connection params in config
- Apply migrations: `make migrate`

**GitLab not starting:**

- Check logs: `docker logs gitlab`
- GitLab takes 5-10 minutes to initialize
- Ensure 4GB+ RAM available

For more issues, see application logs or open an issue on GitHub.

## Documentation

- **[README.md](README.md)** (this file) - Overview and quick start
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System design and architecture
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Development guidelines and quality standards
- **[SECURITY.md](SECURITY.md)** - Security policy and vulnerability reporting
- **[METRICS.md](METRICS.md)** - Metrics, Prometheus, Grafana dashboards
- **[CHANGELOG.md](CHANGELOG.md)** - Version history and changes
- **[config.example.yaml](config.example.yaml)** - Complete configuration reference
- **[helm/reviewer-roulette/README.md](helm/reviewer-roulette/README.md)** - Helm chart documentation

## Contributing

We welcome contributions! Quick start:

```bash
make install-tools         # Install dev tools
make pre-commit-install    # Install hooks
make check                 # Run quality checks
```

**Commit format:** [Conventional Commits](https://www.conventionalcommits.org/)
Example: `feat(roulette): add expertise matching`

**Quality standards:**

- Code formatting (gofmt)
- Linting (golangci-lint)
- >80% test coverage for business logic
- Security checks (gosec, govulncheck)

See [CONTRIBUTING.md](./CONTRIBUTING.md) for complete guidelines.

## License

MIT License
