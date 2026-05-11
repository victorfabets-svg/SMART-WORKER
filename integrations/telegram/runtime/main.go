package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// RuntimeState tracks the continuous operation
type RuntimeState struct {
	LastAuditRun     time.Time          `json:"last_audit_run"`
	LastNotification time.Time         `json:"last_notification"`
	FindingsCount    int               `json:"findings_count"`
	NewFindingsCount int               `json:"new_findings_count"`
	ExecutorActivity string            `json:"executor_activity"`
	KnownFindings   map[string]bool   `json:"known_findings"`
	IsRunning        bool              `json:"is_running"`
	mu              sync.RWMutex
}

var (
	bot             *tgbotapi.BotAPI
	runtimeState    RuntimeState
	TICKER_INTERVAL = 5 * time.Minute // 5-minute loop
	AUDITORS_PATH   = "/workspace/project/SMART-WORKER/agents/auditor/run.sh"
	EXECUTOR_PATH   = "/workspace/project/SMART-WORKER/agents/executor/run.sh"
	STATE_FILE     = "/tmp/runtime_state.json"
)

func main() {
	// Initialize bot
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Printf("BEATSET Runtime Bot started on %s", bot.Self.FirstName)

	// Initialize runtime state
	runtimeState = RuntimeState{
		KnownFindings: make(map[string]bool),
		IsRunning:     true,
	}
	loadState()

	// Start continuous loop in background
	go runContinuousLoop()

	// Set up webhook/commands
	updates := bot.GetUpdatesChan(nil)
	for update := range updates {
		handleUpdate(update)
	}
}

func runContinuousLoop() {
	log.Printf("[RUNTIME] Starting continuous audit loop (every %v)...", TICKER_INTERVAL)
	
	ticker := time.NewTicker(TICKER_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runScheduledAudit()
		}
	}
}

func runScheduledAudit() {
	runtimeState.mu.Lock()
	runtimeState.LastAuditRun = time.Now()
	runtimeState.NewFindingsCount = 0
	runtimeState.mu.Unlock()

	log.Println("[RUNTIME] Running scheduled audit...")

	// Run auditor
	cmd := exec.Command("bash", AUDITORS_PATH)
	cmd.Dir = "/workspace/project/SMART-WORKER"
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[RUNTIME] Auditor error: %v", err)
		sendAlert(fmt.Sprintf("Auditor failed: %v\n%s", err, output))
		return
	}

	log.Printf("[RUNTIME] Auditor completed")

	// Load new findings
	findings := loadFindings()
	runtimeState.FindingsCount = len(findings)

	// Compare against known findings
	newFindings := findNewFindings(findings)
	runtimeState.NewFindingsCount = len(newFindings)
	runtimeState.FindingsCount = len(findings)

	if len(newFindings) > 0 {
		log.Printf("[RUNTIME] New findings detected: %d", len(newFindings))
		
		// Notify Telegram
		sendAlert(formatFindingsNotification(newFindings))
		runtimeState.mu.Lock()
		runtimeState.LastNotification = time.Now()
		runtimeState.mu.Unlock()

		// Trigger executor for each new finding
		for _, f := range newFindings {
			triggerExecutor(f)
		}
	} else {
		log.Printf("[RUNTIME] No new findings")
	}

	saveState()
}

func findNewFindings(findings []map[string]interface{}) []map[string]interface{} {
	var new []map[string]interface{}
	
	for _, f := range findings {
		hash := generateFindingHash(f)
		
		runtimeState.mu.RLock()
		known := runtimeState.KnownFindings[hash]
		runtimeState.mu.RUnlock()
		
		if !known {
			new = append(new, f)
			runtimeState.mu.Lock()
			runtimeState.KnownFindings[hash] = true
			runtimeState.mu.Unlock()
		}
	}
	
	return new
}

func generateFindingHash(f map[string]interface{}) string {
	title, _ := f["title"].(string)
	severity, _ := f["severity"].(string)
	evidence, _ := f["evidence"].(string)
	return fmt.Sprintf("%s|%s|%s", title, severity, evidence)
}

func loadFindings() []map[string]interface{} {
	// Find latest findings file
	files, err := os.ReadDir("/tmp")
	if err != nil {
		return nil
	}

	var latestFile string
	var latestTime time.Time

	for _, f := range files {
		if strings.HasPrefix(f.Name(), "auditor_findings_") {
			info, _ := f.Info()
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = "/tmp/" + f.Name()
			}
		}
	}

	if latestFile == "" {
		return nil
	}

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil
	}

	var findings []map[string]interface{}
	if err := json.Unmarshal(data, &findings); err != nil {
		// Try single object
		var finding map[string]interface{}
		if err := json.Unmarshal(data, &finding); err == nil {
			findings = []map[string]interface{}{finding}
		}
	}

	return findings
}

