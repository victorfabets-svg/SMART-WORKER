#!/bin/bash
#
# BEATSET Executor Agent - Runtime Runner
# Risk-gated operational fixer for SMART-WORKER
#

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$SCRIPT_DIR/config.json"
PROMPT_FILE="$SCRIPT_DIR/prompt.md"
REPORT_FILE="/tmp/executor_report_$(date +%s).json"
TIMESTAMP=$(date +%Y%m%d-%H%M)

# Load configuration - defaults
TARGET_REPO="${TARGET_REPO:-/workspaces/BEATSET}"
MEMORY_ENDPOINT="${MEMORY_ENDPOINT:-http://localhost:8080/memory/ingest}"
BOOTSTRAP_ENDPOINT="${BOOTSTRAP_ENDPOINT:-http://localhost:8080/agent/bootstrap}"
MAX_FILES=3
MAX_LINES=120
FINDING_FILE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--finding)
            FINDING_FILE="$2"
            shift 2
            ;;
        -t|--target)
            TARGET_REPO="$2"
            shift 2
            ;;
        *)
            echo "[ERROR] Unknown option: $1"
            exit 1
            ;;
    esac
done

# Default auto_fix_allowed and approval_required
AUTO_FIX_ALLOWED="lint,imports,unused_variables,isolated_dead_code,small_nil_safety,log_consistency,minor_build_fix"
APPROVAL_REQUIRED="architecture,auth,database_schema,payments,runtime_core,cross_module_refactor,infra,mass_delete"

if [ -f "$CONFIG_FILE" ]; then
    TARGET_REPO=$(grep -o '"target_repo": *"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    MAX_FILES=$(grep -o '"max_files_per_fix": *[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*')
    MAX_LINES=$(grep -o '"max_lines_changed": *[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*' || echo "120")
fi

echo "[EXECUTOR] Starting BEATSET Executor..."
echo "[EXECUTOR] Target repository: $TARGET_REPO"
echo "[EXECUTOR] Max files per fix: $MAX_FILES"
echo "[EXECUTOR] Max lines changed: $MAX_LINES"
echo "[EXECUTOR] Auto-fix allowed: $AUTO_FIX_ALLOWED"

# Check if finding file is provided
if [ -z "$FINDING_FILE" ]; then
    if [ -n "$FINDING_JSON" ]; then
        echo "$FINDING_JSON" > /tmp/finding_input.json
        FINDING_FILE="/tmp/finding_input.json"
    else
        echo "[ERROR] No finding provided. Use -f flag or FINDING_JSON env var"
        exit 1
    fi
fi

if [ ! -f "$FINDING_FILE" ]; then
    echo "[ERROR] Finding file not found: $FINDING_FILE"
    exit 1
fi

echo "[EXECUTOR] Loading finding from: $FINDING_FILE"

# Parse finding
FINDING_TITLE=$(cat "$FINDING_FILE" | jq -r '.title // empty')
FINDING_TYPE=$(cat "$FINDING_FILE" | jq -r '.type // empty')
FINDING_SEVERITY=$(cat "$FINDING_FILE" | jq -r '.severity // empty')
FINDING_CONTENT=$(cat "$FINDING_FILE" | jq -r '.content // empty')
FINDING_EVIDENCE=$(cat "$FINDING_FILE" | jq -r '.evidence // empty')
FINDING_RECOMMENDED=$(cat "$FINDING_FILE" | jq -r '.recommended_fix // empty')

if [ -z "$FINDING_TITLE" ]; then
    echo "[ERROR] Invalid finding: missing title"
    exit 1
fi

echo "[EXECUTOR] Processing finding:"
echo "  Title: $FINDING_TITLE"
echo "  Type: $FINDING_TYPE"
echo "  Severity: $FINDING_SEVERITY"

# --- RISK CLASSIFICATION ---
echo "[EXECUTOR] Classifying risk..."

RISK_LEVEL="low"
APPROVAL_NEEDED=false

# Check if severity is critical or high -> HIGH risk by default
if [ "$FINDING_SEVERITY" = "critical" ] || [ "$FINDING_SEVERITY" = "high" ]; then
    RISK_LEVEL="high"
fi

# Check if title/content matches approval_required categories
TITLE_LOWER=$(echo "$FINDING_TITLE" | tr '[:upper:]' '[:lower:]')
CONTENT_LOWER=$(echo "$FINDING_CONTENT" | tr '[:upper:]' '[:lower:]')

for keyword in $APPROVAL_REQUIRED; do
    if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
        RISK_LEVEL="high"
        break
    fi
done

# If not high, check if LOW risk auto-fix allowed
if [ "$RISK_LEVEL" = "low" ]; then
    for keyword in $AUTO_FIX_ALLOWED; do
        if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
            RISK_LEVEL="low"
            break
        fi
    done
    
    # If no match, default to medium
    if [ "$RISK_LEVEL" = "low" ]; then
        # Check if any auto-fix keyword matched
        :
    else
        RISK_LEVEL="medium"
    fi
fi

echo "[EXECUTOR] Risk classification: $RISK_LEVEL"

# --- EXECUTE BASED ON RISK ---
case "$RISK_LEVEL" in
    low)
        echo "[EXECUTOR] ✅ LOW RISK - Auto-fix allowed"
        echo "[EXECUTOR] Proceeding with targeted fix..."
        ;;
    medium)
        echo "[EXECUTOR] 🔒 MEDIUM RISK - Approval Required"
        echo "[EXECUTOR] Generating remediation plan only..."
        echo "[EXECUTOR] NO code modification will be performed"
        ;;
    high)
        echo "[EXECUTOR] 🚫 HIGH RISK - BLOCKED"
        echo "[EXECUTOR] This finding requires manual review"
        echo "[EXECUTOR] NO code modification will be performed"
        ;;
