package intent

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aoms/smart-worker/internal/openhands"
)

// ExecutionLifecycle manages the conversational execution flow
type ExecutionLifecycle struct {
	mu             sync.RWMutex
	client        *openhands.Client
	Bridge        *openhandsBridge
	ActiveTask    *TaskExecution
	History      []Message
	CancelChan   chan struct{}
	IsCancelled bool
}

// TaskExecution represents an active task execution
type TaskExecution struct {
	ID          string
	Intent     *Intent
	Description string
	Status     string
	StartTime  time.Time
	Result    *openhands.ExecutionResult
}

// openhandsBridge wraps OpenHands client for lifecycle
type openhandsBridge struct {
	client *openhands.Client
}

// NewExecutionLifecycle creates a new execution lifecycle manager
func NewExecutionLifecycle() (*ExecutionLifecycle, error) {
	lc := &ExecutionLifecycle{
		CancelChan: make(chan struct{}),
	}
	
	// Try to create OpenHands client
	client, err := openhands.NewClientFromEnv()
	if err == nil && client != nil {
		lc.client = client
		lc.Bridge = &openhandsBridge{client: client}
	}
	
	return lc, nil
}

// Execute handles the execution flow based on inferred intent
func (lc *ExecutionLifecycle) Execute(intent *Intent, message string, repo string) (string, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	if intent == nil {
		return "I need more context to understand what you'd like me to do. Can you clarify?", nil
	}
	
	switch intent.Type {
	case IntentBlocking:
		return lc.handleBlocking(intent, message)
		
	case IntentInvestigation:
		return lc.handleInvestigation(intent, message)
		
	case IntentExecution:
		return lc.handleExecution(intent, message, repo)
		
	case IntentPlanning:
		return lc.handlePlanning(intent, message)
		
	case IntentPrioritization:
		return lc.handlePrioritization(intent, message)
		
	case IntentAuditing:
		return lc.handleAuditing(intent, message)
		
	case IntentClarification:
		return lc.handleClarification(intent, message)
		
	case IntentAcknowledgment:
		return lc.handleAcknowledgment(intent, message)
		
	default:
		return "I'm not sure what you'd like me to do. Could you clarify?", nil
	}
}

// handleBlocking stops any active execution
func (lc *ExecutionLifecycle) handleBlocking(intent *Intent, message string) (string, error) {
	// Cancel any active execution
	lc.IsCancelled = true
	close(lc.CancelChan)
	lc.CancelChan = make(chan struct{})
	
	lc.ActiveTask = nil
	
	return "Execution cancelled. What would you like me to do next?", nil
}

// handleInvestigation performs analysis only
func (lc *ExecutionLifecycle) handleInvestigation(intent *Intent, message string) (string, error) {
	// Analyze but do NOT execute
	response := "Analyzing the current state...\n\n"
	
	if lc.ActiveTask != nil {
		response += fmt.Sprintf("Current execution: %s (%s)\n", 
			lc.ActiveTask.Description, lc.ActiveTask.Status)
	}
	
	response += "\nAnalysis complete. Would you like me to suggest fixes, or would you prefer to investigate further?"
	
	return response, nil
}

// handleExecution dispatches to OpenHands
func (lc *ExecutionLifecycle) handleExecution(intent *Intent, message string, repo string) (string, error) {
	if lc.client == nil {
		return "OpenHands execution is not configured. Please set OPENHANDS_API_KEY.", nil
	}
	
	// Check if execution already in progress
	if lc.ActiveTask != nil && (lc.ActiveTask.Status == "RUNNING" || lc.ActiveTask.Status == "IN_PROGRESS") {
		return "There's already an execution in progress. Would you like me to stop it first?", nil
	}
	
	// Determine task description
	taskDesc := intent.Task
	if taskDesc == "" {
		taskDesc = message
	}
	
	// Start execution
	lc.IsCancelled = false
	lc.ActiveTask = &TaskExecution{
		ID:          "",
		Intent:     intent,
		Description: taskDesc,
		Status:     "STARTING",
		StartTime:  time.Now(),
	}
	
	// Dispatch to OpenHands in background
	go lc.dispatchExecution(taskDesc, repo)
	
	return "Dispatching execution to OpenHands...\n\nI'll monitor the execution and report back.", nil
}

// dispatchExecution handles async OpenHands dispatch
func (lc *ExecutionLifecycle) dispatchExecution(taskDesc, repo string) {
	timeout := 10 * time.Minute
	if lc.ActiveTask != nil && lc.ActiveTask.Intent != nil && lc.ActiveTask.Intent.Urgency == "high" {
		timeout = 5 * time.Minute
	}
	
	result, err := lc.client.Dispatch(taskDesc, timeout)
	
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	if lc.ActiveTask == nil {
		return
	}
	
	if err != nil || result == nil {
		lc.ActiveTask.Status = "FAILED"
		lc.ActiveTask.Result = &openhands.ExecutionResult{
			Status: "failed",
			Error:  fmt.Sprintf("%v", err),
		}
		log.Printf("[EXECUTION] Failed: %v", err)
		return
	}
	
	lc.ActiveTask.ID = result.TaskID
	lc.ActiveTask.Result = result
	
	if result.Status == "COMPLETED" {
		lc.ActiveTask.Status = "COMPLETED"
	} else if result.Status == "FAILED" {
		lc.ActiveTask.Status = "FAILED"
	} else {
		lc.ActiveTask.Status = result.Status
	}
	
	log.Printf("[EXECUTION] Completed: %s", result.Status)
}

