package retrieval

import "context"

// RetrievalEngine defines the interface for semantic retrieval
type RetrievalEngine interface {
	// Search performs semantic search
	Search(ctx context.Context, query string, limit int) ([]*SearchResult, error)
	
	// SearchWithFilters performs filtered search
	SearchWithFilters(ctx context.Context, query string, filters map[string]interface{}, limit int) ([]*SearchResult, error)
}

// SearchResult represents a search result
type SearchResult struct {
	ID       string
	Score   float32
	Content string
	Source string
	Metadata map[string]interface{}
}