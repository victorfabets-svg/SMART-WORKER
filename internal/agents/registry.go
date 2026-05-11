package agents

import "context"

// Agent represents an AI agent
type Agent struct {
	ID        string
	Name      string
	Type      string
	Capabilities []string
	Metadata  map[string]interface{}
}

// AgentRegistry defines the interface for agent management
type AgentRegistry interface {
	// Register registers a new agent
	Register(ctx context.Context, agent *Agent) error
	
	// Get retrieves an agent by ID
	Get(ctx context.Context, id string) (*Agent, error)
	
	// List lists all registered agents
	List(ctx context.Context) ([]*Agent, error)
	
	// Unregister removes an agent
	Unregister(ctx context.Context, id string) error
}