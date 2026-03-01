package tools

import (
	"context"
	"fmt"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/sdk-go/plugin"
	"google.golang.org/protobuf/types/known/structpb"
)

// --- spawn_session ---

// SpawnSessionSchema returns the JSON Schema for the spawn_session tool.
func SpawnSessionSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID for multi-turn conversation",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The prompt to send",
			},
			"resume": map[string]any{
				"type":        "boolean",
				"description": "Resume existing session and append to history (default: false)",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model to use (e.g., llama3.2, mistral, codellama). Default: llama3.2",
			},
			"workspace": map[string]any{
				"type":        "string",
				"description": "Working directory context (informational)",
			},
			"system_prompt": map[string]any{
				"type":        "string",
				"description": "Custom system prompt",
			},
			"env": map[string]any{
				"type":        "string",
				"description": "JSON object of environment variables (e.g., {\"OLLAMA_HOST\": \"http://localhost:11434\"})",
			},
		},
		"required": []any{"session_id", "prompt"},
	})
	return s
}

// SpawnSession returns a tool handler that creates or resumes a persistent
// Ollama conversation session. Sessions maintain message history for multi-turn
// conversations. Each call appends the new prompt and response to history.
func SpawnSession(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id", "prompt"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")
		resume := helpers.GetBool(req.Arguments, "resume")

		opts, err := parseCommonOpts(req.Arguments)
		if err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}
		opts.SessionID = sessionID
		opts.Resume = resume

		// If resuming, load history from existing session.
		existing := bridge.Plugin.GetSession(sessionID)
		if resume && existing != nil {
			// Use existing model if none specified.
			if opts.Model == "" {
				opts.Model = existing.Model
			}
		} else if resume && existing == nil {
			return helpers.ErrorResult("not_found",
				fmt.Sprintf("no existing session %q to resume", sessionID)), nil
		}

		resp, err := bridge.Call(ctx, opts)
		if err != nil {
			return helpers.ErrorResult("ollama_error", err.Error()), nil
		}

		return helpers.TextResult(formatChatResponse(resp)), nil
	}
}

// --- kill_session ---

// KillSessionSchema returns the JSON Schema for the kill_session tool.
func KillSessionSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID to kill",
			},
		},
		"required": []any{"session_id"},
	})
	return s
}

// KillSession returns a tool handler that removes an Ollama session and its
// conversation history.
func KillSession(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")

		session := bridge.Plugin.RemoveSession(sessionID)
		if session == nil {
			return helpers.ErrorResult("not_found",
				fmt.Sprintf("no active session found with ID %q", sessionID)), nil
		}

		return helpers.TextResult(
			fmt.Sprintf("Killed session **%s** (%d messages in history)",
				sessionID, len(session.History))), nil
	}
}

// --- session_status ---

// SessionStatusSchema returns the JSON Schema for the session_status tool.
func SessionStatusSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID to check",
			},
		},
		"required": []any{"session_id"},
	})
	return s
}

// SessionStatus returns a tool handler that reports the current status of an
// Ollama session, including conversation history length and token usage.
func SessionStatus(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")

		session := bridge.Plugin.GetSession(sessionID)
		if session == nil {
			return helpers.ErrorResult("not_found",
				fmt.Sprintf("no session found with ID %q", sessionID)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Session: %s\n\n", sessionID)
		fmt.Fprintf(&b, "- **Status:** %s\n", session.Status)
		fmt.Fprintf(&b, "- **Model:** %s\n", session.Model)
		fmt.Fprintf(&b, "- **Started:** %s\n", session.StartedAt)
		fmt.Fprintf(&b, "- **Messages:** %d\n", len(session.History))
		fmt.Fprintf(&b, "- **Total tokens:** %d in / %d out\n", session.TotalIn, session.TotalOut)
		fmt.Fprintf(&b, "- **Cost:** $0.0000 (local)\n")

		if session.LastResp != nil {
			b.WriteString("\n### Last Response\n\n")
			b.WriteString(formatChatResponse(session.LastResp))
		}

		return helpers.TextResult(b.String()), nil
	}
}

// --- list_active ---

// ListActiveSchema returns the JSON Schema for the list_active tool.
func ListActiveSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	})
	return s
}

// ListActive returns a tool handler that lists all active Ollama sessions
// with their current status and message counts.
func ListActive(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		sessions := bridge.Plugin.ListSessions()

		if len(sessions) == 0 {
			return helpers.TextResult("## Active Sessions\n\nNo active sessions.\n"), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Active Sessions (%d)\n\n", len(sessions))
		fmt.Fprintf(&b, "| Session ID | Model | Status | Messages | Tokens In | Tokens Out |\n")
		fmt.Fprintf(&b, "|------------|-------|--------|----------|-----------|------------|\n")

		for _, s := range sessions {
			fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d |\n",
				s.ID, s.Model, s.Status, len(s.History), s.TotalIn, s.TotalOut)
		}

		return helpers.TextResult(b.String()), nil
	}
}
