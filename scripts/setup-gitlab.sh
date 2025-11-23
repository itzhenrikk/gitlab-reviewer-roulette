#!/bin/bash
#
# setup-gitlab.sh - Unified GitLab setup automation script
#
# This script automates GitLab configuration for local testing with two modes:
#   --minimal   : Bot user creation + token generation + config update
#   --realistic : Minimal setup + comprehensive test data (users, projects, MRs)
#
# Usage:
#   ./scripts/setup-gitlab.sh --minimal [--skip-wait]
#   ./scripts/setup-gitlab.sh --realistic [--dry-run] [--verbose]
#   ./scripts/setup-gitlab.sh --help
#
# Prerequisites:
#   - GitLab running at http://localhost:8000 (or GITLAB_URL env var)
#   - Docker Compose running
#   - jq installed (for realistic mode)
#
# Environment Variables:
#   GITLAB_URL          GitLab instance URL (default: http://localhost:8000)
#   GITLAB_ROOT_TOKEN   Admin token (will be prompted if not set)
#   WEBHOOK_URL         Webhook endpoint (default: http://172.17.0.1:8080/webhook/gitlab)
#

set -e
set -o pipefail

# ==================================================================================== #
# CONFIGURATION
# ==================================================================================== #

GITLAB_URL="${GITLAB_URL:-http://localhost:8000}"
GITLAB_ROOT_TOKEN="${GITLAB_ROOT_TOKEN:-}"
WEBHOOK_URL="${WEBHOOK_URL:-http://172.17.0.1:8080/webhook/gitlab}"
WEBHOOK_SECRET=""
BOT_USERNAME="reviewer-roulette-bot"
BOT_PASSWORD="botpassword123"
BOT_EMAIL="bot@example.com"
BOT_USER_ID=""
BOT_TOKEN=""
ROOT_USER="root"
ROOT_PASSWORD=""

# Modes
MODE=""
SKIP_WAIT=false
DRY_RUN=false
VERBOSE=false
AUTOMATED=false

# Auto-detect Kind cluster and adjust webhook URL
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q 'reviewer-roulette-control-plane'; then
    # Use Docker host IP for webhooks (requires Kind extraPortMappings)
    WEBHOOK_URL="http://172.17.0.1:30080/webhook/gitlab"
    echo "✅ Detected Kind cluster"
    echo "📡 Using webhook URL: $WEBHOOK_URL (via Docker host)"
    echo "ℹ️  Requires Kind cluster created with extraPortMappings for port 30080"
fi

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Progress tracking (for realistic mode)
CURRENT_STEP=0

# Caches for API responses (realistic mode)
declare -A USER_IDS
declare -A PROJECT_IDS
GROUP_ID=""

# ==================================================================================== #
# HELPER FUNCTIONS
# ==================================================================================== #

