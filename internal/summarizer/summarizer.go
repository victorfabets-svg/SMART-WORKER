package summarizer

import "context"

// Summarizer defines the interface for text summarization
type Summarizer interface {
	// Summarize creates a summary of the given text
	Summarize(ctx context.Context, text string) (string, error)
	
	// SummarizeWithContext creates a summary with context
	SummarizeWithContext(ctx context.Context, text string, context string) (string, error)
}