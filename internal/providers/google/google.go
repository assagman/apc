package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	// "github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/core"
	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/tools"
)

const chatCompletionRequestUrlTemplate = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
const (
	roleUser  = "user"
	roleModel = "model"
)
const (
	finishReasonStop      = "STOP"
	finishReasonMaxTokens = "MAX_TOKENS"
)

type Provider struct {
	Name         string
	Endpoint     string
	Model        string
	SystemPrompt string
	History      []Content
	Tools        Tools
}

type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Text             string            `json:"text,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

type FunctionCall struct {
	Id        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"args,omitempty"`
}

type Tools struct {
	FunctionDeclarations []Tool `json:"functionDeclarations"`
}

type Tool tools.FunctionDefinition

type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

type Request struct {
	SystemInstruction *SystemInstruction `json:"system_instruction,omitempty"`
	Contents          []Content          `json:"contents"`
	Tools             Tools              `json:"tools"`
}

type FunctionResponse struct {
	Id       string         `json:"id"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finishReason"`
}

type Response struct {
	Candidates []Candidate `json:"candidates"`
}

func CheckModelName(model string) error {
	models := []string{"google/gemini-2.5-pro", "google/gemini-2.5-flash"}
	if !slices.Contains(models, model) {
		return fmt.Errorf("UnsupportedModelName: `%s`", model)
	}
	return nil
}

func New(config core.ProviderConfig) (core.IProvider, error) {
	CheckModelName(config.Model)
	p := &Provider{
		Name:         "google",
		Endpoint:     fmt.Sprintf(chatCompletionRequestUrlTemplate, config.Model),
		Model:        config.Model,
		SystemPrompt: config.SystemPrompt,
		History:      make([]Content, 0),
		Tools:        Tools{FunctionDeclarations: make([]Tool, 0)},
	}
	p.Tools = p.GetToolsAdapter(config.APCTools.Tools)
	return p, nil
}

func (p *Provider) SetupChannels(ctx context.Context) {

}

func (p *Provider) GetApiKey() string { return os.Getenv("GEMINI_API_KEY") }
func (p *Provider) GetEndpoint() string {
	return fmt.Sprintf(chatCompletionRequestUrlTemplate, p.Model)
}
func (p *Provider) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": p.GetApiKey(),
	}
}

func (p *Provider) AppendMessageHistory(msg core.GenericMessage) error {
	message, ok := msg.(Content)
	if !ok {
		return fmt.Errorf("[AppendMessageHistory] Failed to cast core.GenericMessage -> %s.Message", p.Name)
	}

	p.History = append(p.History, message)
	return nil
}

func (p *Provider) FinishReasonStop() string { return finishReasonStop }

func (p *Provider) FinishReasonToolCall() string { return finishReasonStop }

func (p *Provider) GetAnswerFromResponse(resp core.GenericResponse) (string, error) {
	response, ok := resp.(Response)
	if !ok {
		return "", fmt.Errorf("[GetAnswerFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	answer := ""
	for _, part := range response.Candidates[0].Content.Parts {
		answer += part.Text
	}
	return answer, nil
}

func (p *Provider) GetFinishReasonFromResponse(resp core.GenericResponse) (string, error) {
	response, ok := resp.(Response)
	if !ok {
		return "", fmt.Errorf("[GetFinishReasonFromResponse] Failed to cast core.GenericResponse -> %s.Response", p.Name)
	}
	return response.Candidates[0].FinishReason, nil
}

func (p *Provider) GetMessageFromResponse(resp core.GenericResponse) (core.GenericMessage, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetMessageFromResponse] Failed to cast core.GenericResponse -> %s.Response\n", p.Name)
	}
	return response.Candidates[0].Content, nil
}

func (p *Provider) GetToolCallsFromResponse(resp core.GenericResponse) ([]tools.ToolCall, error) {
	response, ok := resp.(Response)
	if !ok {
		return nil, fmt.Errorf("[GetToolCallsFromResponse] Failed to cast core.GenericResponse -> %s.Response.", p.Name)
	}
	var toolCalls []tools.ToolCall
	for _, part := range response.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			toolCall := tools.ToolCall{
				Id:   part.FunctionCall.Id,
				Type: "", // no type in google, just passing
				Function: tools.Function{
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Arguments,
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
	message, ok := msg.(Content)
	if !ok {
		return false, fmt.Errorf("[IsSenderRole] Failed to cast core.GenericMessage -> %s.Content", p.Name)
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
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("Unexpected finish reason: %s", finishReason)
	}
}

func (p *Provider) IsToolCallValid(toolCall tools.ToolCall) (bool, error) {
	if toolCall.Function.Name != "" {
		return true, nil
	}
	return false, nil
}

func (p *Provider) NewRequest() (core.GenericRequest, error) {
	return Request{
		SystemInstruction: p.GetSystemPrompt(),
		Tools:             p.Tools,
		Contents:          p.History,
	}, nil
}

func (p *Provider) GetToolsAdapter(genericTools []tools.Tool) Tools {
	tools := Tools{}
	tools.FunctionDeclarations = make([]Tool, 0)
	for _, fsTool := range genericTools {
		tools.FunctionDeclarations = append(tools.FunctionDeclarations, Tool{
			Name:        fsTool.Function.Name,
			Description: fsTool.Function.Description,
			Parameters:  fsTool.Function.Parameters,
		})
	}
	return tools
}

func (p *Provider) ConstructSystemPromptMessage() Content {
	return Content{} // TODO: No sys msg in anthropic, it's at top level
}

func (p *Provider) ConstructToolMessage(tooCall tools.ToolCall, toolResult string) core.GenericMessage {
	return Content{
		Role: roleUser,
		Parts: []Part{
			{
				FunctionResponse: &FunctionResponse{
					Id:   tooCall.Id,
					Name: tooCall.Function.Name,
					Response: map[string]any{
						"result": toolResult,
					},
				},
			},
		},
	}

}

func (p *Provider) ConstructUserPromptMessage(prompt string) core.GenericMessage {
	return Content{
		Role: roleUser,
		Parts: []Part{
			{
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

func (p *Provider) GetSystemPrompt() *SystemInstruction {
	if p.SystemPrompt == "" {
		return nil
	}

	return &SystemInstruction{
		Parts: []Part{
			{
				Text: p.SystemPrompt,
			},
		},
	}
}
