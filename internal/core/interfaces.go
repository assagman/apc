package core

import (
	"context"

	"github.com/assagman/apc/internal/tools"
)

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
