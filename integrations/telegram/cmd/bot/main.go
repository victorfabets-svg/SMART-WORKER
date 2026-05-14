package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"

	telegramcontext "github.com/aoms/smart-worker/integrations/telegram/context"
)

// resolveRoot determines the SMART-WORKER root dynamically
func resolveRoot() string {
	if root := os.Getenv("SMART_WORKER_ROOT"); root != "" {
		return root
	}
	execPath, err := os.Executable()
	if err == nil {
		absPath, _ := filepath.EvalSymlinks(execPath)
		dir := filepath.Dir(absPath)
		if strings.Contains(dir, "integrations/telegram") {
			return filepath.Join(dir, "..", "..")
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		testPath := filepath.Join(cwd, "integrations", "telegram")
		if _, err := os.Stat(testPath); err == nil {
			return cwd
		}
	}
	return "/workspace/project/SMART-WORKER"
}

// Runtime state for continuous operation
type BotRuntime struct {
	mu              sync.RWMutex
	LastAuditRun    time.Time
	KnownFindings   map[string]bool
	FindingsCount   int
	IsRunning       bool
}

var botRuntime BotRuntime

// Bot configuration (initialized in init())
var (
	AUDITORS_PATH string
	EXECUTOR_PATH string
	STATE_FILE   = "/tmp/runtime_state.json"
	ENV_FILE     string
	TICKER_INTERVAL = 5 * time.Minute
)

func init() {
	root := resolveRoot()
	AUDITORS_PATH = filepath.Join(root, "agents", "auditor", "run.sh")
	EXECUTOR_PATH = filepath.Join(root, "agents", "executor", "run.sh")
	ENV_FILE = filepath.Join(root, "integrations", "telegram", ".env")
}

func main() {
	// Load environment
	loadEnv()

	// Initialize bot
	token := getEnv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("[BOT] Authorized on account %s", bot.Self.UserName)

	// Initialize runtime state for continuous loop
	botRuntime = BotRuntime{
		KnownFindings: make(map[string]bool),
		IsRunning:     true,
	}

	// Load previous state
	loadBotState()

	// Load operational memory
	mem := telegramcontext.NewOperationalMemory()
	_ = mem.LoadFromDisk()

	// Create router
	router := NewRouter(mem)
	
	// Log startup environment status
	log.Printf("[BOOT] ================================================")
	log.Printf("[BOOT] Telegram Conversational Runtime Starting")
	log.Printf("[BOOT] ================================================")
	
	// Check OPENHANDS_API_KEY availability
	apiKey := os.Getenv("OPENHANDS_API_KEY")
	if apiKey != "" {
		log.Printf("[BOOT] OPENHANDS_API_KEY detected: YES (length=%d)", len(apiKey))
	} else {
		log.Printf("[BOOT] OPENHANDS_API_KEY detected: NO")
	}
	
	// Check if lifecycle has OpenHands
	if router.lifecycle != nil {
		hasOH := router.lifecycle.HasOpenHands()
		log.Printf("[BOOT] OpenHands enabled: %v", hasOH)
	} else {
		log.Printf("[BOOT] Lifecycle: not created")
	}
	
	log.Printf("[BOOT] ================================================")

	// Start continuous audit loop in background
	go runBotContinuousLoop(bot)

	// Handle incoming messages - THIS IS THE CONVERSATIONAL LOOP
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	// Main loop - handles conversational messages
	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		text := update.Message.Text

		log.Printf("[BOT] Message from %d: %s", chatID, text)

		response := handleMessage(text, router, mem)
		sendMessage(bot, chatID, response)
	}
}

func handleMessage(text string, router *Router, mem *telegramcontext.OperationalMemory) string {
	text = strings.TrimSpace(text)

	// Handle commands first
	if strings.HasPrefix(text, "/") {
		return handleCommand(text, mem)
	}

	// Route conversational messages
	// First refresh memory
	_ = mem.LoadFromDisk()

	// Pass to router
	resp := router.Route(text)
	return resp.Content
}

