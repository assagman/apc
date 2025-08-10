package apc

import (
	"fmt"

	"github.com/assagman/apc/internal/environ"
	"github.com/assagman/apc/internal/providers"
	"github.com/assagman/apc/internal/providers/anthropic"
	"github.com/assagman/apc/internal/providers/cerebras"
	"github.com/assagman/apc/internal/providers/google"
	"github.com/assagman/apc/internal/providers/groq"
	"github.com/assagman/apc/internal/providers/openai"
	"github.com/assagman/apc/internal/providers/openrouter"
)

//	type AudioData struct {
//		Data   string `json:"data"`   // Base64 encoded audio data
//		Format string `json:"format"` // Audio format, e.g., "wav"
//	}
//
//	type ImageUrl struct {
//		Url string `json:"url"`
//	}
//
//	type RequestContent struct {
//		Type       string     `json:"type"`                  // "text" "image_url" "input_audio"
//		Text       string     `json:"text,omitempty"`        // Text content (if type is "text")
//		ImageUrl   *ImageUrl  `json:"image_url,omitempty"`   // Image url string (if type is "image_url")
//		InputAudio *AudioData `json:"input_audio,omitempty"` // Audio data (if type is "input_audio")
//	}
//
// type ResponseContent struct {
// }
//
//	type RequestMessage struct {
//		Role    string           `json:"role"`
//		Content []RequestContent `json:"content"`
//	}
//
//	type ResponseMessage struct {
//		Role    string `json:"role"`
//		Content string `json:"content"`
//		// ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`   // tool call request returned FROM AI
//		// ToolCallId string           `json:"tool_call_id,omitempty"` // tool call request returned TO AI
//		// Name       string           `json:"name,omitempty"`         // tool call request returned TO AI
//	}
//
// // ================================================================
// type openrouterChatCompletionResponse struct {
// 	ID       string                            `json:"id"`
// 	Provider string                            `json:"provider"`
// 	Model    string                            `json:"model"`
// 	Object   string                            `json:"object"` // "chat.completion"
// 	Created  int64                             `json:"created"`
// 	Choices  []openrouterChatCompletionChoice  `json:"choices"`
// 	Usage    openrouterChatCompletionUsageInfo `json:"usage"`
// }
//
// type openrouterChatCompletionChoice struct {
// 	Index              int       `json:"index"`
// 	Message            Message   `json:"message"`
// 	FinishReason       string    `json:"finish_reason"`
// 	NativeFinishReason string    `json:"native_finish_reason"`
// 	Logprobs           *Logprobs `json:"logprobs"`
// }
//
// type openrouterChatCompletionUsageInfo struct {
// 	PromptTokens        int       `json:"prompt_tokens"`
// 	CompletionTokens    int       `json:"completion_tokens"`
// 	TotalTokens         int       `json:"total_tokens"`
// 	PromptTokensDetails *struct{} `json:"prompt_tokens_details"` // null for now
// }

// type Logprobs struct{}
// type PromptTokensDetails struct{}

// ================================================================

// type Client struct {
// 	providerName    string
// 	modelName       string
// 	httpClient      http.BaseHttpClient
// 	requestHistory  [][]byte
// 	responseHistory [][]byte
// }

var env = environ.LoadEnv()

const (
	R_SYSTEM = iota
	R_USER
	R_ASSISTANT
	R_TOOL
)

const (
	CT_TEXT = iota
	CT_IMAGE
	CT_AUDIO
)

func New(providerName string) (providers.Client, error) {
	switch providerName {
	case "openrouter":
		return openrouter.New(), nil
	case "groq":
		return groq.New(), nil
	case "cerebras":
		return cerebras.New(), nil
	case "openai":
		return openai.New(), nil
	case "google":
		return google.New(), nil
	case "anthropic":
		return anthropic.New(), nil
	default:
		return nil, fmt.Errorf("Unsupported provider: %s", providerName)
	}
	//    var provider Provider
	//    switch providerName {
	//    case "openrouter":
	//
	//    }
	// return &Client{
	// 	provider:   ,
	// 	modelName:  modelName,
	// 	httpClient: *http.New(),
	// }
}

// func (apc *Client) GetProviderName() string {
// 	return apc.providerName
// }
//
// func (apc *Client) SetSystemPrompt(prompt string) {
// 	apc.messageHistory = append(apc.messageHistory, Message{
// 		Role: "system",
// 		Content: []Content{
// 			{
// 				Type: "text",
// 				Text: prompt,
// 			},
// 		},
// 	})
// }
//
// func (apc *Client) SendPrompt(prompt string) (string, error) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	apc.messageHistory = append(apc.messageHistory, Message{
// 		Role: "user",
// 		Content: []Content{
// 			{
// 				Type: "text",
// 				Text: prompt,
// 			},
// 		},
// 	})
//
// 	reqBody := map[string]any{
// 		"model":    apc.modelName,
// 		"messages": apc.messageHistory,
// 	}
//
// 	reqBodyBytes, err := json.Marshal(reqBody)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	respBytes, err := apc.httpClient.Post(ctx, "https://openrouter.ai/api/v1/chat/completions", nil, reqBodyBytes)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	switch apc.providerName {
// 	case "openrouter":
// 		var resp openrouterChatCompletionResponse
// 		if err := json.Unmarshal(reqBodyBytes, &resp); err != nil {
// 			return "", err
// 		}
// 		return resp.Choices[0].Message.Content, nil
// 	}
// 	return "", nil
// }
