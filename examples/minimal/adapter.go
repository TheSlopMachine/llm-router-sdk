// Package minimal demonstrates a minimal llm-router adapter implementation.
package minimal

import (
	"context"
	"fmt"
	"io"
	"time"

	sdk "github.com/TheSlopMachine/llm-router-sdk"
)

func init() {
	sdk.Register(&Adapter{})
}

type Adapter struct{}

func (a *Adapter) TypeKey() string { return "minimal" }

func (a *Adapter) AuthType() sdk.AuthType { return sdk.AuthTypeAPIKey }

func (a *Adapter) ValidateCredentials(data map[string]string) error {
	if data["api_key"] == "" {
		return fmt.Errorf("minimal: api_key is required")
	}
	return nil
}

func (a *Adapter) Complete(
	ctx context.Context,
	cred *sdk.Credential,
	req *sdk.ChatCompletionRequest,
) (*sdk.ChatCompletionResponse, error) {
	return &sdk.ChatCompletionResponse{
		ID:      "minimal-" + fmt.Sprintf("%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   string(req.Model),
		Choices: []sdk.ChatCompletionChoice{
			{
				Index: 0,
				Message: sdk.ChatMessage{
					Role:    "assistant",
					Content: "This is a minimal adapter response.",
				},
				FinishReason: "stop",
			},
		},
		Usage: sdk.ChatCompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (a *Adapter) CompleteStream(
	ctx context.Context,
	cred *sdk.Credential,
	req *sdk.ChatCompletionRequest,
	w io.Writer,
) error {
	// Minimal streaming implementation
	return fmt.Errorf("streaming not implemented in minimal adapter")
}

func (a *Adapter) NeedsRefresh(cred *sdk.Credential) bool {
	return false
}

func (a *Adapter) RefreshCredential(ctx context.Context, cred *sdk.Credential) (*sdk.Credential, error) {
	return nil, sdk.ErrNoRefreshNeeded
}

func (a *Adapter) GetModelInfos(ctx context.Context, cred *sdk.Credential, providerQualifier string) ([]sdk.ModelInfo, error) {
	return []sdk.ModelInfo{
		{
			Name:          "minimal-model",
			DisplayName:   "Minimal Test Model",
			RPM:           100,
			TPM:           10000,
			RPD:           10000,
			ContextWindow: 4096,
			MaxTokens:     2048,
		},
	}, nil
}

func (a *Adapter) GetAuthFlow() sdk.AuthFlowHandler {
	return nil
}

func (a *Adapter) GetDefaultProviders() []sdk.ProviderInfo {
	return nil
}

var _ sdk.Adapter = (*Adapter)(nil)