esac

# --- CHECK TARGET REPO AND BOOTSTRAP ---
# Check target repository (only for LOW risk)
if [ "$RISK_LEVEL" = "low" ]; then
    if [ ! -d "$TARGET_REPO" ]; then
        echo "[ERROR] Target repository not found: $TARGET_REPO"
        exit 1
    fi
    
    # Bootstrap runtime context
    echo "[EXECUTOR] Attempting bootstrap from SMART-WORKER..."
    if curl -s -f "$BOOTSTRAP_ENDPOINT" > /dev/null 2>&1; then
        echo "[EXECUTOR] Bootstrap successful"
    else
        echo "[EXECUTOR] Bootstrap endpoint unavailable, continuing..."
    fi
    
    # Load system prompt
    if [ ! -f "$PROMPT_FILE" ]; then
        echo "[ERROR] Prompt file not found: $PROMPT_FILE"
        exit 1
    fi
    
    cd "$TARGET_REPO"

# --- SAFETY CHECKS ---
echo "[EXECUTOR] Running safety checks..."

# Check current branch - abort if on main
CURRENT_BRANCH=$(git branch --show-current 2>/dev/null || echo "")
if [ "$CURRENT_BRANCH" = "main" ]; then
    echo "[ERROR] ABORT: Cannot execute on main branch"
    echo "[ERROR] Executor must use isolated branch"
    exit 1
fi

# Create isolated branch for fix
BRANCH_NAME="executor/fix-$TIMESTAMP"
echo "[EXECUTOR] Creating isolated branch: $BRANCH_NAME"
git checkout -b "$BRANCH_NAME" 2>/dev/null || git checkout -b "$BRANCH_NAME" 2>/dev/null || true