func handleCommand(text string, mem *telegramcontext.OperationalMemory) string {
	parts := strings.Fields(text)
	command := parts[0]

	switch command {
	case "/start", "/help":
		return `Welcome to SMART-WORKER Operations

Commands:
/audit - Run audit
/findings - Show latest findings  
/status - Runtime status

Or talk naturally:
"auditor, explain findings"
"executor, why blocked?"
"runtime, status"

NOT: execute code, modify repo`

	case "/audit":
		return runAudit()

	case "/findings":
		_ = mem.LoadFromDisk()
		findings := mem.GetLatestFindings()
		if len(findings) == 0 {
			return "No findings. Run /audit first."
		}
		content := "Latest findings:\n\n"
		for i, f := range findings {
			if i >= 5 {
				break
			}
			emoji := "🟡"
			if f.Severity == "critical" || f.Severity == "high" {
				emoji = "🔴"
			} else if f.Severity == "low" {
				emoji = "🟢"
			}
			content += fmt.Sprintf("%s [%s] %s\n", emoji, f.Severity, f.Title)
			content += fmt.Sprintf("   %s\n\n", f.Evidence)
		}
		return content

	case "/status":
		_ = mem.LoadFromDisk()
		_, pid, running := mem.GetRuntimeStatus()
		content := "Runtime Status:\n"
		if running {
			content += fmt.Sprintf("Status: RUNNING (PID: %d)\n", pid)
		} else {
			content += "Status: STOPPED\n"
		}
		content += "Ticker: 5 minutes\n"
		content += "Next audit: automatic\n"
		return content

	default:
		return fmt.Sprintf("Unknown command: %s", command)
	}
}

func runAudit() string {
	// Run auditor
	cmd := fmt.Sprintf("bash %s", AUDITORS_PATH)
	_ = cmd
	return "Running audit...\n(Audit would run asynchronously)"
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"

	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("[BOT] Send error: %v", err)
	}
}

func getEnv(key string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	// Try .env file
	if data, err := os.ReadFile(ENV_FILE); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, key+"=") {
				return strings.TrimPrefix(line, key+"=")
			}
		}
	}
	return ""
}

func loadEnv() {
	log.Printf("[BOT] ================================================")
	log.Printf("[BOT] Loading Environment Configuration")
	log.Printf("[BOT] ================================================")
	
	// FIRST: Check what environment variables are ALREADY available
	debugEnvVars := []string{
		"OPENHANDS_API_KEY",
		"TELEGRAM_BOT_TOKEN",
		"GITHUB_REPOSITORY",
		"OPENHANDS_BASE_URL",
	}
	for _, key := range debugEnvVars {
		if val := os.Getenv(key); val != "" {
			log.Printf("[BOT] %s: set (length=%d)", key, len(val))
		} else {
			log.Printf("[BOT] %s: NOT set", key)
		}
	}
	
	// THEN: Load .env file if present (but prefer existing environment vars)
	if data, err := os.ReadFile(ENV_FILE); err == nil {
		log.Printf("[BOT] Reading .env from: %s", ENV_FILE)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			// Skip comments and empty lines
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Parse KEY=VALUE
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Skip if key starts with $ (it's a reference to itself)
				if strings.HasPrefix(value, "$") && strings.Contains(value, key) {
					// This is a self-reference like KEY=${KEY}, skip
					continue
				}
				
				// Check if already set in environment - prefer that
				if existing, exists := os.LookupEnv(key); exists && existing != "" {
					// Already set, skip .env value
					continue
				}
				
				// Try to resolve ${VAR} from environment
				re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
				matches := re.FindAllStringSubmatch(value, -1)
				for _, match := range matches {
					if len(match) == 2 {
						envKey := match[1]
						envVal := os.Getenv(envKey)
						if envVal != "" {
							value = strings.ReplaceAll(value, match[0], envVal)
						}
					}
				}
				
				// Set if we have a value
				if value != "" {
					os.Setenv(key, value)
					log.Printf("[BOT] Set from .env: %s", key)
				}
			}
		}
	} else {
		log.Printf("[BOT] No .env file at %s (this is OK)", ENV_FILE)
	}
	
	// FINAL: Debug check after loading
	log.Printf("[BOT] ================================================")
	log.Printf("[BOT] Final Environment State:")
	if apiKey := os.Getenv("OPENHANDS_API_KEY"); apiKey != "" {
		log.Printf("[BOT] OPENHANDS_API_KEY: AVAILABLE (length=%d)", len(apiKey))
	} else {
		log.Printf("[BOT] OPENHANDS_API_KEY: NOT AVAILABLE")
	}
	log.Printf("[BOT] ================================================")
}

func loadBotState() {
	data, err := os.ReadFile(STATE_FILE)
	if err != nil {
		return
	}
	var state struct {
		LastAuditRun       time.Time         `json:"last_audit_run"`
		FindingsCount    int              `json:"findings_count"`
		KnownFindings   map[string]bool  `json:"known_findings"`
		IsRunning       bool            `json:"is_running"`
	}
	if err := json.Unmarshal(data, &state); err == nil {
		botRuntime.LastAuditRun = state.LastAuditRun
		botRuntime.FindingsCount = state.FindingsCount
		botRuntime.IsRunning = state.IsRunning
		if state.KnownFindings != nil {
			botRuntime.KnownFindings = state.KnownFindings
		}
	}
}

func saveBotState() {
	data, err := json.Marshal(botRuntime)
	if err != nil {
		return
	}
	os.WriteFile(STATE_FILE, data, 0644)
}

