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
	ApiKey       string
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
		ApiKey:       os.Getenv("ANTHROPIC"),
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
	// // DEBUG
	// b, e := json.MarshalIndent(p.History, "", "  ")
	// if e != nil {
	// 	return "", e
	// }
	// println(string(b))
	// // DEBUG

	answer := finalResp.Content[0].Text
	return answer, nil
}

func (p *Provider) SendToolResult(ctx context.Context, f ToolCall) (*Response, error) {

	// ======================================================== Parse function arguments
	var argsMap = make(map[string]any)
	if f.Arguments != nil && string(f.Arguments) != "{}" {
		// var argsStr string
		//
		// // Unmarshal the RawMessage into the map
		// // fmt.Println("Unmarshalling arguments RawMessages to string")
		// err := json.Unmarshal([]byte(f.Arguments), &argsStr)
		// if err != nil {
		// 	fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(f.Arguments))
		// 	return nil, err
		// }

		// Unmarshal the string into the map
		// fmt.Println("Unmarshalling arguments strings to map")
		errr := json.Unmarshal([]byte(f.Arguments), &argsMap)
		if errr != nil {
			return nil, errr
		}
	}
	// ======================================================== Parse function arguments

	// ======================================================== Execute tool
	toolResult, toolErr := tools.ExecTool(f.Name, argsMap)
	if toolErr != nil {
		return nil, toolErr
	}
	// ======================================================== Execute tool

	// ======================================================== CONTENT
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
	// ======================================================== CONTENT

	// ======================================================== Request Construction
	p.History = append(p.History, content)
	req := Request{
		Model:     p.Model,
		System:    p.SystemPrompt,
		Tools:     *p.GetTools(),
		Messages:  p.History,
		MaxTokens: maxTokens,
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

	if finalResp.StopReason == stopReasonStop {
		return finalResp, err
	}
	// ======================================================== Check finish/stop reason for tool call

	return nil, fmt.Errorf("[SendToolResult] Unexpected finish reason: %s", resp.StopReason)
}

// import (
// 	"context"
// 	"encoding/json"
// 	// "fmt"
// 	"os"
//
// 	"github.com/assagman/apc/internal/http"
// 	"github.com/assagman/apc/internal/tools"
// )
//
// const ChatCompletionRequestUrl = "https://api.anthropic.com/v1/messages"
// const MaxTokens = 10000
//
// type AnthropicMessage struct {
// 	Role    string    `json:"role"`
// 	Content []Content `json:"content"`
// }
//
// type AnthropicFunctionDefinition struct {
// 	Name        string                       `json:"name"`
// 	Description string                       `json:"description"`
// 	InputSchema tools.ToolFunctionParameters `json:"input_schema"`
// }
//
// type AnthropicChatCompletionRequest struct {
// 	Model     string                        `json:"model"`
// 	Messages  []AnthropicMessage            `json:"messages"`
// 	Tools     []AnthropicFunctionDefinition `json:"tools"`
// 	MaxTokens int                           `json:"max_tokens"`
// }
//
// type Content struct {
// 	Type              string          `json:"type,omitempty"`
// 	Text              string          `json:"text,omitempty"`
// 	ToolId            string          `json:"id,omitempty"`
// 	ToolUseId         string          `json:"tool_use_id,omitempty"`
// 	ToolName          string          `json:"name,omitempty"`
// 	ToolInput         json.RawMessage `json:"input,omitempty"`
// 	ToolResultContent string          `json:"content,omitempty"`
// }
//
// type ChatCompletionResponse struct {
// 	ID           string    `json:"id"`
// 	Type         string    `json:"type"`
// 	Model        string    `json:"model"`
// 	Role         string    `json:"role"`
// 	Created      int64     `json:"created"`
// 	Content      []Content `json:"content"`
// 	Usage        UsageInfo `json:"usage"`
// 	StopReason   string    `json:"stop_reason"`
// 	StopSequence *struct{} `json:"stop_sequence"`
// }
//
// type UsageInfo struct {
// 	InputTokens              int       `json:"input_tokens"`
// 	OutputTokens             int       `json:"completion_tokens"`
// 	CacheCreationInputTokens int       `json:"cache_creation_input_tokens"`
// 	CacheReadInputTokens     int       `json:"cache_read_input_tokens"`
// 	ServiceTier              string    `json:"service_tier"`
// 	PromptTokensDetails      *struct{} `json:"prompt_tokens_details"`
// }
//
// type Logprobs struct{}
// type PromptTokensDetails struct{}
//
// type Client struct {
// 	Headers        map[string]string
// 	ConversationId int
// 	MessageHistory []AnthropicMessage
// }
//
// func New() *Client {
// 	return &Client{
// 		Headers: map[string]string{
// 			"Content-Type":      "application/json",
// 			"x-api-key":         os.Getenv("ANTHROPIC_API_KEY"),
// 			"anthropic-version": "2023-06-01",
// 		},
// 		ConversationId: 0, // not started yet
// 		MessageHistory: make([]AnthropicMessage, 0),
// 	}
// }
//
// func (c *Client) Name() string {
// 	return "anthropic"
// }
//
// func (c *Client) SendChatCompletionRequest(model string, role string, content string) ([]byte, error) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	fsTools, err := tools.GetFsTools()
// 	if err != nil {
// 		return nil, err
// 	}
// 	var atools []AnthropicFunctionDefinition
// 	for _, fsTool := range fsTools {
// 		atools = append(atools, AnthropicFunctionDefinition{
// 			Name:        fsTool.Function.Name,
// 			Description: fsTool.Function.Description,
// 			InputSchema: fsTool.Function.Parameters,
// 		})
// 	}
//
// 	if role == "user" {
// 		c.ConversationId++
// 		c.MessageHistory = append(c.MessageHistory, AnthropicMessage{
// 			Role: "user",
// 			Content: []Content{
// 				{
// 					Type: "text",
// 					Text: content,
// 				},
// 			},
// 		})
// 	}
// 	requestBody := AnthropicChatCompletionRequest{
// 		Model:     model,
// 		Messages:  c.MessageHistory,
// 		Tools:     atools,
// 		MaxTokens: MaxTokens,
// 	}
// 	reqBodyBytes, err := json.Marshal(requestBody)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	httpClient := http.New()
// 	respBytes, err := httpClient.Post(ctx, ChatCompletionRequestUrl, c.Headers, reqBodyBytes)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var resp ChatCompletionResponse
// 	if err := json.Unmarshal(respBytes, &resp); err != nil {
// 		return nil, err
// 	}
// 	for _, respContent := range resp.Content {
// 		c.MessageHistory = append(c.MessageHistory, AnthropicMessage{
// 			Role:    resp.Role,
// 			Content: []Content{respContent},
// 		})
// 	}
//
// 	if resp.StopReason == "tool_use" {
// 		var toolMessages []AnthropicMessage
// 		var toolCalls []Content
// 		for _, respContent := range resp.Content {
// 			if respContent.Type == "tool_use" {
// 				toolCalls = append(toolCalls, respContent)
// 			}
// 		}
//
// 		for _, toolCall := range toolCalls {
// 			argsMap := make(map[string]any)
// 			if toolCall.ToolInput != nil && string(toolCall.ToolInput) != "{}" {
// 				// // fmt.Printf("tool: %s\n", toolCall.Function.Name)
// 				// var argsStr string
// 				//
// 				// // Unmarshal the RawMessage into the map
// 				// // fmt.Println("Unmarshalling arguments RawMessages to string")
// 				// err := json.Unmarshal([]byte(toolCall.ToolInput), &argsStr)
// 				// if err != nil {
// 				// 	fmt.Printf("Failed to unmarshal toolCall.ToolInput. Value: %s\n", string(toolCall.ToolInput))
// 				// 	return nil, err
// 				// }
//
// 				// Unmarshal the string into the map
// 				// fmt.Println("Unmarshalling arguments strings to map")
// 				errr := json.Unmarshal([]byte(toolCall.ToolInput), &argsMap)
// 				if errr != nil {
// 					return nil, errr
// 				}
// 			}
//
// 			toolResult, toolErr := tools.ExecTool(toolCall.ToolName, argsMap)
// 			if toolErr != nil {
// 				return nil, toolErr
// 			}
// 			toolMessages = append(toolMessages, AnthropicMessage{
// 				Role: "user",
// 				Content: []Content{
// 					{
// 						Type:              "tool_result",
// 						ToolUseId:         toolCall.ToolId,
// 						ToolResultContent: toolResult.(string),
// 					},
// 				},
// 			})
// 			c.MessageHistory = append(c.MessageHistory, toolMessages...)
// 			// toolMessages = append(toolMessages, common.Message{
// 			// 	Role:       "tool",
// 			// 	Content:    toolResult.(string), // can be dangerous. func signatures, returns must be handled in a better and more solid way
// 			// 	Name:       toolCall.Function.Name,
// 			// 	ToolCallId: toolCall.Id,
// 			// })
// 		}
// 		return c.SendChatCompletionRequest(model, "tool", "")
// 	}
//
// 	/*
// 		        // Example response
// 				x := map[string]any{
// 					"id":    "msg_01NqT9oCuVSwvNQ7FAWRCdvQ",
// 					"type":  "message",
// 					"role":  "assistant",
// 					"model": "claude-opus-4-1-20250805",
// 					"content": []map[string]any{
// 						{
// 							"type": "text",
// 							"text": "I'll get the current working directory for you.",
// 						},
// 						{
// 							"type":  "tool_use",
// 							"id":    "toolu_01XTfFndu3ZcYBr4iNUhmgEc",
// 							"name":  "ToolGetCurrentWorkingDirectory",
// 							"input": []string{},
// 						},
// 					},
// 					"stop_reason":   "tool_use",
// 					"stop_sequence": nil,
// 					"usage": map[string]any{
// 						"input_tokens":                608,
// 						"cache_creation_input_tokens": 0,
// 						"cache_read_input_tokens":     0,
// 						"output_tokens":               51,
// 						"service_tier":                "standard",
// 					},
// 				}
// 	*/
// 	return respBytes, nil
// }
//
// func (c *Client) ExtractAnswerFromChatCompletionResponse(respBytes []byte) (string, error) {
// 	var resp ChatCompletionResponse
// 	if err := json.Unmarshal(respBytes, &resp); err != nil {
// 		return "", err
// 	}
// 	return resp.Content[0].Text, nil
// }