func triggerExecutor(finding map[string]interface{}) {
	title, _ := finding["title"].(string)
	severity, _ := finding["severity"].(string)

	// Determine risk level
	risk := classifyRisk(finding)

	runtimeState.mu.Lock()
	runtimeState.ExecutorActivity = fmt.Sprintf("processing: %s", title)
	runtimeState.mu.Unlock()

	log.Printf("[RUNTIME] Triggering Executor for: %s (risk: %s)", title, risk)

	switch risk {
	case "low":
		log.Printf("[RUNTIME] LOW risk - Executor auto-runs")
		// Save finding to temp file and run executor
		findingJSON, _ := json.Marshal(finding)
		tmpFile := "/tmp/executor_finding.json"
		os.WriteFile(tmpFile, findingJSON, 0644)
		
		cmd := exec.Command("bash", EXECUTOR_PATH, "-f", tmpFile)
		cmd.Dir = "/workspace/project/SMART-WORKER"
		output, err := cmd.CombinedOutput()
		
		if err != nil {
			log.Printf("[RUNTIME] Executor error: %v", err)
			sendAlert(fmt.Sprintf("Executor failed for '%s': %v\n%s", title, err, output))
		} else {
			log.Printf("[RUNTIME] Executor completed: %s", title)
			sendAlert(fmt.Sprintf("✅ Executor applied fix for: %s", title)))
		}

	case "medium":
		log.Printf("[RUNTIME] MEDIUM risk - Approval required")
		sendAlert(fmt.Sprintf("🔒 MEDIUM RISK: %s\nRequires approval", title)))

	case "high":
		log.Printf("[RUNTIME] HIGH risk - BLOCKED")
		sendAlert(fmt.Sprintf("🚫 HIGH RISK: %s\nBLOCKED - manual review required", title)))
	}

	runtimeState.mu.Lock()
	runtimeState.ExecutorActivity = "idle"
	runtimeState.mu.Unlock()
}

func classifyRisk(finding map[string]interface{}) string {
	title, _ := finding["title"].(string)
	severity, _ := finding["severity"].(string)
	content, _ := finding["content"].(string)

	titleLower := strings.ToLower(title)
	contentLower := strings.ToLower(content)

	// High risk keywords
	highRiskKeywords := []string{"auth", "payment", "runtime", "core", "architecture", "schema", "infra", "security"}
	for _, kw := range highRiskKeywords {
		if strings.Contains(titleLower, kw) || strings.Contains(contentLower, kw) {
			return "high"
		}
	}

	if severity == "critical" || severity == "high" {
		return "high"
	}

	// Low risk keywords (auto-fix allowed)
	lowRiskKeywords := []string{"lint", "import", "unused", "nil", "log", "build"}
	for _, kw := range lowRiskKeywords {
		if strings.Contains(titleLower, kw) {
			return "low"
		}
	}

	return "medium"
}

func handleUpdate(update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	command := update.Message.Command()
	args := update.Message.CommandArguments()

	log.Printf("[TELEGRAM] Command: %s", command)

	switch command {
	case "run-audit":
		// Manual audit trigger
		go runScheduledAudit()
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🔄 Running audit..."))

	case "findings":
		// Show current findings
		state := getRuntimeState()
		msg := fmt.Sprintf("📊 Runtime Status\n\nLast Audit: %s\nFindings: %d\nNew: %d\nLast Notification: %s",
			state.LastAuditRun.Format("2006-01-02 15:04"),
			state.FindingsCount,
			state.NewFindingsCount,
			state.LastNotification.Format("2006-01-02 15:04"))
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

	case "status":
		// Runtime status
		state := getRuntimeState()
		msg := fmt.Sprintf("⚡ Runtime Status\n\nRunning: %v\nLast Audit: %s\nExecutor: %s",
			state.IsRunning,
			state.LastAuditRun.Format("2006-01-02 15:04"),
			state.ExecutorActivity)
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

	case "executor-status":
		// Executor activity
		state := getRuntimeState()
		msg := fmt.Sprintf("🤖 Executor Status\n\nActivity: %s\nLast Run: %s",
			state.ExecutorActivity,
			state.LastAuditRun.Format("2006-01-02 15:04"))
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msg))

	case "help":
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, `Commands:
/run-audit - Run audit now
/findings - Show findings
/status - Runtime status
/executor-status - Executor activity
/help - Show this help`))

	default:
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command. Use /help"))
	}
}

func sendAlert(message string) {
	// Send to configured chat ID or all active chats
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if chatID != "" {
		var id int64
		fmt.Sscanf(chatID, "%d", &id)
		msg := tgbotapi.NewMessage(id, message)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
	}
}

func formatFindingsNotification(findings []map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("🔔 *NEW FINDINGS DETECTED*\n\n")

	for i, f := range findings {
		title, _ := f["title"].(string)
		severity, _ := f["severity"].(string)
		evidence, _ := f["evidence"].(string)
		risk := classifyRisk(f)

		emoji := "🟡"
		if risk == "high" {
			emoji = "🔴"
		} else if risk == "low" {
			emoji = "🟢"
		}

		sb.WriteString(fmt.Sprintf("%s *%s*\n", emoji, title))
		sb.WriteString(fmt.Sprintf("Severity: %s | Risk: %s\n", severity, risk))
		if evidence != "" {
			sb.WriteString(fmt.Sprintf("Evidence: %s\n", evidence))
		}
		sb.WriteString("\n")

		if i >= 4 { // Limit notifications
			sb.WriteString(fmt.Sprintf("... and %d more", len(findings)-4))
			break
		}
	}

	return sb.String()
}

func getRuntimeState() RuntimeState {
	runtimeState.mu.RLock()
	defer runtimeState.mu.RUnlock()
	return runtimeState
}

func saveState() {
	data, err := json.Marshal(runtimeState)
	if err != nil {
		return
	}
	os.WriteFile(STATE_FILE, data, 0644)
}

func loadState() {
	data, err := os.ReadFile(STATE_FILE)
	if err != nil {
		return
	}
	json.Unmarshal(data, &runtimeState)
}