#!/bin/bash
#
# BEATSET Continuous Runtime
# Persistent operational loop for SMART-WORKER agents
#
# Usage:
#   ./runtime.sh start    - Start continuous runtime
#   ./runtime.sh stop    - Stop runtime
#   ./runtime.sh status - Show runtime status
#   ./runtime.sh once   - Run single audit cycle
#

set -e

SCRIPT_DIR="/workspaces/SMART-WORKER/integrations/telegram/runtime"
RUNTIME_DIR="/workspaces/SMART-WORKER"
AUDITOR_SCRIPT="/workspaces/SMART-WORKER/agents/auditor/run.sh"
EXECUTOR_SCRIPT="/workspaces/SMART-WORKER/agents/executor/run.sh"
STATE_FILE="/tmp/runtime_state.json"
PID_FILE="/tmp/runtime.pid"
LOG_FILE="/tmp/runtime.log"

# Load environment
ENV_FILE="/workspaces/SMART-WORKER/integrations/telegram/.env"
if [ -f "$ENV_FILE" ]; then
    set -a
    source "$ENV_FILE"
    set +a
fi

TICKER_INTERVAL=300  # 5 minutes in seconds
FINDINGS_DIR="/tmp"

# Validate configuration
log_validate() {
    if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
        log "Telegram configured successfully (BOT: ${TELEGRAM_BOT_TOKEN:0:8}...)"
    else
        log "Telegram not fully configured - notifications disabled"
        [ -z "$TELEGRAM_BOT_TOKEN" ] && log "Missing TELEGRAM_BOT_TOKEN"
        [ -z "$TELEGRAM_CHAT_ID" ] && log "Missing TELEGRAM_CHAT_ID"
    fi
    
    if [ -n "$SMART_WORKER_URL" ]; then
        log "SMART_WORKER_URL: $SMART_WORKER_URL"
    fi
}

log() {
    echo "[RUNTIME] $(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

save_state() {
    cat > "$STATE_FILE" << STATE_EOF
{
  "last_audit_run": "$(date -Iseconds)",
  "runtime_pid": $(cat "$PID_FILE" 2>/dev/null || echo "0"),
  "findings_count": $FINDINGS_COUNT,
  "new_findings_count": $NEW_FINDINGS_COUNT,
  "last_notification": "$LAST_NOTIFICATION"
}
STATE_EOF
}

load_state() {
    if [ -f "$STATE_FILE" ]; then
        LAST_AUDIT_RUN=$(grep -o '"last_audit_run": *"[^"]*"' "$STATE_FILE" | cut -d'"' -f4)
        FINDINGS_COUNT=$(grep -o '"findings_count": *[0-9]*' "$STATE_FILE" | grep -o '[0-9]*')
    fi
}

run_audit() {
    log "Running audit..."
    
    # Run auditor
    bash "$AUDITOR_SCRIPT" 2>&1 | tee -a "$LOG_FILE"
    
    # Find latest findings
    find_latest_findings
}

find_latest_findings() {
    local latest=""
    local latest_time=0
    
    for f in "$FINDINGS_DIR"/auditor_findings_*.json; do
        [ -f "$f" ] || continue
        mtime=$(stat -c %Y "$f" 2>/dev/null || stat -f %m "$f" 2>/dev/null)
        if [ "$mtime" -gt "$latest_time" ]; then
            latest_time=$mtime
            latest="$f"
        fi
    done
    
    FINDINGS_FILE="$latest"
}

check_new_findings() {
    find_latest_findings
    
    if [ -z "$FINDINGS_FILE" ]; then
        return 1
    fi
    
    local hash=$(sha256sum "$FINDINGS_FILE" 2>/dev/null | cut -d' ' -f1)
    
    # Check against known hash
    if [ "$KNOWN_FINDINGS_HASH" = "$hash" ]; then
        log "No new findings (unchanged)"
        return 1
    fi
    
    KNOWN_FINDINGS_HASH="$hash"
    NEW_FINDINGS_COUNT=$(cat "$FINDINGS_FILE" | jq 'length' 2>/dev/null || echo "1")
    FINDINGS_COUNT=$(cat "$FINDINGS_FILE" | jq 'length' 2>/dev/null || echo "1")
    
    if [ "$NEW_FINDINGS_COUNT" -gt 0 ]; then
        log "New findings detected: $NEW_FINDINGS_COUNT"
        return 0
    fi
    
    return 1
}

trigger_executor() {
    local finding_file="$1"
    local risk="$2"
    
    log "Triggering Executor (risk: $risk)..."
    
    case "$risk" in
        low)
            log "LOW risk - Executor auto-runs"
            bash "$EXECUTOR_SCRIPT" -f "$finding_file" 2>&1 | tee -a "$LOG_FILE"
            echo "Executor completed for LOW risk finding"
            ;;
        medium)
            log "MEDIUM risk - Approval required"
            echo "[APPROVAL REQUIRED] Run /approve to proceed"
            ;;
        high)
            log "HIGH risk - BLOCKED"
            echo "[BLOCKED] High risk finding requires manual review"
            ;;
    esac
}

