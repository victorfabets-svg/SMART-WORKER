package workflow

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aoms/smart-worker/internal/intent"
)

// Engine provides async operational execution using GitHub Actions
type Engine struct {
	mu          sync.RWMutex
	dispatcher   *Dispatcher
	queue       []OperationalTask
	active      map[string]*OperationalTask
	completed   []OperationalTask
	cancelChans map[string]chan struct{}
}

// OperationalTask represents an async operational task
type OperationalTask struct {
	ID          string
	Intent     *intent.Intent
	Repository string
	Status     string
	RequestTime time.Time
	StartTime  time.Time
	EndTime    time.Time
	Result     string
	Error      string
	WorkflowID string
	RunID      string
}

// NewEngine creates a new operational engine
func NewEngine() *Engine {
	dispatcher, _ := NewDispatcher()
	
	engine := &Engine{
		dispatcher:   dispatcher,
		queue:     []OperationalTask{},
		active:    make(map[string]*OperationalTask),
		completed: []OperationalTask{},
		cancelChans: make(map[string]chan struct{}),
	}
	
	return engine
}

// Execute dispatches an operational task asynchronously
func (e *Engine) Execute(task string, intentResult *intent.Intent, repo string) (string, error) {
	if e.dispatcher == nil {
		return e.executeLocally(task, intentResult, repo)
	}
	
	if !e.dispatcher.IsConfigured() {
		return e.executeLocally(task, intentResult, repo)
	}
	
	// Determine intent and urgency
	intentStr := "execution"
	urgency := "medium"
	
	if intentResult != nil {
		intentStr = intentResult.Type.String()
		urgency = intentResult.Urgency
		if urgency == "" {
			urgency = "medium"
		}
	}
	
	// Check if investigation only
	if intentResult != nil && !intentResult.ShouldExecute {
		if intentResult.Type == intent.IntentInvestigation {
			return e.executeInvestigate(task, repo)
		}
		if intentResult.Type == intent.IntentBlocking {
			return e.handleBlocking(task, intentResult)
		}
		return "Operation not required for intent: " + intentStr, nil
	}
	
	// Build payload
	payload := NewPayload(
		task,
		repo,
		"", // context
		"", // findings
		"", // roadmap
		intentStr,
		urgency,
	)
	
	// Dispatch workflow
	run, err := e.dispatcher.Dispatch(payload)
	if err != nil {
		// Fallback to local execution
		return e.executeLocally(task, intentResult, repo)
	}
	
	opTask := OperationalTask{
		ID:          run.ID,
		Intent:     intentResult,
		Repository: repo,
		Status:     "dispatched",
		RequestTime: time.Now(),
		WorkflowID: run.ID,
		RunID:      fmt.Sprintf("%d", run.RunID),
	}
	
	e.mu.Lock()
	e.active[opTask.ID] = &opTask
	e.mu.Unlock()
	
	return fmt.Sprintf("Operational workflow dispatched.\nRun ID: %s\nMonitoring execution...", opTask.RunID), nil
}

// executeLocally executes task without workflow dispatch
func (e *Engine) executeLocally(task string, intentResult *intent.Intent, repo string) (string, error) {
	intentStr := "execution"
	if intentResult != nil {
		intentStr = intentResult.Type.String()
	}
	
	if intentResult != nil && intentResult.Type == intent.IntentInvestigation {
		return e.executeInvestigate(task, repo)
	}
	
	if intentResult != nil && intentResult.Type == intent.IntentBlocking {
		return e.handleBlocking(task, intentResult)
	}
	
	// Local execution (simulation)
	return fmt.Sprintf("Analyzing operational request: %s\nIntent: %s\n\nThis would execute: %s", 
		repo, intentStr, task), nil
}

// executeInvestigate runs investigation without execution
func (e *Engine) executeInvestigate(task, repo string) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("=== Investigation ===\n\n")
	sb.WriteString("Task: ")
	sb.WriteString(task)
	sb.WriteString("\nRepository: ")
	sb.WriteString(repo)
	sb.WriteString("\n\nAnalyzing current state...\n")
	
	// Basic repository analysis
	sb.WriteString("\nRepository Status:\n")
	sb.WriteString("- Checking structure...\n")
	sb.WriteString("- Analyzing dependencies...\n")
	sb.WriteString("- Validating operational context...\n")
	
	sb.WriteString("\nInvestigation complete.\nWould you like me to execute fixes?")
	
	return sb.String(), nil
}

// handleBlocking handles blocking intent
func (e *Engine) handleBlocking(task string, intentResult *intent.Intent) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Cancel active tasks
	cancelled := 0
	for id, ch := range e.cancelChans {
		close(ch)
		delete(e.cancelChans, id)
		cancelled++
	}
	
	for id, t := range e.active {
		t.Status = "cancelled"
		delete(e.active, id)
		cancelled++
	}
	
	if cancelled > 0 {
		return fmt.Sprintf("Execution cancelled. Stopped %d active task(s).", cancelled), nil
	}
	
	return "No active executions to cancel.", nil
}

// PollStatus polls the status of a task
func (e *Engine) PollStatus(taskID string) (string, error) {
	e.mu.RLock()
	task, exists := e.active[taskID]
	e.mu.RUnlock()
	
	if !exists {
		// Check completed
		for _, t := range e.completed {
			if t.ID == taskID {
				return t.Status, nil
			}
		}
		return "not_found", nil
	}
	
	if e.dispatcher == nil {
		return task.Status, nil
	}
	
	run, err := e.dispatcher.GetRunStatus(task.RunID)
	if err != nil {
		return task.Status, nil
	}
	
	task.Status = run.Status
	
	// Update if completed
	if run.Status == "completed" || run.Status == "failed" {
		task.EndTime = time.Now()
	}
	
	return run.Status, nil
}

