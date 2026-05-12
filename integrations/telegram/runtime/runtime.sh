#!/bin/bash
#
# BEATSET Continuous Runtime
# Persistent operational loop for SMART-WORKER agents

# Dynamic root resolution
resolve_root() {
  # Priority 1: SMART_WORKER_ROOT environment variable
  if [ -n "$SMART_WORKER_ROOT" ]; then
    echo "$SMART_WORKER_ROOT"
    return
  fi
  
  # Priority 2: Derive from script location
  local script_path="${BASH_SOURCE[0]}"
  if [ -n "$script_path" ]; then
    local resolved=$(readlink -f "$script_path" 2>/dev/null)
    local dir=$(dirname "$resolved")
    # Go up from integrations/telegram/runtime to root
    if [[ "$dir" == *"integrations/telegram/runtime" ]]; then
      echo "$dir/../../.." | sed 's|/integrations/telegram/runtime||'
      return
    fi
  fi
  
  # Priority 3: Current working directory
  if [ -d "$(pwd)/integrations/telegram" ]; then
    echo "$(pwd)"
    return
  fi
  
  # Fallback: fail
  echo "ERROR: Cannot determine SMART_WORKER_ROOT. Set SMART_WORKER_ROOT env var." >&2
  exit 1
}

# Resolve root once
ROOT_DIR="$(resolve_root)"

# Now set paths relative to dynamic root
SCRIPT_DIR="$ROOT_DIR/integrations/telegram/runtime"
RUNTIME_DIR="$ROOT_DIR"
AUDITOR_SCRIPT="$ROOT_DIR/agents/auditor/run.sh"
EXECUTOR_SCRIPT="$ROOT_DIR/agents/executor/run.sh"
STATE_FILE="/tmp/runtime_state.json"
PID_FILE="/tmp/runtime.pid"
LOG_FILE="/tmp/runtime.log"
TICKER_INTERVAL=300
FINDINGS_DIR="/tmp"

# Load environment
ENV_FILE="$ROOT_DIR/integrations/telegram/.env"
if [ -f "$ENV_FILE" ]; then
  while IFS="=" read -r key value; do
    [[ -z "$key" ]] && continue
    [[ "$key" =~ ^# ]] && continue
    export "$key=$value"
  done < "$ENV_FILE"
fi

log() {
  echo "[RUNTIME] $(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

save_state() {
  cat > "$STATE_FILE" << STATEEOF
{
  "last_audit_run": "$(date -Iseconds)",
  "runtime_pid": $(cat "$PID_FILE" 2>/dev/null || echo "0"),
  "findings_count": $FINDINGS_COUNT,
  "new_findings_count": $NEW_FINDINGS_COUNT,
  "last_notification": "$LAST_NOTIFICATION"
}
STATEEOF
}

load_state() {
  if [ -f "$STATE_FILE" ]; then
    LAST_AUDIT_RUN=$(grep -o '"last_audit_run": *"[^"]*"' "$STATE_FILE" | cut -d'"' -f4)
    FINDINGS_COUNT=$(grep -o '"findings_count": *[0-9]*' "$STATE_FILE" | grep -o '[0-9]*')
  fi
}

log_validate() {
  if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
    log "Telegram configured successfully (BOT: ${TELEGRAM_BOT_TOKEN:0:8}...)"
  else
    log "Telegram not fully configured - notifications disabled"
    [ -z "$TELEGRAM_BOT_TOKEN" ] && log "Missing TELEGRAM_BOT_TOKEN"
    [ -z "$TELEGRAM_CHAT_ID" ] && log "Missing TELEGRAM_CHAT_ID"
  fi
  [ -n "$SMART_WORKER_URL" ] && log "SMART_WORKER_URL: $SMART_WORKER_URL"
}

run_audit() {
  log "Running audit..."
  bash "$AUDITOR_SCRIPT"
}

find_latest_findings() {
  latest=""
  latest_time=0
  for f in "$FINDINGS_DIR"/auditor_findings_*.json; do
    [ -f "$f" ] || continue
    mtime=$(stat -c %Y "$f" 2>/dev/null)
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
  hash=$(sha256sum "$FINDINGS_FILE" 2>/dev/null | cut -d' ' -f1)
  if [ "$KNOWN_FINDINGS_HASH" = "$hash" ]; then
    log "No new findings (unchanged)"
    return 1
  fi
  KNOWN_FINDINGS_HASH="$hash"
  NEW_FINDINGS_COUNT=$(cat "$FINDINGS_FILE" | jq 'length' 2>/dev/null || echo "1")
  FINDINGS_COUNT=$NEW_FINDINGS_COUNT
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
      bash "$EXECUTOR_SCRIPT" -f "$finding_file"
      ;;
    medium)
      log "MEDIUM risk - Approval required"
      ;;
    high)
      log "HIGH risk - BLOCKED"
      ;;
  esac
}

classify_risk() {
  local title="$1"
  local severity="$2"
  title_lower=$(echo "$title" | tr '[:upper:]' '[:lower:]')
  if echo "$title_lower" | grep -qE "auth|payment|runtime|core|architecture|schema|infra|security"; then
    echo "high"
    return
  fi
  if [ "$severity" = "critical" ] || [ "$severity" = "high" ]; then
    echo "high"
    return
  fi
  if echo "$title_lower" | grep -qE "lint|import|unused|nil|log|build|dead.?code"; then
    echo "low"
    return
  fi
  echo "medium"
}

send_telegram() {
  local msg="$1"
  local token="$TELEGRAM_BOT_TOKEN"
  local chat_id="$TELEGRAM_CHAT_ID"
  if [ -z "$token" ] || [ -z "$chat_id" ]; then
    log "Telegram not configured, skipping"
    return
  fi
  curl -s -X POST "https://api.telegram.org/bot$token/sendMessage" -d "chat_id=$chat_id" -d "text=$msg" -d "parse_mode=Markdown"
}

format_notification() {
  local findings_file="$1"
  local count="$2"
  local msg="🔔 *NEW FINDINGS*

"
  local i=0
  while [ $i -lt "$count" ] && [ $i -lt 5 ]; do
    local title=$(cat "$findings_file" | jq -r ".[$i].title")
    local severity=$(cat "$findings_file" | jq -r ".[$i].severity")
    local risk=$(classify_risk "$title" "$severity")
    local emoji="🟡"
    [ "$risk" = "high" ] && emoji="🔴"
    [ "$risk" = "low" ] && emoji="🟢"
    msg="${msg}${emoji} *${title}*
Severity: ${severity} | Risk: ${risk}

"
    i=$((i + 1))
  done
  echo "$msg"
}

runtime_loop() {
  log "Starting continuous runtime loop (every ${TICKER_INTERVAL}s)..."
  log_validate
  echo $$ > "$PID_FILE"
  while true; do
    run_audit
    if check_new_findings; then
      local notification=$(format_notification "$FINDINGS_FILE" "$NEW_FINDINGS_COUNT")
      send_telegram "$notification"
      LAST_NOTIFICATION=$(date -Iseconds)
      local i=0
    fi
    save_state
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
        echo "Runtime running (PID: $pid)"
      else
        echo "Runtime not running (stale PID file)"
      fi
    else
      echo "Runtime not running"
    fi
    load_state
    echo "Last audit: $LAST_AUDIT_RUN"
    echo "Findings: $FINDINGS_COUNT"
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
