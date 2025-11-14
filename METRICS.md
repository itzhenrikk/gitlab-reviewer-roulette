# Metrics Documentation

## Overview

The GitLab Reviewer Roulette Bot tracks comprehensive metrics about code review processes to help teams improve their review efficiency and quality. This document describes all metrics, their collection methods, and how to use them for insights.

## Metrics Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Webhook Events │────>│ Metrics Service  │────>│   PostgreSQL    │
│   (Real-time)   │     │  (Event-driven)  │     │ (review_metrics)│
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                           │
                        ┌──────────────────┐              │
                        │   Aggregator     │<─────────────┘
                        │   (Daily Batch)  │
                        └────────┬─────────┘
                                 │
                        ┌────────▼─────────┐
                        │   Prometheus     │
                        │   (Metrics API)  │
                        └────────┬─────────┘
                                 │
                        ┌────────▼─────────┐
                        │     Grafana      │
                        │   (Dashboards)   │
                        └──────────────────┘
```

## Metric Types

### 1. Real-time Counters (Prometheus)

Collected immediately when events occur:

#### `roulette_triggers_total{team, status}`

- **Type**: Counter
- **Description**: Total number of `/roulette` commands triggered
- **Labels**:
  - `team`: Team name (e.g., "team-frontend", "team-platform")
  - `status`: Success or failure ("success", "error")
- **Use Case**: Track adoption of roulette system

#### `active_reviews{team, user}`

- **Type**: Gauge
- **Description**: Current number of active reviews per user/team
- **Labels**:
  - `team`: Team name
  - `user`: Username (optional)
- **Use Case**: Monitor workload distribution

#### `reviews_completed_total{team, user, role}`

- **Type**: Counter
- **Description**: Total completed reviews (merged)
- **Labels**:
  - `team`: Team name
  - `user`: Username
  - `role`: Reviewer role ("codeowner", "team_member", "external")
- **Use Case**: Track individual and team productivity

#### `reviews_abandoned_total{team}`

- **Type**: Counter
- **Description**: Total reviews closed without merging
- **Labels**:
  - `team`: Team name
- **Use Case**: Identify process issues

### 2. Real-time Histograms (Prometheus)

Collected when reviews complete:

#### `review_ttfr_seconds{team}`

- **Type**: Histogram
- **Description**: Time To First Review (TTFR) in seconds
- **Labels**:
  - `team`: Team name
- **Buckets**: [300, 900, 1800, 3600, 7200, 14400, 28800, 86400]
  - 5min, 15min, 30min, 1h, 2h, 4h, 8h, 24h
- **Use Case**: Measure responsiveness

#### `review_time_to_approval_seconds{team}`

- **Type**: Histogram
- **Description**: Time from roulette trigger to approval
- **Labels**:
  - `team`: Team name
- **Buckets**: Same as TTFR
- **Use Case**: Measure review cycle time

#### `review_comment_count{team}`

- **Type**: Histogram
- **Description**: Number of comments per review
- **Labels**:
  - `team`: Team name
- **Buckets**: [1, 2, 5, 10, 20, 50, 100]
- **Use Case**: Measure review thoroughness

#### `review_comment_length{team}`

- **Type**: Histogram
- **Description**: Total comment character count
- **Labels**:
  - `team`: Team name
- **Buckets**: [50, 100, 200, 500, 1000, 2000, 5000]
- **Use Case**: Measure review depth

### 3. Aggregated Metrics (PostgreSQL)

Daily batch aggregation stored in `review_metrics` table:

#### Team-Level Metrics

- **Granularity**: Daily, per team
- **Fields**:
  - `date`: Date of aggregation (start of day)
  - `team`: Team name
  - `total_reviews`: Number of reviews triggered
  - `completed_reviews`: Number of reviews merged
  - `avg_ttfr`: Average Time To First Review (minutes)
  - `avg_time_to_approval`: Average time to approval (minutes)
  - `avg_comment_count`: Average comments per review
  - `avg_comment_length`: Average comment length per review
  - `engagement_score`: Calculated engagement metric

#### User-Level Metrics

- **Granularity**: Daily, per user, per project
- **Fields**: Same as team-level, plus:
  - `user_id`: User identifier
  - `project_id`: GitLab project ID

#### Engagement Score Calculation

For **user-level** metrics (per assignment):

```
engagement_score = (comment_count * 10) + (comment_length / 100)
                 + time_bonus
                 + depth_bonus

