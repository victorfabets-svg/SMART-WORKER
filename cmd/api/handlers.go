package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/aoms/smart-worker/internal/memory"
)

type Server struct {
	repo *memory.PostgresRepository
}

func NewServer(repo *memory.PostgresRepository) *Server {
	return &Server{repo: repo}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /memory", s.handleCreateMemory)
	mux.HandleFunc("GET /memory", s.handleListMemory)
	mux.HandleFunc("GET /memory/", s.handleGetMemory)
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