classify_risk() {
    local title="$1"
    local severity="$2"
    
    title_lower=$(echo "$title" | tr '[:upper:]' '[:lower:]')
    
    # High risk keywords
    if echo "$title_lower" | grep -qE "auth|payment|runtime|core|architecture|schema|infra|security"; then
        echo "high"
        return
    fi
    
    if [ "$severity" = "critical" ] || [ "$severity" = "high" ]; then
        echo "high"
        return
    fi
    
    # Low risk keywords
    if echo "$title_lower" | grep -qE "lint|import|unused|nil|log|build|dead.?code"; then
        echo "low"
        return
    fi
    
    echo "medium"
}

send_telegram_notification() {
    local message="$1"
    local token="$TELEGRAM_BOT_TOKEN"
    local chat_id="$TELEGRAM_CHAT_ID"
    
    if [ -z "$token" ] || [ -z "$chat_id" ]; then
        log "Telegram not configured, skipping notification"
        return
    fi
    
    curl -s -X POST "https://api.telegram.org/bot$token/sendMessage" \
        -d "chat_id=$chat_id" \
        -d "text=$message" \
        -d "parse_mode=Markdown" 2>&1 | tee -a "$LOG_FILE"
}

format_notification() {
    local findings_file="$1"
    local count="$2"
    
    local msg="🔔 *NEW FINDINGS*

"
    
    local i=0
    while [ $i -lt "$count" ] && [ $i -lt 5 ]; do
        local title=$(cat "$findings_file" | jq -r ".[$i].title // empty")
        local severity=$(cat "$findings_file" | jq -r ".[$i].severity // empty")
        local risk=$(classify_risk "$title" "$severity")
        
        local emoji="🟡"
        [ "$risk" = "high" ] && emoji="🔴"
        [ "$risk" = "low" ] && emoji="🟢"
        
        msg="${msg}${emoji} *${title}*
Severity: ${severity} | Risk: ${risk}

"
        i=$((i + 1))
    done
    
    [ "$count" -gt 5 ] && msg="${msg}... and $((count - 5)) more"
    
    echo "$msg"
}

runtime_loop() {
    log "Starting continuous runtime loop (every ${TICKER_INTERVAL}s)..."
    log_validate()
    
    echo $$ > "$PID_FILE"
    
    while true; do
        run_audit
        
        if check_new_findings; then
            # Send notification
            local notification=$(format_notification "$FINDINGS_FILE" "$NEW_FINDINGS_COUNT")
            send_telegram_notification "$notification"
            LAST_NOTIFICATION=$(date -Iseconds)
            
            # Trigger executor for each finding
            local i=0
            while [ $i -lt "$NEW_FINDINGS_COUNT" ]; do
                local finding=$(cat "$FINDINGS_FILE" | jq ".[$i]")
                local title=$(echo "$finding" | jq -r '.title')
                local severity=$(echo "$finding" | jq -r '.severity')
                local risk=$(classify_risk "$title" "$severity")
                
                # Save finding to temp file
                echo "$finding" > "/tmp/finding_$i.json"
                trigger_executor "/tmp/finding_$i.json" "$risk"
                
                i=$((i + 1))
            done
        fi
        
        save_state
        
        # Wait for next interval
        sleep "$TICKER_INTERVAL"
    done
}

case "${1:-}" in
    start)
        if [ -f "$PID_FILE" ]; then
            pid=$(cat "$PID_FILE")
            if kill -0 "$pid" 2>/dev/null; then
                echo "Runtime already running (PID: $pid)"
                exit 0
            fi
        fi
        
        runtime_loop &
        log "Runtime started"
        ;;
    
    stop)
        if [ -f "$PID_FILE" ]; then
            pid=$(cat "$PID_FILE")
            kill "$pid" 2>/dev/null && echo "Runtime stopped" || echo "Runtime not running"
            rm -f "$PID_FILE"
        fi
        ;;
    
    status)
        if [ -f "$PID_FILE" ]; then
            pid=$(cat "$PID_FILE")
            if kill -0 "$pid" 2>/dev/null; then
                echo "✅ Runtime running (PID: $pid)"
            else
                echo "❌ Runtime not running (stale PID file)"
            fi
        else
            echo "❌ Runtime not running"
        fi
        load_state
        echo "Last audit: $LAST_AUDIT_RUN"
        echo "Findings count: $FINDINGS_COUNT"
        ;;
    
    once)
        run_audit
        check_new_findings && echo "New findings: $NEW_FINDINGS_COUNT"
        ;;
    
    *)
        echo "Usage: $0 {start|stop|status|once}"
        exit 1
        ;;
esac