where:
  time_bonus = 20 if first_comment_time < 1 hour, else 10 if < 4 hours, else 0
  depth_bonus = 10 if comment_length > 200, else 0
```

For **team-level** metrics (aggregated):

```
engagement_score = (avg_comment_count * 10) + (avg_comment_length / 100)
```

## Data Collection Flow

### Event-Driven Collection

1. **Roulette Triggered** (`/roulette` comment)
   - Create `MRReview` record with `roulette_triggered_at` timestamp
   - Create `ReviewerAssignment` records for selected reviewers
   - Increment `roulette_triggers_total` counter
   - Update `active_reviews` gauge

2. **First Comment Added**
   - Update `ReviewerAssignment.first_comment_at`
   - Update `MRReview.first_review_at` (if first across all reviewers)

3. **MR Approved**
   - Update `MRReview.approved_at`

4. **MR Merged**
   - Update `MRReview.merged_at`, set `status = 'merged'`
   - Calculate and observe histogram metrics:
     - TTFR: `first_review_at - roulette_triggered_at`
     - Time to approval: `approved_at - roulette_triggered_at`
     - Comment counts and lengths per assignment
   - Increment `reviews_completed_total` counter
   - Decrement `active_reviews` gauge

5. **MR Closed (not merged)**
   - Update `MRReview.closed_at`, set `status = 'closed'`
   - Increment `reviews_abandoned_total` counter
   - Decrement `active_reviews` gauge

### Daily Aggregation

Runs daily at configurable time (default: 2:00 AM UTC):

1. Query completed reviews (merged or closed) from previous day
2. Group by team and calculate:
   - Average TTFR (if `first_review_at` and `roulette_triggered_at` exist)
   - Average time to approval (if `approved_at` exists)
   - Average comment metrics from assignments
   - Engagement scores
3. For each team, create or update `review_metrics` record
4. For each user+project combination, create or update user-level metrics
5. Aggregation is **idempotent** - can be re-run safely for the same date

## Accessing Metrics

### 1. Prometheus Endpoint

**Endpoint**: `GET /metrics`

**Example**:

```bash
curl http://localhost:9090/metrics

# Output:
# HELP roulette_triggers_total Total number of roulette triggers
# TYPE roulette_triggers_total counter
roulette_triggers_total{status="success",team="team-frontend"} 145
roulette_triggers_total{status="success",team="team-platform"} 98

# HELP active_reviews Current number of active reviews
# TYPE active_reviews gauge
active_reviews{team="team-frontend"} 23
active_reviews{team="team-platform"} 15

# HELP review_ttfr_seconds Time to first review in seconds
# TYPE review_ttfr_seconds histogram
review_ttfr_seconds_bucket{team="team-frontend",le="300"} 12
review_ttfr_seconds_bucket{team="team-frontend",le="900"} 45
review_ttfr_seconds_bucket{team="team-frontend",le="1800"} 78
...
```

### 2. PostgreSQL Queries

#### Get Team Performance (Last 7 Days)

```sql
SELECT
    team,
    AVG(avg_ttfr) as avg_ttfr_minutes,
    AVG(avg_time_to_approval) as avg_approval_minutes,
    SUM(total_reviews) as total_reviews,
    SUM(completed_reviews) as completed_reviews,
    ROUND(AVG(engagement_score), 2) as avg_engagement
FROM review_metrics
WHERE date >= CURRENT_DATE - INTERVAL '7 days'
  AND user_id IS NULL  -- team-level only
