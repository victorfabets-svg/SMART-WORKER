package memory

import (
	"context"
)

// PDR represents a Product Requirements Document
type PDR struct {
	ID        string
	Title     string
	Content   string
	CreatedAt string
}

// ADR represents an Architecture Decision Record
type ADR struct {
	ID           string
	Title        string
	Context      string
	Decision     string
	Consequences string
	Tags         []string
	CreatedAt    string
}

// CodingPreference represents coding standards and preferences
type CodingPreference struct {
	ID        string
	Category  string
	Rule      string
	Example   string
	CreatedAt string
}

// MemoryRepository defines the interface for memory storage operations
type MemoryRepository interface {
	// PDR operations
	StorePDR(ctx context.Context, pdr *PDR) error
	GetPDR(ctx context.Context, id string) (*PDR, error)
	ListPDRs(ctx context.Context) ([]*PDR, error)
	SearchPDRs(ctx context.Context, query string) ([]*PDR, error)

	// ADR operations
	StoreADR(ctx context.Context, adr *ADR) error
	GetADR(ctx context.Context, id string) (*ADR, error)
	ListADRs(ctx context.Context) ([]*ADR, error)
	SearchADRs(ctx context.Context, query string) ([]*ADR, error)

	// Coding preferences
	StorePreference(ctx context.Context, pref *CodingPreference) error
	GetPreference(ctx context.Context, id string) (*CodingPreference, error)
	ListPreferences(ctx context.Context, category string) ([]*CodingPreference, error)
}