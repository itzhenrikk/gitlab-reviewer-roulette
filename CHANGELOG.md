# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.8.0] - 2025-11-18

### Added

- GitHub Actions CI workflow (`.github/workflows/ci.yml`)
  - Format checking, go vet, golangci-lint
  - Unit tests with coverage reporting (Codecov integration)
  - Multi-platform binary builds
  - Security scanning (gosec, govulncheck)
- GitHub Actions release workflow (`.github/workflows/release.yml`)
  - Multi-platform binaries (Linux/macOS/Windows, amd64/arm64)
  - Docker images published to GitHub Container Registry (ghcr.io)
  - Automated changelog extraction from CHANGELOG.md
  - GitHub Release creation with binaries attached
- GitHub Actions security workflow (`.github/workflows/security.yml`)
  - Daily security scans (gosec, govulncheck, Trivy, TruffleHog)
  - Dependency review on pull requests
  - SARIF reports uploaded to GitHub Security tab
  - Secret scanning with TruffleHog
- Docker image: `ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0`
  - Multi-architecture support (linux/amd64, linux/arm64)
  - Semantic version tags (v1.8.0, 1.8, 1, latest)
  - VERSION build arg embedded in binaries
- CI status badges in README.md
- `make check` now includes security and vulnerability checks
- CONTRIBUTING.md documentation for workflow layers

### Changed

- Updated all documentation to reference GitHub Container Registry images
- Helm chart: Updated repository to GHCR (Chart v1.1.0, App v1.8.0)
- README.md: Added Docker image section with pull examples
- Dockerfile: Added VERSION build arg for version embedding in binaries

### Fixed

