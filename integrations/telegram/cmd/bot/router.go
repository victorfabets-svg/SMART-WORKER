package main

import (
	"fmt"
	"os"
	"strings"

	telegramcontext "github.com/aoms/smart-worker/integrations/telegram/context"
	"github.com/aoms/smart-worker/internal/intent"
	"github.com/aoms/smart-worker/internal/workflow"
)

// AgentType the type of operational agent
type AgentType int

const (
	AgentUnknown AgentType = iota
	AgentAuditor
	AgentExecutor
	AgentRuntime
	AgentSystem
	AgentCognition // New cognitive agent
)

// AgentResponse response from an agent
type AgentResponse struct {
	Agent  AgentType
	Name   string
	Content string
}

// Router routes messages to appropriate agent using operational cognition
type Router struct {
	memory     *telegramcontext.OperationalMemory
	cognition *intent.CognitionEngine
	lifecycle *intent.ExecutionLifecycle
	workflow  *workflow.Engine
}

// NewRouter creates a new router
func NewRouter(mem *telegramcontext.OperationalMemory) *Router {
	log.Printf("[ROUTER] ================================================")
	log.Printf("[ROUTER] NewRouter ENTERED")
	log.Printf("[ROUTER] ================================================")
	
	router := &Router{
		memory: mem,
	}
	
	// Initialize cognitive intent engine
	if cognition, err := intent.NewCognitionEngine(); err == nil {
		router.cognition = cognition
		log.Printf("[ROUTER] cognition engine created (not required)")
	} else {
		log.Printf("[ROUTER] cognition engine NOT created (optional)")
	}
	
	// Initialize execution lifecycle
	log.Printf("[ROUTER] Creating ExecutionLifecycle...")
	if lifecycle, err := intent.NewExecutionLifecycle(); err == nil {
		router.lifecycle = lifecycle
		hasOH := lifecycle.HasOpenHands()
		log.Printf("[ROUTER] lifecycle created, HasOpenHands: %v", hasOH)
		
		if !hasOH {
			log.Printf("[ROUTER] WARNING: lifecycle created but OpenHands NOT available")
			log.Printf("[ROUTER] This means OPENHANDS_API_KEY was not found in environment")
		}
	} else {
		log.Printf("[ROUTER] FATAL: lifecycle creation failed: %v", err)
	}
	
	// Initialize workflow operational engine
	router.workflow = workflow.NewEngine()
	
	log.Printf("[ROUTER] ================================================")
	log.Printf("[ROUTER] Router initialization complete")
	log.Printf("[ROUTER] lifecycle: %v, HasOpenHands: %v", 
		router.lifecycle != nil, 
		func() bool { 
			if router.lifecycle != nil {
				return router.lifecycle.HasOpenHands()
			}
			return false 
		}())
	log.Printf("[ROUTER] ================================================")
	
	return router
}

// Route routes a message to the appropriate agent
// CRITICAL: Must return REAL error if OpenHands not available - NO templates allowed
func (r *Router) Route(message string) AgentResponse {
	log.Printf("[ROUTER] Route ENTERED: message=%.50s", message)
	
	// If lifecycle with OpenHands is available, use it directly
	if r.lifecycle != nil && r.lifecycle.HasOpenHands() {
		log.Printf("[ROUTER] OpenHands available, using routeCognitively")
		return r.routeCognitively(message)
	}
	
	// CRITICAL: OpenHands NOT available - return REAL error, NOT template
	log.Printf("[ROUTER] CRITICAL: OpenHands NOT available - returning error")
	return AgentResponse{
		Agent:  AgentSystem,
		Name:   "NO_OPENHANDS",
		Content: "OpenHands is not configured. Please ensure OPENHANDS_API_KEY is available in the runtime environment.",
	}
}

