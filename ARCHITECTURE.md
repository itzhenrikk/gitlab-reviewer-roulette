# Architecture

This document describes the system architecture, design philosophy, and technical decisions behind the GitLab Reviewer Roulette Bot.

---

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Component Design](#component-design)
- [Data Flow](#data-flow)
- [Database Schema](#database-schema)
- [API Design](#api-design)
- [Technology Stack](#technology-stack)
- [Deployment Architecture](#deployment-architecture)
- [Integration Points](#integration-points)
- [Design Decisions](#design-decisions)

---

## Overview

### Purpose

The GitLab Reviewer Roulette Bot is an intelligent code review assignment system that automatically selects optimal reviewers based on availability, current workload, file expertise, and team distribution.

### Design Philosophy

1. **Intelligence over Randomness**: Use weighted algorithms considering real-world factors (workload, expertise, availability)
2. **Motivation through Gamification**: Encourage engagement with badges, leaderboards, and achievements
3. **Data-Driven Decisions**: Comprehensive metrics enable teams to optimize their review processes
4. **Production-First**: Built for reliability, scalability, and observability from day one
5. **Separation of Concerns**: Clear service boundaries for independent scaling and development

### Target Environment

- GitLab 16.11+ (self-hosted installations)
- Kubernetes (production) or Docker Compose (development)
- Teams of 10-100+ developers across multiple projects

---

## System Architecture

### High-Level Architecture

```
┌─────────────┐
│   GitLab    │ ──webhook──> ┌──────────────────┐
└─────────────┘              │  Webhook Handler │
                             └────────┬─────────┘
                                      │
                   ┌──────────────────┼──────────────────┐
                   │                  │                  │
            ┌──────▼──────┐  ┌───────▼────────┐  ┌─────▼──────┐
            │  Roulette   │  │    Metrics     │  │ Aggregator │
            │   Service   │  │   Service      │  │  Service   │
            └──────┬──────┘  └───────┬────────┘  └─────┬──────┘
                   │                  │                  │
                   └──────────────────┼──────────────────┘
                                      │
                   ┌──────────────────┴──────────────────┐
                   │                                     │
            ┌──────▼────────┐                    ┌──────▼──────┐
            │  PostgreSQL   │                    │    Redis    │
            │   (Primary)   │                    │   (Cache)   │
            └───────────────┘                    └─────────────┘
```

### Request Flow

**Reviewer Assignment Flow (`/roulette` command):**

1. GitLab webhook → `POST /webhook/gitlab`
2. Webhook handler validates signature, parses JSON, detects `/roulette` command
3. Roulette service:
   - Fetches MR context (labels, modified files, CODEOWNERS)
   - Builds candidate pools (code owners, team members, external)
   - Checks availability (GitLab status + OOO database + Redis cache)
   - Applies weighted scoring algorithm
   - Selects 3 unique reviewers
4. Metrics service records event to Prometheus
5. Webhook handler posts formatted response to GitLab MR
6. Assignment persisted to PostgreSQL

**Metrics Collection Flow:**

1. Events trigger metrics recording (review triggered, approved, commented)
2. Metrics service writes to Prometheus (counters, gauges, histograms)
3. Aggregator service runs daily batch job:
   - Reads raw events from database
   - Calculates daily aggregates (per team, user, project)
   - Stores in `review_metrics` table (idempotent CreateOrUpdate)
4. Grafana queries Prometheus (real-time) and PostgreSQL (historical trends)

---

## Component Design

### Project Structure

```
cmd/
  server/main.go       # Main application entry point
  migrate/main.go      # Database migration tool
  init/main.go         # User synchronization utility

internal/
  api/
    health/            # Health check handlers (/health, /readiness, /liveness)
    webhook/           # GitLab webhook handler (POST /webhook/gitlab)
    dashboard/         # Dashboard API (leaderboard, badges, stats)
  service/
    roulette/          # Reviewer selection logic
    metrics/           # Metrics calculation & recording
    aggregator/        # Daily batch aggregation
    scheduler/         # Daily notifications & badge evaluation
    badges/            # Badge evaluation and awarding
    leaderboard/       # Leaderboard rankings
  repository/          # Data access layer (GORM)
  models/              # Domain models (User, Review, Badge, Metrics)
  config/              # Configuration (Viper)
  gitlab/              # GitLab API client wrapper
  mattermost/          # Mattermost webhook client
  i18n/                # Internationalization (en/fr)
  cache/               # Redis wrapper
  metrics/             # Prometheus exporters

migrations/            # SQL migrations (versioned)
scripts/               # Automation scripts (setup, initialization)
helm/                  # Kubernetes Helm charts
```

### Service Responsibilities

#### 1. Webhook Handler (`internal/api/webhook`)

- Validates GitLab webhook signatures
- Parses webhook payloads (note events, MR events)
- Routes to appropriate services (roulette, metrics)
- Posts responses back to GitLab

#### 2. Roulette Service (`internal/service/roulette`)

- **Core selection algorithm**: Weighted scoring based on workload, expertise, availability
- CODEOWNERS parsing with glob pattern matching
- Candidate pool construction (code owner, team member, external)
- Availability checking (GitLab status, OOO database, Redis cache)
- Reviewer uniqueness enforcement

#### 3. Metrics Service (`internal/service/metrics`)

- Event-driven metrics recording to Prometheus
- Tracks: TTFR, approval time, comment count/length, engagement score
- Real-time counters, gauges, histograms, summaries
- Prometheus exporter on port 9090

#### 4. Aggregator Service (`internal/service/aggregator`)

- Daily batch aggregation (default: 2:00 AM UTC)
- Calculates daily statistics per team/user/project
- Idempotent CreateOrUpdate to `review_metrics` table
- Supports backfill for historical dates

#### 5. Scheduler Service (`internal/service/scheduler`)

- Daily cron jobs for:
  - Mattermost notifications for pending reviews
  - Badge evaluation and awarding
- Configurable schedule (default: 9:00 AM UTC)

#### 6. Badge Service (`internal/service/badges`)

- Evaluates badge criteria for all users
- Awards badges when conditions met
- Tracks badge history in `user_badges` table
- Default badges: Speed Demon, Thorough Reviewer, Team Player, Mentor

#### 7. Leaderboard Service (`internal/service/leaderboard`)

- Calculates global and team rankings
- Aggregates user statistics (reviews, approvals, badges)
- Powers Dashboard API endpoints

#### 8. Dashboard API (`internal/api/dashboard`)

- REST API for front-end dashboards
- Endpoints: leaderboard, badges, user stats
- Returns JSON for easy integration

#### 9. Repository Layer (`internal/repository`)

- GORM-based data access abstraction
- Repositories: Users, Reviews, Assignments, Metrics, Badges
- Transaction management
- Query optimization (indexes, prepared statements)

---

## Data Flow

### Intelligent Selection Algorithm

```
┌─────────────────┐
│ Parse MR Context│
│ - Team label    │
│ - Modified files│
│ - CODEOWNERS    │
└────────┬────────┘
         │
         ▼
┌────────────────────┐
│ Build Candidate    │
│ Pools:             │
│ - Code Owners      │
│ - Team Members     │
│ - External         │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Filter Availability│
│ - GitLab status    │
│ - OOO database     │
│ - Redis cache (5m) │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Apply Weighting    │
│ score = 100        │
│   - (load × 10)    │
│   - (recent ? 5:0) │
│   + (expert ? 2:0) │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Select & Dedupe    │
│ - Sort by score    │
│ - Random among top │
│ - Ensure unique    │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│ Post to GitLab MR  │
│ - Formatted message│
│ - Active counts    │
└────────────────────┘
```

### Scoring Formula

```
score = 100
      - (active_reviews × 10)       # Workload penalty
      - (recent_reviews ? 5 : 0)    # Recent activity penalty
      + (has_expertise ? 2 : 0)     # File expertise bonus
```

**Rationale:**

- Base score: 100 (all reviewers equal without penalties/bonuses)
- Active reviews: Heavy penalty (-10 per review) to balance workload
- Recent activity: Small penalty (-5) to avoid reviewer fatigue
- File expertise: Small bonus (+2) to leverage domain knowledge

---

## Database Schema

### PostgreSQL Tables

#### 1. `users`

User accounts synchronized from GitLab or config.

```sql
id, gitlab_id, username, email, role, team, created_at, updated_at
```

#### 2. `ooo_status`

Out-of-office periods for availability management.

```sql
id, user_id, start_date, end_date, reason, created_at, updated_at
```

#### 3. `mr_reviews`

Core review tracking (MR lifecycle from trigger to merge/close).

```sql
id, project_id, mr_iid, author_id, team, status,
triggered_at, first_review_at, approved_at, merged_at, closed_at
```

#### 4. `reviewer_assignments`

Individual reviewer assignments (3 per MR: code owner, team, external).

```sql
id, mr_review_id, reviewer_id, role (codeowner|team|external),
assigned_at, first_response_at, approved_at
```

#### 5. `review_metrics`

Daily aggregated metrics (team/user/project granularity).

```sql
id, date, team, user_id, project_id,
reviews_triggered, reviews_completed, avg_ttfr, avg_approval_time,
total_comments, avg_comment_length, engagement_score
```

#### 6. `badges`

Badge definitions (name, description, criteria, icon).

```sql
id, name, description, criteria_type, criteria_value, icon, tier
```

#### 7. `user_badges`

User badge achievements (when awarded, progress tracking).

```sql
id, user_id, badge_id, awarded_at, progress
```

#### 8. `configuration`

Dynamic configuration (JSONB for flexibility).

```sql
key, value (jsonb), updated_at
```

### Redis Cache

**Cache Keys:**

```
user:availability:{gitlab_id}    → {available, status, until}  (5min TTL)
user:review_count:{gitlab_id}    → {count}                     (5min TTL)
user:recent_reviews:{gitlab_id}  → [timestamps]                (24h TTL)
mr:pending                       → set of "project_id:mr_iid"  (no TTL)
```

**Rationale:**

- Short TTLs (5 min) balance freshness with GitLab API load
- Recent reviews tracked for 24h to detect fatigue
- Pending MRs stored indefinitely (cleared on merge/close)

---

## API Design

### Public Endpoints

```
POST /webhook/gitlab     # GitLab webhooks (signature validation required)
GET  /health             # Health check (simple "ok" response)
GET  /readiness          # Readiness probe (checks DB + Redis)
GET  /liveness           # Liveness probe (always returns 200)
GET  /metrics            # Prometheus metrics (port 9090)
```

### Dashboard API (v1)

```
GET /api/v1/leaderboard            # Global leaderboard (top 100)
GET /api/v1/leaderboard/:team      # Team leaderboard
GET /api/v1/users/:id/stats        # User statistics
GET /api/v1/users/:id/badges       # User badges
GET /api/v1/badges                 # Badge catalog
GET /api/v1/badges/:id             # Badge details
GET /api/v1/badges/:id/holders     # Badge holders
```

### Future API (Phase 6)

```
POST /api/v1/ooo                   # Set OOO status (OIDC auth)
GET  /api/v1/availability          # Check availability
POST /api/v1/admin/badges          # Admin badge operations
```

---

## Technology Stack

### Core Technologies

| Technology         | Purpose             | Rationale                                                 |
| ------------------ | ------------------- | --------------------------------------------------------- |
| **Go 1.25+**       | Application runtime | Performance, concurrency, strong typing, excellent stdlib |
| **Gin**            | HTTP framework      | Lightweight, fast, good middleware, well-documented       |
| **GORM**           | ORM                 | Mature, easy mocking, migration support, query builder    |
| **PostgreSQL 15+** | Primary database    | Relational data, complex queries, JSONB, reliable         |
| **Redis 7+**       | Cache layer         | Fast lookups, distributed operations, TTL support         |
| **Viper**          | Configuration       | Env vars, file-based, validation, defaults                |
| **Zerolog**        | Logging             | Structured JSON logging, zero-allocation, fast            |

### Infrastructure

| Technology     | Purpose            | Rationale                                       |
| -------------- | ------------------ | ----------------------------------------------- |
| **Docker**     | Containerization   | Consistent environments, easy deployment        |
| **Kubernetes** | Orchestration      | HA, autoscaling, rolling updates, health checks |
| **Helm**       | K8s packaging      | Templating, versioning, upgrades, rollbacks     |
| **Prometheus** | Metrics collection | Industry standard, PromQL, Grafana integration  |
| **Grafana**    | Visualization      | Dashboards, alerting, multi-data-source support |

### Integration

| Technology              | Purpose                        | Rationale                                    |
| ----------------------- | ------------------------------ | -------------------------------------------- |
| **GitLab API v4**       | Webhooks, MR data, user status | Official API, comprehensive, well-documented |
| **Mattermost Webhooks** | Notifications                  | Simple, reliable, team communication         |

---

## Deployment Architecture

### Kubernetes Production Setup

```
┌──────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                    │
│                                                            │
│  ┌─────────────────────────────────────────────────────┐ │
│  │              Reviewer Roulette Namespace             │ │
│  │                                                       │ │
│  │  ┌──────────────────┐  ┌──────────────────┐        │ │
│  │  │   App Pods (3)   │  │  PostgreSQL Pod  │        │ │
│  │  │  - HPA: 2-10     │  │  - StatefulSet   │        │ │
│  │  │  - Resources:    │  │  - PVC: 50Gi     │        │ │
│  │  │    CPU: 500m-2   │  │  - Backup: Daily │        │ │
│  │  │    Mem: 512M-2G  │  │                  │        │ │
│  │  └──────────────────┘  └──────────────────┘        │ │
│  │                                                       │ │
│  │  ┌──────────────────┐  ┌──────────────────┐        │ │
│  │  │    Redis Pod     │  │   Prometheus     │        │ │
│  │  │  - StatefulSet   │  │  - Metrics store │        │ │
│  │  │  - PVC: 10Gi     │  │  - PVC: 100Gi    │        │ │
│  │  └──────────────────┘  └──────────────────┘        │ │
│  │                                                       │ │
│  │  ┌──────────────────────────────────────────────┐   │ │
│  │  │              Grafana Pod                      │   │ │
│  │  │  - Dashboards: Team, Reviewer, Quality       │   │ │
│  │  └──────────────────────────────────────────────┘   │ │
│  │                                                       │ │
│  │  ┌──────────────────────────────────────────────┐   │ │
│  │  │            ConfigMap & Secrets                │   │ │
│  │  │  - config.yaml, tokens, database creds        │   │ │
│  │  └──────────────────────────────────────────────┘   │ │
│  └───────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### High Availability Features

1. **App Pods**: 3 replicas with HPA (scales 2-10 based on CPU/memory)
2. **Database**: PostgreSQL StatefulSet with persistent volumes, daily backups
3. **Cache**: Redis StatefulSet with AOF persistence
4. **Health Checks**: Liveness (restart unhealthy pods), Readiness (remove from service)
5. **Rolling Updates**: Zero-downtime deployments
6. **Init Containers**: Automatic database migrations before app starts

### Resource Allocation

**App Pods:**

- Requests: 500m CPU, 512Mi memory (guaranteed)
- Limits: 2 CPU, 2Gi memory (maximum)

**PostgreSQL:**

- Requests: 1 CPU, 2Gi memory
- Storage: 50Gi persistent volume

**Redis:**

- Requests: 250m CPU, 512Mi memory
- Storage: 10Gi persistent volume

---

## Integration Points

### GitLab Integration

**Webhook Events:**

- `note` events (comments on MRs) - triggers `/roulette` command detection
- `merge_request` events (open, approve, merge) - tracks review lifecycle

**API Calls:**

- `GET /projects/:id/merge_requests/:mr_iid` - Fetch MR details
- `GET /projects/:id/merge_requests/:mr_iid/changes` - Get modified files
- `GET /projects/:id/repository/files/CODEOWNERS` - Parse CODEOWNERS
- `POST /projects/:id/merge_requests/:mr_iid/notes` - Post bot responses
- `GET /users/:id` - Check user status (OOO, busy)
- `GET /projects/:id/merge_requests/:mr_iid/notes` - Analyze comments
- `GET /projects/:id/merge_requests/:mr_iid/approvals` - Check approvals

### Mattermost Integration

**Webhook Notifications:**

- Daily summary of pending reviews (scheduled 9:00 AM UTC)
- Badge awards (optional, real-time)

### Prometheus Integration

**Metrics Exported:**

- `roulette_triggers_total` (counter) - Total `/roulette` invocations
- `reviews_completed_total` (counter) - Completed reviews by user/team
- `reviews_abandoned_total` (counter) - Closed without merge
- `active_reviews` (gauge) - Current active reviews per user/team
- `available_reviewers` (gauge) - Available reviewers per team/role
- `review_ttfr_seconds` (histogram) - Time to first review
- `review_time_to_approval_seconds` (histogram) - Time to approval
- `review_comment_count` (histogram) - Comments per review
- `review_comment_length` (histogram) - Average comment length
- `reviewer_engagement_score` (summary) - Engagement score distribution
- `badge_awarded_total` (counter) - Badges awarded by type

---

## Design Decisions

### Why Go?

**Pros:**

- Excellent concurrency (goroutines for webhook processing, daily jobs)
- Strong typing reduces runtime errors
- Fast compilation and execution
- Rich standard library (HTTP, JSON, time, crypto)
- Easy deployment (single binary, no runtime dependencies)

### Why Gin Framework?

**Pros:**

- Lightweight and fast (minimal overhead)
- Excellent middleware ecosystem (auth, logging, CORS)
- Well-documented with active community
- Built-in validation and binding
- Gin-specific features not heavily used (easy to migrate if needed)

### Why PostgreSQL + Redis?

**PostgreSQL:**

- Relational data (users, reviews, badges) with foreign keys
- Complex queries (leaderboards, aggregations, joins)
- JSONB support for flexible configuration
- Mature, reliable, battle-tested
- Excellent performance with proper indexing

**Redis:**

- Fast in-memory cache (sub-millisecond lookups)
- TTL support for automatic expiration (availability, review counts)
- Atomic operations (increment, set with expiration)
- Distributed operations (future: rate limiting, locks)

**Why Both:**

- PostgreSQL: Source of truth (durable, consistent)
- Redis: Performance layer (fast, ephemeral)
- Clear separation: PostgreSQL for persistence, Redis for speed

### Why GORM?

**Pros:**

- Mature ORM with extensive features
- Easy mocking for unit tests (repository pattern)
- Built-in migration support (versioned schema changes)
- Query builder reduces SQL injection risk
- Hooks for lifecycle events (BeforeCreate, AfterUpdate)

**Cons:**

- Slight performance overhead vs raw SQL (acceptable for this use case)
- Complex queries can be less readable (use raw SQL for these)

### Why Separate Services?

**Roulette, Metrics, Aggregator, Scheduler, Badges, Leaderboard as distinct services:**

**Pros:**

- Independent scaling (e.g., scale Roulette without scaling Aggregator)
- Resource isolation (heavy aggregation jobs don't impact live requests)
- Focused responsibilities (easier to understand, test, maintain)
- Flexible deployment (run services on different schedules/nodes)

**Cons:**

- More code organization overhead (acceptable with clear boundaries)
- Potential for over-engineering (services are lightweight, share repository layer)

**Decision:** Benefits outweigh costs for production environment. Clear service boundaries enable future growth.

---

## Configuration Management

### Configuration Hierarchy

1. **Default Values**: Hardcoded in `internal/config/config.go`
2. **Config File**: `config.yaml` (development, local overrides)
3. **Environment Variables**: Highest priority (production, Kubernetes)

### Environment Variable Expansion

Config file supports `${VAR:default}` syntax:

```yaml
database:
  host: ${POSTGRES_HOST:localhost}
  port: ${POSTGRES_PORT:5432}
```

### Deployment Patterns

- **Local Development**: `make run` uses `config.yaml` directly
- **Docker Compose**: Environment variables in `docker-compose.yml` override config
- **Kubernetes**: ConfigMap for non-sensitive + Secret for credentials
- **Init Containers**: Run migrations before app starts

---

## Security Considerations

### Webhook Signature Validation

All GitLab webhooks must include `X-Gitlab-Token` header matching configured secret. Requests without valid signature are rejected (401 Unauthorized).

### Database Credentials

Never hardcoded. Always provided via:

- Environment variables (production)
- Secret management (Kubernetes Secrets)
- Vault integration (future)

### API Authentication

- Public endpoints: Webhook (signature), Health (none needed), Metrics (Prometheus scraping)
- Dashboard API: Currently public (read-only), OIDC auth planned (Phase 6)
- Future Admin API: OIDC auth required (Phase 6)

### SQL Injection Prevention

GORM query builder and prepared statements prevent SQL injection. Raw SQL queries use parameterized placeholders (`?`).

### Redis Security

- Password authentication required (production)
- Network isolation (private network only)
- No sensitive data cached (user IDs, counts only - no passwords/tokens)

---

## Scalability Considerations

### Horizontal Scaling

**App Pods:**

- Stateless design enables horizontal scaling (2-10 pods via HPA)
- Load balancer distributes webhook requests
- No session affinity required

**Database:**

- PostgreSQL read replicas for reporting queries (future)
- Connection pooling (default: 25 max connections per pod)

**Cache:**

- Redis single instance sufficient for current scale (10-100 users)
- Redis Cluster for larger deployments (future, 100+ users)

### Performance Optimizations

**Database:**

- Indexes on foreign keys, frequently queried columns
- Partitioning for large tables (future, `review_metrics` by month)
- VACUUM and ANALYZE for query plan optimization

**Cache:**

- Short TTLs (5 min) balance freshness with load
- Cache warming on startup (preload active users)
- Batch operations where possible (Redis pipelines)

**Application:**

- Goroutines for concurrent processing (webhook handling)
- Connection pooling (database, Redis, HTTP clients)
- Lazy loading (fetch data only when needed)

---

## Observability

### Health Checks

- `/health` - Simple "ok" response (liveness probe)
- `/readiness` - Checks database + Redis connectivity (readiness probe)
- `/liveness` - Always returns 200 (heartbeat check)

### Logging

Structured JSON logs (Zerolog):

```json
{
  "level": "info",
  "time": "2025-01-12T10:30:00Z",
  "service": "roulette",
  "gitlab_project_id": 123,
  "mr_iid": 456,
  "selected_reviewers": ["alice", "bob", "charlie"],
  "message": "Reviewers selected successfully"
}
```

**Log Levels:**

- Debug: Verbose details (disabled in production)
- Info: Normal operations (reviewer selection, badge awards)
- Warn: Expected errors (no CODEOWNERS, insufficient reviewers)
- Error: Unexpected errors (database failures, API errors)
- Fatal: Unrecoverable errors (startup failures)

### Metrics

See [METRICS.md](METRICS.md) for comprehensive metrics documentation.

**Key Metrics:**

- Request rate (webhooks/sec)
- Response time (p50, p95, p99)
- Error rate (4xx, 5xx)
- Business metrics (reviews triggered, TTFR, approval time)

### Tracing

Currently not implemented. Future consideration: OpenTelemetry integration for distributed tracing.

---

## Future Architecture Enhancements

### Phase 6: Authentication & Admin APIs

- OIDC authentication for admin endpoints
- Admin APIs for badge management, user management
- Enhanced availability API (self-service OOO)

### Phase 7: Integration Tests & Hardening

- End-to-end tests with real GitLab instance
- Chaos engineering (fault injection, failure scenarios)
- Performance testing (load tests, stress tests)

### Phase 8: Advanced Gamification

- Seasonal badges (monthly challenges)
- Team competitions (department leaderboards)
- Badge notifications (Mattermost, email)
- Custom badges (admin-defined criteria)

### Phase 9: ML/AI Features

- ML-based expertise detection (analyze commit history)
- Review time prediction (estimate review duration)
- Reviewer recommendation (suggest best reviewer beyond algorithm)

---

## References

**Documentation:**

- [README.md](README.md) - Quick start and setup
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [METRICS.md](METRICS.md) - Metrics and observability

**External Resources:**

- [GitLab API Documentation](https://docs.gitlab.com/ee/api/)
- [GitLab Webhooks](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html)
- [CODEOWNERS Syntax](https://docs.gitlab.com/ee/user/project/code_owners.html)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Gin Framework](https://gin-gonic.com/docs/)
- [GORM Documentation](https://gorm.io/docs/)
