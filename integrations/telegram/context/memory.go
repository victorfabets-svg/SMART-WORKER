package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OperationalMemory stores real-time operational state
type OperationalMemory struct {
	mu sync.RWMutex

	LatestFindings     []Finding         `json:"latest_findings"`
	ExecutorDecisions []ExecutorDecision `json:"executor_decisions"`
	BlockedOperations []BlockedOp        `json:"blocked_operations"`
	RuntimeStatus    string            `json:"runtime_status"`
	LastAuditRun    time.Time         `json:"last_audit_run"`
	RuntimePID      int               `json:"runtime_pid"`
	IsRunning       bool              `json:"is_running"`
	ExecutorActivity string            `json:"executor_activity"`
	LastExecutorRun time.Time         `json:"last_executor_run"`
	ExecutorSuccess bool              `json:"executor_success"`
}

// Finding an auditor finding
type Finding struct {
	Type            string    `json:"type"`
	Severity        string    `json:"severity"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Evidence       string    `json:"evidence"`
	RecommendedFix string    `json:"recommended_fix"`
	Timestamp     time.Time `json:"timestamp"`
}

// ExecutorDecision an executor execution decision
type ExecutorDecision struct {
	FindingTitle string    `json:"finding_title"`
	Risk         string    `json:"risk"`
	Action       string    `json:"action"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
}

// BlockedOp a blocked operation
type BlockedOp struct {
	Finding    string    `json:"finding"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// NewOperationalMemory creates fresh memory
func NewOperationalMemory() *OperationalMemory {
	return &OperationalMemory{
		RuntimeStatus: "stopped",
	}
}

// LoadFromDisk loads operational state from files
func (m *OperationalMemory) LoadFromDisk() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	findings, _ := loadLatestFindings()
	m.LatestFindings = findings

	history, _ := loadExecutorHistory()
	m.ExecutorDecisions = history

	if pid, err := getRuntimePID(); err == nil {
		m.RuntimePID = pid
		m.RuntimeStatus = "running"
		m.IsRunning = true
	}

	return nil
}

func loadLatestFindings() ([]Finding, error) {
	matches, _ := filepath.Glob("/tmp/auditor_findings_*.json")
	if len(matches) == 0 {
		return nil, nil
	}

	latest := matches[0]
	latestTime := time.Time{}
	for _, f := range matches {
		if info, err := os.Stat(f); err == nil {
			if info.ModTime().After(latestTime) {
				latest = f
				latestTime = info.ModTime()
			}
		}
	}

	data, err := os.ReadFile(latest)
	if err != nil {
		return nil, err
	}

	var findings []Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, err
	}

	return findings, nil
}

func loadExecutorHistory() ([]ExecutorDecision, error) {
	data, err := os.ReadFile("/tmp/runtime_state.json")
	if err != nil {
		return nil, nil
	}

	var state struct {
		ExecutorDecisions []ExecutorDecision `json:"executor_decisions"`
	}
	_ = json.Unmarshal(data, &state)
	return state.ExecutorDecisions, nil
}

func getRuntimePID() (int, error) {
	data, err := os.ReadFile("/tmp/runtime.pid")
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	return pid, err
}

// GetLatestFindings returns current findings
func (m *OperationalMemory) GetLatestFindings() []Finding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LatestFindings
}

// GetExecutorDecisions returns executor history
func (m *OperationalMemory) GetExecutorDecisions() []ExecutorDecision {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExecutorDecisions
}

// GetBlockedOperations returns blocked ops
func (m *OperationalMemory) GetBlockedOperations() []BlockedOp {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.BlockedOperations
}

// GetRuntimeStatus returns runtime state
func (m *OperationalMemory) GetRuntimeStatus() (string, int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.RuntimeStatus, m.RuntimePID, m.IsRunning
}