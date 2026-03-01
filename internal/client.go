// Package internal contains the core logic for the bridge.ollama plugin.
// It calls the Ollama REST API (POST /api/chat) using plain net/http.
package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaRequest is the request body for POST /api/chat.
type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// OllamaMessage represents a single message in the Ollama chat format.
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaResponse is the response from POST /api/chat (non-streaming).
type OllamaResponse struct {
	Model           string        `json:"model"`
	Message         OllamaMessage `json:"message"`
	Done            bool          `json:"done"`
	TotalDuration   int64         `json:"total_duration"`
	LoadDuration    int64         `json:"load_duration"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

// CallOptions configures a single Ollama API call.
type CallOptions struct {
	SessionID    string
	Prompt       string
	Model        string
	SystemPrompt string
	Host         string // Ollama host URL (default: http://localhost:11434)
	History      []HistoryMessage
}

// HistoryMessage represents a previous message in a conversation.
type HistoryMessage struct {
	Role    string
	Content string
}

// ChatResponse holds the result of a completed Ollama API call.
type ChatResponse struct {
	ResponseText string  `json:"response_text"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	ModelUsed    string  `json:"model_used"`
	DurationMs   int64   `json:"duration_ms"`
	SessionID    string  `json:"session_id"`
}

// CallOllama sends a chat request to the Ollama REST API and returns the
// response. It uses non-streaming mode (stream: false) for simplicity.
func CallOllama(ctx context.Context, opts CallOptions) (*ChatResponse, error) {
	host := opts.Host
	if host == "" {
		host = "http://localhost:11434"
	}

	model := opts.Model
	if model == "" {
		model = "llama3.2"
	}

	messages := []OllamaMessage{}
	if opts.SystemPrompt != "" {
		messages = append(messages, OllamaMessage{Role: "system", Content: opts.SystemPrompt})
	}
	for _, h := range opts.History {
		messages = append(messages, OllamaMessage{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, OllamaMessage{Role: "user", Content: opts.Prompt})

	reqBody := OllamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()

	return &ChatResponse{
		ResponseText: ollamaResp.Message.Content,
		TokensIn:     int64(ollamaResp.PromptEvalCount),
		TokensOut:    int64(ollamaResp.EvalCount),
		CostUSD:      0, // Local inference, no cost
		ModelUsed:    ollamaResp.Model,
		DurationMs:   durationMs,
		SessionID:    opts.SessionID,
	}, nil
}
