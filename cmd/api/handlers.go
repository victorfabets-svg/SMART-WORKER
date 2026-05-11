package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aoms/smart-worker/internal/embeddings"
	"github.com/aoms/smart-worker/internal/memory"
)

type Server struct {
	repo     *memory.PostgresRepository
	embedder *embeddings.Client
}

func NewServer(repo *memory.PostgresRepository, embedder *embeddings.Client) *Server {
	return &Server{repo: repo, embedder: embedder}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /memory", s.handleCreateMemory)
	mux.HandleFunc("GET /memory", s.handleListMemory)
	mux.HandleFunc("GET /memory/", s.handleGetMemory)
	mux.HandleFunc("GET /memory/search", s.handleSearch)
	mux.HandleFunc("POST /context/build", s.handleBuildContext)
}

func (s *Server) handleCreateMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var entry memory.MemoryEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if entry.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	if err := s.repo.CreateMemory(r.Context(), &entry); err != nil {
		log.Printf("create memory error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (s *Server) handleListMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	memType := r.URL.Query().Get("type")
	if memType == "" {
		http.Error(w, "type query param required", http.StatusBadRequest)
		return
	}

	entries, err := s.repo.ListMemory(r.Context(), memType)
	if err != nil {
		log.Printf("list memory error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleGetMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/memory/")
	if path == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	memType := r.URL.Query().Get("type")
	if memType == "" {
		http.Error(w, "type query param required", http.StatusBadRequest)
		return
	}

	var entry interface{}
	var err error

	switch memType {
	case memory.TypePDR:
		entry, err = s.repo.GetPDR(r.Context(), path)
	case memory.TypeADR:
		entry, err = s.repo.GetADR(r.Context(), path)
	case memory.TypePref:
		entry, err = s.repo.GetPreference(r.Context(), path)
	default:
		http.Error(w, "unknown type", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "q query param required", http.StatusBadRequest)
		return
	}

	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	results, err := s.repo.SearchMemory(r.Context(), query, limit)
	if err != nil {
		log.Printf("search error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type ContextRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type Source struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Score  float32 `json:"score"`
}

type ContextResponse struct {
	Query   string   `json:"query"`
	Context string   `json:"context"`
	Sources []Source `json:"sources"`
}

func (s *Server) handleBuildContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "query is required", http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 10
	}

	// Retrieve relevant memory
	results, err := s.repo.SearchMemory(r.Context(), req.Query, req.Limit)
	if err != nil {
		log.Printf("context search error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Assemble context
	var context strings.Builder
	var sources []Source

	// Group by type and build context
	for _, res := range results {
		sources = append(sources, Source{
			Type:    res.Type,
			Title:  res.Title,
			Content: res.Content,
			Score: res.Score,
		})
		context.WriteString(fmt.Sprintf("[%s: %s]\n%s\n\n", res.Type, res.Title, res.Content))
	}

	// Trim trailing whitespace
	resp := ContextResponse{
		Query:   req.Query,
		Context: strings.TrimSpace(context.String()),
		Sources: sources,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}