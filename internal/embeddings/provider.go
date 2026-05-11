package embeddings

import "context"

// EmbeddingProvider defines the interface for generating embeddings
type EmbeddingProvider interface {
	// Generate creates embeddings for the given text
	Generate(ctx context.Context, text string) ([]float32, error)
	
	// GenerateBatch creates embeddings for multiple texts
	GenerateBatch(ctx context.Context, texts []string) ([][]float32, error)
	
	// Dimension returns the embedding dimension
	Dimension() int
}