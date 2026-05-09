// Package sdk defines error types for llm-router provider adapters.
package sdk

import (
	"fmt"
	"time"
)

// ErrNoRefreshNeeded is returned by RefreshCredential for credential types
// that do not expire (e.g. static API keys).
var ErrNoRefreshNeeded = fmt.Errorf("no refresh needed for this credential type")

// ProviderError represents errors returned by provider adapters.
// Adapters should return this type to enable intelligent retry and credential rotation.
type ProviderError struct {
	StatusCode int
	Message    string
	Type       ErrorType
	RetryAfter *time.Time // Required for ErrorTypeQuotaExceeded, optional for ErrorTypeRateLimit
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider error (%d): %s", e.StatusCode, e.Message)
}

// IsRetryable returns true if this error should trigger credential rotation.
func (e *ProviderError) IsRetryable() bool {
	return e.Type == ErrorTypeRateLimit || e.Type == ErrorTypeQuotaExceeded
}

// ErrorType classifies provider errors for retry logic.
type ErrorType int

const (
	ErrorTypeUnknown       ErrorType = iota
	ErrorTypeRateLimit     // Temporary rate limit, rotate credential
	ErrorTypeQuotaExceeded // Credential quota exhausted, deprioritize (MUST have RetryAfter)
	ErrorTypeAuth          // Auth failure, credential may be invalid
	ErrorTypeUpstream      // Upstream error, don't retry
	ErrorTypeTimeout       // Timeout, may retry
)
