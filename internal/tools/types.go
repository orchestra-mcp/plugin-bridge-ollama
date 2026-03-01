// Package tools contains the tool schemas and handler functions for the
// bridge.ollama plugin. Each exported function pair (Schema + Handler) follows
// the same pattern used across all Orchestra plugins.
package tools

import (
	"context"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
)

// ToolHandler is an alias for readability.
type ToolHandler = func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error)

// BridgePluginInterface defines the methods the tools package needs from the
// BridgePlugin. This avoids a circular import between internal and tools.
type BridgePluginInterface interface {
	TrackSession(s *Session)
	GetSession(sessionID string) *Session
	RemoveSession(sessionID string) *Session
	ListSessions() []*Session
}

// ChatResponse mirrors the internal ChatResponse for use by tool handlers.
type ChatResponse struct {
	ResponseText string  `json:"response_text"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	ModelUsed    string  `json:"model_used"`
	DurationMs   int64   `json:"duration_ms"`
	SessionID    string  `json:"session_id"`
}

// SpawnOptions holds the parameters for spawning an Ollama session or prompt.
type SpawnOptions struct {
	SessionID      string
	Resume         bool
	Prompt         string
	Model          string
	Workspace      string
	AllowedTools   []string
	PermissionMode string
	MaxBudget      float64
	SystemPrompt   string
	Env            map[string]string
}

// Session represents an active Ollama conversation with history.
type Session struct {
	ID        string
	Model     string
	Status    string // "running", "idle", "finished"
	StartedAt string
	History   []HistoryMessage
	LastResp  *ChatResponse
	TotalIn   int64
	TotalOut  int64
	TotalCost float64
}

// HistoryMessage represents a single message in conversation history.
type HistoryMessage struct {
	Role    string
	Content string
}

// Bridge holds the injected dependencies that tool handlers need.
type Bridge struct {
	Call   func(ctx context.Context, opts SpawnOptions) (*ChatResponse, error)
	Plugin BridgePluginInterface
}