// routeCognitively uses intent cognition and workflow execution
// When OpenHands is configured, messages are sent directly to OpenHands LLM
func (r *Router) routeCognitively(message string) AgentResponse {
	log.Printf("[ROUTER] routeCognitively ENTERED with message: %s", message)
	
	// Build execution context from memory
	ctx := r.buildExecutionContext()
	
	// Get conversational history
	var history []intent.Message
	if r.lifecycle != nil {
		history = r.lifecycle.GetHistory(5)
		log.Printf("[ROUTER] lifecycle available, history length: %d", len(history))
	} else {
		log.Printf("[ROUTER] WARNING - r.lifecycle is NIL")
	}
	
	// Check if OpenHands is configured - if so, use it directly for responses
	if r.lifecycle != nil && r.lifecycle.HasOpenHands() {
		log.Printf("[ROUTER] OpenHands IS configured, dispatching...")
		
		repo := os.Getenv("GITHUB_REPOSITORY")
		if repo == "" {
			repo = "aoms/smart-worker"
		}
		
		// Try to execute via OpenHands dispatch
		response, err := r.lifecycle.Execute(nil, message, repo)
		log.Printf("[ROUTER] lifecycle.Execute returned, response: %.100s, err: %v", response, err)
		
		if err == nil && response != "" {
			// Add to history
			r.lifecycle.AddToHistory("user", message)
			r.lifecycle.AddToHistory("assistant", response)
			
			return AgentResponse{
				Agent:  AgentCognition,
				Name:   "OPENHANDS",
				Content: response,
			}
		}
		
		// Error case - should NOT fallback to templates
		if err != nil {
			log.Printf("[ROUTER] ERROR - lifecycle failed: %v", err)
			return AgentResponse{
				Agent:  AgentCognition,
				Name:   "ERROR",
				Content: fmt.Sprintf("OpenHands dispatch error: %v", err),
			}
		}
		if response == "" {
			log.Printf("[ROUTER] WARNING - lifecycle returned empty response")
			return AgentResponse{
				Agent:  AgentCognition,
				Name:   "EMPTY",
				Content: "OpenHands returned empty response",
			}
		}
	} else {
		log.Printf("[ROUTER] WARNING - OpenHands NOT configured or HasOpenHands() returns false")
	}
	
	// FINAL CHECK: If no usable OpenHands, return REAL error - NOT template
	log.Printf("[ROUTER] FINAL: OpenHands not available")
	return AgentResponse{
		Agent:  AgentCognition,
		Name:   "NO_OPENHANDS",
		Content: "OpenHands is not configured. Please set OPENHANDS_API_KEY environment variable.",
	}
}
	

// buildExecutionContext builds execution context from operational memory
func (r *Router) buildExecutionContext() *intent.ExecutionContext {
	ctx := &intent.ExecutionContext{
		Repository: os.Getenv("GITHUB_REPOSITORY"),
	}
	
	// Get operational state from memory
	if r.memory != nil {
		_ = r.memory.LoadFromDisk()
		ctx.Operational_state = "active"
		
		// Get findings
		findings := r.memory.GetLatestFindings()
		for _, f := range findings {
			ctx.RecentFindings = append(ctx.RecentFindings, intent.Finding{
				Title:    f.Title,
				Severity: f.Severity,
				Evidence: f.Evidence,
			})
		}
	}
	
	return ctx
}

// detectAgent detects which agent to route to
func (r *Router) detectAgent(message string) AgentType {
	auditorTriggers := []string{"auditor", "audit", "finding", "bug", "error", "explain"}
	executorTriggers := []string{"executor", "execute", "fix", "apply", "why blocked", "didn't fix", "blocked", "run", "corrija", "execute task"}
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
	// Check if this is an execution request that should be dispatched to OpenHands
	lower := strings.ToLower(message)
	executionTriggers := []string{"corrija", "execute", "execute task", "run task", "fix", "apply fix"}
	
	for _, t := range executionTriggers {
		if strings.Contains(lower, t) && !strings.Contains(lower, "why") && !strings.Contains(lower, "blocked") {
			// This is an execution request - dispatch to OpenHands
			return r.handleOpenHandsExecution(message)
		}
	}
	
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

// handleGeneral removes all procedural responses - uses pure cognition
func (r *Router) handleGeneral(message string, intent string) AgentResponse {
	// ALWAYS use cognitive routing - NEVER procedural
	return r.routeCognitively(message)
}

// handleOpenHandsExecution handles execution requests via OpenHands
func (r *Router) handleOpenHandsExecution(message string) AgentResponse {
	// Create OpenHands bridge
	bridge, err := NewOpenHandsBridge()
	if err != nil {
		return AgentResponse{
			Agent:  AgentExecutor,
			Name:   "EXECUTOR",
			Content: fmt.Sprintf("OpenHands initialization error: %v", err),
		}
	}
	
	if !bridge.IsAuthorized() {
		return AgentResponse{
			Agent:  AgentExecutor,
			Name:   "EXECUTOR",
			Content: "OpenHands execution not authorized. Please configure OPENHANDS_API_KEY.",
		}
	}
	
	// Get repository from environment
	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		repo = "aoms/smart-worker"
	}
	
	// Dispatch to OpenHands
	result, err := bridge.Execute(message, repo)
	if err != nil {
		return AgentResponse{
			Agent:  AgentExecutor,
			Name:   "EXECUTOR",
			Content: fmt.Sprintf("Execution failed: %v", err),
		}
	}
	
	return AgentResponse{
		Agent:  AgentExecutor,
		Name:   "EXECUTOR",
		Content: "Dispatching execution to OpenHands...\n\n" + result,
	}
}