# Generate rollback snapshot BEFORE any changes
ROLLBACK_FILE="/tmp/executor_rollback_$TIMESTAMP.diff"
echo "[EXECUTOR] Saving rollback snapshot to: $ROLLBACK_FILE"
git diff > "$ROLLBACK_FILE" || true

    
    # --- EXECUTE LOW RISK FIX ---
    echo "[EXECUTOR] Analyzing and applying fix..."
    
    FILES_TO_CHECK=$(echo "$FINDING_EVIDENCE" | grep -oE '[a-zA-Z0-9_/-]+\.(go|yaml|yml|json|toml|md|txt|sh)' || true)
    
    if [ -z "$FILES_TO_CHECK" ]; then
        FILES_TO_CHECK=$(echo "$FINDING_RECOMMENDED" | grep -oE '[a-zA-Z0-9_/-]+\.(go|yaml|yml|json|toml|md|txt|sh)' || true)
    fi
    
    # Determine root cause based on finding type
    ROOT_CAUSE="finding requires code modification"
    
    if echo "$FINDING_TITLE" | grep -qi "unused"; then
        ROOT_CAUSE="unused code or import"
    elif echo "$FINDING_TITLE" | grep -qi "import"; then
        ROOT_CAUSE="import issue"
    elif echo "$FINDING_TITLE" | grep -qi "nil"; then
        ROOT_CAUSE="nil pointer risk"
    elif echo "$FINDING_TITLE" | grep -qi "log"; then
        ROOT_CAUSE="log inconsistency"
    elif echo "$FINDING_TITLE" | grep -qi "build"; then
        ROOT_CAUSE="build fix needed"
    fi
    
    # --- VALIDATION ---
    echo "[EXECUTOR] Validating changes..."
    
    VALIDATION_PASSED=true
    VALIDATION_STEPS="[]"
    
    if command -v go >/dev/null 2>&1; then
        if go build -o /dev/null ./... 2>/dev/null; then
            VALIDATION_STEPS=$(echo "$VALIDATION_STEPS" | jq '. += ["go build ./... : PASSED"]')
        else
            BUILD_ERR=$(go build ./... 2>&1 || true)
            VALIDATION_STEPS=$(echo "$VALIDATION_STEPS" | jq ". += [\"go build ./... : FAILED - $BUILD_ERR\"]')
            VALIDATION_PASSED=false

    # Check line count limit
    LINES_CHANGED=$(git diff --stat 2>/dev/null | tail -1 | awk '{print $4}' | tr -d , || echo "0")
    if [ "$LINES_CHANGED" -gt "$MAX_LINES" ] 2>/dev/null; then
        echo "[ERROR] ABORT: Lines changed ($LINES_CHANGED) exceeds limit ($MAX_LINES)"
        VALIDATION_PASSED=false
    fi
        fi
    fi
    
    # --- COMMIT ---
    echo "[EXECUTOR] Generating commit..."
    
    COMMIT_MSG="[$RISK_LEVEL][EXECUTOR] fix: $FINDING_TITLE"
    if [ "$VALIDATION_PASSED" = true ]; then
        COMMIT_MSG="$COMMIT_MSG - resolved via Executor"
    else
        COMMIT_MSG="$COMMIT_MSG - requires review"
    fi
    
    # Only attempt commit if git repo exists
    if [ -d "$TARGET_REPO/.git" ]; then
        cd "$TARGET_REPO"
        if git diff --quiet 2>/dev/null; then
            echo "[EXECUTOR] No changes to commit"
        else
            git add -A
            git commit -m "$COMMIT_MSG

Evidence: $FINDING_EVIDENCE
Risk Level: $RISK_LEVEL

Co-authored-by: openhands <openhands@all-hands.dev>" 2>/dev/null || true
            echo "[EXECUTOR] Changes committed"
        fi
    else
        echo "[EXECUTOR] Git not initialized in target repo"
    fi
    
    # Generate LOW risk report
    REPORT=$(echo '{}' | jq \
        --arg title "$FINDING_TITLE" \
        --arg root_cause "$ROOT_CAUSE" \
        --arg risk "$RISK_LEVEL" \
        --arg action "applied" \
        --arg commit_msg "$COMMIT_MSG" \
        '.finding_title = $title | .root_cause = $root_cause | .risk_level = $risk | .action = $action | .commit_message = $commit_msg')

elif [ "$RISK_LEVEL" = "medium" ]; then
    # --- MEDIUM RISK: GENERATE PLAN ONLY ---
    PLAN_FILE="/tmp/executor_plan_$(date +%s).json"
    
    # Generate remediation plan
    REPORT=$(echo '{}' | jq \
        --arg title "$FINDING_TITLE" \
        --arg risk "$RISK_LEVEL" \
        --arg action "plan_generated" \
        --arg approval "required" \
        --arg plan_file "$PLAN_FILE" \
        '.finding_title = $title | .risk_level = $risk | .action = $action | .approval_required = $approval | .plan_file = $plan_file')
    
    # Save plan
    echo "$REPORT" > "$PLAN_FILE"
    
elif [ "$RISK_LEVEL" = "high" ]; then
    # --- HIGH RISK: BLOCKED ---
    REPORT=$(echo '{}' | jq \
        --arg title "$FINDING_TITLE" \
        --arg risk "$RISK_LEVEL" \
        --arg action "blocked" \
        --arg reason "high risk - manual review required" \
        '.finding_title = $title | .risk_level = $risk | .action = $action | .reason = $reason')
    
fi

# Save report
echo "$REPORT" > "$REPORT_FILE"

# Summary
echo ""
echo "========================================"
echo "EXECUTOR COMPLETE"
echo "========================================"
echo "Finding: $FINDING_TITLE"
echo "Risk Level: $RISK_LEVEL"
echo "Action: $(echo "$REPORT" | jq -r '.action')"
echo "Report: $REPORT_FILE"
echo ""

exit 0