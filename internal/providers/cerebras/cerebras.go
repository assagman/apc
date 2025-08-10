package cerebras

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/providers/common"
	"github.com/assagman/apc/internal/tools"
)

const ChatCompletionRequestUrl = "https://api.cerebras.ai/v1/chat/completions"

type TimeInfo struct {
	QueueTime      float64 `json:"queue_time"`
	PromptTime     float64 `json:"prompt_time"`
	CompletionTime float64 `json:"completion_time"`
	TotalTime      float64 `json:"total_time"`
	Created        int64   `json:"created"`
}

type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

type ChatCompletionResponseMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Reasoning string           `json:"reasoning"`
	ToolCalls []tools.ToolCall `json:"tool_calls"`
}

type ChatCompletionResponseChoice struct {
	Index        int            `json:"index"`
	Message      common.Message `json:"message"`
	LogProbs     any            `json:"logprobs"`
	FinishReason string         `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	Id                string                         `json:"id"`
	Object            string                         `json:"object"`
	Created           int64                          `json:"created"`
	Model             string                         `json:"model"`
	Choices           []ChatCompletionResponseChoice `json:"choices"`
	Usage             Usage                          `json:"usage"`
	TimeInfo          TimeInfo                       `json:"time_info"`
	SystemFingerprint string                         `json:"system_fingerprint"`
	ServiceTier       string                         `json:"service_tier"`
}

type Client struct {
	Headers        map[string]string
	ConversationId int
	MessageHistory []common.Message
}

func New() *Client {
	return &Client{
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer " + os.Getenv("CEREBRAS_API_KEY"),
		},
		ConversationId: 0, // not started yet
		MessageHistory: make([]common.Message, 0),
	}
}

func (c *Client) Name() string {
	return "cerebras"
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
		}
		return c.SendChatCompletionRequest(model, "tool", "")
	}
	/*
	   // Example response:
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
