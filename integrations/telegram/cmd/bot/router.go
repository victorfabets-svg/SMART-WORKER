package main

import (
	"fmt"
	"strings"

	telegramcontext "github.com/aoms/smart-worker/integrations/telegram/context"
)

// AgentType the type of operational agent
type AgentType int

const (
	AgentUnknown AgentType = iota
	AgentAuditor
	AgentExecutor
	AgentRuntime
	AgentSystem
)

// AgentResponse response from an agent
type AgentResponse struct {
	Agent  AgentType
	Name   string
	Content string
}

// Router routes messages to appropriate agent
type Router struct {
	memory *telegramcontext.OperationalMemory
}

// NewRouter creates a new router
func NewRouter(mem *telegramcontext.OperationalMemory) *Router {
	return &Router{
		memory: mem,
	}
}

// Route routes a message to the appropriate agent
func (r *Router) Route(message string) AgentResponse {
	lower := strings.ToLower(message)
	agent := r.detectAgent(lower)
	intent := r.detectIntent(lower)

	switch agent {
	case AgentAuditor:
		return r.handleAuditor(message, intent)
	case AgentExecutor:
		return r.handleExecutor(message, intent)
	case AgentRuntime:
		return r.handleRuntime(message, intent)
	default:
		return r.handleGeneral(message, intent)
	}
}

// detectAgent detects which agent to route to
func (r *Router) detectAgent(message string) AgentType {
	auditorTriggers := []string{"auditor", "audit", "finding", "bug", "error", "explain"}
	executorTriggers := []string{"executor", "execute", "fix", "apply", "why blocked", "didn't fix", "blocked"}
	runtimeTriggers := []string{"runtime", "status", "running", "loop", "ticker", "alive"}

	for _, t := range auditorTriggers {
		if strings.Contains(message, t) {
			return AgentAuditor
		}
	}
	for _, t := range executorTriggers {
		if strings.Contains(message, t) {
			return AgentExecutor
		}
	}
	for _, t := range runtimeTriggers {
		if strings.Contains(message, t) {
			return AgentRuntime
		}
	}

	return AgentSystem
}

// detectIntent detects what the user wants
func (r *Router) detectIntent(message string) string {
	intents := map[string][]string{
		"explain":  {"explain", "why", "because", "cause"},
		"summarize": {"summarize", "summary", "overview", "what"},
		"status":   {"status", "state", "health"},
		"findings": {"findings", "bugs", "issues"},
		"list":     {"list", "show", "what files"},
		"risk":     {"risk", "risky"},
		"blocked":  {"blocked", "block"},
	}

	for intent, triggers := range intents {
		for _, t := range triggers {
			if strings.Contains(message, t) {
				return intent
			}
		}
	}

	return "general"
}

// handleAuditor handles auditor-specific queries
func (r *Router) handleAuditor(message string, intent string) AgentResponse {
	findings := r.memory.GetLatestFindings()

	switch intent {
	case "explain", "summarize", "findings":
		if len(findings) == 0 {
			return AgentResponse{
				Agent:  AgentAuditor,
				Name:   "AUDITOR",
				Content: "No findings available. Run /audit to generate new findings.",
			}
		}

		summary := "[AUDITOR]\n\n"
		summary += "Latest findings:\n\n"
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
			summary += fmt.Sprintf("%s [%s] %s\n", emoji, f.Severity, f.Title)
			summary += fmt.Sprintf("   Evidence: %s\n\n", f.Evidence)
		}
		if len(findings) > 5 {
			summary += fmt.Sprintf("... and %d more", len(findings)-5)
		}
		return AgentResponse{Agent: AgentAuditor, Name: "AUDITOR", Content: summary}

	case "list":
		if len(findings) == 0 {
			return AgentResponse{
				Agent:  AgentAuditor,
				Name:   "AUDITOR",
				Content: "No findings. Run /audit first.",
			}
		}
		content := "[AUDITOR]\n\nFindings:\n"
		for i, f := range findings {
			content += fmt.Sprintf("%d. %s (%s)\n", i+1, f.Title, f.Severity)
		}
		return AgentResponse{Agent: AgentAuditor, Name: "AUDITOR", Content: content}

	default:
		return AgentResponse{
			Agent:  AgentAuditor,
			Name:   "AUDITOR",
			Content: "Try 'auditor, explain findings' or 'auditor, show findings'.",
		}
	}
}

// handleExecutor handles executor-specific queries
func (r *Router) handleExecutor(message string, intent string) AgentResponse {
	decisions := r.memory.GetExecutorDecisions()
	blocked := r.memory.GetBlockedOperations()

	switch intent {
	case "explain", "why", "blocked":
		if len(blocked) == 0 {
			return AgentResponse{
				Agent:  AgentExecutor,
				Name:   "EXECUTOR",
				Content: "No operations have been blocked.",
			}
		}

		content := "[EXECUTOR]\n\nBlocked operations:\n\n"
		for _, b := range blocked {
			content += fmt.Sprintf(" Finding: %s\n", b.Finding)
			content += fmt.Sprintf(" Reason: %s\n\n", b.Reason)
		}
		return AgentResponse{Agent: AgentExecutor, Name: "EXECUTOR", Content: content}

	case "status", "summarize":
		if len(decisions) == 0 {
			return AgentResponse{
				Agent:  AgentExecutor,
				Name:   "EXECUTOR",
				Content: "No executor activity recorded.",
			}
		}

		autoCount := 0
		blockCount := 0
		for _, d := range decisions {
			if d.Action == "auto" {
				autoCount++
			}
			if d.Action == "blocked" {
				blockCount++
			}
		}

		content := "[EXECUTOR]\n\nActivity summary:\n"
		content += fmt.Sprintf("- Auto-executed: %d\n", autoCount)
		content += fmt.Sprintf("- Blocked: %d\n", blockCount)
		return AgentResponse{Agent: AgentExecutor, Name: "EXECUTOR", Content: content}

	default:
		return AgentResponse{
			Agent:  AgentExecutor,
			Name:   "EXECUTOR",
			Content: "Try 'executor, why blocked?' or 'executor, show activity'.",
		}
	}
}

// handleRuntime handles runtime-specific queries
func (r *Router) handleRuntime(message string, intent string) AgentResponse {
	_, pid, running := r.memory.GetRuntimeStatus()

	switch intent {
	case "status", "summarize":
		content := "[RUNTIME]\n\n"
		if running {
			content += fmt.Sprintf("Status: RUNNING (PID: %d)\n", pid)
		} else {
			content += "Status: STOPPED\n"
		}
		content += "Ticker: 5 minutes\n"
		content += "Next audit: automatic\n"
		return AgentResponse{Agent: AgentRuntime, Name: "RUNTIME", Content: content}

	default:
		return AgentResponse{
			Agent:  AgentRuntime,
			Name:   "RUNTIME",
			Content: "Try 'runtime, status'.",
		}
	}
}

// handleGeneral handles general queries
func (r *Router) handleGeneral(message string, intent string) AgentResponse {
	_ = r.memory.LoadFromDisk()

	content := "[SYSTEM]\n\n"
	content += "Available agents:\n"
	content += "- auditor: Explain/summarize findings\n"
	content += "- executor: Explain blocked operations\n"
	content += "- runtime: Show runtime status\n"
	content += "\nCommands: /audit /findings /status\n"
	content += "\nOr talk naturally: 'auditor, explain this error'"

	return AgentResponse{Agent: AgentSystem, Name: "SYSTEM", Content: content}
}