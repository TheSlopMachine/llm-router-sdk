// Package sdk defines the Adapter interface and registry for llm-router provider adapters.
//
// To add a new provider type:
//  1. Implement the Adapter interface in a new package.
//  2. Call sdk.Register(yourAdapter{}) from an init() function.
//  3. That's it — the router will use it automatically.
package sdk

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// ─────────────────────────────────────────────
// Adapter interface
// ─────────────────────────────────────────────

// Adapter is implemented by every provider type (e.g. OpenAI, Anthropic, Ollama).
// Each Adapter is stateless; per-provider state lives in Credential records.
type Adapter interface {
	// TypeKey returns the unique string identifier for this provider type.
	// This value is stored in Provider.Type and used for adapter lookup.
	// Examples: "openai", "anthropic", "ollama"
	TypeKey() string

	// AuthType declares what kind of credentials this provider uses.
	AuthType() AuthType

	// ValidateCredentials validates and normalises the raw credential data
	// submitted through the dashboard and returns a ready-to-store Credential.
	ValidateCredentials(data map[string]string) error

	// Complete sends a non-streaming chat completion request.
	Complete(ctx context.Context, cred *Credential, req *ChatCompletionRequest) (*ChatCompletionResponse, error)

	// CompleteStream sends a streaming chat completion request, writing
	// Server-Sent Events to w.
	CompleteStream(ctx context.Context, cred *Credential, req *ChatCompletionRequest, w io.Writer) error

	// NeedsRefresh returns true if the credential should be proactively
	// refreshed before it expires (e.g. within the next 5 minutes).
	NeedsRefresh(cred *Credential) bool

	// RefreshCredential obtains new auth data using the existing credential
	// (e.g. using a refresh_token for OAuth 2.0 flows).
	// For static API keys, this should return ErrNoRefreshNeeded.
	RefreshCredential(ctx context.Context, cred *Credential) (*Credential, error)

	// GetModelInfos retrieves metadata for all available models from the provider.
	// Called by the modelinfo service when cache is expired or missing.
	//
	// Parameters:
	//   - ctx: Context for timeout/cancellation
	//   - cred: Credential to use for API authentication
	//   - providerQualifier: Provider qualifier (e.g., "", "azure")
	//
	// Returns:
	//   - Array of ModelInfo with rate limits and capabilities
	//   - Error if fetching fails (network, auth, parsing, etc.)
	//
	// Implementation notes:
	//   - Should fetch from provider API when possible
	//   - Can return hardcoded values if API doesn't provide metadata
	//   - Should respect ctx timeout/cancellation
	//   - Return empty array (not error) if provider has no models
	//   - Framework will cache results with TTL
	GetModelInfos(ctx context.Context, cred *Credential, providerQualifier string) ([]ModelInfo, error)

	// GetAuthFlow returns an optional handler for automated provider-specific auth.
	GetAuthFlow() AuthFlowHandler

	// GetDefaultProviders returns a list of default providers this adapter supports.
	// These providers will be automatically created during bootstrap.
	// Return an empty slice if this adapter should not have default providers.
	GetDefaultProviders() []ProviderInfo
}

// ProviderInfo defines a default provider configuration returned by adapters.
type ProviderInfo struct {
	Name      string // Display name (e.g., "OpenAI", "Azure OpenAI")
	Qualifier string // Optional qualifier (e.g., "azure"), empty for default
	BaseURL   string // API base URL
	IconURL   string // Icon URL (remote CDN or data URI)
}

// AuthStore provides isolated, ephemeral storage for auth flows.
type AuthStore interface {
	Set(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
}

// AuthFlowHandler defines the custom auth orchestration flow.
type AuthFlowHandler interface {
	// InitiateFlow starts the auth flow and returns the initial state.
	InitiateFlow(ctx AuthFlowContext) (AuthFlowState, error)

	// HandleStep processes user input and returns the next state.
	// Called on every form submission, callback, or interaction.
	HandleStep(ctx AuthFlowContext, input map[string][]string) (AuthFlowState, error)
}

// AuthFlowContext provides framework services to the adapter
type AuthFlowContext struct {
	ProviderID string
	FlowID     string
	Store      AuthStore // For adapter to persist its own state between steps
}

// AuthFlowState represents the current state of the auth flow
type AuthFlowState struct {
	// Exactly ONE of these should be set:

	RenderHTML  string            // Render this HTML in the wizard (can include error messages)
	ExternalURL string            // Open this URL in new tab (OAuth)
	Credentials map[string]string // Flow complete, save these credentials

	// Optional metadata:
	WaitingForCallback bool // True if waiting for external OAuth callback
}

// ─────────────────────────────────────────────
// Registry
// ─────────────────────────────────────────────

var (
	mu       sync.RWMutex
	registry = map[string]Adapter{}
)

// Register adds an Adapter to the global registry.
// Panics if an adapter with the same TypeKey is already registered.
// Call this from your adapter package's init() function.
func Register(a Adapter) {
	mu.Lock()
	defer mu.Unlock()
	key := a.TypeKey()
	if _, exists := registry[key]; exists {
		panic(fmt.Sprintf("provider adapter %q already registered", key))
	}
	registry[key] = a
}

// Lookup returns the Adapter for the given provider type key.
func Lookup(typeKey string) (Adapter, error) {
	mu.RLock()
	defer mu.RUnlock()
	a, ok := registry[typeKey]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for provider type %q", typeKey)
	}
	return a, nil
}

// Registered returns all registered provider type keys.
func Registered() []string {
	mu.RLock()
	defer mu.RUnlock()
	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	return keys
}

// GetAllDefaultProviders collects default providers from all registered adapters.
func GetAllDefaultProviders() map[string][]ProviderInfo {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[string][]ProviderInfo)
	for typeKey, adapter := range registry {
		defaults := adapter.GetDefaultProviders()
		if len(defaults) > 0 {
			result[typeKey] = defaults
		}
	}
	return result
}
