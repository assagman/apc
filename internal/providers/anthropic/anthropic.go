package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/logger"
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
	Tools     Tools     `json:"tools"`
}

type ToolCall struct {
	Id        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"args,omitempty"`
}

type Content struct {
	Type              string          `json:"type,omitempty"`
	Text              string          `json:"text,omitempty"`
	ToolId            string          `json:"id,omitempty"`
	ToolUseId         string          `json:"tool_use_id,omitempty"`
	ToolName          string          `json:"name,omitempty"`
	ToolInput         json.RawMessage `json:"input,omitempty"`
	ToolResultContent string          `json:"content,omitempty"`
}

type Response struct {
	Content    []Content `json:"content"`
	StopReason string    `json:"stop_reason"`
}

type Tool struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	InputSchema tools.ToolFunctionParameters `json:"input_schema"`
}

type Tools []Tool

func CheckModelName(model string) error {
	models := []string{"moonshotai/kimi-k2", "google/gemini-2.5-flash"}
	if !slices.Contains(models, model) {
		return fmt.Errorf("UnsupportedModelName: `%s`", model)
	}
	return nil
}

func New(model string, systemPrompt string) (core.IProvider, error) {
	CheckModelName(model)
	return &Provider{
		Name:         "anthropic",
		Endpoint:     chatCompletionRequestUrl,
		Model:        model,
		SystemPrompt: systemPrompt,
		History:      make([]Message, 0),
	}, nil
}

func (p *Provider) GetName() string {
	return p.Name
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

func (p *Provider) GetTools() *Tools {
	fsTools, err := tools.GetFsTools()
	if err != nil {
		logger.Warning("Failed to get fs tools")
	}
	tools := make(Tools, 0)
	for _, fsTool := range fsTools {
		tools = append(tools, Tool{
			Name:        fsTool.Function.Name,
			Description: fsTool.Function.Description,
			InputSchema: fsTool.Function.Parameters,
		})
	}
	return &tools
}

func (p *Provider) ConstructUserPromptMessage(prompt string) Message {
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

func (p *Provider) SendRequest(ctx context.Context, req Request) (*Response, error) {
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

	p.History = append(p.History, Message{
		Role:    roleModel,
		Content: resp.Content,
	})
	return &resp, nil
}

func (p *Provider) HandleToolCalls(ctx context.Context, resp Response) (*Response, error) {
	if resp.StopReason == stopReasonToolUse {
		for _, content := range resp.Content {
			if content.Type == "tool_use" {
				return p.SendToolResult(ctx, ToolCall{
					Id:        content.ToolId,
					Name:      content.ToolName,
					Arguments: content.ToolInput,
				})
			}
		}
		return nil, fmt.Errorf("Unable to find tool_use")
	}

	return &resp, nil // no tool call
}

func (p *Provider) SendUserPrompt(ctx context.Context, userPrompt string) (string, error) {
	p.History = append(p.History, p.ConstructUserPromptMessage(userPrompt))
	req := Request{
		Model:     p.Model,
		System:    p.SystemPrompt,
		Tools:     *p.GetTools(),
		Messages:  p.History,
		MaxTokens: maxTokens,
	}

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return "", err
	}

	answer := finalResp.Content[0].Text
	return answer, nil
}

func (p *Provider) SendToolResult(ctx context.Context, f ToolCall) (*Response, error) {

	var argsMap = make(map[string]any)
	if f.Arguments != nil && string(f.Arguments) != "{}" {

		// Unmarshal the string into the map
		// fmt.Println("Unmarshalling arguments strings to map")
		errr := json.Unmarshal([]byte(f.Arguments), &argsMap)
		if errr != nil {
			return nil, errr
		}
	}

	toolResult, toolErr := tools.ExecTool(f.Name, argsMap)
	if toolErr != nil {
		return nil, toolErr
	}

	content := Message{
		Role: roleUser,
		Content: []Content{
			{
				Type:              "tool_result",
				ToolUseId:         f.Id,
				ToolResultContent: toolResult.(string),
			},
		},
	}

	p.History = append(p.History, content)
	req := Request{
		Model:     p.Model,
		System:    p.SystemPrompt,
		Tools:     *p.GetTools(),
		Messages:  p.History,
		MaxTokens: maxTokens,
	}

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return nil, err
	}

	if finalResp.StopReason == stopReasonStop {
		return finalResp, err
	}

	return nil, fmt.Errorf("[SendToolResult] Unexpected finish reason: %s", resp.StopReason)
}
