// Package sdk defines shared types for llm-router provider adapters.
package sdk

import (
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────
// ModelId
// ─────────────────────────────────────────────

// ModelId uniquely identifies a model in the format: provider/model-name[:version]
// Examples: "openai/gpt-4o", "openai:azure/gpt-4o", "anthropic/claude-3-opus:20240229"
type ModelId string

func (m ModelId) String() string { return string(m) }

// Parse splits a ModelId into providerID and model name.
// The providerID may be composite (e.g., "openai" or "openai:azure").
// Examples:
//   "openai/gpt-4o" -> ("openai", "gpt-4o")
//   "openai:azure/gpt-4o" -> ("openai:azure", "gpt-4o")
func (m ModelId) Parse() (providerID, model string, err error) {
	s := string(m)
	idx := strings.Index(s, "/")
	if idx == -1 {
		return "", "", fmt.Errorf("invalid ModelId %q: missing '/' separator (expected provider/model-name)", s)
	}
	return s[:idx], s[idx+1:], nil
}

// ParseFull splits a ModelId into adapter type, qualifier, and model name.
// Examples:
//   "openai/gpt-4o" -> ("openai", "", "gpt-4o")
//   "openai:azure/gpt-4o" -> ("openai", "azure", "gpt-4o")
func (m ModelId) ParseFull() (adapterType, qualifier, model string, err error) {
	providerID, model, err := m.Parse()
	if err != nil {
		return "", "", "", err
	}

	if idx := strings.Index(providerID, ":"); idx != -1 {
		return providerID[:idx], providerID[idx+1:], model, nil
	}
	return providerID, "", model, nil
}

// ─────────────────────────────────────────────
// Authentication
// ─────────────────────────────────────────────

// AuthType describes the authentication mechanism a provider uses.
type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key" // static API key
	AuthTypeOAuth2 AuthType = "oauth2"  // OAuth 2.0 (with refresh)
	AuthTypeBasic  AuthType = "basic"   // HTTP Basic Auth
)

// Credential holds provider-specific authentication data.
type Credential struct {
	ID   string            `json:"id"`
	Data map[string]string `json:"data"` // e.g. {"api_key": "sk-…"} or {"access_token": "…", "refresh_token": "…"}
}

// ─────────────────────────────────────────────
// OpenAI-compatible wire types
// ─────────────────────────────────────────────

// ChatMessage is a single turn in a conversation.
type ChatMessage struct {
	Role    string `json:"role" example:"user" enums:"system,user,assistant"`
	Content string `json:"content" example:"Hello, how are you?"`
}

// ChatCompletionRequest is the incoming /v1/chat/completions body.
type ChatCompletionRequest struct {
	Model       ModelId       `json:"model" example:"openai/gpt-4o"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty" example:"false"`
	MaxTokens   int           `json:"max_tokens,omitempty" example:"1000"`
	Temperature float64       `json:"temperature,omitempty" example:"0.7"`
	TopP        float64       `json:"top_p,omitempty" example:"1.0"`
}

// ChatCompletionResponse mirrors the OpenAI response schema.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk is a single SSE data payload for streaming responses.
type StreamChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []StreamChunkChoice `json:"choices"`
}

type StreamChunkChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// ─────────────────────────────────────────────
// Model Metadata
// ─────────────────────────────────────────────

// ModelInfo contains metadata about a specific model.
type ModelInfo struct {
	Name          string `json:"name"`                     // Model name without provider prefix
	DisplayName   string `json:"display_name"`             // Human-readable name (optional)
	RPM           int64  `json:"rpm"`                      // Requests per minute (0 = unlimited/unknown)
	TPM           int64  `json:"tpm"`                      // Tokens per minute (0 = unlimited/unknown)
	RPD           int64  `json:"rpd"`                      // Requests per day (0 = unlimited/unknown)
	ContextWindow int64  `json:"context_window,omitempty"` // Max context tokens
	MaxTokens     int64  `json:"max_tokens,omitempty"`     // Max output tokens
}
