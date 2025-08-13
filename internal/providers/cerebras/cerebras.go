package cerebras

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

const chatCompletionRequestUrl = "https://api.cerebras.ai/v1/chat/completions"
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
	Name         string
	Endpoint     string
	Model        string
	SystemPrompt string
	History      []Message
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCall struct {
	Function tools.Function `json:"function"`
	Id       string         `json:"id"`
	Type     string         `json:"type"`
}

type Tools []tools.Tool

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"` // req: string or array, resp: string or null
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string     `json:"tool_call_id,omitempty"`
}

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    *Tools    `json:"tools"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Response struct {
	Choices []Choice `json:"choices"`
}

func CheckModelName(model string) error {
	models := []string{"gpt-oss:q-120b", "qwen-3-coder-480b", "qwen-3-235b-a22b-thinking-2507", "qwen-3-235b-a22b-instruct-2507"}
	if !slices.Contains(models, model) {
		return fmt.Errorf("UnsupportedModelName: `%s`", model)
	}
	return nil
}

func New(model string, systemPrompt string) (core.IProvider, error) {
	CheckModelName(model)
	return &Provider{
		Name:         "cerebras",
		Endpoint:     chatCompletionRequestUrl,
		Model:        model,
		SystemPrompt: systemPrompt,
		History:      make([]Message, 0),
	}, nil
}

func (p *Provider) GetName() string {
	return p.Name
}

func (p *Provider) GetApiKey() string { return os.Getenv("CEREBRAS_API_KEY") }

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

func (p *Provider) GetTools() *Tools {
	fsTools, err := tools.GetFsTools()
	if err != nil {
		logger.Warning("Failed to get fs tools")
	}
	tools := make(Tools, 0)
	for _, fsTool := range fsTools {
		tools = append(tools, *fsTool)
	}
	return &tools
}

func (p *Provider) ConstructSystemPromptMessage(prompt string) Message {
	return Message{
		Role:    roleSys,
		Content: prompt,
	}
}

func (p *Provider) ConstructUserPromptMessage(prompt string) Message {
	return Message{
		Role:    roleUser,
		Content: prompt,
	}
}

func (p *Provider) SendRequest(ctx context.Context, req Request) (*Response, error) {
	// ======================================================== Request Conversion to bytes
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// ======================================================== REST API CALL
	c := http.New()
	respBytes, err := c.Post(ctx, p.Endpoint, p.GetHeaders(), reqBytes)
	if err != nil {
		return nil, err
	}

	// ======================================================== Response conversion from bytes
	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		fmt.Println("x")
		return nil, err
	}

	p.History = append(p.History, resp.Choices[0].Message)
	return &resp, nil
}

func (p *Provider) HandleToolCalls(ctx context.Context, resp Response) (*Response, error) {
	if resp.Choices[0].FinishReason == stopReasonToolUse {
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {
			if toolCall.Type == "function" {
				return p.SendToolResult(ctx, toolCall)
			}
		}
		return nil, fmt.Errorf("Unable to find tool_use")
	}

	return &resp, nil // no tool call
}

func (p *Provider) SendUserPrompt(ctx context.Context, userPrompt string) (string, error) {
	p.History = append(p.History, p.ConstructSystemPromptMessage(p.SystemPrompt))
	p.History = append(p.History, p.ConstructUserPromptMessage(userPrompt))
	req := Request{
		Model:    p.Model,
		Tools:    p.GetTools(),
		Messages: p.History,
	}

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return "", err
	}

	answer, err := finalResp.Choices[0].Message.GetContentAsString()
	if err != nil {
		return "", err
	}
	return answer, nil
}

func (p *Provider) SendToolResult(ctx context.Context, f ToolCall) (*Response, error) {
	// ======================================================== Parse function arguments
	var argsMap = make(map[string]any)
	if f.Function.Arguments != nil && string(f.Function.Arguments) != "{}" {
		var argsStr string

		// Unmarshal the RawMessage into the map
		// fmt.Println("Unmarshalling arguments RawMessages to string")
		err := json.Unmarshal([]byte(f.Function.Arguments), &argsStr)
		if err != nil {
			fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(f.Function.Arguments))
			return nil, err
		}

		// Unmarshal the string into the map
		// fmt.Println("Unmarshalling arguments strings to map")
		errr := json.Unmarshal([]byte(argsStr), &argsMap)
		if errr != nil {
			fmt.Println("y")
			return nil, errr
		}
	}
	// ======================================================== Parse function arguments

	// ======================================================== Execute tool
	toolResult, toolErr := tools.ExecTool(f.Function.Name, argsMap)
	if toolErr != nil {
		return nil, toolErr
	}
	// ======================================================== Execute tool

	// ======================================================== CONTENT
	content := Message{
		Role:       roleTool,
		Content:    toolResult.(string),
		ToolCallId: f.Id,
	}
	// ======================================================== CONTENT

	// ======================================================== Request Construction
	p.History = append(p.History, content)
	req := Request{
		Model:    p.Model,
		Tools:    p.GetTools(),
		Messages: p.History,
	}
	// ======================================================== Request Construction

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// ======================================================== Check finish/stop reason for tool call
	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return nil, err
	}

	if finalResp.Choices[0].FinishReason == stopReasonStop {
		return finalResp, err
	}
	// ======================================================== Check finish/stop reason for tool call

	return nil, fmt.Errorf("[SendToolResult] Unexpected finish reason: %s", finalResp.Choices[0].FinishReason)
}
