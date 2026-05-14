package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aoms/smart-worker/internal/openhands"
)

// OpenHandsBridge provides the execution bridge to OpenHands runtime
type OpenHandsBridge struct {
	client     *openhands.Client
	Enabled   bool
	Ready     bool
}

// NewOpenHandsBridge creates a new OpenHands execution bridge
func NewOpenHandsBridge() (*OpenHandsBridge, error) {
	bridge := &OpenHandsBridge{
		Enabled: false,
		Ready:   false,
	}
	
	// Try to create client from environment
	client, err := openhands.NewClientFromEnv()
	if err != nil {
		log.Printf("[OPENHANDS] Client creation failed: %v", err)
		return bridge, nil
	}
	
	bridge.client = client
	bridge.Enabled = true
	
	// Validate auth
	if err := client.ValidateAuth(); err != nil {
		log.Printf("[OPENHANDS] Auth validation failed: %v", err)
		// Not ready but client exists
		return bridge, nil
	}
	
	bridge.Ready = true
	log.Printf("[OPENHANDS] Bridge initialized and authenticated")
	
	return bridge, nil
}

// IsAuthorized checks if execution is authorized
func (b *OpenHandsBridge) IsAuthorized() bool {
	return b.Enabled && b.Ready && b.client != nil && b.client.IsAuthorized()
}

// Execute dispatches an execution task to OpenHands
func (b *OpenHandsBridge) Execute(taskDescription string, repo string) (string, error) {
	if !b.IsAuthorized() {
		return "", fmt.Errorf("OpenHands execution not authorized")
	}
	
	// Update client with repository
	b.client.Repository = repo
	
	log.Printf("[OPENHANDS] Dispatching task: %s", truncate(taskDescription, 100))
	
	// Start execution
	timeout := 10 * time.Minute
	result, err := b.client.Dispatch(taskDescription, timeout)
	if err != nil {
		return "", fmt.Errorf("dispatch failed: %w", err)
	}
	
	// Format response
	return formatExecutionResult(result), nil
}

// ExecuteWithProgress executes with progress updates via callback
func (b *OpenHandsBridge) ExecuteWithProgress(taskDescription string, repo string, onProgress func(string)) (string, error) {
	if !b.IsAuthorized() {
		return "", fmt.Errorf("OpenHands execution not authorized")
	}
	
	b.client.Repository = repo
	
	log.Printf("[OPENHANDS] Dispatching task with progress: %s", truncate(taskDescription, 100))
	
	// Start execution
	startResult, err := b.client.StartExecution(taskDescription)
	if err != nil {
		return "", fmt.Errorf("failed to start: %w", err)
	}
	
	onProgress(fmt.Sprintf("Task started: %s", startResult.TaskID))
	
	// Poll for completion
	timeout := 10 * time.Minute
	result, err := b.client.PollExecution(startResult.TaskID, timeout, 10*time.Second)
	if err != nil {
		return "", fmt.Errorf("polling failed: %w", err)
	}
	
	return formatExecutionResult(result), nil
}

// GetStatus returns execution status
func (b *OpenHandsBridge) GetStatus() string {
	if !b.Enabled {
		return "disabled"
	}
	
	if !b.Ready {
		return "not_ready"
	}
	
	return "ready"
}

// GetBaseURL returns the configured base URL
func (b *OpenHandsBridge) GetBaseURL() string {
	if b.client == nil {
		return ""
	}
	return b.client.GetBaseURL()
}

// formatExecutionResult formats the execution result for display
func formatExecutionResult(result *openhands.ExecutionResult) string {
	var sb strings.Builder
	
	switch result.Status {
	case "COMPLETED":
		sb.WriteString("✅ Execution completed successfully\n\n")
		
		if len(result.FilesModified) > 0 {
			sb.WriteString("Files modified:\n")
			for _, f := range result.FilesModified {
				sb.WriteString(fmt.Sprintf("  - %s\n", f))
			}
			sb.WriteString("\n")
		}
		
		if result.Output != "" {
			sb.WriteString("Output:\n")
			sb.WriteString(result.Output)
		}
		
	case "FAILED", "ERROR":
		sb.WriteString("❌ Execution failed\n\n")
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		}
		
	case "TIMEOUT":
		sb.WriteString("⏱️ Execution timed out\n")
		
	default:
		sb.WriteString(fmt.Sprintf("Status: %s\n", result.Status))
	}
	
	return sb.String()
}

// truncate truncates a string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// initOpenHandsRuntime initializes OpenHands bridge and logs status
func initOpenHandsRuntime() string {
	bridge, err := NewOpenHandsBridge()
	if err != nil {
		return fmt.Sprintf("OpenHands runtime init error: %v", err)
	}
	
	if !bridge.Enabled {
		return "OpenHands runtime: credentials not configured"
	}
	
	if !bridge.Ready {
		return "OpenHands runtime initialized (auth pending validation)"
	}
	
	return "OpenHands runtime initialized\nExecution runtime ready\nAuthenticated successfully"
}

// checkOpenHandsReadiness returns the readiness status
func checkOpenHandsReadiness() map[string]string {
	status := map[string]string{
		"connectivity":     "unknown",
		"execution":      "unknown",
		"authorization":  "unknown",
	}
	
	// Check connectivity
	resp, err := http.Get("https://app.all-hands.dev/api/v1/status")
	if err != nil {
		status["connectivity"] = "failed"
	} else {
		resp.Body.Close()
		if resp.StatusCode < 500 {
			status["connectivity"] = "ok"
		} else {
			status["connectivity"] = "error"
		}
	}
	
	// Check client authorization
	bridge, err := NewOpenHandsBridge()
	if err != nil || !bridge.Enabled {
		status["execution"] = "not_configured"
		status["authorization"] = "not_configured"
		return status
	}
	
	status["execution"] = "configured"
	
	if bridge.IsAuthorized() {
		status["authorization"] = "authorized"
	} else {
		status["authorization"] = "not_authorized"
	}
	
	return status
}