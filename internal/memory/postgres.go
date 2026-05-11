package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// PDR operations
func (r *PostgresRepository) StorePDR(ctx context.Context, pdr *PDR) error {
	pdr.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	query := `INSERT INTO pdr_documents (title, content, created_at) VALUES ($1, $2, $3) RETURNING id`
	return r.db.QueryRowContext(ctx, query, pdr.Title, pdr.Content, pdr.CreatedAt).Scan(&pdr.ID)
}

func (r *PostgresRepository) GetPDR(ctx context.Context, id string) (*PDR, error) {
	var pdr PDR
	query := `SELECT id, title, content, created_at FROM pdr_documents WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&pdr.ID, &pdr.Title, &pdr.Content, &pdr.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &pdr, nil
}

func (r *PostgresRepository) ListPDRs(ctx context.Context) ([]*PDR, error) {
	query := `SELECT id, title, content, created_at FROM pdr_documents ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pdrs []*PDR
	for rows.Next() {
		var pdr PDR
		if err := rows.Scan(&pdr.ID, &pdr.Title, &pdr.Content, &pdr.CreatedAt); err != nil {
			return nil, err
		}
		pdrs = append(pdrs, &pdr)
	}
	return pdrs, nil
}

func (r *PostgresRepository) SearchPDRs(ctx context.Context, query string) ([]*PDR, error) {
	q := `%` + query + `%`
	sqlQuery := `SELECT id, title, content, created_at FROM pdr_documents WHERE title ILIKE $1 OR content ILIKE $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, sqlQuery, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pdrs []*PDR
	for rows.Next() {
		var pdr PDR
		if err := rows.Scan(&pdr.ID, &pdr.Title, &pdr.Content, &pdr.CreatedAt); err != nil {
			return nil, err
		}
		pdrs = append(pdrs, &pdr)
	}
	return pdrs, nil
}

// ADR operations
func (r *PostgresRepository) StoreADR(ctx context.Context, adr *ADR) error {
	adr.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	query := `INSERT INTO architecture_decisions (title, context, decision, consequences, tags, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	return r.db.QueryRowContext(ctx, query, adr.Title, adr.Context, adr.Decision, adr.Consequences, adr.Tags, adr.CreatedAt).Scan(&adr.ID)
}

func (r *PostgresRepository) GetADR(ctx context.Context, id string) (*ADR, error) {
	var adr ADR
	query := `SELECT id, title, context, decision, consequences, tags, created_at FROM architecture_decisions WHERE id = $1`
	var tags string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&adr.ID, &adr.Title, &adr.Context, &adr.Decision, &adr.Consequences, &tags, &adr.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &adr, nil
}

func (r *PostgresRepository) ListADRs(ctx context.Context) ([]*ADR, error) {
	query := `SELECT id, title, context, decision, consequences, tags, created_at FROM architecture_decisions ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var adrs []*ADR
	for rows.Next() {
		var adr ADR
		if err := rows.Scan(&adr.ID, &adr.Title, &adr.Context, &adr.Decision, &adr.Consequences, &adr.Tags, &adr.CreatedAt); err != nil {
			return nil, err
		}
		adrs = append(adrs, &adr)
	}
	return adrs, nil
}

func (r *PostgresRepository) SearchADRs(ctx context.Context, query string) ([]*ADR, error) {
	q := `%` + query + `%`
	sqlQuery := `SELECT id, title, context, decision, consequences, tags, created_at FROM architecture_decisions WHERE title ILIKE $1 OR context ILIKE $1 OR decision ILIKE $1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, sqlQuery, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var adrs []*ADR
	for rows.Next() {
		var adr ADR
		if err := rows.Scan(&adr.ID, &adr.Title, &adr.Context, &adr.Decision, &adr.Consequences, &adr.Tags, &adr.CreatedAt); err != nil {
			return nil, err
		}
		adrs = append(adrs, &adr)
	}
	return adrs, nil
}

// Coding preferences
func (r *PostgresRepository) StorePreference(ctx context.Context, pref *CodingPreference) error {
	pref.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	query := `INSERT INTO coding_preferences (category, rule, example, created_at) VALUES ($1, $2, $3, $4) RETURNING id`
	return r.db.QueryRowContext(ctx, query, pref.Category, pref.Rule, pref.Example, pref.CreatedAt).Scan(&pref.ID)
}

func (r *PostgresRepository) GetPreference(ctx context.Context, id string) (*CodingPreference, error) {
	var pref CodingPreference
	query := `SELECT id, category, rule, example, created_at FROM coding_preferences WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&pref.ID, &pref.Category, &pref.Rule, &pref.Example, &pref.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

func (r *PostgresRepository) ListPreferences(ctx context.Context, category string) ([]*CodingPreference, error) {
	var query string
	var args []interface{}
	if category != "" {
		query = `SELECT id, category, rule, example, created_at FROM coding_preferences WHERE category = $1 ORDER BY created_at DESC`
		args = append(args, category)
	} else {
		query = `SELECT id, category, rule, example, created_at FROM coding_preferences ORDER BY created_at DESC`
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []*CodingPreference
	for rows.Next() {
		var pref CodingPreference
		if err := rows.Scan(&pref.ID, &pref.Category, &pref.Rule, &pref.Example, &pref.CreatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, &pref)
	}
	return prefs, nil
}

// Memory type constants
const (
	TypePDR      = "pdr"
	TypeADR     = "adr"
	TypePref    = "pref"
	TypeMistake = "mistake"
	TypeOp     = "op"
)

// MemoryEntry represents a generic memory entry
type MemoryEntry struct {
	ID        string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Content string `json:"content"`
	Data    string `json:"data,omitempty"`
	CreatedAt string `json:"created_at"`
}

// CreateMemory stores a memory entry based on type
func (r *PostgresRepository) CreateMemory(ctx context.Context, entry *MemoryEntry) error {
	entry.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	
	switch entry.Type {
	case TypePDR:
		pdr := &PDR{Title: entry.Title, Content: entry.Content}
		return r.StorePDR(ctx, pdr)
	case TypeADR:
		adr := &ADR{Title: entry.Title, Context: entry.Content}
		return r.StoreADR(ctx, adr)
	case TypePref:
		pref := &CodingPreference{Category: entry.Data, Rule: entry.Title, Example: entry.Content}
		return r.StorePreference(ctx, pref)
	default:
		return fmt.Errorf("unknown memory type: %s", entry.Type)
	}
}

// ListMemory lists memory entries by type
func (r *PostgresRepository) ListMemory(ctx context.Context, memType string) ([]*MemoryEntry, error) {
	switch memType {
	case TypePDR:
		pdrs, err := r.ListPDRs(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]*MemoryEntry, len(pdrs))
		for i, p := range pdrs {
			entries[i] = &MemoryEntry{ID: p.ID, Type: TypePDR, Title: p.Title, Content: p.Content, CreatedAt: p.CreatedAt}
		}
		return entries, nil
	case TypeADR:
		adrs, err := r.ListADRs(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]*MemoryEntry, len(adrs))
		for i, a := range adrs {
			entries[i] = &MemoryEntry{ID: a.ID, Type: TypeADR, Title: a.Title, Content: a.Context, CreatedAt: a.CreatedAt}
		}
		return entries, nil
	case TypePref:
		prefs, err := r.ListPreferences(ctx, "")
		if err != nil {
			return nil, err
		}
		entries := make([]*MemoryEntry, len(prefs))
		for i, p := range prefs {
			entries[i] = &MemoryEntry{ID: p.ID, Type: TypePref, Title: p.Rule, Content: p.Example, Data: p.Category, CreatedAt: p.CreatedAt}
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unknown memory type: %s", memType)
	}
}

// SearchResult represents a semantic search result
type SearchResult struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	Title    string  `json:"title"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
}

// SearchMemory performs semantic search across all memory types
func (r *PostgresRepository) SearchMemory(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	// Since we don't have an embedder passed here, do a simple text search for now
	// In production, this would use pgvector cosine similarity
	
	var results []*SearchResult
	
	// Search PDRs
	pdrs, err := r.SearchPDRs(ctx, query)
	if err == nil {
		for _, p := range pdrs {
			results = append(results, &SearchResult{ID: p.ID, Type: TypePDR, Title: p.Title, Content: p.Content, Score: 1.0})
		}
	}
	
	// Search ADRs
	adrs, err := r.SearchADRs(ctx, query)
	if err == nil {
		for _, a := range adrs {
			results = append(results, &SearchResult{ID: a.ID, Type: TypeADR, Title: a.Title, Content: a.Decision, Score: 1.0})
		}
	}
	
	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}