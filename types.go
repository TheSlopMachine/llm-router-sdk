// Package sdk defines shared types for llm-router provider adapters.
package sdk

import (
	"encoding/json"
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
//
//	"openai/gpt-4o" -> ("openai", "gpt-4o")
//	"openai:azure/gpt-4o" -> ("openai:azure", "gpt-4o")
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
//
//	"openai/gpt-4o" -> ("openai", "", "gpt-4o")
//	"openai:azure/gpt-4o" -> ("openai", "azure", "gpt-4o")
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

type ChatMessageContentPart struct {
	Type        string                   `json:"type,omitempty"`
	Text        string                   `json:"text,omitempty"`
	ToolUseID   string                   `json:"tool_use_id,omitempty"`
	ID          string                   `json:"id,omitempty"`
	Name        string                   `json:"name,omitempty"`
	Input       json.RawMessage          `json:"input,omitempty"`
	Content     []ChatMessageContentPart `json:"-"`
	ContentText string                   `json:"-"`
}

func (p *ChatMessageContentPart) UnmarshalJSON(data []byte) error {
	type rawPart struct {
		Type      string          `json:"type,omitempty"`
		Text      string          `json:"text,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		Content   json.RawMessage `json:"content,omitempty"`
	}

	var raw rawPart
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	p.Type = raw.Type
	p.Text = raw.Text
	p.ToolUseID = raw.ToolUseID
	p.ID = raw.ID
	p.Name = raw.Name
	p.Input = raw.Input

	rawContent := strings.TrimSpace(string(raw.Content))
	switch {
	case rawContent == "", rawContent == "null":
	case strings.HasPrefix(rawContent, "\""):
		if err := json.Unmarshal(raw.Content, &p.ContentText); err != nil {
			return err
		}
	case strings.HasPrefix(rawContent, "["):
		if err := json.Unmarshal(raw.Content, &p.Content); err != nil {
			return err
		}
	}

	return nil
}

func (p ChatMessageContentPart) MarshalJSON() ([]byte, error) {
	type rawPart struct {
		Type      string          `json:"type,omitempty"`
		Text      string          `json:"text,omitempty"`
		ToolUseID string          `json:"tool_use_id,omitempty"`
		ID        string          `json:"id,omitempty"`
		Name      string          `json:"name,omitempty"`
		Input     json.RawMessage `json:"input,omitempty"`
		Content   any             `json:"content,omitempty"`
	}

	out := rawPart{
		Type:      p.Type,
		Text:      p.Text,
		ToolUseID: p.ToolUseID,
		ID:        p.ID,
		Name:      p.Name,
		Input:     p.Input,
	}
	if len(p.Content) > 0 {
		out.Content = p.Content
	} else if p.ContentText != "" {
		out.Content = p.ContentText
	}
	return json.Marshal(out)
}

func (p ChatMessageContentPart) TextContent() string {
	if text := strings.TrimSpace(p.Text); text != "" {
		return text
	}
	if text := strings.TrimSpace(p.ContentText); text != "" {
		return text
	}

	parts := make([]string, 0, len(p.Content))
	for _, child := range p.Content {
		if text := strings.TrimSpace(child.TextContent()); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

type ChatToolFunction struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Arguments   string         `json:"arguments,omitempty"`
	Strict      *bool          `json:"strict,omitempty"`
}

type ChatToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ChatToolFunction `json:"function"`
}

type ChatTool struct {
	Type        string            `json:"type,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Parameters  map[string]any    `json:"parameters,omitempty"`
	InputSchema map[string]any    `json:"input_schema,omitempty"`
	Function    *ChatToolFunction `json:"function,omitempty"`
	Strict      *bool             `json:"strict,omitempty"`
}

// ChatMessage is a single turn in a conversation.
type ChatMessage struct {
	Role         string                   `json:"role" example:"user" enums:"system,user,assistant,tool,developer"`
	Content      string                   `json:"-" example:"Hello, how are you?"`
	ContentParts []ChatMessageContentPart `json:"-"`
	ToolCalls    []ChatToolCall           `json:"tool_calls,omitempty"`
	ToolCallID   string                   `json:"tool_call_id,omitempty"`
	Name         string                   `json:"name,omitempty"`
	Refusal      *string                  `json:"refusal,omitempty"`
}

func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	type rawMessage struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content"`
		ToolCalls  []ChatToolCall  `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
		Name       string          `json:"name,omitempty"`
		Refusal    *string         `json:"refusal,omitempty"`
	}

	var raw rawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.Role = raw.Role
	m.ToolCalls = raw.ToolCalls
	m.ToolCallID = raw.ToolCallID
	m.Name = raw.Name
	m.Refusal = raw.Refusal
	m.Content = ""
	m.ContentParts = nil

	rawContent := strings.TrimSpace(string(raw.Content))
	switch {
	case rawContent == "", rawContent == "null":
	case strings.HasPrefix(rawContent, "\""):
		if err := json.Unmarshal(raw.Content, &m.Content); err != nil {
			return err
		}
	case strings.HasPrefix(rawContent, "["):
		if err := json.Unmarshal(raw.Content, &m.ContentParts); err != nil {
			return err
		}
		m.Content = flattenContentParts(m.ContentParts)
	default:
		return fmt.Errorf("unsupported message content shape")
	}

	return nil
}

func (m ChatMessage) MarshalJSON() ([]byte, error) {
	type rawMessage struct {
		Role       string         `json:"role"`
		Content    any            `json:"content"`
		ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
		ToolCallID string         `json:"tool_call_id,omitempty"`
		Name       string         `json:"name,omitempty"`
		Refusal    *string        `json:"refusal,omitempty"`
	}

	content := any(m.Content)
	if len(m.ContentParts) > 0 {
		content = m.ContentParts
	}

	return json.Marshal(rawMessage{
		Role:       m.Role,
		Content:    content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
		Refusal:    m.Refusal,
	})
}

func (m ChatMessage) TextContent() string {
	parts := make([]string, 0, len(m.ToolCalls)+1)
	if text := strings.TrimSpace(m.Content); text != "" {
		parts = append(parts, text)
	} else if len(m.ContentParts) > 0 {
		if text := strings.TrimSpace(flattenContentParts(m.ContentParts)); text != "" {
			parts = append(parts, text)
		}
	}

	for _, toolCall := range m.ToolCalls {
		if name := strings.TrimSpace(toolCall.Function.Name); name != "" {
			parts = append(parts, name)
		}
		if args := strings.TrimSpace(toolCall.Function.Arguments); args != "" {
			parts = append(parts, args)
		}
	}

	return strings.Join(parts, "\n")
}

func flattenContentParts(parts []ChatMessageContentPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if text := strings.TrimSpace(part.TextContent()); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

// StreamOptions for streaming
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatCompletionRequest is the incoming /v1/chat/completions body.
type ChatCompletionRequest struct {
	Model               ModelId        `json:"model" example:"openai/gpt-4o"`
	Messages            []ChatMessage  `json:"messages"`
	Tools               []ChatTool     `json:"tools,omitempty"`
	ToolChoice          any            `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool          `json:"parallel_tool_calls,omitempty"`
	Stream              bool           `json:"stream,omitempty" example:"false"`
	StreamOptions       *StreamOptions `json:"stream_options,omitempty"`
	MaxTokens           int            `json:"max_tokens,omitempty" example:"1000"`
	MaxCompletionTokens *int           `json:"max_completion_tokens,omitempty"`
	Temperature         float64        `json:"temperature,omitempty" example:"0.7"`
	TopP                float64        `json:"top_p,omitempty" example:"1.0"`
	N                   *int           `json:"n,omitempty"`
	Stop                any            `json:"stop,omitempty"`
	Seed                *int64         `json:"seed,omitempty"`
	FrequencyPenalty    *float64       `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64       `json:"presence_penalty,omitempty"`
	Logprobs            *bool          `json:"logprobs,omitempty"`
	TopLogprobs         *int           `json:"top_logprobs,omitempty"`
	ResponseFormat      any            `json:"response_format,omitempty"`
	User                *string        `json:"user,omitempty"`
	ServiceTier         *string        `json:"service_tier,omitempty"`
	ReasoningEffort     *string        `json:"reasoning_effort,omitempty"`
	Verbosity           *string        `json:"verbosity,omitempty"`
}

// ChatCompletionResponse mirrors the OpenAI response schema.
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             ChatCompletionUsage    `json:"usage"`
	SystemFingerprint *string                `json:"system_fingerprint,omitempty"`
	ServiceTier       *string                `json:"service_tier,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     any         `json:"logprobs,omitempty"`
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
	Usage   *ChatCompletionUsage `json:"usage,omitempty"`
}

type StreamChunkChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
	Logprobs     any         `json:"logprobs,omitempty"`
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
