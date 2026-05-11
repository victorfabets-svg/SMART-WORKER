package vectorstore

import "context"

// VectorStore defines the interface for vector storage operations
type VectorStore interface {
	// Add adds vectors to the store
	Add(ctx context.Context, vectors []*Vector) error
	
	// Search performs similarity search
	Search(ctx context.Context, query []float32, limit int) ([]*SearchResult, error)
	
	// Delete removes vectors by ID
	Delete(ctx context.Context, ids []string) error
}

// Vector represents an embedding vector with metadata
type Vector struct {
	ID        string
	Values   []float32
	Metadata map[string]interface{}
}

// SearchResult represents a search result
type SearchResult struct {
	Vector *Vector
	Score  float32
}