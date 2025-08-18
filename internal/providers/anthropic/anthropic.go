package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	// "github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/tools"
)

const chatCompletionRequestUrl = "https://api.anthropic.com/v1/messages"
const maxTokens = 10000
const (
	roleUser  = "user"
	roleModel = "assistant"
)

const (
	stopReasonStop      = "end_turn"
	stopReasonMaxTokens = "max_tokens"
	stopReasonToolUse   = "tool_use"
)

type Provider struct {
	Name         string
	Endpoint     string
	Model        string
	SystemPrompt string
	History      []Message
	Tools        []Tool
}

type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Tools     []Tool    `json:"tools"`
}

type ToolCall struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"args,omitempty"`
}

type Content struct {
	Type              string          `json:"type,omitempty"` // text, tool_use
	Text              string          `json:"text,omitempty"`
	ToolId            string          `json:"id,omitempty"`
	ToolUseId         string          `json:"tool_use_id,omitempty"`
	ToolName          string          `json:"name,omitempty"`
	ToolInput         json.RawMessage `json:"input,omitempty"`
	ToolResultContent string          `json:"content,omitempty"`
}

type Response struct {
	Role       string    `json:"role"`
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
}

type Tool struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	InputSchema tools.ToolFunctionParameters `json:"input_schema"`
}

func CheckModelName(model string) error {
	models := []string{"moonshotai/kimi-k2", "google/gemini-2.5-flash"}
	if !slices.Contains(models, model) {
		return fmt.Errorf("UnsupportedModelName: `%s`", model)
	}
	return nil
}

func New(model string, systemPrompt string, toolList []tools.Tool) (core.IProvider, error) {
	CheckModelName(model)
	p := &Provider{
		Name:         "anthropic",
		Endpoint:     chatCompletionRequestUrl,
		Model:        model,
		SystemPrompt: systemPrompt,
		History:      make([]Message, 0),
		Tools:        make([]Tool, 0),
	}
	p.Tools = append(p.Tools, p.GetToolsAdapter(toolList)...)
	return p, nil
}

func (p *Provider) GetApiKey() string { return os.Getenv("ANTHROPIC_API_KEY") }

func (p *Provider) GetEndpoint() string { return chatCompletionRequestUrl }

func (p *Provider) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":      "application/json",
		"x-api-key":         p.GetApiKey(),
		"anthropic-version": "2023-06-01",
	}
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
	if response.StopReason == stopReasonStop {
		if len(response.Content) > 0 {
			if response.Content[0].Type == "text" {
				return response.Content[0].Text, nil
			}
			return "", fmt.Errorf("[GetAnswerFromResponse] Response.Content[0].Type -> expected `%s`, got: `%s`", "text", response.Content[0].Type)
		}
		return "", fmt.Errorf("[GetAnswerFromResponse] Empty Response.Content 🤔")
	}

	return "", fmt.Errorf("[GetAnswerFromResponse] Finish reason -> expected: `%s`, got: `%s`", stopReasonStop, response.StopReason)
}

func (p *Provider) GetFinishReasonFromResponse(resp core.GenericResponse) (string, error) {
	response, ok := resp.(Response)
	if !ok {
		return "", fmt.Errorf("[GetFinishReasonFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	return response.StopReason, nil
}

func (p *Provider) GetMessageFromResponse(resp core.GenericResponse) (core.GenericMessage, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetMessageFromResponse] Failed to cast core.GenericResponse -> %s.Response\n", p.Name)
	}
	return Message{
		Role:    response.Role,
		Content: response.Content,
	}, nil
}

func (p *Provider) GetToolCallsFromResponse(resp core.GenericResponse) ([]tools.ToolCall, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetToolCallsFromResponse] Failed to cast core.GenericResponse -> %s.Response.", p.Name)
	}
	var toolCalls []tools.ToolCall
	for _, content := range response.Content {
		if content.Type == "tool_use" {
			toolCall := tools.ToolCall{
				Id:   content.ToolId,
				Type: content.Type,
				Function: tools.Function{
					Name:      content.ToolName,
					Arguments: content.ToolInput,
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}

	}
	return toolCalls, nil
}

func (p *Provider) GetMessageHistory() any {
	return p.History
}

func (p *Provider) IsSenderRole(msg core.GenericMessage) (bool, error) {
	message, ok := msg.(Message)
	if !ok {
		return false, fmt.Errorf("[IsSenderRole] Failed to cast core.GenericMessage -> %s.message", p.Name)
	}
	senderRoles := []string{roleUser}
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
	if toolCall.Type == "tool_use" {
		return true, nil
	}
	return false, nil
}

func (p *Provider) NewRequest() (core.GenericRequest, error) {
	return Request{
		Model:     p.Model,
		System:    p.SystemPrompt,
		Tools:     p.Tools,
		Messages:  p.History,
		MaxTokens: maxTokens,
	}, nil
}

func (p *Provider) GetToolsAdapter(genericTools []tools.Tool) []Tool {
	tools := make([]Tool, 0)
	for _, fsTool := range genericTools {
		tools = append(tools, Tool{
			Name:        fsTool.Function.Name,
			Description: fsTool.Function.Description,
			InputSchema: fsTool.Function.Parameters,
		})
	}
	return tools
}

func (p *Provider) ConstructSystemPromptMessage() Message {
	return Message{} // TODO: No sys msg in anthropic, it's at top level
}

func (p *Provider) ConstructToolMessage(tooCall tools.ToolCall, toolResult string) core.GenericMessage {
	return Message{
		Role: roleUser,
		Content: []Content{
			{
				Type:              "tool_result",
				ToolUseId:         tooCall.Id,
				ToolResultContent: toolResult,
			},
		},
	}

}

func (p *Provider) ConstructUserPromptMessage(prompt string) core.GenericMessage {
	return Message{
		Role: roleUser,
		Content: []Content{
			{
				Type: "text",
				Text: prompt,
			},
		},
	}
}

func (p *Provider) SendRequest(ctx context.Context, req core.GenericRequest) (core.GenericResponse, error) {
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
