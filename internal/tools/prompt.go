package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/sdk-go/plugin"
	"google.golang.org/protobuf/types/known/structpb"
)

// --- ai_prompt ---

// AIPromptSchema returns the JSON Schema for the ai_prompt tool.
func AIPromptSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The prompt to send to Ollama",
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
		"required": []any{"prompt"},
	})
	return s
}

// AIPrompt returns a tool handler that sends a one-shot prompt to Ollama.
// The request is synchronous (blocks until the response is ready).
func AIPrompt(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "prompt"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		opts, err := parseCommonOpts(req.Arguments)
		if err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}
		// One-shot: no session ID.
		opts.SessionID = ""

		resp, err := bridge.Call(ctx, opts)
		if err != nil {
			return helpers.ErrorResult("ollama_error", err.Error()), nil
		}

		return helpers.TextResult(formatChatResponse(resp)), nil
	}
}

// --- Common helpers ---

// parseCommonOpts extracts the shared spawn options from tool arguments.
func parseCommonOpts(args *structpb.Struct) (SpawnOptions, error) {
	prompt := helpers.GetString(args, "prompt")
	model := helpers.GetString(args, "model")
	workspace := helpers.GetString(args, "workspace")
	systemPrompt := helpers.GetString(args, "system_prompt")
	envRaw := helpers.GetString(args, "env")

	// Parse env from JSON string.
	var envMap map[string]string
	if envRaw != "" {
		if err := json.Unmarshal([]byte(envRaw), &envMap); err != nil {
			return SpawnOptions{}, fmt.Errorf("invalid env JSON: %w", err)
		}
	}

	return SpawnOptions{
		Prompt:       prompt,
		Model:        model,
		Workspace:    workspace,
		SystemPrompt: systemPrompt,
		Env:          envMap,
	}, nil
}

// formatChatResponse formats a ChatResponse as a Markdown string for display.
func formatChatResponse(resp *ChatResponse) string {
	var b strings.Builder
	b.WriteString(resp.ResponseText)
	b.WriteString("\n\n---\n")

	if resp.SessionID != "" {
		fmt.Fprintf(&b, "- **Session:** %s\n", resp.SessionID)
	}
	if resp.ModelUsed != "" {
		fmt.Fprintf(&b, "- **Model:** %s\n", resp.ModelUsed)
	}
	if resp.TokensIn > 0 || resp.TokensOut > 0 {
		fmt.Fprintf(&b, "- **Tokens:** %d in / %d out\n", resp.TokensIn, resp.TokensOut)
	}
	fmt.Fprintf(&b, "- **Cost:** $0.0000 (local)\n")
	if resp.DurationMs > 0 {
		fmt.Fprintf(&b, "- **Duration:** %dms\n", resp.DurationMs)
	}

	return b.String()
}
