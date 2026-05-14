package memory

import "time"

// PDR represents a Problem Definition Record
type PDR struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at,omitempty"`
}

// ADR represents an Architecture Decision Record
type ADR struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status     string    `json:"status"` // proposed, accepted, deprecated, superseded
	Context    string    `json:"context"`
	Decision   string    `json:"decision"`
	Consequences string `json:"consequences"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// CodingPreference represents coding style preferences
type CodingPreference struct {
	ID        string    `json:"id"`
	Rule      string    `json:"rule"`
	Language  string    `json:"language"`
	Preference string  `json:"preference"`
	Rationale string    `json:"rationale"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ConversationMemory stores conversational context
type ConversationMemory struct {
	ID        string                 `json:"id"`
	Messages []ConversationMessage `json:"messages"`
	Meta     map[string]interface{} `json:"meta,omitempty"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at,omitempty"`
}

// ConversationMessage represents a single message
type ConversationMessage struct {
	Role    string    `json:"role"` // user, assistant, system
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// OperationalMemory stores operational state
type OperationalMemory struct {
	RuntimeStatus string `json:"runtime_status"`
	Findings    []Finding `json:"findings"`
	Decisions   []Decision `json:"decisions"`
	Blockers   []Blocker `json:"blockers"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Finding represents an operational finding
type Finding struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"` // critical, high, medium, low
	Evidence string `json:"evidence"`
	Content string `json:"content,omitempty"`
}

// Decision represents an executor decision
type Decision struct {
	ID        string    `json:"id"`
	Finding  string    `json:"finding"`
	Action   string    `json:"action"` // auto, blocked, manual
	Reason   string    `json:"reason"`
	Time     time.Time `json:"time"`
}

// Blocker represents a blocked operation
type Blocker struct {
	ID       string    `json:"id"`
	Finding string    `json:"finding"`
	Reason  string    `json:"reason"`
	Time    time.Time `json:"time"`
}