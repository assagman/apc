package openrouter

import (
	// "context"
	// "fmt"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/assagman/apc/core"
	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/tools"
)

const chatCompletionRequestUrl = "https://openrouter.ai/api/v1/chat/completions"
const (
	roleSys   = "system"
	roleUser  = "user"
	roleDev   = "developer"
	roleModel = "assistant"
	roleTool  = "tool"
)
const (
	stopReasonStop      = "stop"
	stopReasonMaxTokens = "max_tokens"
	stopReasonToolUse   = "tool_calls"
)

type Provider struct {
	Name     string
	Endpoint string
	Config   core.ProviderConfig
	History  []Message
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Message struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // req: string or array, resp: string or null
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string           `json:"tool_call_id,omitempty"`
}

type Request struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Tools    []tools.Tool           `json:"tools"`
	Provider core.SubProviderConfig `json:"provider"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Response struct {
	Choices []Choice `json:"choices"`
}

func CheckModelName(model string) error {
	models := []string{"moonshotai/kimi-k2", "google/gemini-2.5-flash"}
	if !slices.Contains(models, model) {
		return fmt.Errorf("UnsupportedModelName: `%s`", model)
	}
	return nil
}

func New(config core.ProviderConfig) (core.IProvider, error) {
	CheckModelName(config.Model)
	p := &Provider{
		Name:     "openrouter",
		Endpoint: chatCompletionRequestUrl,
		History:  make([]Message, 0),
		Config:   config,
	}
	p.History = append(p.History, p.ConstructSystemPromptMessage())
	return p, nil
}

func (p *Provider) AppendMessageHistory(msg core.GenericMessage) error {
	message, ok := msg.(Message)
	if !ok {
		return fmt.Errorf("[AppendMessageHistory] Failed to cast core.GenericMessage -> %s.Message", p.Name)
	}

	p.History = append(p.History, message)
	return nil
}

func (p *Provider) FinishReasonStop() string { return stopReasonStop }

func (p *Provider) FinishReasonToolCall() string { return stopReasonToolUse }

func (p *Provider) GetAnswerFromResponse(resp core.GenericResponse) (string, error) {
	response, ok := resp.(Response)
	if !ok {
		return "", fmt.Errorf("[GetAnswerFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	if len(response.Choices) > 0 {
		answer, err := response.Choices[0].Message.GetContentAsString()
		if err != nil {
			return "", fmt.Errorf("Failed to GetContentAsString: ")
		}
		return answer, nil
	}
	return "", fmt.Errorf("[GetAnswerFromResponse][%s] Empty choices in response", p.Name)
}

func (p *Provider) GetFinishReasonFromResponse(resp core.GenericResponse) (string, error) {
	response, ok := resp.(Response)
	if !ok {
		return "", fmt.Errorf("[GetFinishReasonFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}

	if len(response.Choices) > 0 {
		return response.Choices[0].FinishReason, nil
	}
	return "", fmt.Errorf("[GetFinishReasonFromResponse][%s] Empty choices in response", p.Name)
}

func (p *Provider) GetMessageFromResponse(resp core.GenericResponse) (core.GenericMessage, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetMessageFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	if len(response.Choices) > 0 {
		return response.Choices[0].Message, nil
	}
	return "", fmt.Errorf("[GetMessageFromResponse][%s] Empty choices in response", p.Name)
}

func (p *Provider) GetToolCallsFromResponse(resp core.GenericResponse) ([]tools.ToolCall, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetToolCallsFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	if len(response.Choices) > 0 {
		return response.Choices[0].Message.ToolCalls, nil
	}
	return nil, fmt.Errorf("[GetToolCallsFromResponse][%s] Empty choices in response", p.Name)
}

func (p *Provider) GetMessageHistory() any {
	return p.History
}

func (p *Provider) IsSenderRole(msg core.GenericMessage) (bool, error) {
	message, ok := msg.(Message)
	if !ok {
		return false, fmt.Errorf("[IsSenderRole] Failed to cast core.GenericMessage -> %s.message", p.Name)
	}
	senderRoles := []string{roleUser, roleDev, roleTool}
	if slices.Contains(senderRoles, message.Role) {
		return true, nil
	}
	return false, nil
}

func (p *Provider) IsToolCall(genericResponse core.GenericResponse) (bool, error) {
	resp, ok := genericResponse.(Response)
	if !ok {
		return false, fmt.Errorf("[GetToolCallsFromResponse] Failed to cast core.GenericResponse -> %s.Response.", p.Name)
	}
	finishReason, err := p.GetFinishReasonFromResponse(resp)
	if err != nil {
		return false, err
	}
	switch finishReason {
	case p.FinishReasonStop():
		return false, nil
	case p.FinishReasonToolCall():
		return true, nil
	default:
		return false, fmt.Errorf("Unexpected finish reason: %s", finishReason)
	}
}

func (p *Provider) IsToolCallValid(toolCall tools.ToolCall) (bool, error) {
	if toolCall.Type == "function" {
		return true, nil
	}
	return false, nil
}

func (p *Provider) NewRequest() (core.GenericRequest, error) {
	return Request{
		Model:    p.Config.Model,
		Tools:    p.Config.APCTools.Tools,
		Messages: p.History,
		Provider: p.Config.SubProvider,
	}, nil
}

func (p *Provider) GetApiKey() string { return os.Getenv("OPENROUTER_API_KEY") }

func (p *Provider) GetEndpoint() string { return chatCompletionRequestUrl }

func (p *Provider) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + p.GetApiKey(),
	}
}

func (m *Message) GetContentAsString() (string, error) {
	if m.Content == nil {
		return "", fmt.Errorf("[GetContentAsString: Content = nil")
	}
	if str, ok := m.Content.(string); ok {
		return str, nil
	}
	return "", fmt.Errorf("[GetContentAsString: string cast failed]")
}

func (m *Message) GetContentAsArray() ([]Part, error) {
	if m.Content == nil {
		return nil, fmt.Errorf("[GetContentAsString: Content = nil")
	}
	if parts, ok := m.Content.([]Part); ok {
		return parts, nil
	}
	return nil, fmt.Errorf("[GetContentAsString: []Part cast failed]")
}

func (p *Provider) ConstructSystemPromptMessage() Message {
	return Message{
		Role: roleSys,
		Content: []Part{
			{
				Type: "text",
				Text: p.Config.SystemPrompt,
			},
		},
	}
}

func (p *Provider) ConstructUserPromptMessage(prompt string) core.GenericMessage {
	return Message{
		Role: roleUser,
		Content: []Part{
			{
				Type: "text",
				Text: prompt,
			},
		},
	}
}

func (p *Provider) ConstructToolMessage(tooCall tools.ToolCall, toolResult string) core.GenericMessage {
	return Message{
		Role: roleTool,
		Content: []Part{
			{
				Type: "text",
				Text: toolResult,
			},
		},
		ToolCallId: tooCall.Id,
	}
}

func (p *Provider) SendRequest(ctx context.Context, request core.GenericRequest) (core.GenericResponse, error) {
	req, ok := request.(Request)
	if !ok {
		return nil, fmt.Errorf("[SendRequest] Failed to cast core.GenericRequest -> %s.Request", p.Name)
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	c := http.New()
	respBytes, err := c.Post(ctx, p.Endpoint, p.GetHeaders(), reqBytes)
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	return resp, nil
}
