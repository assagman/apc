package anthropic

import (
	"context"
	"encoding/json"
	// "fmt"
	"os"

	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/tools"
)

const ChatCompletionRequestUrl = "https://api.anthropic.com/v1/messages"
const MaxTokens = 10000

type AnthropicMessage struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

type AnthropicFunctionDefinition struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	InputSchema tools.ToolFunctionParameters `json:"input_schema"`
}

type AnthropicChatCompletionRequest struct {
	Model     string                        `json:"model"`
	Messages  []AnthropicMessage            `json:"messages"`
	Tools     []AnthropicFunctionDefinition `json:"tools"`
	MaxTokens int                           `json:"max_tokens"`
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

type ChatCompletionResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Model        string    `json:"model"`
	Role         string    `json:"role"`
	Created      int64     `json:"created"`
	Content      []Content `json:"content"`
	Usage        UsageInfo `json:"usage"`
	StopReason   string    `json:"stop_reason"`
	StopSequence *struct{} `json:"stop_sequence"`
}

type UsageInfo struct {
	InputTokens              int       `json:"input_tokens"`
	OutputTokens             int       `json:"completion_tokens"`
	CacheCreationInputTokens int       `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int       `json:"cache_read_input_tokens"`
	ServiceTier              string    `json:"service_tier"`
	PromptTokensDetails      *struct{} `json:"prompt_tokens_details"`
}

type Logprobs struct{}
type PromptTokensDetails struct{}

type Client struct {
	Headers        map[string]string
	ConversationId int
	MessageHistory []AnthropicMessage
}

func New() *Client {
	return &Client{
		Headers: map[string]string{
			"Content-Type":      "application/json",
			"x-api-key":         os.Getenv("ANTHROPIC_API_KEY"),
			"anthropic-version": "2023-06-01",
		},
		ConversationId: 0, // not started yet
		MessageHistory: make([]AnthropicMessage, 0),
	}
}

func (c *Client) Name() string {
	return "anthropic"
}

func (c *Client) SendChatCompletionRequest(model string, role string, content string) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsTools, err := tools.GetFsTools()
	if err != nil {
		return nil, err
	}
	var atools []AnthropicFunctionDefinition
	for _, fsTool := range fsTools {
		atools = append(atools, AnthropicFunctionDefinition{
			Name:        fsTool.Function.Name,
			Description: fsTool.Function.Description,
			InputSchema: fsTool.Function.Parameters,
		})
	}

	if role == "user" {
		c.ConversationId++
		c.MessageHistory = append(c.MessageHistory, AnthropicMessage{
			Role: "user",
			Content: []Content{
				{
					Type: "text",
					Text: content,
				},
			},
		})
	}
	requestBody := AnthropicChatCompletionRequest{
		Model:     model,
		Messages:  c.MessageHistory,
		Tools:     atools,
		MaxTokens: MaxTokens,
	}
	reqBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpClient := http.New()
	respBytes, err := httpClient.Post(ctx, ChatCompletionRequestUrl, c.Headers, reqBodyBytes)
	if err != nil {
		return nil, err
	}

	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	for _, respContent := range resp.Content {
		c.MessageHistory = append(c.MessageHistory, AnthropicMessage{
			Role:    resp.Role,
			Content: []Content{respContent},
		})
	}

	if resp.StopReason == "tool_use" {
		var toolMessages []AnthropicMessage
		var toolCalls []Content
		for _, respContent := range resp.Content {
			if respContent.Type == "tool_use" {
				toolCalls = append(toolCalls, respContent)
			}
		}

		for _, toolCall := range toolCalls {
			argsMap := make(map[string]any)
			if toolCall.ToolInput != nil && string(toolCall.ToolInput) != "{}" {
				// // fmt.Printf("tool: %s\n", toolCall.Function.Name)
				// var argsStr string
				//
				// // Unmarshal the RawMessage into the map
				// // fmt.Println("Unmarshalling arguments RawMessages to string")
				// err := json.Unmarshal([]byte(toolCall.ToolInput), &argsStr)
				// if err != nil {
				// 	fmt.Printf("Failed to unmarshal toolCall.ToolInput. Value: %s\n", string(toolCall.ToolInput))
				// 	return nil, err
				// }

				// Unmarshal the string into the map
				// fmt.Println("Unmarshalling arguments strings to map")
				errr := json.Unmarshal([]byte(toolCall.ToolInput), &argsMap)
				if errr != nil {
					return nil, errr
				}
			}

			toolResult, toolErr := tools.ExecTool(toolCall.ToolName, argsMap)
			if toolErr != nil {
				return nil, toolErr
			}
			toolMessages = append(toolMessages, AnthropicMessage{
				Role: "user",
				Content: []Content{
					{
						Type:              "tool_result",
						ToolUseId:         toolCall.ToolId,
						ToolResultContent: toolResult.(string),
					},
				},
			})
			c.MessageHistory = append(c.MessageHistory, toolMessages...)
			// toolMessages = append(toolMessages, common.Message{
			// 	Role:       "tool",
			// 	Content:    toolResult.(string), // can be dangerous. func signatures, returns must be handled in a better and more solid way
			// 	Name:       toolCall.Function.Name,
			// 	ToolCallId: toolCall.Id,
			// })
		}
		return c.SendChatCompletionRequest(model, "tool", "")
	}

	/*
		        // Example response
				x := map[string]any{
					"id":    "msg_01NqT9oCuVSwvNQ7FAWRCdvQ",
					"type":  "message",
					"role":  "assistant",
					"model": "claude-opus-4-1-20250805",
					"content": []map[string]any{
						{
							"type": "text",
							"text": "I'll get the current working directory for you.",
						},
						{
							"type":  "tool_use",
							"id":    "toolu_01XTfFndu3ZcYBr4iNUhmgEc",
							"name":  "ToolGetCurrentWorkingDirectory",
							"input": []string{},
						},
					},
					"stop_reason":   "tool_use",
					"stop_sequence": nil,
					"usage": map[string]any{
						"input_tokens":                608,
						"cache_creation_input_tokens": 0,
						"cache_read_input_tokens":     0,
						"output_tokens":               51,
						"service_tier":                "standard",
					},
				}
	*/
	return respBytes, nil
}

func (c *Client) ExtractAnswerFromChatCompletionResponse(respBytes []byte) (string, error) {
	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return "", err
	}
	return resp.Content[0].Text, nil
}