- Security vulnerability: Updated quic-go from v0.54.0 to v0.56.0 (CVE fix)
  - Resolves high-severity DoS vulnerability (Dependabot alert #1)
  - Malicious server could crash client via premature HANDSHAKE_DONE frame
- golangci-lint-action: Updated from v6 to v7 for compatibility with v2.6.1
- gosec SARIF upload: Made continue-on-error to handle malformed SARIF gracefully
- Trivy false positive: Obfuscated example token in setup script

### Security

- Fixed high-severity DoS vulnerability in quic-go dependency
- Added 6 security scanning tools to CI/CD pipeline
- Daily automated security scans via GitHub Actions
- Dependency verification in all builds

**Benefits**: Native GitHub integration, unlimited CI minutes for public repos, comprehensive security scanning, automated releases, free Docker image hosting.

**Docker Image**: `ghcr.io/aimd54/gitlab-reviewer-roulette:v1.8.0`

---

## [1.7.0] - 2025-11-15

### Added

- Public GitHub repository with GPG-signed commits
- GitHub-specific issue templates (bug report, feature request)
- Pull request template with quality checklist
- SECURITY.md for vulnerability reporting
- Separate GPG keys for GitHub vs GitLab
- SSH authentication configuration
- Branch protection ruleset (restrict deletions, force pushes, require signed commits)
- Directory-based git configuration for automatic email/GPG switching

### Changed

- Migrated repository from private GitLab to public GitHub
- Rewrote git history to remove all personal email addresses
- All commits re-signed with GitHub GPG key

### Security

- Removed personal email from codebase and git history
- Implemented separate identities for GitHub (public) and GitLab (private)
- GPG commit signing enforced via branch protection

---

## [1.6.0] - 2025-11-xx

### Added

- Production-ready Helm chart for Kubernetes deployment
- High Availability support with 2+ replicas
- HorizontalPodAutoscaler (HPA) for automatic scaling (2-5 replicas)
- ServiceMonitor for Prometheus Operator integration
- PodDisruptionBudget to ensure 1 pod always available
- Ingress template with TLS and cert-manager support
- MIT License for open source distribution
- Comprehensive INSTALLATION.md guide (400+ lines)
- Complete OPERATIONS.md runbook (400+ lines)
- Helm chart README with configuration reference

### Changed

- README.md reorganized with clear "Production Deployment" section
- Highlighted Helm as recommended production method
- Updated cmd/server/main.go with API documentation comments

### Infrastructure

- Init container for database migrations
- ConfigMap for application configuration
- Secret template for sensitive data (tokens, passwords)
- Resource requests and limits configured
- Health check probes (liveness, readiness)

---

## [1.5.0] - 2024-11-14

### Added (Phase 5: Gamification)

- Badge system with automatic evaluation
- 4 pre-configured badges (speed_demon, thorough_reviewer, team_player, mentor)
- Badge criteria evaluator with 6 operators (<, <=, >, >=, ==, top)
- Support for 5 periods (day, week, month, year, all_time)
- Global and team-specific leaderboards
- User statistics dashboard
- Dashboard REST API with 7 endpoints:
  - GET /api/v1/leaderboard (global)
  - GET /api/v1/leaderboard/:team
  - GET /api/v1/users/:id/stats
  - GET /api/v1/users/:id/badges
  - GET /api/v1/badges (catalog)
  - GET /api/v1/badges/:id
  - GET /api/v1/badges/:id/holders
- Badge evaluation scheduler (daily cron job)
- Prometheus metrics for badges (4 new metrics)
- GAMIFICATION.md documentation (900+ lines)

### Database

- badges table for badge definitions
- user_badges table for badge achievements

---

## [1.4.0] - 2024-11-XX

### Added (Phase 4: Scheduler & Notifications)

- Daily Mattermost notifications for pending merge request reviews
- Cron scheduler with configurable time and timezone
- Weekend skip logic (configurable)
- Age-based filtering (excludes MRs < 4 hours old)
- Graceful shutdown support
- Prometheus metrics for scheduler (6 new metrics)
- Enhanced structured logging with duration tracking

### Changed

- Fixed HTTP timeout security vulnerability (G112 - Slowloris prevention)
- Removed duplicate security:gosec job from GitLab CI
- Fixed Makefile security target to use golangci-lint

---

## [1.3.0] - 2024-11-XX

### Added (Phase 3: Metrics & Tracking)

- Comprehensive Prometheus metrics system (13 metric types)
- PostgreSQL metrics aggregation (daily batch)
- Grafana dashboards (3 dashboards: team, reviewer, quality)
- Metrics calculator (TTFR, Time to Approval, Engagement Score)
- Observability stack (Docker Compose with Prometheus + Grafana)
- METRICS.md documentation
- Production-ready Dockerfile with multiple binaries
- Environment variable configuration support (12-factor app)

### Metrics

**Prometheus (Real-time):**

- roulette_triggers_total{team, status}
- reviews_completed_total{team, user, role}
- reviews_abandoned_total{team}
- active_reviews{team, user}
- available_reviewers{team, role}
- review_ttfr_seconds{team}
- review_time_to_approval_seconds{team}
- review_comment_count{team}
- review_comment_length{team}
- reviewer_engagement_score{team, user}

**PostgreSQL (Historical):**

- Daily aggregation per team/user/project
- Forever retention (configurable)
- Idempotent CreateOrUpdate

**See Also:** [METRICS.md](METRICS.md) for metrics documentation.

---

## [1.2.0] - 2024-11-XX

### Added (Phase 2: Enhanced Selection)

- Redis caching for availability checks (5-minute TTL)
- Redis caching for review counts (5-minute TTL)
- File-based expertise matching (dev/ops file patterns)
- Advanced scoring algorithm with expertise bonus (+2 points)
- Multilingual support (English/French)
- Template-based translations (i18n package)
- Embedded locales at compile-time
- Automatic language detection from config

### Performance

- 90% reduction in GitLab API calls for availability checks
- Sub-second response time for /roulette commands
- Reduced database queries through caching

---

## [1.1.0] - 2024-10-XX

### Added

- Core /roulette command functionality
- Intelligent reviewer selection with basic weighting algorithm
- GitLab webhook handling with signature validation
- Database setup with 8 tables
- Availability checks (GitLab status + OOO database)
- Docker Compose development environment
- CODEOWNERS parsing and pattern matching
- Team label detection (name::team-name)
- Role label detection (dev, ops)
- Command variations (--force, --include, --exclude, --no-codeowner)
- PostgreSQL + Redis integration
- Health check endpoints (/health, /readiness, /liveness)
- Prometheus metrics endpoint (basic, port 9090)
- Structured logging with Zerolog (JSON format)
- User sync from config.yaml to database
- Seed data script with 12 test users across 5 teams

### Database Schema

- users - GitLab user information
- ooo_status - Out-of-office tracking
- mr_reviews - Merge request review lifecycle
- reviewer_assignments - Individual reviewer assignments
- review_metrics - Metrics aggregation
- badges - Badge definitions
- user_badges - User badge achievements
- configuration - Dynamic configuration

---

## [1.0.0] - 2024-10-XX

### Added

- Initial project setup
- Basic project structure
- README.md and initial documentation
