#!/bin/bash
#
# Telegram Conversational Runtime Startup Script
# Exports required environment variables before launching the bot
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "================================================"
echo "Telegram Conversational Runtime"
echo "================================================"

# Load .env file if exists
ENV_FILE="$PROJECT_ROOT/integrations/telegram/.env"
if [ -f "$ENV_FILE" ]; then
    echo "Loading environment from: $ENV_FILE"
    set -a
    source "$ENV_FILE"
    set +a
else
    echo "No .env file at: $ENV_FILE"
fi

# Check if OPENHANDS_API_KEY is available
if [ -z "$OPENHANDS_API_KEY" ]; then
    echo "ERROR: OPENHANDS_API_KEY not set in environment"
    echo ""
    echo "Please ensure OPENHANDS_API_KEY is available:"
    echo "  1. Set in current shell: export OPENHANDS_API_KEY=..."
    echo "  2. Add to .env file"
    echo "  3. Add to docker-compose environment"
    exit 1
fi

echo "Environment check:"
echo "  OPENHANDS_API_KEY: set (length=${#OPENHANDS_API_KEY})"
if [ -n "$GITHUB_REPOSITORY" ]; then
    echo "  GITHUB_REPOSITORY: $GITHUB_REPOSITORY"
fi
echo "================================================"

# Run the bot
cd "$PROJECT_ROOT"
exec go run ./integrations/telegram/cmd/bot "$@"
