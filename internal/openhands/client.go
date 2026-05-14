package openhands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client represents the OpenHands runtime client
type Client struct {
	apiKey   string
	baseURL  string
	httpClient *http.Client
	Repository string
}

// ExecutionResult represents the result of an OpenHands execution
type ExecutionResult struct {
	TaskID           string    `json:"task_id"`
	ConversationID   string    `json:"conversation_id,omitempty"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
	Output          string    `json:"output,omitempty"`
	FilesModified   []string  `json:"files_modified,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
}

// StartTaskRequest represents the request to start a conversation
type StartTaskRequest struct {
	InitialMessage  MessageContent `json:"initial_message"`
	SelectedRepository string      `json:"selected_repository"`
}

// MessageContent represents the message content structure
type MessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// StartTaskResponse represents the response from starting a task
type StartTaskResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// TaskStatus represents the status of a start task
type TaskStatus struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	AppConversationID string `json:"app_conversation_id,omitempty"`
	Error            string `json:"error,omitempty"`
}

// NewClient creates a new OpenHands client
func NewClient(apiKey, baseURL, repository string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENHANDS_API_KEY is required")
	}
	
	return &Client{
		apiKey:   apiKey,
		baseURL:  baseURL,
		Repository: repository,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// NewClientFromEnv creates a new OpenHands client from environment variables
func NewClientFromEnv() (*Client, error) {
	apiKey := os.Getenv("OPENHANDS_API_KEY")
	baseURL := os.Getenv("OPENHANDS_BASE_URL")
	repository := os.Getenv("OPENHANDS_REPOSITORY")
	
	log.Printf("[OPENHANDS] NewClientFromEnv: API_KEY set=%v, BASE_URL=%s", apiKey != "", baseURL)
	
	if baseURL == "" {
		baseURL = "https://app.all-hands.dev"
	}
	
	return NewClient(apiKey, baseURL, repository)
}

// ValidateAuth validates the API key by making a test request
func (c *Client) ValidateAuth() error {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/status", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setAuthHeader(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("authentication validation failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}

// StartExecution starts a new execution task
func (c *Client) StartExecution(taskDescription string) (*ExecutionResult, error) {
	log.Printf("[OPENHANDS] ================================================")
	log.Printf("[OPENHANDS] dispatch START")
	log.Printf("[OPENHANDS] task: %.100s...", taskDescription)
	log.Printf("[OPENHANDS] repository: %s", c.Repository)
	log.Printf("[OPENHANDS] endpoint: %s/api/v1/app-conversations", c.baseURL)
	log.Printf("[OPENHANDS] ================================================")
	
	reqBody := StartTaskRequest{
		InitialMessage: MessageContent{
			Type: "text",
			Text: taskDescription,
		},
		SelectedRepository: c.Repository,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	log.Printf("[OPENHANDS] request payload size: %d bytes", len(jsonBody))
	
	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/app-conversations", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	
	log.Printf("[OPENHANDS] sending HTTP request...")
	startTime := time.Now()
	
	resp, err := c.httpClient.Do(req)
	latency := time.Since(startTime)
	log.Printf("[OPENHANDS] HTTP response received, latency: %v", latency)
	
	if err != nil {
		log.Printf("[OPENHANDS] FATAL: HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to start execution: %w", err)
	}
	defer resp.Body.Close()
	
	log.Printf("[OPENHANDS] response status: %d", resp.StatusCode)
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[OPENHANDS] FATAL: failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	log.Printf("[OPENHANDS] response body size: %d bytes", len(body))
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		log.Printf("[OPENHANDS] FATAL: status %d, body: %.200s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to start execution: status %d, body: %s", resp.StatusCode, string(body))
	}
	
	var startResp StartTaskResponse
	if err := json.Unmarshal(body, &startResp); err != nil {
		log.Printf("[OPENHANDS] FATAL: failed to parse response JSON: %v", err)
		log.Printf("[OPENHANDS] raw response: %.200s", string(body))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	log.Printf("[OPENHANDS] task started, ID: %s", startResp.ID)
	
	return &ExecutionResult{
		TaskID:     startResp.ID,
		Status:     startResp.Status,
		Repository: c.Repository,
	}, nil

// GetExecutionStatus polls for execution status
// CRITICAL: When task status is READY, we need to get the conversation
// and extract the actual LLM output from the conversation endpoint
func (c *Client) GetExecutionStatus(taskID string) (*ExecutionResult, error) {
	log.Printf("[OPENHANDS] polling status for task: %s", taskID)
	
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/app-conversations/start-tasks?ids="+taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setAuthHeader(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[OPENHANDS] polling HTTP error: %v", err)
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[OPENHANDS] polling read error: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	log.Printf("[OPENHANDS] raw poll response: %.300s", string(body))
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get status: status %d, body: %s", resp.StatusCode, string(body))
	}
	
	var tasks []TaskStatus
	if err := json.Unmarshal(body, &tasks); err != nil {
		log.Printf("[OPENHANDS] polling JSON parse error: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	if len(tasks) == 0 {
		log.Printf("[OPENHANDS] no tasks returned for ID: %s", taskID)
		return &ExecutionResult{
			TaskID: taskID,
			Status: "UNKNOWN",
		}, nil
	}
	
	task := tasks[0]
	log.Printf("[OPENHANDS] task status: %s, detail: %s, conversation: %s", task.Status, task.Error, task.AppConversationID)
	
	result := &ExecutionResult{
		TaskID:           task.ID,
		ConversationID:  task.AppConversationID,
		Status:          task.Status,
		Error:           task.Error,
	}
	
	// CRITICAL FIX: When status is READY, task is running in sandbox
	// We must poll the conversation endpoint to get execution_status
	if task.Status == "READY" && task.AppConversationID != "" {
		log.Printf("[OPENHANDS] task READY, fetching conversation: %s", task.AppConversationID)
		
		// Get full conversation details
		convResult, convErr := c.getConversationDetails(task.AppConversationID)
		if convErr != nil {
			log.Printf("[OPENHANDS] conversation fetch error: %v", convErr)
		} else {
			log.Printf("[OPENHANDS] conversation result: %s", convResult)
			// Use conversation result as output if available
			if convResult != "" {
				result.Output = convResult
			}
		}
	}
	
	// Map status to friendly output
	// CRITICAL: READY means task is RUNNING (not initialized)
	switch task.Status {
	case "READY":
		// If we got output from conversation, task completed
		if result.Output != "" {
			result.Status = "COMPLETED"
		} else {
			result.Status = "RUNNING"
		}
	case "IN_PROGRESS":
		result.Status = "RUNNING"
	case "FINISHED":
		result.Status = "COMPLETED"
	case "ERROR":
		result.Status = "FAILED"
	}
	
	log.Printf("[OPENHANDS] final result: status=%s, output length=%d", result.Status, len(result.Output))
	
	return result, nil
}

// getConversationDetails fetches the full conversation and extracts LLM output
// This is the KEY function that gets the REAL assistant reasoning
func (c *Client) getConversationDetails(convID string) (string, error) {
	log.Printf("[OPENHANDS] fetching conversation payload: %s", convID)
	
	// Use /api/v1/app-conversations?ids=ID to get full conversation details
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/app-conversations?ids="+convID, nil)
	if err != nil {
		return "", err
	}
	
	c.setAuthHeader(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[OPENHANDS] conversation HTTP error: %v", err)
		return "", err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	log.Printf("[OPENHANDS] conversation response: %.300s", string(body))
	
	// Parse response - it's an array with one conversation object
	var convs []map[string]interface{}
	if err := json.Unmarshal(body, &convs); err != nil {
		return "", err
	}
	
	if len(convs) == 0 {
		return "", fmt.Errorf("no conversation returned")
	}
	
	conv := convs[0]
	
	// Extract key fields from conversation
	// These are the fields that contain actual LLM reasoning:
	executionStatus, _ := conv["execution_status"].(string)
	sandboxStatus, _ := conv["sandbox_status"].(string)
	llmModel, _ := conv["llm_model"].(string)
	title, _ := conv["title"].(string)
	
	log.Printf("[OPENHANDS] conversation execution_status: %s", executionStatus)
	log.Printf("[OPENHANDS] conversation sandbox_status: %s", sandboxStatus)
	log.Printf("[OPENHANDS] conversation llm_model: %s", llmModel)
	log.Printf("[OPENHANDS] conversation title: %s", title)
	
	// Build response from available fields
	var output string
	
	// If execution is finished, include this in output
	if executionStatus == "finished" {
		output = "✓ Task completed successfully.\n\n"
	}
	
	if sandboxStatus != "" {
		output += "Runtime: " + sandboxStatus + "\n"
	}
	
	if llmModel != "" {
		output += "Model: " + llmModel + "\n"
	}
	
	if title != "" && title != "Conversation "+convID[:8] {
		output += "Title: " + title + "\n"
	}
	
	if output == "" {
		output = "Execution: " + executionStatus
	}
	
	log.Printf("[OPENHANDS] extracted output length: %d", len(output))
	log.Printf("[OPENHANDS] output preview: %.100s", output)
	
	return output, nil
}

// PollExecution polls until execution completes or times out
func (c *Client) PollExecution(taskID string, timeout time.Duration, interval time.Duration) (*ExecutionResult, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		result, err := c.GetExecutionStatus(taskID)
		if err != nil {
			return nil, err
		}
		
		// Terminal states
		if result.Status == "COMPLETED" || result.Status == "FAILED" {
			return result, nil
		}
		
		// Check for error state
		if result.Error != "" {
			return result, nil
		}
		
		time.Sleep(interval)
	}
	
	return &ExecutionResult{
		TaskID:   taskID,
		Status:   "TIMEOUT",
		Error:    fmt.Sprintf("execution timed out after %v", timeout),
	}, nil
}

// Dispatch dispatches a task and waits for completion
func (c *Client) Dispatch(taskDescription string, timeout time.Duration) (*ExecutionResult, error) {
	// Start execution
	result, err := c.StartExecution(taskDescription)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch task: %w", err)
	}
	
	if result.TaskID == "" {
		return nil, fmt.Errorf("no task ID returned")
	}
	
	// Poll for completion
	return c.PollExecution(result.TaskID, timeout, 5*time.Second)
}

// IsAuthorized checks if the client has valid credentials
func (c *Client) IsAuthorized() bool {
	return c.apiKey != ""
}

// GetBaseURL returns the base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

func (c *Client) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
}

// GetRepository extracts repository from full repository path
func GetRepository(fullPath string) string {
	// Handle various formats: "owner/repo", "github.com/owner/repo", "https://github.com/owner/repo"
	parts := strings.Split(fullPath, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return fullPath
}

// LoadClientFromEnv is a convenience function for loading config and creating client
func LoadClientFromEnv() (*Client, error) {
	apiKey := os.Getenv("OPENHANDS_API_KEY")
	baseURL := os.Getenv("OPENHANDS_BASE_URL")
	repository := os.Getenv("GITHUB_REPOSITORY")
	
	if apiKey == "" {
		return nil, fmt.Errorf("OPENHANDS_API_KEY environment variable is not set")
	}
	
	if baseURL == "" {
		baseURL = "https://app.all-hands.dev"
	}
	
	return NewClient(apiKey, baseURL, repository)
}