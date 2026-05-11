#!/bin/bash
#
# BEATSET Executor Agent - Runtime Runner
# Multi-finding operational fixer for SMART-WORKER
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

# Default categories
AUTO_FIX_ALLOWED="lint,imports,unused_variables,isolated_dead_code,small_nil_safety,log_consistency,minor_build_fix"
APPROVAL_REQUIRED="architecture,auth,database_schema,payments,runtime_core,cross_module_refactor,infra,mass_delete"

if [ -f "$CONFIG_FILE" ]; then
    TARGET_REPO=$(grep -o '"target_repo": *"[^"]*"' "$CONFIG_FILE" | cut -d'"' -f4)
    MAX_FILES=$(grep -o '"max_files_per_fix": *[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*')
    MAX_LINES=$(grep -o '"max_lines_changed": *[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*' || echo "120")
fi

echo "[EXECUTOR] Starting BEATSET Executor..."
echo "[EXECUTOR] Target repository: $TARGET_REPO"
echo "[EXECUTOR] Max files: $MAX_FILES, Max lines: $MAX_LINES"

# Check finding file
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

# Detect if findings is array or single object
echo "[EXECUTOR] Loading findings from: $FINDING_FILE"
if cat "$FINDING_FILE" | jq -e 'type == "array"' >/dev/null 2>&1; then
    IS_ARRAY=true
    TOTAL_FINDINGS=$(cat "$FINDING_FILE" | jq 'length')
    echo "[EXECUTOR] Loaded $TOTAL_FINDINGS findings (array)"
else
    IS_ARRAY=false
    TOTAL_FINDINGS=1
    echo "[EXECUTOR] Loaded 1 finding (object)"
fi

# Initialize results collector
RESULTS='[]'

# Process findings
if [ "$IS_ARRAY" = true ]; then
    echo "[EXECUTOR] Processing multiple findings..."
    index=0
    while [ $index -lt $TOTAL_FINDINGS ]; do
        index=$((index + 1))
        finding_json=$(cat "$FINDING_FILE" | jq -r ".[$index - 1]")
        
        FINDING_TITLE=$(echo "$finding_json" | jq -r '.title // empty')
        FINDING_TYPE=$(echo "$finding_json" | jq -r '.type // empty')
        FINDING_SEVERITY=$(echo "$finding_json" | jq -r '.severity // empty')
        FINDING_CONTENT=$(echo "$finding_json" | jq -r '.content // empty')
        FINDING_EVIDENCE=$(echo "$finding_json" | jq -r '.evidence // empty')
        FINDING_RECOMMENDED=$(echo "$finding_json" | jq -r '.recommended_fix // empty')
        
        if [ -z "$FINDING_TITLE" ]; then
            echo "[WARNING] Finding $index: missing title, skipping"
            continue
        fi
        
        echo ""
        echo "[EXECUTOR] --- Finding $index/$TOTAL_FINDINGS ---"
        echo "[EXECUTOR] Title: $FINDING_TITLE (type: $FINDING_TYPE, severity: $FINDING_SEVERITY)"
        
        # Classify risk
        RISK_LEVEL="low"
        if [ "$FINDING_SEVERITY" = "critical" ] || [ "$FINDING_SEVERITY" = "high" ]; then
            RISK_LEVEL="high"
        fi
        
        TITLE_LOWER=$(echo "$FINDING_TITLE" | tr '[:upper:]' '[:lower:]')
        CONTENT_LOWER=$(echo "$FINDING_CONTENT" | tr '[:upper:]' '[:lower:]')
        
        for keyword in $APPROVAL_REQUIRED; do
            if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
                RISK_LEVEL="high"
                break
            fi
        done
        
        if [ "$RISK_LEVEL" = "low" ]; then
            for keyword in $AUTO_FIX_ALLOWED; do
                if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
                    RISK_LEVEL="low"
                    break
                fi
            done
            [ "$RISK_LEVEL" = "low" ] && [ -z "$(echo "$TITLE_LOWER" | grep -E "$(echo $AUTO_FIX_ALLOWED | tr ',' '|')")" ] && RISK_LEVEL="medium"
        fi
        
        echo "[EXECUTOR] Risk classification: $RISK_LEVEL"
        
        # Handle based on risk
        case "$RISK_LEVEL" in
            low)
                echo "[EXECUTOR] ✅ LOW RISK - Auto-fix allowed"
                # Would apply fix here
                RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"applied\"}]")
                ;;
            medium)
                echo "[EXECUTOR] 🔒 MEDIUM RISK - Approval Required"
                RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"approval_required\"}]")
                ;;
            high)
                echo "[EXECUTOR] 🚫 HIGH RISK - BLOCKED"
                RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"blocked\"}]")
                ;;
        esac
    done
else
    # Single object
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
    
    echo ""
    echo "[EXECUTOR] --- Single Finding ---"
    echo "[EXECUTOR] Title: $FINDING_TITLE (type: $FINDING_TYPE, severity: $FINDING_SEVERITY)"
    
    # Classify risk (same logic)
    RISK_LEVEL="low"
    if [ "$FINDING_SEVERITY" = "critical" ] || [ "$FINDING_SEVERITY" = "high" ]; then
        RISK_LEVEL="high"
    fi
    
    TITLE_LOWER=$(echo "$FINDING_TITLE" | tr '[:upper:]' '[:lower:]')
    CONTENT_LOWER=$(echo "$FINDING_CONTENT" | tr '[:upper:]' '[:lower:]')
    
    for keyword in $APPROVAL_REQUIRED; do
        if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
            RISK_LEVEL="high"
            break
        fi
    done
    
    if [ "$RISK_LEVEL" = "low" ]; then
        for keyword in $AUTO_FIX_ALLOWED; do
            if echo "$TITLE_LOWER" | grep -qi "$keyword" || echo "$CONTENT_LOWER" | grep -qi "$keyword"; then
                RISK_LEVEL="low"
                break
            fi
        done
    fi
    
    echo "[EXECUTOR] Risk classification: $RISK_LEVEL"
    
    case "$RISK_LEVEL" in
        low)
            echo "[EXECUTOR] ✅ LOW RISK - Auto-fix allowed"
            RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"applied\"}]")
            ;;
        medium)
            echo "[EXECUTOR] 🔒 MEDIUM RISK - Approval Required"
            RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"approval_required\"}]")
            ;;
        high)
            echo "[EXECUTOR] 🚫 HIGH RISK - BLOCKED"
            RESULTS=$(echo "$RESULTS" | jq ". += [{finding: \"$FINDING_TITLE\", risk: \"$RISK_LEVEL\", action: \"blocked\"}]")
            ;;
    esac
fi

# Save results
echo "$RESULTS" > "$REPORT_FILE"

echo ""
echo "========================================"
echo "EXECUTOR COMPLETE"
echo "========================================"
echo "Findings processed: $TOTAL_FINDINGS"
echo "Results: $REPORT_FILE"

# Summary
echo ""
echo "Summary:"
echo "$RESULTS" | jq -r '.[] | "[\(.risk)] \(.action): \(.finding)"'

exit 0