show_help() {
    cat <<EOF
GitLab Setup Automation Script

Usage:
  $0 --minimal [--skip-wait]
  $0 --realistic [--dry-run] [--verbose]
  $0 --help

Modes:
  --minimal      Bot user creation + token generation + config update (~5 min)
  --realistic    Minimal setup + comprehensive test data creation (~10 min)
                 Creates: 12 users, 5 projects, CODEOWNERS, labels, 9 MRs

Options:
  --skip-wait    Skip GitLab readiness check (assumes already ready)
  --automated    Suppress "Next Steps" messages (for automated workflows)
  --dry-run      Show what would be created without making changes
  --verbose      Show detailed API responses
  --help         Show this help message

Environment Variables:
  GITLAB_URL          GitLab instance URL (default: http://localhost:8000)
  GITLAB_ROOT_TOKEN   Admin access token (will be prompted if not set)

Examples:
  # Minimal setup for quick testing
  ./scripts/setup-gitlab.sh --minimal

  # Full realistic test environment
  export GITLAB_ROOT_TOKEN="glpat-your-token-here"
  ./scripts/setup-gitlab.sh --realistic

  # Preview what would be created
  ./scripts/setup-gitlab.sh --realistic --dry-run

Prerequisites:
  - GitLab container running: docker compose up -d
  - jq installed (for realistic mode): apt-get install jq / brew install jq

EOF
    exit 0
}

info() {
    echo -e "${CYAN}ℹ${NC} $1"
}

success() {
    echo -e "${GREEN}✓${NC} $1"
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

error() {
    echo -e "${RED}✗${NC} $1"
}

progress() {
    if [ "$MODE" = "realistic" ]; then
        CURRENT_STEP=$((CURRENT_STEP + 1))
        echo -e "${GREEN}[$CURRENT_STEP]${NC} $1"
    else
        echo -e "${YELLOW}→${NC} $1"
    fi
}

# ==================================================================================== #
# SHARED CORE FUNCTIONS
# ==================================================================================== #

# Get future date (cross-platform)
get_future_date() {
    local days=$1
    if date --version >/dev/null 2>&1; then
        date -d "+${days} days" +%Y-%m-%d  # GNU date (Linux)
    else
        date -v+${days}d +%Y-%m-%d  # BSD date (macOS)
    fi
}

# Wait for GitLab to be ready
wait_for_gitlab() {
    echo -e "${YELLOW}Waiting for GitLab to be ready...${NC}"

    local max_attempts=60
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if curl -s -o /dev/null -w "%{http_code}" "$GITLAB_URL" | grep -q "200\|302"; then
            echo -e "${GREEN}✓ GitLab is ready!${NC}"
            return 0
        fi

        echo -n "."
        sleep 5
        attempt=$((attempt + 1))
    done

    echo -e "${RED}✗ GitLab failed to start within 5 minutes${NC}"
    exit 1
}

# GitLab API wrapper
gitlab_api() {
    local method="$1"
    local endpoint="$2"
    local data="${3:-}"
    local token="${4:-$GITLAB_ROOT_TOKEN}"

    if [ "$DRY_RUN" = "true" ] && [ "$method" != "GET" ]; then
        echo "[DRY RUN] Would call: $method $endpoint" >&2
        return 0
    fi

    local response
    response=$(curl -s -X "$method" \
        -H "PRIVATE-TOKEN: $token" \
        -H "Content-Type: application/json" \
        "${GITLAB_URL}/api/v4${endpoint}" \
        ${data:+-d "$data"})

    if [ "$VERBOSE" = "true" ]; then
        echo "API Response: $response" >&2
    fi

    echo "$response"
}

# Get root token
get_root_token() {
    echo -e "${YELLOW}Getting root access token...${NC}"

    # Try to read the initial root password from the container
    local container_id
    container_id=$(docker ps -q -f name=gitlab)

    if [ -z "$container_id" ]; then
        error "GitLab container not found. Is docker compose running?"
        exit 1
    fi

    info "Attempting to retrieve initial root password..."
    local initial_password
    initial_password=$(docker exec "$container_id" cat /etc/gitlab/initial_root_password 2>/dev/null | grep "^Password:" | cut -d' ' -f2 | tr -d '[:space:]')

    if [ -n "$initial_password" ]; then
        success "Found initial root password"
        ROOT_PASSWORD="$initial_password"
    fi

    # Manual token entry
    echo ""
    echo -e "${BLUE}Please follow these steps to create a token manually:${NC}"
    echo -e "1. Open GitLab in your browser: ${GREEN}$GITLAB_URL/-/user_settings/personal_access_tokens${NC}"
    echo "2. Log in with:"
    echo -e "   - Username: ${GREEN}$ROOT_USER${NC}"
    echo -e "   - Password: ${GREEN}$ROOT_PASSWORD${NC}"
    echo "3. Create a new token with:"
    echo -e "   - Name: ${GREEN}automation-token${NC}"
    echo -e "   - Scopes: ${GREEN}api, read_api, write_repository${NC}"
    echo -e "   - Expiration: ${GREEN}1 year from now${NC}"
    echo "4. Copy the generated token"
    echo ""

    read -p "$(echo -e ${YELLOW}Paste the access token here:${NC} )" GITLAB_ROOT_TOKEN
    GITLAB_ROOT_TOKEN=$(echo "$GITLAB_ROOT_TOKEN" | tr -d '[:space:]')

    if [ -z "$GITLAB_ROOT_TOKEN" ] || [ ${#GITLAB_ROOT_TOKEN} -lt 20 ]; then
        error "Invalid token provided (too short)"
        exit 1
    fi

    # Test the token
    info "Validating token..."
    local validation
    validation=$(curl -s -o /dev/null -w "%{http_code}" "$GITLAB_URL/api/v4/user" \
        -H "PRIVATE-TOKEN: $GITLAB_ROOT_TOKEN")

    if [ "$validation" = "200" ]; then
        success "Token validated successfully"
    else
        error "Token validation failed (HTTP $validation)"
        exit 1
    fi
}

# Enable local network webhooks in GitLab
enable_local_webhooks() {
    info "Enabling local network webhooks in GitLab..."

    local response
    response=$(curl -s -X PUT "$GITLAB_URL/api/v4/application/settings" \
        -H "PRIVATE-TOKEN: $GITLAB_ROOT_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"allow_local_requests_from_web_hooks_and_services": true}')

    local enabled
    enabled=$(echo "$response" | grep -o '"allow_local_requests_from_web_hooks_and_services":true' || true)

    if [ -n "$enabled" ]; then
        success "Local network webhooks enabled"
    else
        warn "Could not verify webhook setting (may already be enabled)"
    fi
}

# Create bot user
create_bot_user() {
    info "Creating bot user..."

    local response
    response=$(curl -s -X POST "$GITLAB_URL/api/v4/users" \
        -H "PRIVATE-TOKEN: $GITLAB_ROOT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$BOT_EMAIL\",
            \"username\": \"$BOT_USERNAME\",
            \"name\": \"Reviewer Roulette Bot\",
            \"password\": \"$BOT_PASSWORD\",
            \"skip_confirmation\": true
        }")

    BOT_USER_ID=$(echo "$response" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2 || true)

    if [ -z "$BOT_USER_ID" ]; then
        # Check if user already exists
        BOT_USER_ID=$(curl -s "$GITLAB_URL/api/v4/users?username=$BOT_USERNAME" \
            -H "PRIVATE-TOKEN: $GITLAB_ROOT_TOKEN" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2 || true)

        if [ -z "$BOT_USER_ID" ]; then
            error "Failed to create bot user"
            exit 1
        else
            success "Bot user already exists (ID: $BOT_USER_ID)"
        fi
    else
        success "Bot user created (ID: $BOT_USER_ID)"
    fi
}

# Create bot access token
create_bot_token() {
    info "Creating bot access token..."

    if [ -z "$BOT_USER_ID" ]; then
        error "Bot user ID not set"
        exit 1
    fi

    # Try API impersonation first
    info "Attempting to create token via API (impersonation)..."

    local pat_response
    pat_response=$(curl -s -X POST "$GITLAB_URL/api/v4/users/$BOT_USER_ID/impersonation_tokens" \
        -H "PRIVATE-TOKEN: $GITLAB_ROOT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"bot-access-token\",
            \"scopes\": [\"api\", \"read_api\", \"write_repository\"],
            \"expires_at\": \"$(get_future_date 365)\"
        }" 2>/dev/null)

    BOT_TOKEN=$(echo "$pat_response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4 || true)

    if [ -n "$BOT_TOKEN" ] && [ ${#BOT_TOKEN} -gt 20 ]; then
        success "Bot token created via API"
        return 0
    fi

    # Fallback: Manual token creation
    warn "Automatic bot token creation failed. Manual setup required."
    echo ""
    echo -e "${BLUE}Please follow these steps:${NC}"
    echo "1. Log in to GitLab as the bot user:"
    echo -e "   - URL: ${GREEN}$GITLAB_URL${NC}"
    echo -e "   - Username: ${GREEN}$BOT_USERNAME${NC}"
    echo -e "   - Password: ${GREEN}$BOT_PASSWORD${NC}"
    echo -e "2. Go to: ${GREEN}User Settings > Access Tokens${NC}"
    echo "3. Create a new token with:"
    echo -e "   - Name: ${GREEN}bot-access-token${NC}"
    echo -e "   - Scopes: ${GREEN}api, read_api, write_repository${NC}"
    echo -e "   - Expiration: ${GREEN}1 year from now${NC}"
    echo "4. Copy the generated token"
    echo ""

    read -p "$(echo -e ${YELLOW}Paste the bot access token here:${NC} )" BOT_TOKEN
    BOT_TOKEN=$(echo "$BOT_TOKEN" | tr -d '[:space:]')

    if [ -z "$BOT_TOKEN" ] || [ ${#BOT_TOKEN} -lt 20 ]; then
        error "Invalid token provided (too short)"
        exit 1
    fi

    # Test the token
    info "Validating bot token..."
    local validation
    validation=$(curl -s -o /dev/null -w "%{http_code}" "$GITLAB_URL/api/v4/user" \
        -H "PRIVATE-TOKEN: $BOT_TOKEN")

    if [ "$validation" = "200" ]; then
        success "Bot token validated successfully"
    else
        error "Bot token validation failed (HTTP $validation)"
        exit 1
    fi
}

# Generate webhook secret
generate_webhook_secret() {
    info "Generating webhook secret..."
    WEBHOOK_SECRET=$(openssl rand -hex 32)
    success "Webhook secret generated"
}

# Update config.yaml and create .env
update_config() {
    info "Updating configuration files..."

    local config_file="config.yaml"

    if [ ! -f "$config_file" ]; then
        warn "config.yaml not found, creating from example..."
        cp config.example.yaml config.yaml
    fi

    # Backup original
    cp config.yaml config.yaml.backup

    # Update using sed
    sed -i.tmp "s|url: .*|url: $GITLAB_URL|g" config.yaml
    sed -i.tmp "s|token: .*|token: $BOT_TOKEN|g" config.yaml
    sed -i.tmp "s|webhook_secret: .*|webhook_secret: $WEBHOOK_SECRET|g" config.yaml
    sed -i.tmp "s|bot_username: .*|bot_username: $BOT_USERNAME|g" config.yaml

    rm -f config.yaml.tmp

    success "config.yaml updated (backup saved as config.yaml.backup)"

    # Write .env file for docker-compose
    info "Writing .env file for docker-compose..."
    cat > .env <<EOF
# Generated by setup-gitlab.sh on $(date)
# This file is used by docker compose to pass secrets to the application

GITLAB_BOT_TOKEN=$BOT_TOKEN
GITLAB_WEBHOOK_SECRET=$WEBHOOK_SECRET

# Root token for optional test data generation
GITLAB_ROOT_TOKEN=$GITLAB_ROOT_TOKEN
EOF
    success ".env file created"
}

# ==================================================================================== #
# MINIMAL MODE
# ==================================================================================== #

run_minimal_mode() {
    echo ""
    echo -e "${BLUE}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║   GitLab Minimal Setup                                 ║"
    echo "║   Bot user + token + configuration                     ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    # Check if already configured
    if [ -f ".env" ] && grep -q "^GITLAB_ROOT_TOKEN=" .env && grep -q "^GITLAB_BOT_TOKEN=" .env; then
        info "Setup already complete (found .env with tokens)"
        info "To reconfigure, delete .env and run again"
        if [ "$AUTOMATED" = "false" ]; then
            echo ""
            echo -e "${BLUE}=== Next Steps ===${NC}"
            echo ""
            echo "  make migrate-up"
            echo "  make init"
            echo "  make run"
            echo ""
        fi
        return 0
    fi

    # Check prerequisites
    progress "Checking prerequisites..."

    if ! docker ps | grep -q gitlab; then
        error "GitLab container not running"
        info "Starting docker-compose..."
        docker compose up -d
    fi

    success "Docker containers running"

    if [ "$SKIP_WAIT" = false ]; then
        wait_for_gitlab
    else
        warn "Skipping GitLab readiness check (assuming already ready)"
    fi

    # Setup
    echo ""
    progress "Setting up GitLab configuration..."
    get_root_token
    enable_local_webhooks

    echo ""
    progress "Creating bot user..."
    create_bot_user
    create_bot_token

    echo ""
    progress "Generating webhook secret..."
    generate_webhook_secret

    echo ""
    progress "Updating configuration..."
    update_config

    # Summary
    echo ""
    echo -e "${GREEN}"
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║   Minimal Setup Complete! 🎉                           ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    if [ "$AUTOMATED" = "false" ]; then
        echo ""
        echo -e "${BLUE}=== Next Steps ===${NC}"
        echo ""
        echo "If you ran this via 'make setup-gitlab':"
        echo "  The Makefile will automatically run:"
        echo "  • Database migrations (make migrate-up)"
        echo "  • User sync (make init)"
        echo "  • Start server (make run)"
        echo ""
        echo "If you ran this script manually:"
        echo "  Continue with:"
        echo "    make migrate-up"
        echo "    make init"
        echo "    make run"
        echo ""
        echo -e "${YELLOW}For realistic test data, run: make setup-complete${NC}"
        echo ""
    fi
}

# ==================================================================================== #
# REALISTIC MODE (Test Data Creation)
# ==================================================================================== #

# Source: setup-gitlab-realistic.sh content
# This mode requires jq and creates comprehensive test data

run_realistic_mode() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║  GitLab Realistic Setup                                    ║"
    echo "║  Bot + 12 users + 5 projects + MRs + CODEOWNERS            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""

    # Validate prerequisites
    if ! command -v jq &> /dev/null; then
        error "jq is required but not installed. Install with: apt-get install jq / brew install jq"
        exit 1
    fi

    # Check if .env exists (bot already created)
    if [ -f ".env" ]; then
        # Load GITLAB_ROOT_TOKEN from .env if not already set
        if [ -z "$GITLAB_ROOT_TOKEN" ]; then
            GITLAB_ROOT_TOKEN=$(grep "^GITLAB_ROOT_TOKEN=" .env | cut -d'=' -f2)
        fi

        if [ -n "$GITLAB_ROOT_TOKEN" ]; then
            info "Bot user already configured (found .env with GITLAB_ROOT_TOKEN)"
            info "Skipping minimal setup, proceeding with test data creation..."

            # Ensure local webhooks are enabled
            enable_local_webhooks

            # Load webhook secret from config.yaml
            if [ -f "config.yaml" ]; then
                WEBHOOK_SECRET=$(grep "webhook_secret:" config.yaml | sed 's/.*webhook_secret:[[:space:]]*//' | tr -d '"' | tr -d "'")
            fi
        else
            # .env exists but no token, run minimal setup
            info ".env exists but GITLAB_ROOT_TOKEN not found, running minimal setup..."
            MODE="minimal"
            run_minimal_mode
            MODE="realistic"
        fi
    else
        # First run minimal setup to create bot user
        info "Running minimal setup first (bot user creation)..."
        echo ""
        MODE="minimal"
        run_minimal_mode
        MODE="realistic"

        echo ""
        echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
        echo ""
        info "Continuing with realistic test data creation..."
        echo ""

        # Load webhook secret from config.yaml
        if [ -f "config.yaml" ]; then
            WEBHOOK_SECRET=$(grep "webhook_secret:" config.yaml | sed 's/.*webhook_secret:[[:space:]]*//' | tr -d '"' | tr -d "'")
        fi
    fi

    if [ -z "$WEBHOOK_SECRET" ]; then
        warn "Webhook secret not found in config.yaml, webhooks will be created without secret token"
    fi

    # Test GitLab connection
    info "Testing GitLab connection at $GITLAB_URL..."
    if ! gitlab_api GET "/version" > /dev/null 2>&1; then
        error "Failed to connect to GitLab at $GITLAB_URL"
        exit 1
    fi
    success "Connected to GitLab"

    # Create test data
    create_test_group
    create_all_users
    add_users_to_group
    create_all_projects
    create_codeowners_files
    create_project_files
    create_all_labels
    create_all_webhooks
    create_all_merge_requests

    # Validate
    if [ "$DRY_RUN" != "true" ]; then
        validate_setup
    fi

    # Final summary
    echo ""
    success "Realistic setup complete!"
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  What Was Created                                          ║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "  • Bot user: $BOT_USERNAME"
    echo "  • Test organization group: test-org"
    echo "  • 12 users across 5 teams"
    echo "  • 5 projects with CODEOWNERS files"
    echo "  • 35+ labels (team, role, priority, type)"
    echo "  • Diverse file types (JS, Go, Python, YAML, etc.)"
    echo "  • 9 merge requests ready for testing"
    echo "  • Webhooks configured for all projects"
    echo ""
    if [ "$AUTOMATED" = "false" ]; then
        echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${CYAN}║  Next Steps                                                ║${NC}"
        echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "1. Run database migrations: ${GREEN}make migrate-up${NC}"
        echo -e "2. Sync users to database:  ${GREEN}make init${NC}"
        echo -e "3. Start the bot server:    ${GREEN}make run${NC}"
        echo -e "4. Test /roulette command in any MR at: ${GREEN}$GITLAB_URL/test-org${NC}"
        echo ""
    fi
}

# ==================================================================================== #
# REALISTIC MODE - TEST DATA CREATION FUNCTIONS
# ==================================================================================== #

# Get user ID by username (with caching)
get_user_id() {
    local username="$1"

    # Check cache first
    if [ -n "${USER_IDS[$username]}" ]; then
        echo "${USER_IDS[$username]}"
        return 0
    fi

    local response
    response=$(gitlab_api GET "/users?username=$username")
    local user_id
    user_id=$(echo "$response" | jq -r '.[0].id // empty')

    if [ -n "$user_id" ]; then
        USER_IDS[$username]=$user_id
        echo "$user_id"
    fi
}

# Get project ID by path (with caching)
get_project_id() {
    local project_path="$1"

    # Check cache first
    if [ -n "${PROJECT_IDS[$project_path]}" ]; then
        echo "${PROJECT_IDS[$project_path]}"
        return 0
    fi

    local response
    response=$(gitlab_api GET "/projects/test-org%2F$project_path")
    local project_id
    project_id=$(echo "$response" | jq -r '.id // empty')

    if [ -n "$project_id" ]; then
        PROJECT_IDS[$project_path]=$project_id
        echo "$project_id"
    fi
}

# Check if user exists
user_exists() {
    local username="$1"

    if [ -n "${USER_IDS[$username]}" ]; then
        return 0
    fi

    local response
    response=$(gitlab_api GET "/users?username=$username")

    if echo "$response" | jq -e '.[0].username' > /dev/null 2>&1; then
        USER_IDS[$username]=$(echo "$response" | jq -r '.[0].id')
        return 0
    fi

    return 1
}

# Check if project exists
project_exists() {
    local project_path="$1"

    if [ -n "${PROJECT_IDS[$project_path]}" ]; then
        return 0
    fi

    local response
    response=$(gitlab_api GET "/projects/test-org%2F$project_path")

    if echo "$response" | jq -e '.id' > /dev/null 2>&1; then
        PROJECT_IDS[$project_path]=$(echo "$response" | jq -r '.id')
        return 0
    fi

    return 1
}

# Wait for project repository to be initialized
wait_for_project_init() {
    local project_path="$1"
    local max_attempts=10
    local attempt=0

    local project_id
    project_id=$(get_project_id "$project_path")

    if [ -z "$project_id" ]; then
        return 1
    fi

    while [ $attempt -lt $max_attempts ]; do
        local response
        response=$(gitlab_api GET "/projects/$project_id/repository/branches/main" 2>/dev/null)

        if echo "$response" | jq -e '.name' > /dev/null 2>&1; then
            return 0
        fi

        sleep 1
        attempt=$((attempt + 1))
    done

    return 1
}

# Create test organization group
create_test_group() {
    local group_name="Test Organization"
    local group_path="test-org"

    info "Creating test organization group..."

    local response
    response=$(gitlab_api GET "/groups/$group_path")

    if echo "$response" | jq -e '.id' > /dev/null 2>&1; then
        GROUP_ID=$(echo "$response" | jq -r '.id')
        info "Group $group_path already exists (ID: $GROUP_ID)"
        return 0
    fi

    if [ "$DRY_RUN" = "true" ]; then
        progress "[DRY RUN] Would create group $group_path"
        GROUP_ID="1"
        return 0
    fi

    progress "Creating group $group_path"

    local group_data
    group_data=$(cat <<EOF
{
  "name": "$group_name",
  "path": "$group_path",
  "description": "Test organization for reviewer roulette bot",
  "visibility": "public"
}
EOF
)

    response=$(gitlab_api POST "/groups" "$group_data")
    GROUP_ID=$(echo "$response" | jq -r '.id // empty')

    if [ -n "$GROUP_ID" ]; then
        success "Created group $group_path (ID: $GROUP_ID)"
    else
        error "Failed to create group $group_path"
        exit 1
    fi
}

# Create a single user
create_user() {
    local username="$1"
    local name="$2"
    local email="$3"

    if user_exists "$username"; then
        info "User $username already exists (skipping)"
        return 0
    fi

    progress "Creating user $username ($name)"

    local user_data
    user_data=$(cat <<EOF
{
  "username": "$username",
  "name": "$name",
  "email": "$email",
  "password": "Password123!",
  "skip_confirmation": true,
  "admin": false
}
EOF
)

    local response
    response=$(gitlab_api POST "/users" "$user_data")

    if [ "$DRY_RUN" != "true" ]; then
        local user_id
        user_id=$(echo "$response" | jq -r '.id // empty')

        if [ -n "$user_id" ]; then
            USER_IDS[$username]=$user_id
            success "Created user $username (ID: $user_id)"
        else
            error "Failed to create user $username"
        fi
    fi
}

# Create all test users
create_all_users() {
    info "Creating 12 test users across 5 teams..."

    # Team Frontend
    create_user "alice" "Alice Anderson" "alice@example.com"
    create_user "bob" "Bob Brown" "bob@example.com"
    create_user "charlie" "Charlie Chen" "charlie@example.com"

    # Team Backend
    create_user "david" "David Davis" "david@example.com"
    create_user "eve" "Eve Evans" "eve@example.com"
    create_user "frank" "Frank Foster" "frank@example.com"

    # Team Platform
    create_user "grace" "Grace Green" "grace@example.com"
    create_user "henry" "Henry Hill" "henry@example.com"

    # Team Mobile
    create_user "isabel" "Isabel Ivanov" "isabel@example.com"
    create_user "jack" "Jack Johnson" "jack@example.com"

    # Team Data
    create_user "kate" "Kate Kim" "kate@example.com"
    create_user "leo" "Leo Lopez" "leo@example.com"

    success "User creation complete"
}

# Add users to test-org group
add_users_to_group() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would add all users to test-org group"
        return 0
    fi

    info "Adding users to test-org group..."

    local users=("alice" "bob" "charlie" "david" "eve" "frank" "grace" "henry" "isabel" "jack" "kate" "leo")
    local access_level=30  # Developer role

    for username in "${users[@]}"; do
        local user_id
        user_id=$(get_user_id "$username")

        if [ -z "$user_id" ]; then
            warn "User $username not found, skipping"
            continue
        fi

        progress "Adding $username to test-org group"

        local member_data
        member_data=$(cat <<EOF
{
  "user_id": $user_id,
  "access_level": $access_level
}
EOF
)

        gitlab_api POST "/groups/$GROUP_ID/members" "$member_data" > /dev/null || true
    done

    success "Users added to group"
}

# Create a single project
create_project() {
    local project_path="$1"
    local project_name="$2"
    local description="$3"

    if project_exists "$project_path"; then
        info "Project $project_path already exists (skipping)"
        return 0
    fi

    progress "Creating project $project_path"

    local project_data
    project_data=$(cat <<EOF
{
  "name": "$project_name",
  "path": "$project_path",
  "namespace_id": $GROUP_ID,
  "description": "$description",
  "visibility": "public",
  "initialize_with_readme": true
}
EOF
)

    local response
    response=$(gitlab_api POST "/projects" "$project_data")

    if [ "$DRY_RUN" != "true" ]; then
        local project_id
        project_id=$(echo "$response" | jq -r '.id // empty')

        if [ -n "$project_id" ]; then
            PROJECT_IDS[$project_path]=$project_id
            success "Created project $project_path (ID: $project_id)"
        else
            error "Failed to create project $project_path"
        fi
    fi
}

# Add member to project
add_project_member() {
    local project_path="$1"
    local username="$2"
    local access_level="$3"

    if [ "$DRY_RUN" = "true" ]; then
        return 0
    fi

    local project_id
    project_id=$(get_project_id "$project_path")

    if [ -z "$project_id" ]; then
        return 1
    fi

    local user_id
    user_id=$(get_user_id "$username")

    if [ -z "$user_id" ]; then
        return 1
    fi

    progress "Adding $username to $project_path"

    local member_data
    member_data=$(cat <<EOF
{
  "user_id": $user_id,
  "access_level": $access_level
}
EOF
)

    gitlab_api POST "/projects/$project_id/members" "$member_data" > /dev/null
}

# Create all projects
create_all_projects() {
    info "Creating 5 projects..."

    create_project "test-frontend" "Test Frontend" "Frontend application for testing reviewer roulette"
    create_project "test-backend" "Test Backend" "Backend API for testing reviewer roulette"
    create_project "test-platform" "Test Platform" "Platform infrastructure for testing reviewer roulette"
    create_project "test-mobile" "Test Mobile" "Mobile app for testing reviewer roulette"
    create_project "test-data" "Test Data" "Data pipeline for testing reviewer roulette"

    success "Project creation complete"

    info "Adding project members..."

    # Frontend
    add_project_member "test-frontend" "alice" 40
    add_project_member "test-frontend" "bob" 30
    add_project_member "test-frontend" "charlie" 30

    # Backend
    add_project_member "test-backend" "david" 40
    add_project_member "test-backend" "eve" 30
    add_project_member "test-backend" "frank" 30

    # Platform
    add_project_member "test-platform" "grace" 40
    add_project_member "test-platform" "henry" 30

    # Mobile
    add_project_member "test-mobile" "isabel" 40
    add_project_member "test-mobile" "jack" 30

    # Data
    add_project_member "test-data" "kate" 40
    add_project_member "test-data" "leo" 30

    success "Project members added"
}

# Create file in repository
create_file() {
    local project_path="$1"
    local file_path="$2"
    local content="$3"
    local commit_message="$4"
    local branch="${5:-main}"

    if [ "$DRY_RUN" = "true" ]; then
        return 0
    fi

    local project_id
    project_id=$(get_project_id "$project_path")

    if [ -z "$project_id" ]; then
        return 1
    fi

    local encoded_path
    encoded_path=$(echo -n "$file_path" | jq -Rr @uri)

    local file_data
    file_data=$(cat <<EOF
{
  "branch": "$branch",
  "content": $(echo "$content" | jq -sR .),
  "commit_message": "$commit_message"
}
EOF
)

    gitlab_api POST "/projects/$project_id/repository/files/$encoded_path" "$file_data" > /dev/null 2>&1 || true
}

# Create CODEOWNERS files
create_codeowners_files() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would create CODEOWNERS files"
        return 0
    fi

    info "Creating CODEOWNERS files..."

    # Wait for projects to initialize
    local projects=("test-frontend" "test-backend" "test-platform" "test-mobile" "test-data")
    for project in "${projects[@]}"; do
        wait_for_project_init "$project" || true
    done

    # Frontend CODEOWNERS
    create_file "test-frontend" "CODEOWNERS" "# Frontend CODEOWNERS
*.js @alice @bob
*.jsx @alice
*.css @charlie
/src/components/ @bob
* @alice" "Add CODEOWNERS file"

    # Backend CODEOWNERS
    create_file "test-backend" "CODEOWNERS" "# Backend CODEOWNERS
*.go @david @frank
/internal/ @frank
*.py @david
* @david" "Add CODEOWNERS file"

    # Platform CODEOWNERS
    create_file "test-platform" "CODEOWNERS" "# Platform CODEOWNERS
*.yml @grace
*.yaml @henry
Dockerfile* @grace @henry
*.sh @henry
* @grace" "Add CODEOWNERS file"

    # Mobile CODEOWNERS
    create_file "test-mobile" "CODEOWNERS" "# Mobile CODEOWNERS
*.java @isabel
*.swift @jack
*.tsx @isabel @jack
* @isabel" "Add CODEOWNERS file"

    # Data CODEOWNERS
    create_file "test-data" "CODEOWNERS" "# Data CODEOWNERS
*.py @kate
*.sql @leo
*.ipynb @kate
* @kate" "Add CODEOWNERS file"

    success "CODEOWNERS files created"
}

# Create project files
create_project_files() {
    if [ "$DRY_RUN" = "true" ]; then
        return 0
    fi

    info "Creating project files..."

    # Frontend
    create_file "test-frontend" "src/App.js" "import React from 'react';\nexport default function App() { return <div>Hello</div>; }" "Add App.js"

    # Backend
    create_file "test-backend" "main.go" "package main\nfunc main() { println(\"Hello\") }" "Add main.go"

    # Platform
    create_file "test-platform" "Dockerfile" "FROM alpine:latest\nRUN apk add --no-cache bash" "Add Dockerfile"

    # Mobile
    create_file "test-mobile" "App.swift" "import SwiftUI\nstruct App: View {}" "Add Swift app"

    # Data
    create_file "test-data" "pipeline.py" "import pandas as pd\n# Data pipeline" "Add pipeline"

    success "Project files created"
}

# Create labels
create_all_labels() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would create labels"
        return 0
    fi

    info "Creating labels..."

    local projects=("test-frontend" "test-backend" "test-platform" "test-mobile" "test-data")
    local teams=("team-frontend" "team-backend" "team-platform" "team-mobile" "team-data")

    for i in "${!projects[@]}"; do
        local project="${projects[$i]}"
        local team="${teams[$i]}"
        local project_id
        project_id=$(get_project_id "$project")

        if [ -n "$project_id" ]; then
            gitlab_api POST "/projects/$project_id/labels" "{\"name\":\"name::$team\",\"color\":\"#1f77b4\"}" > /dev/null || true
            gitlab_api POST "/projects/$project_id/labels" "{\"name\":\"dev\",\"color\":\"#2ecc71\"}" > /dev/null || true
            gitlab_api POST "/projects/$project_id/labels" "{\"name\":\"ops\",\"color\":\"#e67e22\"}" > /dev/null || true
        fi
    done

    success "Labels created"
}

# Create webhooks
create_all_webhooks() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would create webhooks"
        return 0
    fi

    info "Creating webhooks..."

    local projects=("test-frontend" "test-backend" "test-platform" "test-mobile" "test-data")

    for project in "${projects[@]}"; do
        local project_id
        project_id=$(get_project_id "$project")

        if [ -n "$project_id" ]; then
            progress "Creating webhook for $project"

            local webhook_data
            webhook_data="{\"url\":\"$WEBHOOK_URL\",\"token\":\"$WEBHOOK_SECRET\",\"merge_requests_events\":true,\"note_events\":true,\"confidential_note_events\":true,\"push_events\":false,\"enable_ssl_verification\":false}"

            gitlab_api POST "/projects/$project_id/hooks" "$webhook_data" > /dev/null || true
        fi
    done

    success "Webhooks created"
}

# Create branch
create_branch() {
    local project_path="$1"
    local branch_name="$2"

    if [ "$DRY_RUN" = "true" ]; then
        return 0
    fi

    local project_id
    project_id=$(get_project_id "$project_path")

    if [ -z "$project_id" ]; then
        return 1
    fi

    local branch_data
    branch_data="{\"branch\":\"$branch_name\",\"ref\":\"main\"}"

    gitlab_api POST "/projects/$project_id/repository/branches" "$branch_data" > /dev/null 2>&1 || true
}

# Create merge request
create_merge_request() {
    local project_path="$1"
    local source_branch="$2"
    local title="$3"
    local description="$4"
    shift 4
    local labels=("$@")

    if [ "$DRY_RUN" = "true" ]; then
        return 0
    fi

    local project_id
    project_id=$(get_project_id "$project_path")

    if [ -z "$project_id" ]; then
        return 1
    fi

    progress "Creating MR: $title"

    local labels_json
    labels_json=$(printf '%s\n' "${labels[@]}" | jq -R . | jq -s .)

    local mr_data
    mr_data=$(cat <<EOF
{
  "source_branch": "$source_branch",
  "target_branch": "main",
  "title": "$title",
  "description": "$description",
  "labels": $labels_json
}
EOF
)

    local response
    response=$(gitlab_api POST "/projects/$project_id/merge_requests" "$mr_data")

    local mr_iid
    mr_iid=$(echo "$response" | jq -r '.iid // empty')

    if [ -n "$mr_iid" ]; then
        success "Created MR !$mr_iid: $title"
    fi
}

# Create all merge requests
create_all_merge_requests() {
    if [ "$DRY_RUN" = "true" ]; then
        info "[DRY RUN] Would create merge requests"
        return 0
    fi

    info "Creating merge requests..."

    # Frontend MRs
    create_branch "test-frontend" "feature/add-login"
    create_file "test-frontend" "src/Login.jsx" "export const Login = () => <div>Login</div>;" "Add login" "feature/add-login"
    create_merge_request "test-frontend" "feature/add-login" "Add login component" "Implements user authentication UI" "name::team-frontend" "dev"

    create_branch "test-frontend" "bugfix/header-styling"
    create_file "test-frontend" "src/Header.css" ".header { padding: 20px; }" "Fix header" "bugfix/header-styling"
    create_merge_request "test-frontend" "bugfix/header-styling" "Fix header styling issues" "Fixes alignment and spacing in header component" "name::team-frontend" "type::bug" "priority::high"

    # Backend MRs
    create_branch "test-backend" "feature/add-auth"
    create_file "test-backend" "api/auth.go" "package api\nfunc AuthHandler() {}" "Add auth" "feature/add-auth"
    create_merge_request "test-backend" "feature/add-auth" "Add authentication endpoint" "Implements JWT auth" "name::team-backend" "dev"

    create_branch "test-backend" "feature/database-migration"
    create_file "test-backend" "migrations/001_users.sql" "CREATE TABLE users (id SERIAL PRIMARY KEY);" "Add migration" "feature/database-migration"
    create_merge_request "test-backend" "feature/database-migration" "Add user table migration" "Creates users table with proper indexes" "name::team-backend" "type::database-migration"

    # Platform MRs
    create_branch "test-platform" "feature/k8s"
    create_file "test-platform" "k8s/deployment.yaml" "apiVersion: apps/v1\nkind: Deployment" "Add k8s" "feature/k8s"
    create_merge_request "test-platform" "feature/k8s" "Add Kubernetes deployment" "Sets up k8s deployment" "name::team-platform" "ops"

    create_branch "test-platform" "feature/monitoring-stack"
    create_file "test-platform" "monitoring/prometheus.yaml" "global:\n  scrape_interval: 15s" "Add monitoring" "feature/monitoring-stack"
    create_merge_request "test-platform" "feature/monitoring-stack" "Setup monitoring with Prometheus" "Configures Prometheus and Grafana for production monitoring" "name::team-platform" "ops" "priority::high"

    # Mobile MRs
    create_branch "test-mobile" "feature/user-profile"
    create_file "test-mobile" "screens/UserProfile.tsx" "export const UserProfile = () => <View />" "Add profile" "feature/user-profile"
    create_merge_request "test-mobile" "feature/user-profile" "Add user profile screen" "Implements user profile with avatar and settings" "name::team-mobile" "dev"

    # Data MRs
    create_branch "test-data" "feature/etl-pipeline"
    create_file "test-data" "pipelines/etl.py" "def extract():\n    pass" "Add ETL" "feature/etl-pipeline"
    create_merge_request "test-data" "feature/etl-pipeline" "Implement ETL data pipeline" "Creates data extraction and transformation pipeline" "name::team-data" "dev"

    create_branch "test-data" "feature/analytics-dashboard"
    create_file "test-data" "queries/analytics.sql" "SELECT * FROM events" "Add analytics" "feature/analytics-dashboard"
    create_merge_request "test-data" "feature/analytics-dashboard" "Create analytics dashboard queries" "SQL queries for business metrics dashboard" "name::team-data" "ops"

    success "Merge requests created"
}

# Validate setup
validate_setup() {
    info "Validating test environment..."

    local user_count
    user_count=$(gitlab_api GET "/users" | jq '. | length')
    info "Total users: $user_count"

    local project_count
    project_count=$(gitlab_api GET "/projects" | jq '. | length')
    info "Total projects: $project_count"

    if [ "$user_count" -ge 12 ] && [ "$project_count" -ge 5 ]; then
        success "Validation passed!"
    else
        warn "Validation incomplete - some resources may not have been created"
    fi
}

# ==================================================================================== #
# MAIN
# ==================================================================================== #

main() {
    # Parse command-line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --minimal)
                MODE="minimal"
                shift
                ;;
            --realistic)
                MODE="realistic"
                shift
                ;;
            --skip-wait)
                SKIP_WAIT=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --automated)
                AUTOMATED=true
                shift
                ;;
            --help)
                show_help
                ;;
            *)
                error "Unknown option: $1"
                echo "Run with --help for usage information"
                exit 1
                ;;
        esac
    done

    # Validate mode selected
    if [ -z "$MODE" ]; then
        error "No mode specified. Use --minimal or --realistic"
        echo ""
        show_help
    fi

    # Run selected mode
    case $MODE in
        minimal)
            run_minimal_mode
            ;;
        realistic)
            run_realistic_mode
            ;;
        *)
            error "Invalid mode: $MODE"
            exit 1
            ;;
    esac
}

main "$@"
