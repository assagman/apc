package core

import (
	"context"

	"github.com/assagman/apc/internal/tools"
)

// openrouter only
type SubProviderConfig struct {
	AllowFallbacks bool     `json:"allow_fallbacks"`
	Only           []string `json:"only"`
}

type ProviderConfig struct {
	SubProvider  SubProviderConfig // openrouter only
	Model        string
	SystemPrompt string
	APCTools     APCTools
}

type APCTools struct {
	Tools []tools.Tool
}

func (t *APCTools) EnableFsTools(path string) error {
	fsTools, err := tools.GetFsTools(path)
	if err != nil {
		return err
	}
	t.Tools = append(t.Tools, fsTools...)
	return nil
}

func (t *APCTools) RegisterTool(name string, fn any) error {
	tool, err := tools.RegisterTool(name, fn)
	if err != nil {
		return err
	}
	t.Tools = append(t.Tools, tool)
	return nil
}

func (t *APCTools) RegisterMethods(inst any) error {
	methodTools, err := tools.RegisterMethods(inst)
	if err != nil {
		return err
	}
	for _, tool := range methodTools {
		t.Tools = append(t.Tools, tool)
	}
	return nil
}

type GenericMessage any
type GenericRequest any
type GenericResponse any
type GenericTools any

type IProvider interface {
	// Core Provider Methods
	GetApiKey() string
	GetEndpoint() string
	GetHeaders() map[string]string
	// Message Construction Methods
	ConstructUserPromptMessage(prompt string) GenericMessage
	ConstructToolMessage(toolCall tools.ToolCall, toolResult string) GenericMessage
	// Message History Management
	AppendMessageHistory(msg GenericMessage) error
	GetMessageHistory() any
	// Request/Response Processing
	NewRequest() (GenericRequest, error)
	SendRequest(ctx context.Context, genericRequest GenericRequest) (GenericResponse, error)
	IsSenderRole(genericMessage GenericMessage) (bool, error)
	GetMessageFromResponse(genericResponse GenericResponse) (GenericMessage, error)
	GetFinishReasonFromResponse(genericResponse GenericResponse) (string, error)
	GetAnswerFromResponse(genericResponse GenericResponse) (string, error)
	GetToolCallsFromResponse(genericResponse GenericResponse) ([]tools.ToolCall, error)
	FinishReasonStop() string
	FinishReasonToolCall() string
	IsToolCall(genericResponse GenericResponse) (bool, error)
	IsToolCallValid(toolCall tools.ToolCall) (bool, error)
}