GROUP BY team
ORDER BY avg_engagement DESC;
```

#### Get Top Reviewers (Last 30 Days)

```sql
SELECT
    u.username,
    u.team,
    COUNT(*) as review_days,
    AVG(rm.avg_ttfr) as avg_ttfr,
    AVG(rm.engagement_score) as avg_engagement,
    SUM(rm.total_reviews) as total_reviews
FROM review_metrics rm
JOIN users u ON rm.user_id = u.id
WHERE rm.date >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY u.id, u.username, u.team
HAVING COUNT(*) >= 5  -- at least 5 days of activity
ORDER BY avg_engagement DESC
LIMIT 10;
```

#### Get Project-Specific Metrics

```sql
SELECT
    date,
    COUNT(DISTINCT user_id) as reviewers,
    AVG(avg_ttfr) as avg_ttfr,
    AVG(avg_comment_count) as avg_comments,
    SUM(total_reviews) as total_reviews
FROM review_metrics
WHERE project_id = 123  -- specific project
  AND date >= CURRENT_DATE - INTERVAL '90 days'
GROUP BY date
ORDER BY date DESC;
```

### 3. Repository Methods

Available in `internal/repository/metrics_repository.go`:

```go
// Get metrics for a specific date
GetByDate(date time.Time, team string, userID *uint) (*models.ReviewMetrics, error)

// Get metrics within date range
GetByDateRange(startDate, endDate time.Time, filters map[string]interface{}) ([]models.ReviewMetrics, error)

// Get average TTFR by team
GetAverageTTFRByTeam(startDate, endDate time.Time) (map[string]float64, error)

// Get top reviewers by engagement
GetTopReviewersByEngagement(startDate, endDate time.Time, limit int) ([]models.User, error)

// Get team-specific metrics
GetMetricsByTeam(team string, startDate, endDate time.Time) ([]models.ReviewMetrics, error)

// Get user-specific metrics
GetMetricsByUser(userID uint, startDate, endDate time.Time) ([]models.ReviewMetrics, error)

// Get daily aggregated stats
GetDailyStats(date time.Time) (map[string]interface{}, error)
```

## Grafana Dashboards

Three pre-built dashboards are available in `grafana/dashboards/`:

### 1. Team Performance Dashboard (`team-performance.json`)

**Panels**:

- Total Roulette Triggers (last 24h)
- Active Reviews by Team (current)
- TTFR Distribution (histogram, last 7 days)
- Time to Approval Distribution (histogram, last 7 days)
- Review Completion Rate (completed vs. abandoned)
- Team Workload Over Time (timeseries)

**Filters**:

- Team selector
- Time range selector

### 2. Reviewer Statistics Dashboard (`reviewer-statistics.json`)

**Panels**:

- Reviews Completed by User (last 30 days, bar chart)
- Average TTFR per User (last 30 days, table)
- Engagement Score Leaderboard (top 10)
- User Review Activity (heatmap)
- Active Reviews per User (current, gauge)

**Filters**:

- Team selector
- User selector (multi-select)
- Time range selector

### 3. Review Quality Dashboard (`review-quality.json`)

**Panels**:

- Comment Count Distribution (histogram)
- Comment Length Distribution (histogram)
- Average Comments per Review (timeseries)
- Thorough Reviews (>10 comments, stat)
- Quick Reviews (<5 min TTFR, stat)
- Review Depth Over Time (engagement score trend)

**Filters**:

- Team selector
- Time range selector

## Interpreting Metrics

### TTFR (Time To First Review)

**Good**: < 2 hours
**Average**: 2-4 hours
**Needs Improvement**: > 4 hours

**Factors Affecting TTFR**:

- Reviewer availability
- Workload distribution
- Team timezone coverage
- Notification effectiveness

**Improvement Actions**:

- Balance workload using roulette algorithm
- Adjust team member distribution
- Use Mattermost daily notifications
- Set OOO status properly

### Time to Approval

**Good**: < 4 hours
**Average**: 4-8 hours
**Needs Improvement**: > 1 day

**Factors**:

- Code complexity
- Reviewer familiarity with codebase
- Number of rounds of feedback
- Availability of code owner

### Comment Metrics

**Comment Count**:

- 1-3 comments: Simple changes, quick reviews
- 4-10 comments: Normal complexity
- >10 comments: Complex changes or significant feedback

**Comment Length**:

- <200 chars: Quick approvals or minor issues
- 200-1000 chars: Detailed feedback
- >1000 chars: Comprehensive reviews or major concerns

### Engagement Score

**High (>50)**: Very engaged reviewers providing detailed feedback
**Medium (20-50)**: Normal engagement level
**Low (<20)**: Quick approvals with minimal feedback

**Note**: High engagement isn't always better. Quick, focused reviews can be just as valuable as detailed ones, depending on the context.

### Completion Rate

```
Completion Rate = (Completed Reviews / Total Reviews) * 100
```

**Good**: > 90%
**Average**: 80-90%
**Needs Improvement**: < 80%

High abandonment rate may indicate:

- MRs opened prematurely
- Duplicate work
- Shifting priorities
- Process issues

## Retention and Cleanup

**Current Policy**: Forever retention (configurable via `metrics.retention_days: 0`)

**To Enable Cleanup**:

1. Update `config.yaml`:

```yaml
metrics:
  retention_days: 365  # Keep 1 year of data
