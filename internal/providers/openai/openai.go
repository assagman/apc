package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/providers/common"
	"github.com/assagman/apc/internal/tools"
)

const ChatCompletionRequestUrl = "https://api.openai.com/v1/chat/completions"

type ChatCompletionResponse struct {
	ID                string    `json:"id"`
	Provider          string    `json:"provider"`
	Model             string    `json:"model"`
	Object            string    `json:"object"` // "chat.completion"
	Created           int64     `json:"created"`
	Choices           []Choice  `json:"choices"`
	Usage             UsageInfo `json:"usage"`
	ServiceTier       string    `json:"service_tier"`
	SystemFingerprint string    `json:"system_fingerprint"`
}

type Choice struct {
	Index              int            `json:"index"`
	Message            common.Message `json:"message"`
	FinishReason       string         `json:"finish_reason"`
	NativeFinishReason string         `json:"native_finish_reason"`
	Logprobs           *Logprobs      `json:"logprobs"`
}

type UsageInfo struct {
	PromptTokens        int       `json:"prompt_tokens"`
	CompletionTokens    int       `json:"completion_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	PromptTokensDetails *struct{} `json:"prompt_tokens_details"`
}

type Logprobs struct{}
type PromptTokensDetails struct{}

type Client struct {
	Headers        map[string]string
	ConversationId int
	MessageHistory []common.Message
}

func New() *Client {
	return &Client{
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + os.Getenv("OPENAI_API_KEY"),
		},
		ConversationId: 0, // not started yet
		MessageHistory: make([]common.Message, 0),
	}
}

func (c *Client) Name() string {
	return "openai"
}

func (c *Client) SendChatCompletionRequest(model string, role string, content string) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsTools, err := tools.GetFsTools()
	if err != nil {
		return nil, err
	}

	if role == "user" {
		c.ConversationId++
		c.MessageHistory = append(c.MessageHistory, common.Message{
			Role:    "user",
			Content: content,
		})
	}
	requestBody := common.ChatCompletionRequest{
		Model:    model,
		Messages: c.MessageHistory,
		Tools:    fsTools,
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
	c.MessageHistory = append(c.MessageHistory, resp.Choices[0].Message)

	if resp.Choices[0].FinishReason == "tool_calls" {
		var toolMessages []common.Message
		for _, toolCall := range resp.Choices[0].Message.ToolCalls {

			// fmt.Printf("tool: %s\n", toolCall.Function.Name)
			var argsStr string

			// Unmarshal the RawMessage into the map
			// fmt.Println("Unmarshalling arguments RawMessages to string")
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsStr)
			if err != nil {
				fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(toolCall.Function.Arguments))
				return nil, err
			}

			var argsMap map[string]any

			// Unmarshal the string into the map
			// fmt.Println("Unmarshalling arguments strings to map")
			errr := json.Unmarshal([]byte(argsStr), &argsMap)
			if errr != nil {
				return nil, errr
			}

			toolResult, toolErr := tools.ExecTool(toolCall.Function.Name, argsMap)
			if toolErr != nil {
				return nil, toolErr
			}
			toolMessages = append(toolMessages, common.Message{
				Role:       "tool",
				Content:    toolResult.(string),
				Name:       toolCall.Function.Name,
				ToolCallId: toolCall.Id,
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
				"id":       "gen-1754744936-YJgQnv4UgdPUEQxn3TPE",
				"provider": "Moonshot AI",
				"model":    "moonshotai/kimi-k2",
				"object":   "chat.completion",
				"created":  1754744936,
				"choices": []map[string]any{
					{
						"logprobs":             nil,
						"finish_reason":        "stop",
						"native_finish_reason": "stop",
						"index":                0,
						"message": map[string]any{
							"role":      "assistant",
							"content":   "Not much—just here and ready to chat. What's on your mind today?",
							"refusal":   nil,
							"reasoning": nil,
						},
					},
				},
				"system_fingerprint": "fpv0_a5c14cfb",
				"usage": map[string]int{
					"prompt_tokens":     12,
					"completion_tokens": 17,
					"total_tokens":      29,
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
	return resp.Choices[0].Message.Content, nil
}
