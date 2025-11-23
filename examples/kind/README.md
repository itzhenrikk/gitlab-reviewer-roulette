# Kind Local Deployment

Minimal Kubernetes deployment for testing the Reviewer Roulette Helm chart locally.

## Prerequisites

- [kind](https://kind.sigs.k8s.io/) or [k3d](https://k3d.io/) - Local Kubernetes cluster
- [kubectl](https://kubernetes.io/docs/tasks/tools/) - Kubernetes CLI
- [helm](https://helm.sh/) - Helm package manager

## Quick Start

### 1. Create Cluster

Create Kind cluster with NodePort mapping to expose webhooks:

```bash
kind create cluster --name reviewer-roulette --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
EOF
```

This maps NodePort 30080 from Kind to Docker host, allowing GitLab to reach webhooks.

**If cluster already exists without port mapping:**

```bash
kind delete cluster --name reviewer-roulette
# Then run the create command above with --config
```

### 2. Optional: Start GitLab (for webhook testing)

**Skip this section** if you only want to test the API without webhooks.

For end-to-end webhook testing with a real GitLab instance:

```bash
# From repo root: Start GitLab
make gitlab-up

# Monitor with: make logs-gitlab

# Configure GitLab and create credentials (.env file)
make setup-gitlab

# Populate test data and configure webhooks
# Auto-detects Kind and uses correct webhook URL!
make gitlab-seed
```

**That's it!** The `make gitlab-seed` command automatically detects Kind and configures webhooks with the correct URL.

**What gets created:**

- 12 users (alice, bob, charlie, etc.) across 5 teams
- 5 projects with CODEOWNERS files
- 9 merge requests ready for testing
- Webhooks configured automatically for Kind

Keep your `.env` file handy - it contains the GitLab token needed for the next steps.

### 3. Deploy Dependencies

```bash
kubectl apply -f dependencies/
```

Wait for pods to be ready:

```bash
kubectl wait --for=condition=ready pod -l app=postgres --timeout=60s
kubectl wait --for=condition=ready pod -l app=redis --timeout=60s
```

### 4. Install Application

Choose your installation mode based on whether you set up GitLab in step 2.

#### Option A: With Local GitLab (webhook testing)

Get GitLab token from .env file:

```bash
# From repo root: Get GitLab token
grep GITLAB_TOKEN .env
```

Install with Docker host IP for GitLab API access:

```bash
# From examples/kind/
helm install reviewer-roulette ../../helm/reviewer-roulette -f values.yaml \
  --set config.gitlab.url="http://172.17.0.1:8000" \
  --set config.gitlab.token="<GITLAB_ROOT_TOKEN_FROM>" \
  --set config.gitlab.webhookSecret="<GITLAB_WEBHOOK_SECRET>" \
  --set service.type=NodePort \
  --set service.nodePort=30080
```

**Note:** `172.17.0.1:8000` is the Docker host IP where GitLab is accessible from Kind pods.

#### Option B: Without GitLab (API testing only)

```bash
helm install reviewer-roulette ../../helm/reviewer-roulette -f values.yaml \
  --set config.gitlab.token="dummy-token-for-testing" \
  --set config.gitlab.webhookSecret="dummy-webhook-secret"
```

Wait for deployment:

```bash
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=reviewer-roulette --timeout=120s
```

### 5. Sync Users from GitLab

**Required for CODEOWNERS and team member selection to work!**

After deploying with GitLab (Option A), sync users to the application database:

```bash
# Get the pod name
POD=$(kubectl get pod -l app.kubernetes.io/name=reviewer-roulette -o jsonpath='{.items[0].metadata.name}')

# Sync users from GitLab to database
kubectl exec $POD -- /app/init --config /app/config.yaml --group-path=test-org --mrs=false
```

This syncs users from GitLab's `test-org` group and the team configuration, enabling:

- Code owner selection from CODEOWNERS files
- Team member selection based on configured teams
- External reviewer selection from other teams

**Skip this step** if you deployed without GitLab (Option B).

### 6. Optional: Deploy Observability Stack

For metrics visualization:

```bash
kubectl apply -f observability/

kubectl wait --for=condition=ready pod -l app=prometheus --timeout=60s
kubectl wait --for=condition=ready pod -l app=grafana --timeout=60s
```

### 7. Test GitLab Webhook

If you deployed with GitLab (Option A in step 4) and synced users (step 5), you're ready to test!

**Verify webhooks:**

Open GitLab (<http://localhost:8000>) and check any project:

- Go to **Settings → Webhooks**
- You should see webhooks already configured by `make gitlab-seed`

**Test end-to-end:**

1. Open a merge request (e.g., in `test-org/backend-api`)
2. Add a comment: `/roulette`
3. Bot should respond with reviewer selections!

**Available Commands:**

- `make gitlab-up` - Start GitLab infrastructure
- `make setup-gitlab` - Configure GitLab and create credentials
- `make gitlab-seed` - Populate with test data and configure webhooks
- `make logs-gitlab` - View GitLab logs
- `make gitlab-down` - Stop GitLab
- `make gitlab-restart` - Restart GitLab

**How it works:**

The `make gitlab-seed` command:

1. Auto-detects Kind cluster
2. Configures webhooks with Docker host URL (`http://172.17.0.1:30080/webhook/gitlab`)

This enables bidirectional communication:

- **GitLab → Kind**: Webhooks reach the NodePort service via Docker host (172.17.0.1:30080)
- **Kind → GitLab**: App reaches GitLab API via Docker host (172.17.0.1:8000)

## Access Services

### Application

```bash
# Health check
kubectl port-forward svc/reviewer-roulette 8080:8080
curl http://localhost:8080/health

# Metrics endpoint
kubectl port-forward svc/reviewer-roulette-metrics 9090:9090
curl http://localhost:9090/metrics
```

### Dashboard API

Test the dashboard API endpoints:

```bash
# Global leaderboard
curl http://localhost:8080/api/v1/leaderboard

# Team leaderboard
curl http://localhost:8080/api/v1/leaderboard/backend

# User statistics
curl http://localhost:8080/api/v1/users/1/stats

# User badges
curl http://localhost:8080/api/v1/users/1/badges

# Badge catalog
curl http://localhost:8080/api/v1/badges
```

### Grafana (if deployed)

```bash
kubectl port-forward svc/grafana 3000:3000
```

Open <http://localhost:3000> in your browser:

- Username: `admin`
- Password: `admin`

The Prometheus datasource is pre-configured.

**Dashboards:**

- Pre-built dashboards are available in `../../grafana/dashboards/` for manual import
- See `../../METRICS.md` for dashboard documentation and usage
- Import via Grafana UI: Home → Dashboards → Import → Upload JSON file

### Database Access (debugging)

```bash
kubectl port-forward svc/postgres 5432:5432 &
psql -h localhost -U postgres -d reviewer_roulette
```

Password: `postgres`

### Redis Access (debugging)

```bash
kubectl port-forward svc/redis 6379:6379 &
redis-cli -h localhost -p 6379
```

## View Logs

```bash
# Application logs
kubectl logs -l app.kubernetes.io/name=reviewer-roulette -f

# PostgreSQL logs
kubectl logs -l app=postgres -f

# Redis logs
kubectl logs -l app=redis -f

# Prometheus logs (if deployed)
kubectl logs -l app=prometheus -f

# Grafana logs (if deployed)
kubectl logs -l app=grafana -f
```

## Check Status

```bash
# All pods
kubectl get pods

# Application details
kubectl describe deployment reviewer-roulette

# Services
kubectl get svc

# ConfigMaps and Secrets
kubectl get configmap,secret
```

## Cleanup

```bash
# Delete application
helm uninstall reviewer-roulette

# Delete dependencies
kubectl delete -f dependencies/

# Delete observability stack (if deployed)
kubectl delete -f observability/

# Delete cluster
kind delete cluster --name reviewer-roulette

## Configuration

The `values.yaml` file provides sensible defaults for local testing (reduced resources, single replica, debug logging).

For detailed configuration options, see:

- Main Helm chart: `../../helm/reviewer-roulette/README.md`
- Configuration reference: `../../helm/reviewer-roulette/values.yaml`
- Main project: `../../README.md`

## Troubleshooting

### Pods not starting

```bash
kubectl get events --sort-by='.lastTimestamp'
kubectl describe pod <pod-name>
```

### Database connection errors

```bash
# Check PostgreSQL is ready
kubectl logs -l app=postgres

# Test connection
kubectl exec -it deployment/postgres -- psql -U postgres -d reviewer_roulette -c "SELECT 1;"
```

### Redis connection errors

```bash
# Check Redis is ready
kubectl logs -l app=redis

# Test connection
kubectl exec -it deployment/redis -- redis-cli ping
```

### Image pull errors

Ensure you're pulling from the correct registry:

```bash
kubectl describe pod -l app.kubernetes.io/name=reviewer-roulette | grep -A 5 "Events:"
```

The default image is `ghcr.io/aimd54/gitlab-reviewer-roulette:1.8.0`. To use a different version:

```bash
helm install reviewer-roulette ../../helm/reviewer-roulette -f values.yaml \
  --set image.tag="1.7.0"
```

### Config validation errors

If you see `invalid configuration: gitlab.token is required`, ensure you're passing the token correctly:

```bash
# Wrong (old path):
--set secrets.gitlabToken="token"

# Correct (new path):
--set config.gitlab.token="token"
```

### Init container fails with "config.yaml: no such file or directory"

This was fixed in recent Helm chart updates. Ensure you're using the latest chart:

```bash
helm upgrade reviewer-roulette ../../helm/reviewer-roulette -f values.yaml \
  --set config.gitlab.token="your-token" \
  --set config.gitlab.webhookSecret="your-secret"
```

### Scheduler fails with "invalid time format"

Ensure your `values.yaml` includes the scheduler configuration:

```yaml
scheduler:
  enabled: true
  dailyNotificationTime: "09:00"
  badgeEvaluationTime: "0 2 * * *"
  timezone: "UTC"
```

## Upgrading

To apply configuration changes or Helm chart updates without reinstalling:

```bash
helm upgrade reviewer-roulette ../../helm/reviewer-roulette -f values.yaml \
  --set config.gitlab.token="your-token" \
  --set config.gitlab.webhookSecret="your-secret"

# Wait for rollout
kubectl rollout status deployment/reviewer-roulette

# Check logs
kubectl logs deployment/reviewer-roulette -f
```
