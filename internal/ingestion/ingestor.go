package ingestion

import "context"

// Conversation represents a conversation entry
type Conversation struct {
	ID        string
	Source    string
	RawText   string
	Summary   string
	Embedding []float32
	CreatedAt string
}

// ConversationIngestion defines the interface for ingesting conversations
type ConversationIngestion interface {
	// Ingest processes and stores a conversation
	Ingest(ctx context.Context, conv *Conversation) error
	
	// IngestBatch processes multiple conversations
	IngestBatch(ctx context.Context, convs []*Conversation) error
	
	// GetBySource retrieves conversations by source
	GetBySource(ctx context.Context, source string, limit int) ([]*Conversation, error)
}