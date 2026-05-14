package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Dispatcher manages GitHub Actions workflow execution
type Dispatcher struct {
	mu          sync.RWMutex
	httpClient   *http.Client
	token      string
	owner      string
	repo       string
	workflowID string
	runs       map[string]*WorkflowRun
	webhookURL string
}

// WorkflowRun represents an executing workflow
type WorkflowRun struct {
	ID          string    `json:"id"`
	RunID       int       `json:"run_id"`
	Task        string    `json:"task"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Result     string    `json:"result,omitempty"`
}

// SimplifiedPayload - minimal inputs for autonomous workflow dispatch
type SimplifiedPayload struct {
	Task        string `json:"task"`
	Repository string `json:"repository"`
}

// OperationalPayload kept for compatibility (deprecated)
type OperationalPayload struct {
	Task        string `json:"task"`
	Repository string `json:"repository"`
	Context    string `json:"context,omitempty"`
	Findings  string `json:"findings,omitempty"`
	Roadmap   string `json:"roadmap,omitempty"`
	Intent    string `json:"intent"`
	Urgency   string `json:"urgency"`
}

// NewDispatcher creates a new workflow dispatcher
func NewDispatcher() (*Dispatcher, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_API_TOKEN")
	}
	
	owner := os.Getenv("GITHUB_OWNER")
	repo := os.Getenv("GITHUB_REPOSITORY")
	
	// Extract owner/repo from full path if needed
	if repo == "" && os.Getenv("GITHUB_REPOSITORY") != "" {
		fullPath := os.Getenv("GITHUB_REPOSITORY")
		parts := strings.Split(fullPath, "/")
		if len(parts) >= 2 {
			owner = parts[0]
			repo = parts[1]
		}
	}
	
	return &Dispatcher{
		token:      token,
		owner:     owner,
		repo:      repo,
		workflowID: "operational-runtime",
		runs:      make(map[string]*WorkflowRun),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// DispatchSimplified - minimal payload, workflow auto-infers context
func (d *Dispatcher) DispatchSimplified(task, repository string) (*WorkflowRun, error) {
	if d.token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required for workflow dispatch")
	}
	
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches",
		d.owner, d.repo, d.workflowID)
	
	// Only task and repository - workflow auto-infers everything else
	body := map[string]interface{}{
		"ref": "main",
		"inputs": map[string]interface{}{
			"task":       task,
			"repository": repository,
		},
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch workflow: %w", err)
	}
	defer resp.Body.Close()
	
	respBody, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != 204 && resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("workflow dispatch failed: status %d, body: %s", resp.StatusCode, string(respBody))
	}
	
	// Parse response for run ID
	var result struct {
		RunID int `json:"run_id"`
	}
	json.Unmarshal(respBody, &result)
	
	run := &WorkflowRun{
		ID:         fmt.Sprintf("%d", result.RunID),
		RunID:      result.RunID,
		Task:      task,
		Status:    "queued",
		CreatedAt: time.Now(),
	}
	
	d.mu.Lock()
	d.runs[run.ID] = run
	d.mu.Unlock()
	
	return run, nil
}

// Dispatch dispatches an operational workflow (backward compatible)
func (d *Dispatcher) Dispatch(payload SimplifiedPayload) (*WorkflowRun, error) {
	return d.DispatchSimplified(payload.Task, payload.Repository)
}

// GetRunStatus gets the status of a workflow run
func (d *Dispatcher) GetRunStatus(runID string) (*WorkflowRun, error) {
	d.mu.RLock()
	run, exists := d.runs[runID]
	d.mu.RUnlock()
	
	if !exists {
		return &WorkflowRun{Status: "not_found"}, nil
	}
	
	// If we have a token, poll GitHub API for status
	if d.token == "" {
		return run, nil
	}
	
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s",
		d.owner, d.repo, run.RunID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return run, nil
	}
	
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return run, nil
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return run, nil
	}
	
	var result struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		UpdatedAt string `json:"updated_at"`
	}
	
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &result)
	
	run.Status = result.Status
	run.Conclusion = result.Conclusion
	
	if t, err := time.Parse(time.RFC3339, result.UpdatedAt); err == nil {
		run.UpdatedAt = t
	}
	
	// Map status
	switch result.Status {
	case "completed":
		if result.Conclusion == "success" {
			run.Status = "completed"
		} else {
			run.Status = "failed"
		}
	}
	
	return run, nil
}

// PollRun polls until workflow completes
func (d *Dispatcher) PollRun(runID string, timeout time.Duration, interval time.Duration) (*WorkflowRun, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		run, err := d.GetRunStatus(runID)
		if err != nil {
			return nil, err
		}
		
		// Terminal states
		if run.Status == "completed" || run.Status == "failed" || run.Status == "not_found" {
			return run, nil
		}
		
		time.Sleep(interval)
	}
	
	return &WorkflowRun{
		ID:      runID,
		Status:  "timeout",
		Task:    "unknown",
	}, nil
}

// DispatchAndWait dispatches workflow and waits for completion
func (d *Dispatcher) DispatchAndWait(payload OperationalPayload, timeout time.Duration) (*WorkflowRun, error) {
	run, err := d.Dispatch(payload)
	if err != nil {
		return nil, err
	}
	
	if run.Status == "queued" {
		return d.PollRun(run.ID, timeout, 10*time.Second)
	}
	
	return run, nil
}

// IsConfigured checks if dispatcher is properly configured
func (d *Dispatcher) IsConfigured() bool {
	return d.token != "" && d.owner != "" && d.repo != ""
}

// GetStatus returns dispatcher status
func (d *Dispatcher) GetStatus() string {
	if !d.IsConfigured() {
		return "not_configured"
	}
	
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	active := 0
	for _, run := range d.runs {
		if run.Status == "queued" || run.Status == "in_progress" {
			active++
		}
	}
	
	if active > 0 {
		return fmt.Sprintf("active: %d runs", active)
	}
	
	return "ready"
}

// NewPayload creates a new operational payload
func NewPayload(task, repo, context, findings, roadmap, intent, urgency string) OperationalPayload {
	return OperationalPayload{
		Task:        task,
		Repository: repo,
		Context:    context,
		Findings:  findings,
		Roadmap:   roadmap,
		Intent:    intent,
		Urgency:   urgency,
	}
}

// NewDispatcherFromEnv is convenience constructor
func NewDispatcherFromEnv() (*Dispatcher, error) {
	return NewDispatcher()
}

// FormatRunStatus formats run status for display
func FormatRunStatus(run *WorkflowRun) string {
	if run == nil {
		return "No run data"
	}
	
	var sb strings.Builder
	sb.WriteString("Workflow: ")
	sb.WriteString(run.Status)
	
	if run.Task != "" {
		sb.WriteString("\nTask: ")
		sb.WriteString(run.Task)
	}
	
	if run.Conclusion != "" {
		sb.WriteString("\nConclusion: ")
		sb.WriteString(run.Conclusion)
	}
	
	return sb.String()
}

// CancelRun cancels a running workflow
func (d *Dispatcher) CancelRun(runID string) error {
	if d.token == "" {
		return fmt.Errorf("GITHUB_TOKEN required")
	}
	
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/runs/%s/cancel",
		d.owner, d.repo, runID)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("cancel failed: status %d", resp.StatusCode)
	}
	
	d.mu.Lock()
	if run, exists := d.runs[runID]; exists {
		run.Status = "cancelled"
	}
	d.mu.Unlock()
	
	return nil
}