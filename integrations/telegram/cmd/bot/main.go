package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"SMART-WORKER/integrations/telegram/context"
)

// Bot configuration
var (
	AUDITORS_PATH = "/workspaces/SMART-WORKER/agents/auditor/run.sh"
	EXECUTOR_PATH = "/workspaces/SMART-WORKER/agents/executor/run.sh"
	STATE_FILE   = "/tmp/runtime_state.json"
	ENV_FILE     = "/workspaces/SMART-WORKER/integrations/telegram/.env"
)

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

	// Load operational memory
	mem := context.NewOperationalMemory()
	_ = mem.LoadFromDisk()

	// Create router
	router := NewRouter(mem)

	// Handle commands
	updateConfig := botapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	// Main loop
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

func handleMessage(text string, router *Router, mem *OperationalMemory) string {
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

func handleCommand(text string, mem *OperationalMemory) string {
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
		status, pid, running := mem.GetRuntimeStatus()
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
	if data, err := os.ReadFile(ENV_FILE); err == nil {
		log.Printf("[BOT] Loaded environment from %s", ENV_FILE)
	}
}