# llm-router-sdk

Public API contract for llm-router adapters.

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

---

## Overview

The **llm-router-sdk** defines the interface contract for building external provider adapters for [llm-router](https://github.com/TheSlopMachine/llm-router). Adapters are developed as independent Go modules that implement the `Adapter` interface, enabling llm-router to route requests to any LLM backend.

**Why external adapters?**
- **Decoupled development**: Adapter authors maintain their own repositories
- **Independent versioning**: Update adapters without rebuilding llm-router
- **Community contributions**: Anyone can publish adapters for new providers
- **Clean separation**: Core router logic remains provider-agnostic

---

## Installation

```bash
go get github.com/TheSlopMachine/llm-router-sdk@main
```

**Requirements**: Go 1.25 or later

For bleeding-edge installs, `@main` resolves the current tip of the `main` branch. Go will still pin the resolved result as a pseudo-version in your `go.mod`, so rerun `go get ...@main` when you want to move to newer commits.

---

## Quick Start

Create a new Go module and implement the `Adapter` interface:

```go
package myadapter

import (
    "context"
    "io"
    sdk "github.com/TheSlopMachine/llm-router-sdk"
)

func init() {
    sdk.Register(&Adapter{})
}

type Adapter struct{}

func (a *Adapter) TypeKey() string { return "myprovider" }
func (a *Adapter) AuthType() sdk.AuthType { return sdk.AuthTypeAPIKey }
func (a *Adapter) ValidateCredentials(data map[string]string) error { /* ... */ }
func (a *Adapter) Complete(ctx context.Context, cred *sdk.Credential, req *sdk.ChatCompletionRequest) (*sdk.ChatCompletionResponse, error) { /* ... */ }
func (a *Adapter) CompleteStream(ctx context.Context, cred *sdk.Credential, req *sdk.ChatCompletionRequest, w io.Writer) error { /* ... */ }
func (a *Adapter) NeedsRefresh(cred *sdk.Credential) bool { return false }
func (a *Adapter) RefreshCredential(ctx context.Context, cred *sdk.Credential) (*sdk.Credential, error) { return nil, sdk.ErrNoRefreshNeeded }
func (a *Adapter) GetModelInfos(ctx context.Context, cred *sdk.Credential, providerQualifier string) ([]sdk.ModelInfo, error) { /* ... */ }
func (a *Adapter) GetAuthFlow() sdk.AuthFlowHandler { return &MyProviderAuthFlow{} }
func (a *Adapter) GetDefaultProviders() []sdk.ProviderInfo { return []sdk.ProviderInfo{{Name: "My Provider"}} }

var _ sdk.Adapter = (*Adapter)(nil)
```

Then add your adapter to llm-router's `adapters.conf`:

```
github.com/yourorg/llm-router-adapter-myprovider main
```

Start from [llm-router-adapter-template](https://github.com/TheSlopMachine/llm-router-adapter-template) for the canonical scaffold, or see [examples/minimal](examples/minimal) for the smallest possible local example.

---

## Core Concepts

### Adapter Interface

The `Adapter` interface defines how llm-router communicates with provider backends. Each adapter is stateless; per-provider state lives in `Credential` records.

```go
type Adapter interface {
    TypeKey() string
    AuthType() AuthType
    ValidateCredentials(data map[string]string) error
    Complete(ctx context.Context, cred *Credential, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
    CompleteStream(ctx context.Context, cred *Credential, req *ChatCompletionRequest, w io.Writer) error
    NeedsRefresh(cred *Credential) bool
    RefreshCredential(ctx context.Context, cred *Credential) (*Credential, error)
    GetModelInfos(ctx context.Context, cred *Credential, providerQualifier string) ([]ModelInfo, error)
    GetAuthFlow() AuthFlowHandler
    GetDefaultProviders() []ProviderInfo
}
```

### ModelId Format

Models are identified using the format: `provider/model-name[:version]`

**Examples:**
```
openai/gpt-4o
anthropic/claude-3-opus:20240229
openai:azure/gpt-4o
ollama/llama3
```

The prefix (`openai`, `anthropic`, etc.) maps to the adapter's `TypeKey()`.

### Credential Structure

Credentials use a flexible `map[string]string` to support various authentication schemes:

```go
type Credential struct {
    ID   string            `json:"id"`
    Data map[string]string `json:"data"`
}
```

**Examples:**
```go
// API Key
{"api_key": "sk-..."}

// OAuth2
{"access_token": "...", "refresh_token": "..."}

// Basic Auth
{"username": "...", "password": "..."}
```

### Error Handling

Return `*ProviderError` for errors that should trigger intelligent retry and credential rotation:

```go
type ProviderError struct {
    StatusCode int
    Message    string
    Type       ErrorType
    RetryAfter *time.Time
}
```

**Error Types:**
- `ErrorTypeRateLimit` - Temporary rate limit, rotate credential
- `ErrorTypeQuotaExceeded` - Credential quota exhausted, deprioritize (MUST have RetryAfter)
- `ErrorTypeAuth` - Auth failure, credential may be invalid
- `ErrorTypeUpstream` - Upstream error, don't retry
- `ErrorTypeTimeout` - Timeout, may retry

---

## API Reference

### Adapter Methods

#### TypeKey

```go
TypeKey() string
```

Returns a unique identifier for this provider type. Used in model IDs (e.g., `"openai"` for `openai/gpt-4`).

**Requirements:**
- Must be lowercase
- Must be alphanumeric (no spaces or special characters)
- Must be unique across all registered adapters

#### AuthType

```go
AuthType() AuthType
```

Declares what kind of credentials this provider uses.

**Available types:**
- `AuthTypeAPIKey` - Static API key (most common)
- `AuthTypeOAuth2` - OAuth2 access/refresh tokens
- `AuthTypeBasic` - HTTP Basic Auth (username/password)

#### ValidateCredentials

```go
ValidateCredentials(data map[string]string) error
```

Validates credential data before saving. Called when users manually add credentials or when auth flows complete.

**Best practices:**
- Check for required fields
- Validate field formats (e.g., API key prefix)
- Optionally make a test API call to verify the credential works
- Return descriptive errors that help users fix the issue

#### Complete

```go
Complete(
    ctx context.Context,
    cred *Credential,
    req *ChatCompletionRequest,
) (*ChatCompletionResponse, error)
```

Handles non-streaming chat completion requests. Called when a client makes a POST to `/v1/chat/completions` without `"stream": true`.

**Implementation steps:**
1. Extract credentials from `cred.Data` (e.g., API key)
2. Transform `req` (llm-router format) to your provider's API format
3. Make HTTP request to provider's API
4. Parse response and transform to `ChatCompletionResponse`
5. Return the standardized response

**Error handling:**
- Return `*ProviderError` for retryable errors (rate limits, quota)
- Return regular errors for network failures, parsing errors
- Check `ctx.Done()` for client cancellation

#### CompleteStream

```go
CompleteStream(
    ctx context.Context,
    cred *Credential,
    req *ChatCompletionRequest,
    w io.Writer,
) error
```

Handles streaming chat completion requests. Called when a client makes a POST to `/v1/chat/completions` with `"stream": true`.

**Implementation steps:**
1. Extract credentials from `cred.Data`
2. Transform `req` to your provider's API format
3. Make streaming HTTP request to provider's API
4. Read response stream and transform each chunk to `StreamChunk`
5. Write each chunk as Server-Sent Event (SSE) to `w`

**SSE format:**
```
data: {"id":"...","object":"chat.completion.chunk",...}\n\n
data: [DONE]\n\n
```

**Error handling:**
- Check `ctx.Done()` frequently to detect client disconnection
- Return `*ProviderError` for retryable errors
- Partial writes are acceptable (client may have disconnected)

#### NeedsRefresh

```go
NeedsRefresh(cred *Credential) bool
```

Returns true if the credential needs to be refreshed. Called periodically by the maintenance service.

**Return true if:**
- OAuth token is expired or will expire soon
- Session token needs renewal
- Any other condition that requires credential refresh

**For static API keys that don't expire, always return false.**

#### RefreshCredential

```go
RefreshCredential(
    ctx context.Context,
    cred *Credential,
) (*Credential, error)
```

Obtains fresh credentials using the existing credential data. Called by the maintenance service when `NeedsRefresh()` returns true.

**Implementation patterns:**

**For OAuth2:**
1. Extract `refresh_token` from `cred.Data`
2. Make token refresh request to provider's OAuth endpoint
3. Parse response and create new `Credential` with updated tokens
4. Return the new credential (framework will save it)

**For session-based auth:**
1. Extract session credentials from `cred.Data`
2. Make session renewal request
3. Return updated credential with new session token

**For static API keys:**
- Return `sdk.ErrNoRefreshNeeded`

#### GetModelInfos

```go
GetModelInfos(
    ctx context.Context,
    cred *Credential,
    providerQualifier string,
) ([]ModelInfo, error)
```

Retrieves metadata for all available models from the provider. Called by the modelinfo service when cache is expired or missing.

**Parameters:**
- `ctx` - Context for timeout/cancellation
- `cred` - Credential to use for API authentication
- `providerQualifier` - Provider qualifier (e.g., `""`, `"azure"`)

**Returns:**
- Array of `ModelInfo` with rate limits and capabilities
- Error if fetching fails (network, auth, parsing, etc.)

**Implementation notes:**
- Should fetch from provider API when possible
- Can return hardcoded values if API doesn't provide metadata
- Should respect `ctx` timeout/cancellation
- Return empty array (not error) if provider has no models
- Framework will cache results with TTL

#### GetAuthFlow

```go
GetAuthFlow() AuthFlowHandler
```

Returns an optional handler for automated provider-specific authentication. Return `nil` if your provider doesn't support automated authentication (users will need to manually enter credentials via JSON).

Most adapters should return an `AuthFlowHandler` implementation so the dashboard can guide users through API key, OAuth2, or multi-step authentication. Return `nil` only for intentionally manual-only providers.

#### GetDefaultProviders

```go
GetDefaultProviders() []ProviderInfo
```

Returns a list of default providers this adapter supports. These providers will be automatically created during bootstrap.

Return an empty slice if this adapter should not have default providers.

---

### Types

#### ChatCompletionRequest

```go
type ChatCompletionRequest struct {
    Model       ModelId       `json:"model"`
    Messages    []ChatMessage `json:"messages"`
    Stream      bool          `json:"stream,omitempty"`
    MaxTokens   int           `json:"max_tokens,omitempty"`
    Temperature float64       `json:"temperature,omitempty"`
    TopP        float64       `json:"top_p,omitempty"`
}
```

#### ChatCompletionResponse

```go
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
```

#### StreamChunk

```go
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
```

#### ModelInfo

```go
type ModelInfo struct {
    Name          string `json:"name"`
    DisplayName   string `json:"display_name"`
    RPM           int64  `json:"rpm"`           // Requests per minute (0 = unlimited/unknown)
    TPM           int64  `json:"tpm"`           // Tokens per minute (0 = unlimited/unknown)
    RPD           int64  `json:"rpd"`           // Requests per day (0 = unlimited/unknown)
    ContextWindow int64  `json:"context_window,omitempty"`
    MaxTokens     int64  `json:"max_tokens,omitempty"`
}
```

#### ProviderInfo

```go
type ProviderInfo struct {
    Name      string // Display name (e.g., "OpenAI", "Azure OpenAI")
    Qualifier string // Optional qualifier (e.g., "azure"), empty for default
    BaseURL   string // API base URL
    IconURL   string // Icon URL (remote CDN or data URI)
}
```

---

### Authentication Flow (Optional)

The `AuthFlowHandler` interface enables automated credential acquisition through a wizard-like UI embedded in the dashboard.

```go
type AuthFlowHandler interface {
    InitiateFlow(ctx AuthFlowContext) (AuthFlowState, error)
    HandleStep(ctx AuthFlowContext, input map[string][]string) (AuthFlowState, error)
}

type AuthFlowContext struct {
    ProviderID string
    FlowID     string
    Store      AuthStore // For adapter to persist its own state between steps
}

type AuthFlowState struct {
    RenderHTML         string            // Render this HTML in the wizard
    ExternalURL        string            // Open this URL in new tab (OAuth)
    Credentials        map[string]string // Flow complete, save these credentials
    WaitingForCallback bool              // True if waiting for external OAuth callback
}
```

**Exactly ONE of `RenderHTML`, `ExternalURL`, or `Credentials` should be set.**

See [llm-router-adapter-demo](https://github.com/TheSlopMachine/llm-router-adapter-demo) for comprehensive examples including:
- Simple API key input
- OAuth2 flow
- Multi-step authentication (username → password → MFA)

---

## Error Handling for Credential Rotation

The framework implements intelligent retry with credential rotation. Adapters should return `*ProviderError` for errors that should trigger credential rotation:

```go
// Rate limit error (temporary, rotate to next credential)
if resp.StatusCode == 429 {
    retryAfter := time.Now().Add(60 * time.Second)
    return nil, &sdk.ProviderError{
        StatusCode: 429,
        Message:    "rate limit exceeded",
        Type:       sdk.ErrorTypeRateLimit,
        RetryAfter: &retryAfter,
    }
}

// Quota exceeded (credential exhausted, deprioritize)
if strings.Contains(body, "quota exceeded") {
    resetAt := time.Now().Add(24 * time.Hour)
    return nil, &sdk.ProviderError{
        StatusCode: 429,
        Message:    "quota exceeded",
        Type:       sdk.ErrorTypeQuotaExceeded,
        RetryAfter: &resetAt, // REQUIRED for ErrorTypeQuotaExceeded
    }
}

// Auth failure (credential may be invalid)
if resp.StatusCode == 401 {
    return nil, &sdk.ProviderError{
        StatusCode: 401,
        Message:    "authentication failed",
        Type:       sdk.ErrorTypeAuth,
    }
}
```

**The framework will:**
1. Try each available credential once (sorted by LRU)
2. Apply exponential backoff when all credentials exhausted (1s→2s→4s→8s→16s→32s→64s)
3. Mark quota-exceeded credentials with lower priority
4. Automatically recover credentials when `RetryAfter` time passes

---

## Examples

### Starter Template

Start with [llm-router-adapter-template](https://github.com/TheSlopMachine/llm-router-adapter-template) for the canonical standalone starter repository.

### Minimal Local Example

See [examples/minimal](examples/minimal) for a tiny local example that demonstrates the smallest compiling adapter shape.

### Reference Implementation

See [llm-router-adapter-demo](https://github.com/TheSlopMachine/llm-router-adapter-demo) for a comprehensive reference implementation demonstrating:
- Complete adapter interface
- Auth flow handler
- Rate limiting simulation
- Error handling patterns
- Test models for various scenarios
- Comprehensive inline documentation

---

## Resources

- **Main Router**: [github.com/TheSlopMachine/llm-router](https://github.com/TheSlopMachine/llm-router)
- **Adapter Template**: [github.com/TheSlopMachine/llm-router-adapter-template](https://github.com/TheSlopMachine/llm-router-adapter-template)
- **Demo Adapter**: [github.com/TheSlopMachine/llm-router-adapter-demo](https://github.com/TheSlopMachine/llm-router-adapter-demo)
- **Examples**: [examples/](examples/)

---

## License

MIT