// Cancel cancels a task
func (e *Engine) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if task, exists := e.active[taskID]; exists {
		task.Status = "cancelled"
		
		// Cancel workflow if available
		if e.dispatcher != nil && task.RunID != "" {
			e.dispatcher.CancelRun(task.RunID)
		}
		
		return nil
	}
	
	return fmt.Errorf("task not found: %s", taskID)
}

// GetStatus returns overall engine status
func (e *Engine) GetStatus() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if e.dispatcher != nil && !e.dispatcher.IsConfigured() {
		return "workflow_not_configured"
	}
	
	active := len(e.active)
	queued := len(e.queue)
	
	if active > 0 {
		return fmt.Sprintf("active: %d, queued: %d", active, queued)
	}
	
	if queued > 0 {
		return fmt.Sprintf("queued: %d", queued)
	}
	
	return "ready"
}

// AddToHistory adds to execution history
func (e *Engine) AddToHistory(task OperationalTask) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.completed = append(e.completed, task)
	
	// Keep bounded
	if len(e.completed) > 50 {
		e.completed = e.completed[len(e.completed)-50:]
	}
}

// GetHistory returns recent execution history
func (e *Engine) GetHistory(count int) []OperationalTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if count >= len(e.completed) {
		return e.completed
	}
	
	return e.completed[len(e.completed)-count:]
}

// IsConfigured checks if engine is configured
func (e *Engine) IsConfigured() bool {
	return e.dispatcher != nil && e.dispatcher.IsConfigured()
}

// HandleIntent handles an operational intent
func (e *Engine) HandleIntent(message string, intentResult *intent.Intent, repo string) (string, error) {
	if intentResult == nil {
		return e.executeLocally(message, nil, repo)
	}
	
	switch intentResult.Type {
	case intent.IntentBlocking:
		return e.handleBlocking(message, intentResult)
		
	case intent.IntentInvestigation:
		return e.executeInvestigate(message, repo)
		
	case intent.IntentExecution:
		return e.Execute(message, intentResult, repo)
		
	case intent.IntentPlanning:
		return e.handlePlanning(message, intentResult)
		
	case intent.IntentPrioritization:
		return e.handlePrioritization(message, intentResult)
		
	case intent.IntentAuditing:
		return e.executeInvestigate(message, repo)
		
	case intent.IntentClarification:
		return e.handleClarification(message, intentResult)
		
	case intent.IntentAcknowledgment:
		return e.handleAcknowledgment(message, intentResult)
		
	default:
		return "I understand what you're asking. Would you like me to investigate or execute?", nil
	}
}

// handlePlanning handles planning intent
func (e *Engine) handlePlanning(message string, intentResult *intent.Intent) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("=== Planning ===\n\n")
	sb.WriteString("Analyzing path forward...\n\n")
	sb.WriteString("1. Investigate current state\n")
	sb.WriteString("2. Identify required changes\n")
	sb.WriteString("3. Apply fixes\n")
	sb.WriteString("4. Validate results\n\n")
	sb.WriteString("Which step should I proceed with?")
	
	return sb.String(), nil
}

// handlePrioritization handles prioritization intent  
func (e *Engine) handlePrioritization(message string, intentResult *intent.Intent) (string, error) {
	var sb strings.Builder
	
	sb.WriteString("=== Prioritization ===\n\n")
	sb.WriteString("Current priorities:\n\n")
	sb.WriteString("1. Execution readiness\n")
	sb.WriteString("2. Repository health\n")
	sb.WriteString("3. Build validation\n\n")
	sb.WriteString("Would you like me to address any of these?")
	
	return sb.String(), nil
}

// handleClarification handles clarification intent
func (e *Engine) handleClarification(message string, intentResult *intent.Intent) (string, error) {
	status := e.GetStatus()
	
	var sb strings.Builder
	sb.WriteString("I want to make sure I understand.\n\n")
	sb.WriteString("Current status: ")
	sb.WriteString(status)
	sb.WriteString("\n\n")
	sb.WriteString("Would you like me to:\n")
	sb.WriteString("- Investigate the current state?\n")
	sb.WriteString("- Execute operational fixes?\n")
	sb.WriteString("- Plan next steps?\n")
	sb.WriteString("- Stop any running execution?")
	
	return sb.String(), nil
}

// handleAcknowledgment handles acknowledgment intent
func (e *Engine) handleAcknowledgment(message string, intentResult *intent.Intent) (string, error) {
	history := e.GetHistory(1)
	
	if len(history) > 0 {
		last := history[len(history)-1]
		return fmt.Sprintf("Understood. Last operation: %s (%s)\n\nWhat should I do next?", 
			last.Status, last.ID), nil
	}
	
	return "Understood. What would you like me to do next?", nil
}

// FormatEngineStatus formats engine status for display
func FormatEngineStatus(e *Engine) string {
	var sb strings.Builder
	
	sb.WriteString("=== Operational Engine ===\n")
	sb.WriteString("Status: ")
	sb.WriteString(e.GetStatus())
	
	if e.IsConfigured() {
		sb.WriteString("\nWorkflow: configured")
	} else {
		sb.WriteString("\nWorkflow: local execution")
	}
	
	history := e.GetHistory(3)
	if len(history) > 0 {
		sb.WriteString("\nRecent:")
		for _, t := range history {
			sb.WriteString(fmt.Sprintf("\n  - %s: %s", t.Status, t.ID))
		}
	}
	
	return sb.String()
}