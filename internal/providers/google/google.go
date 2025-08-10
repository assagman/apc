package google

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/assagman/apc/internal/http"
	"github.com/assagman/apc/internal/tools"
)

const ChatCompletionRequestUrlTemplate = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"

type Function struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"args,omitempty"`
}

type Part struct {
	Text         string    `json:"text,omitempty"`
	FunctionCall *Function `json:"functionCall,omitempty"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

type ChatCompletionRequest struct {
	Contents []Content `json:"contents"`
	Tools    Tools     `json:"tools"`
}

type GeminiTool tools.FunctionDefinition

type Tools struct {
	FunctionDeclarations []GeminiTool `json:"functionDeclarations"`
}

type ChatCompletionResponse struct {
	ResponseId    string        `json:"responseId"`
	ModelVersion  string        `json:"modelVersion"`
	UsageMetadata UsageMetadata `json:"usageMetadata"`
	Candidates    []Candidate   `json:"candidates"`
}

type Candidate struct {
	Content      Content `json:"content"`
	FinishReason string  `json:"finish_reason"`
	AvgLogprobs  float64 `json:"avgLogprobs"`
}

type UsageMetadata struct {
	PromptTokenCount       int              `json:"promptTokenCount"`
	CandidatesTokenCount   int              `json:"candidatesTokenCount"`
	TotalTokenCount        int              `json:"totalTokenCount"`
	PromptTokensDetails    []map[string]any `json:"promptTokensDetails"`
	CandidateTokensDetails []map[string]any `json:"CandidateTokensDetails"`
}

type Logprobs struct{}
type PromptTokensDetails struct{}

type Client struct {
	Headers        map[string]string
	ConversationId int
	MessageHistory []Content
}

func New() *Client {
	return &Client{
		Headers: map[string]string{
			"Content-Type":   "application/json",
			"X-goog-api-key": os.Getenv("GEMINI_API_KEY"),
		},
		ConversationId: 0, // not started yet
		MessageHistory: make([]Content, 0),
	}
}

func (c *Client) Name() string {
	return "google"
}

func (c *Client) SendChatCompletionRequest(model string, role string, content string) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fsTools, err := tools.GetFsTools()
	if err != nil {
		return nil, err
	}
	var gtools []GeminiTool
	for _, fsTool := range fsTools {
		gtools = append(gtools, GeminiTool(fsTool.Function))
	}

	if role == "user" {
		c.ConversationId++
		c.MessageHistory = append(c.MessageHistory, Content{
			Parts: []Part{
				{
					Text: content,
				},
			},
			Role: "user",
		})
	}
	requestBody := ChatCompletionRequest{
		Contents: c.MessageHistory,
		Tools: Tools{
			FunctionDeclarations: gtools,
		},
	}
	reqBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	// println(string(reqBodyBytes))

	httpClient := http.New()
	respBytes, err := httpClient.Post(ctx, fmt.Sprintf(ChatCompletionRequestUrlTemplate, model), c.Headers, reqBodyBytes)
	if err != nil {
		return nil, err
	}
	// println(string(respBytes))

	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	c.MessageHistory = append(c.MessageHistory, resp.Candidates[0].Content)

	if resp.Candidates[0].Content.Parts[0].FunctionCall != nil {
		// for _, toolCall := range resp.Candidates[0].Content.Parts[0].FunctionCall {
		toolCall := resp.Candidates[0].Content.Parts[0].FunctionCall
		// fmt.Printf("tool: %s\n", toolCall.Name)

		argsMap := make(map[string]any)
		if toolCall.Arguments != nil && string(toolCall.Arguments) != "{}" {
			fmt.Printf("tool: %s\n", toolCall.Arguments)
			// var argsStr string
			//
			// // Unmarshal the RawMessage into the map
			// // fmt.Println("Unmarshalling arguments RawMessages to string")
			// err := json.Unmarshal([]byte(toolCall.Arguments), &argsStr)
			// if err != nil {
			// 	fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(toolCall.Arguments))
			// 	return nil, err
			// }
			//
			// Unmarshal the string into the map
			// fmt.Println("Unmarshalling arguments strings to map")
			errr := json.Unmarshal([]byte(toolCall.Arguments), &argsMap)
			if errr != nil {
				return nil, errr
			}

		}
		toolResult, toolErr := tools.ExecTool(toolCall.Name, argsMap)
		if toolErr != nil {
			return nil, toolErr
		}
		toolMessage := Content{
			Role: "user",
			Parts: []Part{
				{
					Text: toolResult.(string),
				},
			},
		}
		c.MessageHistory = append(c.MessageHistory, toolMessage)
		return c.SendChatCompletionRequest(model, "tool", "")
	}

	/*
	   // Example response
	   x := map[string]any{
	     "candidates": [
	       {
	         "content": {
	           "parts": [
	             {
	               "text": "The command `CWD` stands for **Change Working Directory**. However, it's most commonly used in the context of:\n\n*   **FTP (File Transfer Protocol):** In an FTP session, `CWD` is a command that you send to the FTP server to change your current directory on that server.\n\n*   **Operating System Commands:**\n    *   In **Linux/macOS**, the command to display the current working directory is `pwd` (print working directory).\n    *   In **Windows**, the command to display the current working directory is `cd` (change directory) without any arguments. `cd` alone will print the current directory.\n\nI hope this clarifies your request.\n"
	             }
	           ],
	           "role": "model"
	         },
	         "finishReason": "STOP",
	         "avgLogprobs": -0.44389447063004889
	       }
	     ],
	     "usageMetadata": {
	       "promptTokenCount": 3,
	       "candidatesTokenCount": 147,
	       "totalTokenCount": 150,
	       "promptTokensDetails": [
	         {
	           "modality": "TEXT",
	           "tokenCount": 3
	         }
	       ],
	       "candidatesTokensDetails": [
	         {
	           "modality": "TEXT",
	           "tokenCount": 147
	         }
	       ]
	     },
	     "modelVersion": "gemini-2.0-flash",
	     "responseId": "GcWXaIWMC9nN1PIPiMWK0Ao"
	   }
	*/
	return respBytes, nil
}

func (c *Client) ExtractAnswerFromChatCompletionResponse(respBytes []byte) (string, error) {
	var resp ChatCompletionResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return "", err
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}
