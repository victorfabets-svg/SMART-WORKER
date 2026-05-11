package contextbuilder

import "context"

// ContextBuilder defines the interface for building runtime context
type ContextBuilder interface {
	// Build creates operational context from memory
	Build(ctx context.Context, agentID string) (*Context, error)
	
	// BuildWithQuery builds context for a specific query
	BuildWithQuery(ctx context.Context, agentID string, query string) (*Context, error)
}

// Context represents assembled operational context
type Context struct {
	AgentID      string
	SessionID    string
	Memory      *MemoryContext
	Project     *ProjectContext
	RecentTasks []*Task
	Metadata    map[string]interface{}
}

// MemoryContext represents memory in context
type MemoryContext struct {
	RelevantPDRs     []string
	RelevantADRs    []string
	Preferences    []string
	LastConversations []string
}

// ProjectContext represents project information
type ProjectContext struct {
	Name        string
	Language    string
	Framework  string
	Patterns   []string
	Standards  []string
}

// Task represents a task or goal
type Task struct {
	ID          string
	Description string
	Status      string
}