```

1. Run cleanup manually:

```go
metricsRepo.DeleteOldMetrics(365)  // Delete metrics older than 365 days
```

1. Or schedule via cron:

```bash
# Add to crontab (runs monthly)
0 0 1 * * /usr/local/bin/reviewer-roulette-cleanup
```

**Note**: Prometheus metrics are stored by Prometheus and follow their own retention policies (separate from application database).

## Troubleshooting

### Missing Metrics in Prometheus

**Symptoms**: Grafana shows "No data" or metrics endpoint returns empty

**Check**:

1. Is the application running? `curl http://localhost:8080/health`
2. Is Prometheus scraping? Check Prometheus UI → Targets
3. Are events being triggered? Check application logs
4. Network connectivity between Prometheus and app

**Fix**:

```bash
# Verify metrics endpoint
curl http://localhost:9090/metrics | grep roulette

# Check Prometheus config
docker exec prometheus cat /etc/prometheus/prometheus.yml

# Restart services
docker compose restart app prometheus
```

### Incorrect TTFR/Approval Times

**Symptoms**: Times are negative, zero, or unreasonably large

**Causes**:

- Timestamps not set properly (check webhook events)
- System clock skew between GitLab and application
- Missing `RouletteTriggeredAt` timestamp

**Fix**:

1. Check database records:

```sql
SELECT
    gitlab_mr_iid,
    roulette_triggered_at,
    first_review_at,
    approved_at,
    EXTRACT(EPOCH FROM (first_review_at - roulette_triggered_at))/60 as ttfr_minutes
FROM mr_reviews
WHERE first_review_at IS NOT NULL
ORDER BY id DESC
LIMIT 10;
```

1. Verify webhook events are being received:

```bash
# Check logs for webhook processing
docker logs app 2>&1 | grep "webhook"
```

### Aggregation Not Running

**Symptoms**: `review_metrics` table is empty or outdated

**Causes**:

- Scheduler not enabled
- No completed reviews to aggregate
- Aggregator service error

**Fix**:

1. Run aggregation manually:

```go
// In Go code
aggregator := aggregator.NewService(reviewRepo, metricsRepo, log)
date := time.Now().AddDate(0, 0, -1) // Yesterday
err := aggregator.AggregateDaily(context.Background(), date)
```

1. Check for errors:

```bash
docker logs app 2>&1 | grep "aggregation"
```

1. Verify data exists to aggregate:

```sql
SELECT COUNT(*)
FROM mr_reviews
WHERE (merged_at::date = CURRENT_DATE - 1)
   OR (closed_at::date = CURRENT_DATE - 1);
```

### Zero Engagement Scores

**Symptoms**: `engagement_score` is 0 or NULL

**Causes**:

