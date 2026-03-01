package internal

import (
	"context"
	"sync"
	"time"

	"github.com/orchestra-mcp/plugin-bridge-ollama/internal/tools"
	"github.com/orchestra-mcp/sdk-go/plugin"
)

// BridgePlugin manages Ollama sessions and registers all bridge tools.
type BridgePlugin struct {
	sessions map[string]*tools.Session
	mu       sync.RWMutex
}

// NewBridgePlugin creates a new BridgePlugin with an empty session map.
func NewBridgePlugin() *BridgePlugin {
	return &BridgePlugin{
		sessions: make(map[string]*tools.Session),
	}
}

// RegisterTools registers all 5 bridge tools with the plugin builder.
func (bp *BridgePlugin) RegisterTools(builder *plugin.PluginBuilder) {
	bridge := &tools.Bridge{
		Call:   bp.callAdapter,
		Plugin: bp,
	}

	// --- Prompt tool (1) ---
	builder.RegisterTool("ai_prompt",
		"Send a one-shot prompt to a local Ollama model and return the response",
		tools.AIPromptSchema(), tools.AIPrompt(bridge))

	// --- Session tools (4) ---
	builder.RegisterTool("spawn_session",
		"Start or resume a persistent Ollama conversation session with a prompt",
		tools.SpawnSessionSchema(), tools.SpawnSession(bridge))

	builder.RegisterTool("kill_session",
		"Kill an active Ollama conversation session and discard its history",
		tools.KillSessionSchema(), tools.KillSession(bridge))

	builder.RegisterTool("session_status",
		"Check the status of an Ollama conversation session",
		tools.SessionStatusSchema(), tools.SessionStatus(bridge))

	builder.RegisterTool("list_active",
		"List all active Ollama conversation sessions",
		tools.ListActiveSchema(), tools.ListActive(bridge))
}

// callAdapter converts from tools.SpawnOptions to internal CallOptions,
// manages session history, and calls the Ollama API.
func (bp *BridgePlugin) callAdapter(ctx context.Context, opts tools.SpawnOptions) (*tools.ChatResponse, error) {
	// Determine host from env.
	host := ""
	if opts.Env != nil {
		host = opts.Env["OLLAMA_HOST"]
	}

	// Build history from existing session if resuming.
	var history []HistoryMessage
	if opts.SessionID != "" {
		bp.mu.RLock()
		existing, ok := bp.sessions[opts.SessionID]
		bp.mu.RUnlock()
		if ok && opts.Resume {
			for _, h := range existing.History {
				history = append(history, HistoryMessage{
					Role:    h.Role,
					Content: h.Content,
				})
			}
		}
	}

	callOpts := CallOptions{
		SessionID:    opts.SessionID,
		Prompt:       opts.Prompt,
		Model:        opts.Model,
		SystemPrompt: opts.SystemPrompt,
		Host:         host,
		History:      history,
	}

	resp, err := CallOllama(ctx, callOpts)
	if err != nil {
		return nil, err
	}

	toolsResp := &tools.ChatResponse{
		ResponseText: resp.ResponseText,
		TokensIn:     resp.TokensIn,
		TokensOut:    resp.TokensOut,
		CostUSD:      resp.CostUSD,
		ModelUsed:    resp.ModelUsed,
		DurationMs:   resp.DurationMs,
		SessionID:    resp.SessionID,
	}

	// Update session state if this is a session call.
	if opts.SessionID != "" {
		bp.mu.Lock()
		session, ok := bp.sessions[opts.SessionID]
		if !ok {
			session = &tools.Session{
				ID:        opts.SessionID,
				Model:     resp.ModelUsed,
				Status:    "idle",
				StartedAt: time.Now().Format(time.RFC3339),
				History:   []tools.HistoryMessage{},
			}
			bp.sessions[opts.SessionID] = session
		}
		// Append user message and assistant response to history.
		session.History = append(session.History,
			tools.HistoryMessage{Role: "user", Content: opts.Prompt},
			tools.HistoryMessage{Role: "assistant", Content: resp.ResponseText},
		)
		session.LastResp = toolsResp
		session.TotalIn += resp.TokensIn
		session.TotalOut += resp.TokensOut
		session.Model = resp.ModelUsed
		session.Status = "idle"
		bp.mu.Unlock()
	}

	return toolsResp, nil
}

// --- BridgePluginInterface implementation ---

// TrackSession adds a session to the active map.
func (bp *BridgePlugin) TrackSession(s *tools.Session) {
	if s == nil {
		return
	}
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.sessions[s.ID] = s
}

// GetSession returns the session for the given ID, or nil if not found.
func (bp *BridgePlugin) GetSession(sessionID string) *tools.Session {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.sessions[sessionID]
}

// RemoveSession removes and returns the session for the given ID.
func (bp *BridgePlugin) RemoveSession(sessionID string) *tools.Session {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	session, ok := bp.sessions[sessionID]
	if !ok {
		return nil
	}
	delete(bp.sessions, sessionID)
	return session
}

// ListSessions returns a snapshot of all active sessions.
func (bp *BridgePlugin) ListSessions() []*tools.Session {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	sessions := make([]*tools.Session, 0, len(bp.sessions))
	for _, s := range bp.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}
