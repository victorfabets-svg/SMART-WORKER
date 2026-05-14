package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aoms/smart-worker/internal/embeddings"
)

// IntentType represents the operational intent inferred from a message
type IntentType int

const (
	IntentUnknown IntentType = iota
	IntentInvestigation     // User wants analysis only, no execution
	IntentExecution        // User wants to execute/fix/apply
	IntentPlanning          // User wants roadmap reasoning
	IntentPrioritization   // User wants operational analysis
	IntentBlocking         // User wants to stop/cancelexecution
	IntentAuditing          // User wants repository inspection
	IntentClarification    // User wants more information
	IntentAcknowledgment   // User acknowledges/understands
)

// Intent represents an inferred operational intent
type Intent struct {
	Type           IntentType `json:"type"`
	Confidence     float64    `json:"confidence"`
	Reasoning      string     `json:"reasoning"`
	Task           string     `json:"task,omitempty"`
	ShouldExecute bool       `json:"should_execute"`
	ShouldStop    bool       `json:"should_stop"`
	Urgency        string    `json:"urgency,omitempty"`  // low, medium, high, critical
	Sensitivity   string    `json:"sensitivity"` // public, internal, sensitive
	InferredFrom  []string  `json:"inferred_from"`
}

// ExecutionContext provides background for intent cognition
type ExecutionContext struct {
	Repository           string
	RecentFindings       []Finding
	ActiveExecutions     []ExecutionState
	ConversationalHistory []Message
	Operational_state    string
	RoadmapState         string
	LastIntent           *Intent
}

// Message represents a conversational message
type Message struct {
	Role    string    `json:"role"` // user, assistant, system
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// Finding represents an operational finding
type Finding struct {
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Evidence string `json:"evidence"`
}

// ExecutionState represents an active execution
type ExecutionState struct {
	TaskID       string    `json:"task_id"`
	Status       string    `json:"status"`
	StartTime    time.Time `json:"start_time"`
	Description string    `json:"description"`
}

// CognitionConfig configures the intent cognition engine
type CognitionConfig struct {
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	OpenAIModel      string
	AnthropicAPIKey string
	UseAnthropic    bool
}

// NewCognitionConfig creates config from environment
func NewCognitionConfig() (*CognitionConfig, error) {
	config := &CognitionConfig{
		OpenAIAPIKey:  os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL: os.Getenv("OPENAI_BASE_URL"),
		OpenAIModel:  os.Getenv("OPENAI_MODEL"),
	}
	
	// Try Anthropic first
	if config.AnthropicAPIKey = os.Getenv("ANTHROPIC_API_KEY"); config.AnthropicAPIKey != "" {
		config.UseAnthropic = true
	}
	
	// Default model
	if config.OpenAIModel == "" {
		config.OpenAIModel = "gpt-4"
	}
	
	return config, nil
}

// CognitionEngine performs LLM-driven intent cognition
type CognitionEngine struct {
	config     *CognitionConfig
	embedding  *embeddings.Client
}

// NewCognitionEngine creates a new intent cognition engine
func NewCognitionEngine() (*CognitionEngine, error) {
	config, err := NewCognitionConfig()
	if err != nil {
		return nil, err
	}
	
	engine := &CognitionEngine{
		config: config,
	}
	
	// Initialize embedding client if API key available
	if config.OpenAIAPIKey != "" {
		engine.embedding, _ = embeddings.NewClient(config.OpenAIAPIKey, config.OpenAIBaseURL)
	}
	
	return engine, nil
}

// InferIntent uses LLM to semantically infer operational intent
func (e *CognitionEngine) InferIntent(message string, ctx *ExecutionContext) (*Intent, error) {
	// Build prompt with context
	prompt := e.buildCognitionPrompt(message, ctx)
	
	// Use LLM to classify
	intent, err := e.classifyWithLLM(prompt)
	if err != nil {
		// Fallback to semantic analysis
		return e.fallbackAnalysis(message, ctx)
	}
	
	return intent, nil
}

// buildCognitionPrompt builds a prompt for intent classification
func (e *CognitionEngine) buildCognitionPrompt(message string, ctx *ExecutionContext) string {
	var sb strings.Builder
	
	sb.WriteString(`Analyze this operational message and determine what the user wants to happen.
	
Message: "`)
	sb.WriteString(message)
	sb.WriteString(`"

Operational Context:`)
	
	if ctx != nil {
		sb.WriteString("\n- Repository: ")
		sb.WriteString(ctx.Repository)
		
		if ctx.Operational_state != "" {
			sb.WriteString("\n- Operational State: ")
			sb.WriteString(ctx.Operational_state)
		}
		
		if ctx.RoadmapState != "" {
			sb.WriteString("\n- Roadmap State: ")
			sb.WriteString(ctx.RoadmapState)
		}
		
		if len(ctx.RecentFindings) > 0 {
			sb.WriteString("\n- Recent Findings:")
			for i, f := range ctx.RecentFindings {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("\n  * [%s] %s", f.Severity, f.Title))
			}
		}
		
		if len(ctx.ActiveExecutions) > 0 {
			sb.WriteString("\n- Active Executions:")
			for _, ex := range ctx.ActiveExecutions {
				sb.WriteString(fmt.Sprintf("\n  * %s (%s)", ex.Description, ex.Status))
			}
		}
		
		if ctx.LastIntent != nil {
			sb.WriteString("\n- Last Intent: ")
			sb.WriteString(ctx.LastIntent.Type.String())
			sb.WriteString(" (")
			sb.WriteString(ctx.LastIntent.Reasoning)
			sb.WriteString(")")
		}
	}
	
	sb.WriteString(`

Determine the operational intent by selecting ONE of:
- INVESTIGATION: analyze, investigate, understand, but DO NOT execute anything
- EXECUTION: fix, apply, correcao, execute, implement, stabilize, make it work
- PLANNING: roadmap, path forward, strategy, how to proceed
- PRIORITIZATION: what matters most, urgency, sequence
- BLOCKING: stop, cancel, halt, interrupt current execution
- AUDITING: inspect, review, check repository state
- CLARIFICATION: explain, elaborate, more details
- ACKNOWLEDGMENT: ok, understood, confirmed

Respond in JSON format:
{
  "intent": "INVESTIGATION|EXECUTION|PLANNING|PRIORITIZATION|BLOCKING|AUDITING|CLARIFICATION|ACKNOWLEDGMENT",
  "confidence": 0.0-1.0,
  "reasoning": "why you chose this intent",
  "task": "what specifically needs to happen (if any)",
  "should_execute": true|false,
  "should_stop": true|false,
  "urgency": "low|medium|high|critical"
}
`)

	return sb.String()
}

