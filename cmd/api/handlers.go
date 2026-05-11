package main

import (
	"context"
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
	mux.HandleFunc("POST /memory/ingest", s.handleIngestMemory)
	mux.HandleFunc("POST /agent/runtime", s.handleAgentRuntime)
	mux.HandleFunc("POST /webhooks/github", s.handleGitHubWebhook)
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

type IngestRequest struct {
	Type    string   `json:"type"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags   []string `json:"tags"`
}

type IngestResponse struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Ingested bool     `json:"ingested"`
}

func (s *Server) handleIngestMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Normalize: trim whitespace
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)

	// Normalize tags
	if req.Tags == nil {
		req.Tags = []string{}
	}

	// Store memory
	entry := &memory.MemoryEntry{
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
	}

	if err := s.repo.CreateMemory(r.Context(), entry); err != nil {
		log.Printf("ingest error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate embedding if embedder is available
	if s.embedder != nil {
		text := req.Title + "\n" + req.Content
		if _, err := s.embedder.Generate(r.Context(), text); err != nil {
			log.Printf("embedding error: %v", err)
		}
	}

	resp := IngestResponse{
		ID:       entry.ID,
		Type:     entry.Type,
		Title:    entry.Title,
		Ingested: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

const maxContextLength = 8000

type AgentRuntimeRequest struct {
	Agent string `json:"agent"`
	Task  string `json:"task"`
}

type AgentRuntimeResponse struct {
	Agent          string   `json:"agent"`
	Task          string   `json:"task"`
	RuntimeContext string   `json:"runtime_context"`
	Sources       []Source `json:"sources"`
}

func (s *Server) handleAgentRuntime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentRuntimeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Agent == "" {
		http.Error(w, "agent is required", http.StatusBadRequest)
		return
	}
	if req.Task == "" {
		http.Error(w, "task is required", http.StatusBadRequest)
		return
	}

	// Retrieve relevant memory for task (priority: ADR, pref, mistake)
	results, err := s.repo.SearchMemory(r.Context(), req.Task, 10)
	if err != nil {
		log.Printf("runtime search error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Assemble and optimize context
	var ctx strings.Builder
	var sources []Source
	seen := make(map[string]bool)

	for _, res := range results {
		// Deduplicate by content
		if seen[res.Content] {
			continue
		}
		seen[res.Content] = true

		// Priority: ADR > pref > mistake > pdr
		priority := map[string]int{
			"adr":   4,
			"pref":  3,
			"pdr":   1,
			"mistake": 2,
		}
		_ = priority

		sources = append(sources, Source{
			Type:    res.Type,
			Title:  res.Title,
			Content: res.Content,
			Score: res.Score,
		})

		// Build context with token-efficient formatting
		ctx.WriteString(fmt.Sprintf("[%s: %s]\n%s\n\n", res.Type, res.Title, res.Content))

		// Truncate if too long
		if ctx.Len() > maxContextLength {
			break
		}
	}

	// Trim trailing whitespace
	runtimeCtx := strings.TrimSpace(ctx.String())

	// Truncate to max length if needed
	if len(runtimeCtx) > maxContextLength {
		runtimeCtx = runtimeCtx[:maxContextLength] + "\n..."
	}

	resp := AgentRuntimeResponse{
		Agent:          req.Agent,
		Task:          req.Task,
		RuntimeContext: runtimeCtx,
		Sources:       sources,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		eventType = "push"
	}

	// Process based on event type
	switch eventType {
	case "push":
		s.processPushEvent(payload)
	case "pull_request":
		s.processPREvent(payload)
	case "issues":
		s.processIssueEvent(payload)
	default:
		log.Printf("unsupported event: %s", eventType)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "processed"})
}

func (s *Server) processPushEvent(payload map[string]interface{}) {
	commits, ok := payload["commits"].([]interface{})
	if !ok {
		return
	}

	for _, c := range commits {
		commit, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		msg, _ := commit["message"].(string)
		if msg == "" {
			continue
		}

		// Classify memory type
		memType := classifyMessage(msg)

		s.repo.CreateMemory(context.Background(), &memory.MemoryEntry{
			Type:    memType,
			Title:   "Commit: " + truncate(msg, 50),
			Content: msg,
		})
	}
}

func (s *Server) processPREvent(payload map[string]interface{}) {
	pr, ok := payload["pull_request"].(map[string]interface{})
	if !ok {
		return
	}

	title, _ := pr["title"].(string)
	body, _ := pr["body"].(string)
	action, _ := pr["action"].(string)

	if title == "" {
		return
	}

	content := title
	if body != "" {
		content += "\n\n" + body
	}

	s.repo.CreateMemory(context.Background(), &memory.MemoryEntry{
		Type:    "adr",
		Title:  fmt.Sprintf("PR %s: %s", action, truncate(title, 50)),
		Content: content,
	})
}

func (s *Server) processIssueEvent(payload map[string]interface{}) {
	issue, ok := payload["issue"].(map[string]interface{})
	if !ok {
		return
	}

	title, _ := issue["title"].(string)
	body, _ := issue["body"].(string)
	action, _ := issue["action"].(string)

	if title == "" {
		return
	}

	content := title
	if body != "" {
		content += "\n\n" + body
	}

	s.repo.CreateMemory(context.Background(), &memory.MemoryEntry{
		Type:    "mistake",
		Title:  fmt.Sprintf("Issue %s: %s", action, truncate(title, 50)),
		Content: content,
	})
}

func classifyMessage(msg string) string {
	msgLower := strings.ToLower(msg)
	if strings.Contains(msgLower, "fix") || strings.Contains(msgLower, "bug") || strings.Contains(msgLower, "hotfix") {
		return "mistake"
	}
	if strings.Contains(msgLower, "decision") || strings.Contains(msgLower, "decide") {
		return "adr"
	}
	if strings.Contains(msgLower, "decision") || strings.Contains(msgLower, "refactor") {
		return "adr"
	}
	if strings.Contains(msgLower, "sprint") {
		return "sprint"
	}
	return "operational"
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}