func runBotContinuousLoop(bot *tgbotapi.BotAPI) {
	log.Printf("[BOT] Starting continuous audit loop (every %v)...", TICKER_INTERVAL)

	ticker := time.NewTicker(TICKER_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			runBotScheduledAudit(bot)
		}
	}
}

func runBotScheduledAudit(bot *tgbotapi.BotAPI) {
	botRuntime.mu.Lock()
	botRuntime.LastAuditRun = time.Now()
	botRuntime.mu.Unlock()

	log.Println("[BOT] Running scheduled audit...")

	// Run auditor
	cmd := exec.Command("bash", AUDITORS_PATH)
	cmd.Dir = resolveRoot()
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[BOT] Auditor error: %v", err)
		sendAlert(bot, fmt.Sprintf("Auditor failed: %v\n%s", err, output))
		return
	}

	log.Printf("[BOT] Auditor completed")

	// Load new findings
	findings := loadBotFindings()
	botRuntime.FindingsCount = len(findings)

	// Compare against known findings
	newFindings := findBotNewFindings(findings)

	if len(newFindings) > 0 {
		log.Printf("[BOT] New findings detected: %d", len(newFindings))
		sendAlert(bot, formatBotNotification(newFindings))

		// Trigger executor for each new finding
		for _, f := range newFindings {
			triggerBotExecutor(bot, f)
		}
	} else {
		log.Printf("[BOT] No new findings")
	}

	saveBotState()
}

func loadBotFindings() []map[string]interface{} {
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
		var finding map[string]interface{}
		if err := json.Unmarshal(data, &finding); err == nil {
			findings = []map[string]interface{}{finding}
		}
	}

	return findings
}

func findBotNewFindings(findings []map[string]interface{}) []map[string]interface{} {
	var new []map[string]interface{}

	for _, f := range findings {
		hash := generateBotFindingHash(f)

		botRuntime.mu.RLock()
		known := botRuntime.KnownFindings[hash]
		botRuntime.mu.RUnlock()

		if !known {
			new = append(new, f)
			botRuntime.mu.Lock()
			botRuntime.KnownFindings[hash] = true
			botRuntime.mu.Unlock()
		}
	}

	return new
}

func generateBotFindingHash(f map[string]interface{}) string {
	title, _ := f["title"].(string)
	severity, _ := f["severity"].(string)
	evidence, _ := f["evidence"].(string)
	return fmt.Sprintf("%s|%s|%s", title, severity, evidence)
}

func classifyBotRisk(finding map[string]interface{}) string {
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

func triggerBotExecutor(bot *tgbotapi.BotAPI, finding map[string]interface{}) {
	title, _ := finding["title"].(string)
	risk := classifyBotRisk(finding)

	log.Printf("[BOT] Triggering Executor for: %s (risk: %s)", title, risk)

	switch risk {
	case "low":
		log.Printf("[BOT] LOW risk - Executor auto-runs")
		findingJSON, _ := json.Marshal(finding)
		tmpFile := "/tmp/executor_finding.json"
		os.WriteFile(tmpFile, findingJSON, 0644)

		cmd := exec.Command("bash", EXECUTOR_PATH, "-f", tmpFile)
		cmd.Dir = resolveRoot()
		output, err := cmd.CombinedOutput()

		if err != nil {
			log.Printf("[BOT] Executor error: %v", err)
			sendAlert(bot, fmt.Sprintf("Executor failed for '%s': %v\n%s", title, err, output))
		} else {
			log.Printf("[BOT] Executor completed: %s", title)
			sendAlert(bot, fmt.Sprintf("✅ Executor applied fix for: %s", title))
		}

	case "medium":
		log.Printf("[BOT] MEDIUM risk - Approval required")
		sendAlert(bot, fmt.Sprintf("🔒 MEDIUM RISK: %s\nRequires approval", title))

	case "high":
		log.Printf("[BOT] HIGH risk - BLOCKED")
		sendAlert(bot, fmt.Sprintf("HIGH RISK: %s - BLOCKED - manual review required", title))
	}
}

func formatBotNotification(findings []map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("🔔 *NEW FINDINGS DETECTED*\n\n")

	for i, f := range findings {
		title, _ := f["title"].(string)
		severity, _ := f["severity"].(string)
		evidence, _ := f["evidence"].(string)
		risk := classifyBotRisk(f)

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

		if i >= 4 {
			sb.WriteString(fmt.Sprintf("... and %d more", len(findings)-4))
			break
		}
	}

	return sb.String()
}

func sendAlert(bot *tgbotapi.BotAPI, message string) {
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if chatID != "" {
		var id int64
		fmt.Sscanf(chatID, "%d", &id)
		msg := tgbotapi.NewMessage(id, message)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
	}
}