- No comments in assignments
- `CommentCount` and `CommentLength` not being tracked

**Fix**:
Check if comment tracking is working:

```sql
SELECT
    ra.id,
    ra.comment_count,
    ra.comment_total_length,
    u.username
FROM reviewer_assignments ra
JOIN users u ON ra.user_id = u.id
WHERE ra.mr_review_id IN (
    SELECT id FROM mr_reviews
    WHERE merged_at IS NOT NULL
    ORDER BY merged_at DESC
    LIMIT 5
);
```

If counts are 0, the comment webhook events aren't being processed properly.

## Performance Considerations

### Database Indexes

Ensure these indexes exist for optimal query performance:

```sql
-- Review queries
CREATE INDEX idx_mr_reviews_merged_at ON mr_reviews(merged_at);
CREATE INDEX idx_mr_reviews_closed_at ON mr_reviews(closed_at);
CREATE INDEX idx_mr_reviews_team ON mr_reviews(team);

-- Metrics queries
CREATE INDEX idx_review_metrics_date ON review_metrics(date);
CREATE INDEX idx_review_metrics_team ON review_metrics(team);
CREATE INDEX idx_review_metrics_user ON review_metrics(user_id);
CREATE INDEX idx_review_metrics_date_team ON review_metrics(date, team);
```

### Query Optimization

For large datasets (>100k records), consider:

1. **Partition by date**:

```sql
CREATE TABLE review_metrics (
    ...
) PARTITION BY RANGE (date);

CREATE TABLE review_metrics_2024_01 PARTITION OF review_metrics
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
```

1. **Use materialized views** for complex aggregations:

```sql
CREATE MATERIALIZED VIEW team_monthly_stats AS
SELECT
    DATE_TRUNC('month', date) as month,
    team,
    AVG(avg_ttfr) as avg_ttfr,
    SUM(total_reviews) as total_reviews,
    AVG(engagement_score) as avg_engagement
FROM review_metrics
WHERE user_id IS NULL
GROUP BY month, team;

-- Refresh daily
REFRESH MATERIALIZED VIEW team_monthly_stats;
```

### Prometheus Retention

Configure Prometheus retention based on needs:

```yaml
# prometheus.yml
global:
  retention: 30d  # Keep 30 days of data
  retention_size: 10GB  # Or limit by size
```

For long-term storage, use Prometheus with remote storage or Thanos.

## API Examples

### Creating Custom Reports

```go
package main

import (
    "context"
    "time"
    "github.com/aimd54/gitlab-reviewer-roulette/internal/repository"
)

func GenerateTeamReport(metricsRepo *repository.MetricsRepository, team string) {
    startDate := time.Now().AddDate(0, -1, 0) // Last month
    endDate := time.Now()

    metrics, err := metricsRepo.GetMetricsByTeam(team, startDate, endDate)
    if err != nil {
        log.Fatal(err)
    }

    // Calculate averages
    var totalTTFR, totalEngagement float64
    count := 0
    for _, m := range metrics {
        if m.UserID == nil && m.AvgTTFR != nil {
            totalTTFR += float64(*m.AvgTTFR)
            count++
        }
        if m.EngagementScore != nil {
            totalEngagement += *m.EngagementScore
        }
    }

    avgTTFR := totalTTFR / float64(count)
    avgEngagement := totalEngagement / float64(len(metrics))

    fmt.Printf("Team: %s\n", team)
    fmt.Printf("Average TTFR: %.2f minutes\n", avgTTFR)
    fmt.Printf("Average Engagement: %.2f\n", avgEngagement)
}
```

## Future Enhancements

Planned metrics for future phases:

- **Reviewer specialization**: Track expertise areas based on file patterns
- **Mentorship metrics**: Measure interactions between senior and junior reviewers
- **Review quality indicators**: Defect density post-merge
- **Team collaboration**: Cross-team review frequency
- **Timezone coverage**: Review availability across time zones
- **Badge achievements**: Milestone tracking (Speed Demon, Thorough Reviewer, etc.)
