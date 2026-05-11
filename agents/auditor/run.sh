#!/bin/bash
#
# BEATSET Auditor Agent - Runtime Runner
# Single operational auditor for SMART-WORKER
#

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.json"
PROMPT_FILE="$SCRIPT_DIR/prompt.md"
FINDINGS_FILE="/tmp/auditor_findings_$(date +%s).json"

# Load configuration - defaults
TARGET_REPO="${TARGET_REPO:-/workspaces/BEATSET}"
MEMORY_ENDPOINT="${MEMORY_ENDPOINT:-http://localhost:8080/memory/ingest}"
BOOTSTRAP_ENDPOINT="${BOOTSTRAP_ENDPOINT:-http://localhost:8080/agent/bootstrap}"
MAX_FINDINGS=20

if [ -f "$CONFIG_FILE" ]; then
    TARGET_REPO=$(grep -o '"target_repo": *"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    MEMORY_ENDPOINT=$(grep -o '"memory_endpoint": *"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    BOOTSTRAP_ENDPOINT=$(grep -o '"bootstrap_endpoint": *"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    MAX_FINDINGS=$(grep -o '"max_findings_per_run": *[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*')
fi

echo "[AUDITOR] Starting BEATSET audit..."
echo "[AUDITOR] Target repository: $TARGET_REPO"
echo "[AUDITOR] Memory endpoint: $MEMORY_ENDPOINT"
echo "[AUDITOR] Max findings: $MAX_FINDINGS"

# Bootstrap runtime context (if endpoint available)
echo "[AUDITOR] Attempting bootstrap from SMART-WORKER..."
if curl -s -f "$BOOTSTRAP_ENDPOINT" > /dev/null 2>&1; then
    BOOTSTRAP_DATA=$(curl -s "$BOOTSTRAP_ENDPOINT")
    echo "[AUDITOR] Bootstrap successful"
else
    echo "[AUDITOR] Bootstrap endpoint unavailable, continuing..."
fi

# Load system prompt
if [ ! -f "$PROMPT_FILE" ]; then
    echo "[ERROR] Prompt file not found: $PROMPT_FILE"
    exit 1
fi

echo "[AUDITOR] Loading system prompt from $PROMPT_FILE"

# Execute audit cycle
echo "[AUDITOR] Executing audit cycle..."

# Initialize findings array
FINDINGS="[]"

# --- AUDIT: Repository Structure ---
echo "[AUDITOR] Inspecting repository structure..."
if [ -d "$TARGET_REPO/cmd" ]; then
    # Check for missing main.go in cmd directories
    for dir in "$TARGET_REPO"/cmd/*/; do
        if [ -d "$dir" ] && [ ! -f "$dir/main.go" ]; then
            FINDINGS=$(echo "$FINDINGS" | jq --arg p "$dir" \
                '. += [{"type":"violation","severity":"low","title":"Missing main.go in cmd directory","content":"Command directory missing entry point","evidence":$p,"recommended_fix":"Add main.go or verify correct structure"}]')
        fi
    done
fi

# --- AUDIT: Go Runtime ---
echo "[AUDITOR] Inspecting Go runtime..."
cd "$TARGET_REPO"

# Check for compilation errors
if command -v go >/dev/null 2>&1; then
    if ! go build -o /dev/null ./... 2>/dev/null; then
        BUILD_OUTPUT=$(go build ./... 2>&1 || true)
        FINDINGS=$(echo "$FINDINGS" | jq --arg o "$BUILD_OUTPUT" \
            '. += [{"type":"bug","severity":"critical","title":"Go compilation failure","content":"Code fails to compile","evidence":$o,"recommended_fix":"Fix compilation errors"}]')
    fi
fi

# Check for common Go issues in source files
while IFS= read -r -d '' file; do
    # Check for unclosed resources
    if grep -q 'os.Open.*defer.*Close' "$file" 2>/dev/null; then
        : # Defer pattern found
    elif grep -q 'os.Open(' "$file" 2>/dev/null; then
        FINDINGS=$(echo "$FINDINGS" | jq --arg f "$file" \
            '. += [{"type":"risk","severity":"medium","title":"Potential unclosed resource","content":"os.Open without clear defer close pattern","evidence":$f,"recommended_fix":"Ensure resource is properly closed"}]')
    fi
    
    # Check for nil pointer risks
    if grep -q '\*.*\.Method\(' "$file" 2>/dev/null; then
        # Check for methods called without nil check
        if ! grep -q 'if.*== nil' "$file" 2>/dev/null; then
            FINDINGS=$(echo "$FINDINGS" | jq --arg f "$file" \
                '. += [{"type":"risk","severity":"medium","title":"Potential nil pointer dereference","content":"Method called without nil check","evidence":$f,"recommended_fix":"Add nil check before method call"}]')
        fi
    fi
done < <(find "$TARGET_REPO" -name "*.go" -type f -print0 2>/dev/null)

# --- AUDIT: Dead Code ---
echo "[AUDITOR] Checking for dead code..."

# Find unused functions
if command -v go >/dev/null 2>&1; then
    for pkg in "$TARGET_REPO"/internal/*/; do
        if [ -d "$pkg" ]; then
            PKG_NAME=$(basename "$pkg")
            UNUSED=$(go build -v ./... 2>&1 | grep -i "unused" || true)
            if [ -n "$UNUSED" ]; then
                FINDINGS=$(echo "$FINDINGS" | jq --arg p "$PKG_NAME" --arg u "$UNUSED" \
                    '. += [{"type":"dead_code","severity":"low","title":"Possibly unused code in package","content":"Detected unused code","evidence":$p,"recommended_fix":"Review and remove unused code"}]')
            fi
        fi
    done
fi

# --- AUDIT: Configuration ---
echo "[AUDITOR] Checking configuration..."

# Check for hardcoded values
while IFS= read -r -d '' file; do
    # Check for hardcoded secrets/keys
    if grep -qE 'password\s*=\s*"[^"]+' "$file" 2>/dev/null; then
        FINDINGS=$(echo "$FINDINGS" | jq --arg f "$file" \
            '. += [{"type":"risk","severity":"critical","title":"Hardcoded password detected","content":"Password found in source code","evidence":$f,"recommended_fix":"Move to environment variables or secrets manager"}]')
    fi
    
    # Check for hardcoded API keys
    if grep -qE 'api[_-]?key\s*=\s*"[^"]+' "$file" 2>/dev/null; then
        FINDINGS=$(echo "$FINDINGS" | jq --arg f "$file" \
            '. += [{"type":"risk","severity":"critical","title":"Hardcoded API key detected","content":"API key found in source code","evidence":$f,"recommended_fix":"Move to environment variables"}]')
    fi
done < <(find "$TARGET_REPO" -name "*.go" -type f -print0 2>/dev/null)

# --- AUDIT: CI/CD ---
echo "[AUDITOR] Inspecting CI/CD..."

if [ -d "$TARGET_REPO/.github" ]; then
    # Check for workflow issues
    for wf in "$TARGET_REPO/.github/workflows/"*.yml "$TARGET_REPO/.github/workflows/"*.yaml; do
        if [ -f "$wf" ]; then
            # Check for missing 'on' trigger
            if ! grep -q '^on:' "$wf" 2>/dev/null; then
                FINDINGS=$(echo "$FINDINGS" | jq --arg f "$wf" \
                    '. += [{"type":"violation","severity":"medium","title":"Missing CI trigger","content":"Workflow missing trigger configuration","evidence":$f,"recommended_fix":"Add 'on' trigger"}]')
            fi
        fi
    done
fi

# --- AUDIT: Operational Inconsistencies ---
echo "[AUDITOR] Checking operational inconsistencies..."

# Check .env.example vs actual usage
if [ -f "$TARGET_REPO/.env.example" ]; then
    # Check if required env vars are documented
    while IFS= read -r -d '' file; do
        if grep -qE 'os.Getenv\(' "$file" 2>/dev/null; then
            ENV_VARS=$(grep -oE 'os.Getenv\("[^"]+"' "$file" | cut -d'"' -f2 || true)
            if [ -n "$ENV_VARS" ]; then
                for var in $ENV_VARS; do
                    if ! grep -q "$var" "$TARGET_REPO/.env.example" 2>/dev/null; then
                        FINDINGS=$(echo "$FINDINGS" | jq --arg v "$var" --arg f "$file" \
                            '. += [{"type":"inconsistency","severity":"medium","title":"Undocumented environment variable","content":"Variable used but not in .env.example","evidence":$v,"recommended_fix":"Add to .env.example"}]')
                    fi
                done
            fi
        fi
    done < <(find "$TARGET_REPO" -name "*.go" -type f -print0 2>/dev/null)
fi

# --- Limit findings ---
echo "[AUDITOR] Limiting findings to $MAX_FINDINGS..."
FINDINGS=$(echo "$FINDINGS" | jq "limit($MAX_FINDINGS; .)")

# Extract findings count
FINDINGS_COUNT=$(echo "$FINDINGS" | jq 'length')
echo "[AUDITOR] Found $FINDINGS_COUNT findings"

# Save findings to file
echo "$FINDINGS" > "$FINDINGS_FILE"
echo "[AUDITOR] Findings saved to $FINDINGS_FILE"

# Send to memory endpoint
echo "[AUDITOR] Sending findings to memory endpoint..."
if curl -s -f -X POST "$MEMORY_ENDPOINT" \
    -H "Content-Type: application/json" \
    -d @"$FINDINGS_FILE" > /dev/null 2>&1; then
    echo "[AUDITOR] Findings ingested successfully"
else
    echo "[AUDITOR] Memory endpoint unavailable, findings saved locally"
fi

# Summary output
echo ""
echo "========================================"
echo "AUDIT COMPLETE"
echo "========================================"
echo "Total findings: $FINDINGS_COUNT"
echo "Findings file: $FINDINGS_FILE"
echo ""

# Print findings summary
if [ "$FINDINGS_COUNT" -gt 0 ]; then
    echo "Finding Summary:"
    echo "$FINDINGS" | jq -r '.[] | "[\(.severity)] \(.title)"'
fi

exit 0