// handlePlanning provides roadmap reasoning
func (lc *ExecutionLifecycle) handlePlanning(intent *Intent, message string) (string, error) {
	response := "Let me think about the path forward...\n\n"
	
	// Analyze current state
	if lc.ActiveTask != nil {
		response += fmt.Sprintf("Current execution: %s\n", lc.ActiveTask.Status)
	}
	
	response += "\nSuggested path:\n"
	response += "1. Investigate current state\n"
	response += "2. Identify required changes\n"
	response += "3. Apply fixes via execution\n"
	response += "4. Validate results\n\n"
	response += "Would you like me to proceed with any of these steps?"
	
	return response, nil
}

// handlePrioritization provides operational analysis
func (lc *ExecutionLifecycle) handlePrioritization(intent *Intent, message string) (string, error) {
	response := "Analyzing operational priorities...\n\n"
	
	if lc.ActiveTask != nil {
		response += fmt.Sprintf("Active: %s\n", lc.ActiveTask.Status)
	}
	
	response += "\nPriority order:\n"
	response += "1. Execution readiness\n"
	response += "2. Repository health\n"
	response += "3. Validation status\n\n"
	response += "Would you like me to address any of these?"
	
	return response, nil
}

// handleAuditing provides repository inspection
func (lc *ExecutionLifecycle) handleAuditing(intent *Intent, message string) (string, error) {
	response := "Auditing repository state...\n\n"
	
	response += "Recent activity:\n"
	for i, msg := range lc.History {
		if len(lc.History) > 5 && i < len(lc.History)-5 {
			continue
		}
		truncated := msg.Content
		if len(truncated) > 50 {
			truncated = truncated[:50] + "..."
		}
		response += fmt.Sprintf("- [%s] %s\n", msg.Role, truncated)
	}
	
	response += "\nWhat would you like me to audit in detail?"
	
	return response, nil
}

// handleClarification asks for more information
func (lc *ExecutionLifecycle) handleClarification(intent *Intent, message string) (string, error) {
	// Ask clarifying question based on context
	if lc.ActiveTask != nil {
		return "There's an active execution in progress. Would you like me to stop it or continue?", nil
	}
	
	return "I want to make sure I understand correctly. Would you like me to:\n- Analyze the current state?\n- Execute fixes?\n- Explain what I found?\n- Stop any running execution?", nil
}

// handleAcknowledgment acknowledges user
func (lc *ExecutionLifecycle) handleAcknowledgment(intent *Intent, message string) (string, error) {
	// Check if there was pending action
	if lc.ActiveTask != nil && lc.ActiveTask.Status == "COMPLETED" {
		return fmt.Sprintf("Execution completed: %s\n\nWould you like me to review the results?", 
			lc.ActiveTask.Description), nil
	}
	
	if lc.ActiveTask != nil {
		return fmt.Sprintf("Understood. Current execution: %s\n\nWhat would you like me to do?", 
			lc.ActiveTask.Status), nil
	}
	
	return "Understood. What would you like me to do next?", nil
}

// GetStatus returns current execution status
func (lc *ExecutionLifecycle) GetStatus() string {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	if lc.ActiveTask == nil {
		return "Ready for operational tasks"
	}
	
	return fmt.Sprintf("%s: %s (%s)", lc.ActiveTask.Status, 
		lc.ActiveTask.Description, 
		time.Since(lc.ActiveTask.StartTime).Round(time.Second))
}

// AddToHistory adds message to conversational history
func (lc *ExecutionLifecycle) AddToHistory(role, content string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	lc.History = append(lc.History, Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
	
	// Keep history bounded
	if len(lc.History) > 50 {
		lc.History = lc.History[len(lc.History)-50:]
	}
}

// GetHistory returns recent history
func (lc *ExecutionLifecycle) GetHistory(count int) []Message {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	if count >= len(lc.History) {
		return lc.History
	}
	
	return lc.History[len(lc.History)-count:]
}

// StopExecution stops any active execution
func (lc *ExecutionLifecycle) StopExecution() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	if lc.ActiveTask == nil {
		return fmt.Errorf("no active execution")
	}
	
	lc.IsCancelled = true
	if lc.CancelChan != nil {
		close(lc.CancelChan)
		lc.CancelChan = make(chan struct{})
	}
	
	lc.ActiveTask.Status = "CANCELLED"
	
	return nil
}

// IsExecuting returns true if there's an active execution
func (lc *ExecutionLifecycle) IsExecuting() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	return lc.ActiveTask != nil && (lc.ActiveTask.Status == "RUNNING" || lc.ActiveTask.Status == "IN_PROGRESS" || lc.ActiveTask.Status == "STARTING")
}

// GetActiveExecution returns the current execution
func (lc *ExecutionLifecycle) GetActiveExecution() *TaskExecution {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	return lc.ActiveTask
}

// HasOpenHands returns true if OpenHands is configured
func (lc *ExecutionLifecycle) HasOpenHands() bool {
	return lc.client != nil
}

// FormatLifecycleStatus formats lifecycle status for display
func FormatLifecycleStatus(lc *ExecutionLifecycle) string {
	var sb strings.Builder
	
	sb.WriteString("=== Execution Lifecycle ===\n")
	
	if lc.HasOpenHands() {
		sb.WriteString("OpenHands: configured\n")
	} else {
		sb.WriteString("OpenHands: not configured\n")
	}
	
	sb.WriteString("Status: ")
	sb.WriteString(lc.GetStatus())
	sb.WriteString("\n")
	
	history := lc.GetHistory(3)
	if len(history) > 0 {
		sb.WriteString("Recent:\n")
		for _, msg := range history {
			truncated := msg.Content
			if len(truncated) > 40 {
				truncated = truncated[:40] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", msg.Role, truncated))
		}
	}
	
	return sb.String()
}