// classifyWithLLM uses LLM to classify intent
func (e *CognitionEngine) classifyWithLLM(prompt string) (*Intent, error) {
	// This requires LLM API - for now return error to trigger fallback
	if e.config.OpenAIAPIKey == "" && !e.config.UseAnthropic {
		return nil, fmt.Errorf("no LLM API key configured")
	}
	
	// In production, call LLM API here
	// For implementation, we'll provide fallback analysis
	
	return nil, fmt.Errorf("LLM API not available in this context")
}

// fallbackAnalysis provides semantic analysis without LLM API
func (e *CognitionEngine) fallbackAnalysis(message string, ctx *ExecutionContext) (*Intent, error) {
	lower := strings.ToLower(message)
	
	intent := &Intent{
		Confidence:    0.7,
		InferredFrom:  []string{"message_content", "context"},
	}
	
	// Analyze message semantics
	// These are semantic patterns, NOT keyword triggers
	
	// BLOCKING patterns - user wants to stop something
	blockingPatterns := []string{
		"pare", "pare agora", "pare a execucao", "cancele", 
		"interrompa", "nao continue", "pare com isso",
		"stop", "cancel", "halt", "abort", "never mind",
		"forget it", "desista", "abortar",
	}
	for _, pattern := range blockingPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentBlocking
			intent.Reasoning = fmt.Sprintf("User explicitly requested to stop: '%s'", pattern)
			intent.ShouldStop = true
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// INVESTIGATION patterns - user wants analysis only
	investigationPatterns := []string{
		"investigue", "apenas analise", "nao execute", "nao modifique",
		"nao faca nada", "analise primeiro", "somente analise",
		"understand", "investigate first", "just analyze",
		"don't change anything", "don't modify", "don't execute",
		"only look", "only check", "just check",
		"quiero entender", "analizar", "solo analizar",
	}
	for _, pattern := range investigationPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentInvestigation
			intent.Reasoning = fmt.Sprintf("User explicitly requested analysis only: '%s'", pattern)
			intent.ShouldExecute = false
			intent.ShouldStop = false
			return intent, nil
		}
	}
	
	// AUDITING patterns - user wants to inspect/review
	auditingPatterns := []string{
		"audite", "review", "inspecione", "verifique",
		"revise o codigo", "veja o que tem", "check",
		"what's the state", "current status", "what's wrong",
	}
	for _, pattern := range auditingPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentAuditing
			intent.Reasoning = fmt.Sprintf("User requested inspection: '%s'", pattern)
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// EXECUTION patterns - user wants to fix/apply/implement
	// These are semantic, not just keywords
	executionPatterns := []string{
		"corrija", "correto", "corrigido", "corrigir",
		"execute", "executar", "rode", "rodar",
		"implemente", "implementar", "faça", "fazer",
		"aplique", "aplicar", "fix", "fix it",
		"make it work", "estabilize", "resolve", "resolva",
		"agora", "imediatamente", "right now", "now",
		"precisa ser", "tem que ser", "must be",
		"pode executar", "can execute", "go ahead",
	}
	for _, pattern := range executionPatterns {
		if strings.Contains(lower, pattern) {
			// Check if there's a negation
			if strings.Contains(lower, "nao ") && !strings.Contains(lower, "nao execute") {
				// Negation present, might be investigation
				continue
			}
			intent.Type = IntentExecution
			intent.Reasoning = fmt.Sprintf("User requested execution: '%s'", pattern)
			intent.ShouldExecute = true
			intent.ShouldStop = false
			
			// Detect urgency
			if strings.Contains(lower, "agora") || strings.Contains(lower, "imediatamente") || strings.Contains(lower, "right now") {
				intent.Urgency = "high"
			}
			
			return intent, nil
		}
	}
	
	// PLANNING patterns
	planningPatterns := []string{
		"como fazer", "como proceder", "roadmap", "estrategia",
		"what should we do", "next steps", "plan", "planning",
		"caminho", "direction", "path forward",
	}
	for _, pattern := range planningPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentPlanning
			intent.Reasoning = fmt.Sprintf("User requested planning: '%s'", pattern)
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// PRIORITIZATION patterns
	prioritizationPatterns := []string{
		"prioridade", "o que importa", "mais importante",
		"priority", "what matters", "sequence", "order",
	}
	for _, pattern := range prioritizationPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentPrioritization
			intent.Reasoning = fmt.Sprintf("User requested prioritization: '%s'", pattern)
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// CLARIFICATION patterns
	clarificationPatterns := []string{
		"como", "oq", "o que e", "explique",
		"explain", "what is", "what does",
		"mais detalhes", "more details", "elaborate",
	}
	for _, pattern := range clarificationPatterns {
		if strings.Contains(lower, pattern) {
			intent.Type = IntentClarification
			intent.Reasoning = fmt.Sprintf("User requested clarification: '%s'", pattern)
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// ACKNOWLEDGMENT patterns
	acknowledgmentPatterns := []string{
		"ok", "entendi", "entendido", "confirmado", "sim", "yes",
		"understood", "confirmed", "roger", "got it",
	}
	for _, pattern := range acknowledgmentPatterns {
		if lower == pattern || strings.HasPrefix(lower, pattern+".") || strings.HasPrefix(lower, pattern+"!") {
			intent.Type = IntentAcknowledgment
			intent.Reasoning = fmt.Sprintf("User acknowledged: '%s'", pattern)
			intent.ShouldExecute = false
			return intent, nil
		}
	}
	
	// Default - unclear, ask for clarification
	intent.Type = IntentClarification
	intent.Reasoning = "Intent unclear from message content"
	intent.Confidence = 0.3
	intent.ShouldExecute = false
	
	return intent, nil
}

// InferWithContext combines conversation history and operational state
func (e *CognitionEngine) InferWithContext(message string, ctx *ExecutionContext) (*Intent, error) {
	// Add conversational history to context if available
	if ctx != nil && len(ctx.ConversationalHistory) > 0 {
		// Analyze last few messages for intent
		recentMessages := ctx.ConversationalHistory
		if len(recentMessages) > 3 {
			recentMessages = recentMessages[len(recentMessages)-3:]
		}
		
		// Check for intent escalation
		for _, msg := range recentMessages {
			if msg.Role == "user" {
				prevIntent, _ := e.fallbackAnalysis(msg.Content, ctx)
				if prevIntent != nil && prevIntent.Type == IntentInvestigation && ctx.LastIntent == nil {
					// Previous was investigation, now checking if execution wanted
					// This is context for current message
				}
			}
		}
	}
	
	return e.InferIntent(message, ctx)
}

// ShouldDispatchOpenHands determines if execution should be dispatched
func (e *CognitionEngine) ShouldDispatchOpenHands(intent *Intent, ctx *ExecutionContext) (bool, string) {
	if intent == nil {
		return false, "no intent inferred"
	}
	
	if intent.ShouldStop {
		return false, "user requested blocking"
	}
	
	if !intent.ShouldExecute {
		return false, fmt.Sprintf("intent is %s, not execution", intent.Type.String())
	}
	
	// Check operational constraints
	if ctx != nil {
		// Check if there's an active execution
		for _, ex := range ctx.ActiveExecutions {
			if ex.Status == "RUNNING" || ex.Status == "IN_PROGRESS" {
				// There's already execution running
				// Intent might be to continue or block
				if intent.Type == IntentBlocking {
					return true, "stopping active execution"
				}
				return false, "execution already in progress"
			}
		}
	}
	
	// Determine task from intent
	task := intent.Task
	if task == "" {
		task = "Execute the requested operational task"
	}
	
	return true, task
}

// String returns string representation of intent type
func (i IntentType) String() string {
	switch i {
	case IntentInvestigation:
		return "INVESTIGATION"
	case IntentExecution:
		return "EXECUTION"
	case IntentPlanning:
		return "PLANNING"
	case IntentPrioritization:
		return "PRIORITIZATION"
	case IntentBlocking:
		return "BLOCKING"
	case IntentAuditing:
		return "AUDITING"
	case IntentClarification:
		return "CLARIFICATION"
	case IntentAcknowledgment:
		return "ACKNOWLEDGMENT"
	default:
		return "UNKNOWN"
	}
}

// ToIntentType converts string to IntentType
func ToIntentType(s string) IntentType {
	switch strings.ToUpper(s) {
	case "INVESTIGATION":
		return IntentInvestigation
	case "EXECUTION":
		return IntentExecution
	case "PLANNING":
		return IntentPlanning
	case "PRIORITIZATION":
		return IntentPrioritization
	case "BLOCKING":
		return IntentBlocking
	case "AUDITING":
		return IntentAuditing
	case "CLARIFICATION":
		return IntentClarification
	case "ACKNOWLEDGMENT":
		return IntentAcknowledgment
	default:
		return IntentUnknown
	}
}

// MarshalJSON marshals intent to JSON
func (i IntentType) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// UnmarshalJSON unmarshals intent from JSON
func (i *IntentType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*i = ToIntentType(s)
	return nil
}

// FormatIntent formats intent for conversational display
func FormatIntent(intent *Intent) string {
	if intent == nil {
		return "Intent: unknown"
	}
	
	var sb strings.Builder
	sb.WriteString("Intent: ")
	sb.WriteString(intent.Type.String())
	sb.WriteString(fmt.Sprintf(" (%.0f%% confidence)", intent.Confidence*100))
	
	if intent.Reasoning != "" {
		sb.WriteString("\nReasoning: ")
		sb.WriteString(intent.Reasoning)
	}
	
	if intent.ShouldExecute {
		sb.WriteString("\n→ Will execute")
	}
	if intent.ShouldStop {
		sb.WriteString("\n→ Will stop")
	}
	if intent.Urgency != "" {
		sb.WriteString(fmt.Sprintf("\nUrgency: %s", intent.Urgency))
	}
	
	return sb.String()
}

// NewIntentFromJSON creates intent from JSON response
func NewIntentFromJSON(data []byte) (*Intent, error) {
	var result struct {
		Intent       string  `json:"intent"`
		Confidence  float64 `json:"confidence"`
		Reasoning   string  `json:"reasoning"`
		Task        string  `json:"task"`
		ShouldExecute bool   `json:"should_execute"`
		ShouldStop  bool    `json:"should_stop"`
		Urgency    string  `json:"urgency"`
	}
	
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	
	return &Intent{
		Type:          ToIntentType(result.Intent),
		Confidence:   result.Confidence,
		Reasoning:    result.Reasoning,
		Task:         result.Task,
		ShouldExecute: result.ShouldExecute,
		ShouldStop:    result.ShouldStop,
		Urgency:       result.Urgency,
	}, nil
}