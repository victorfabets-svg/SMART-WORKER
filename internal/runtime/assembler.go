package runtime

import "context"

// RuntimeAssembler defines the interface for assembling runtime context
type RuntimeAssembler interface {
	// Assemble creates a complete runtime context
	Assemble(ctx context.Context, req *AssembleRequest) (*Runtime, error)
	
	// Refresh updates runtime context
	Refresh(ctx context.Context, runtime *Runtime) error
}

// AssembleRequest represents a runtime assembly request
type AssembleRequest struct {
	AgentID    string
	SessionID  string
	Task      string
	History   []string
	ProjectID string
}

// Runtime represents assembled runtime context
type Runtime struct {
	ID            string
	AgentID       string
	SessionID     string
	Context      string
	Memory       []string
	Capabilities []string
	Metadata     map[string]interface{}
}