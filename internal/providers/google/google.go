package google

import (
	// "context"
	// "fmt"
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
	Tools             *Tools             `json:"tools"`
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

func New(model string, systemPrompt string) (core.IProvider, error) {
	CheckModelName(model)
	return &Provider{
		Name:         "google",
		Endpoint:     fmt.Sprintf(chatCompletionRequestUrlTemplate, model),
		Model:        model,
		SystemPrompt: systemPrompt,
		History:      make([]Content, 0),
	}, nil
}

func (p *Provider) GetApiKey() string { return os.Getenv("OPENROUTER_API_KEY") }
func (p *Provider) GetEndpoint() string {
	return fmt.Sprintf(chatCompletionRequestUrlTemplate, p.Model)
}
func (p *Provider) GetHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": p.GetApiKey(),
	}
}

func (p *Provider) GetSystemPrompt() *SystemInstruction {
	return &SystemInstruction{
		Parts: []Part{
			{
				Text: p.SystemPrompt,
			},
		},
	}
}

func (p *Provider) GetTools() *Tools {
	fsTools, err := tools.GetFsTools()
	if err != nil {
		logger.Warning("Failed to get fs tools")
	}
	tools := &Tools{}
	for _, fsTool := range fsTools {
		tools.FunctionDeclarations = append(tools.FunctionDeclarations, Tool(fsTool.Function))
	}
	return tools
}

func (p *Provider) ConstructContentFomUserPrompt(prompt string) Content {
	return Content{
		Role: roleUser,
		Parts: []Part{
			{
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
	p.History = append(p.History, resp.Candidates[0].Content)
	return &resp, nil
}

func (p *Provider) HandleToolCalls(ctx context.Context, resp Response) (*Response, error) {
	if resp.Candidates[0].FinishReason == finishReasonStop {
		// means everything is OK

		if resp.Candidates[0].Content.Parts[0].FunctionCall != nil {
			var err error
			finalResp, err := p.SendToolResult(ctx, *resp.Candidates[0].Content.Parts[0].FunctionCall)
			if err != nil {
				return nil, err
			}
			p.History = append(p.History, finalResp.Candidates[0].Content)
			return finalResp, nil
		}
	}
	return &resp, nil
}

func (p *Provider) SendUserPrompt(ctx context.Context, userPrompt string) (string, error) {

	// ======================================================== Request Construction
	p.History = append(p.History, p.ConstructContentFomUserPrompt(userPrompt))
	req := Request{
		SystemInstruction: p.GetSystemPrompt(),
		Tools:             p.GetTools(),
		Contents:          p.History,
	}

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	// ======================================================== Check finish/stop reason for tool call
	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return "", nil
	}

	answer := finalResp.Candidates[0].Content.Parts[0].Text
	return answer, nil
}

func (p *Provider) SendToolResult(ctx context.Context, f FunctionCall) (*Response, error) {
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

	// ======================================================== Send result to the model

	// ======================================================== TOOLS
	fsTools, err := tools.GetFsTools()
	if err != nil {
		logger.Warning("Failed to get fs tools")
	}
	var tools Tools
	for _, fsTool := range fsTools {
		tools.FunctionDeclarations = append(tools.FunctionDeclarations, Tool(fsTool.Function))
	}
	// ======================================================== TOOLS

	// ======================================================== CONTENT
	part := Part{
		FunctionResponse: &FunctionResponse{
			Id:   f.Id,
			Name: f.Name,
			Response: map[string]any{
				"result": toolResult,
			},
		},
	}
	content := Content{
		Role:  roleUser,
		Parts: []Part{part},
	}
	// ======================================================== CONTENT

	// ======================================================== Request Construction
	p.History = append(p.History, content)
	req := Request{
		SystemInstruction: p.GetSystemPrompt(),
		Tools:             p.GetTools(),
		Contents:          p.History,
	}
	// ======================================================== Request Construction

	// ======================================================== Request Conversion to bytes
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// ======================================================== Request Conversion to bytes

	// ======================================================== REST API CALL
	c := http.New()
	respBytes, err := c.Post(ctx, p.Endpoint, p.GetHeaders(), reqBytes)
	if err != nil {
		return nil, err
	}
	// ======================================================== REST API CALL

	// ======================================================== Response conversion from bytes
	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}
	// ======================================================== Response conversion from bytes

	// ======================================================== Check finish/stop reason for tool call
	if resp.Candidates[0].FinishReason == finishReasonStop {
		// means everything is OK

		if resp.Candidates[0].Content.Parts[0].FunctionCall != nil {
			return p.SendToolResult(ctx, *resp.Candidates[0].Content.Parts[0].FunctionCall)
		}
		return &resp, err
	}
	// ======================================================== Check finish/stop reason for tool call

	// ======================================================== Send result to the model
	return nil, fmt.Errorf("[SendToolResult] Unexpected finish reason: %s", resp.Candidates[0].FinishReason)
}

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"os"
//
// 	"github.com/assagman/apc/internal/http"
// 	"github.com/assagman/apc/internal/tools"
// )
//
// const ChatCompletionRequestUrlTemplate = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
//
// type Function struct {
// 	Name      string          `json:"name"`
// 	Arguments json.RawMessage `json:"args,omitempty"`
// }
//
// type Part struct {
// 	Text         string    `json:"text,omitempty"`
// 	FunctionCall *Function `json:"functionCall,omitempty"`
// }
//
// type Content struct {
// 	Parts []Part `json:"parts"`
// 	Role  string `json:"role"`
// }
//
// type ChatCompletionRequest struct {
// 	Contents []Content `json:"contents"`
// 	Tools    Tools     `json:"tools"`
// }
//
// type GeminiTool tools.FunctionDefinition
//
// type Tools struct {
// 	FunctionDeclarations []GeminiTool `json:"functionDeclarations"`
// }
//
// type ChatCompletionResponse struct {
// 	ResponseId    string        `json:"responseId"`
// 	ModelVersion  string        `json:"modelVersion"`
// 	UsageMetadata UsageMetadata `json:"usageMetadata"`
// 	Candidates    []Candidate   `json:"candidates"`
// }
//
// type Candidate struct {
// 	Content      Content `json:"content"`
// 	FinishReason string  `json:"finish_reason"`
// 	AvgLogprobs  float64 `json:"avgLogprobs"`
// }
//
// type UsageMetadata struct {
// 	PromptTokenCount       int              `json:"promptTokenCount"`
// 	CandidatesTokenCount   int              `json:"candidatesTokenCount"`
// 	TotalTokenCount        int              `json:"totalTokenCount"`
// 	PromptTokensDetails    []map[string]any `json:"promptTokensDetails"`
// 	CandidateTokensDetails []map[string]any `json:"CandidateTokensDetails"`
// }
//
// type Logprobs struct{}
// type PromptTokensDetails struct{}
//
// type Client struct {
// 	Headers        map[string]string
// 	ConversationId int
// 	MessageHistory []Content
// }
//
// func New() *Client {
// 	return &Client{
// 		Headers: map[string]string{
// 			"Content-Type":   "application/json",
// 			"X-goog-api-key": os.Getenv("GEMINI_API_KEY"),
// 		},
// 		ConversationId: 0, // not started yet
// 		MessageHistory: make([]Content, 0),
// 	}
// }
//
// func (c *Client) Name() string {
// 	return "google"
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
// 	var gtools []GeminiTool
// 	for _, fsTool := range fsTools {
// 		gtools = append(gtools, GeminiTool(fsTool.Function))
// 	}
//
// 	if role == "user" {
// 		c.ConversationId++
// 		c.MessageHistory = append(c.MessageHistory, Content{
// 			Parts: []Part{
// 				{
// 					Text: content,
// 				},
// 			},
// 			Role: "user",
// 		})
// 	}
// 	requestBody := ChatCompletionRequest{
// 		Contents: c.MessageHistory,
// 		Tools: Tools{
// 			FunctionDeclarations: gtools,
// 		},
// 	}
// 	reqBodyBytes, err := json.Marshal(requestBody)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// println(string(reqBodyBytes))
//
// 	httpClient := http.New()
// 	respBytes, err := httpClient.Post(ctx, fmt.Sprintf(ChatCompletionRequestUrlTemplate, model), c.Headers, reqBodyBytes)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// println(string(respBytes))
//
// 	var resp ChatCompletionResponse
// 	if err := json.Unmarshal(respBytes, &resp); err != nil {
// 		return nil, err
// 	}
// 	c.MessageHistory = append(c.MessageHistory, resp.Candidates[0].Content)
//
// 	if resp.Candidates[0].Content.Parts[0].FunctionCall != nil {
// 		// for _, toolCall := range resp.Candidates[0].Content.Parts[0].FunctionCall {
// 		toolCall := resp.Candidates[0].Content.Parts[0].FunctionCall
// 		// fmt.Printf("tool: %s\n", toolCall.Name)
//
// 		argsMap := make(map[string]any)
// 		if toolCall.Arguments != nil && string(toolCall.Arguments) != "{}" {
// 			fmt.Printf("tool: %s\n", toolCall.Arguments)
// 			// var argsStr string
// 			//
// 			// // Unmarshal the RawMessage into the map
// 			// // fmt.Println("Unmarshalling arguments RawMessages to string")
// 			// err := json.Unmarshal([]byte(toolCall.Arguments), &argsStr)
// 			// if err != nil {
// 			// 	fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(toolCall.Arguments))
// 			// 	return nil, err
// 			// }
// 			//
// 			// Unmarshal the string into the map
// 			// fmt.Println("Unmarshalling arguments strings to map")
// 			errr := json.Unmarshal([]byte(toolCall.Arguments), &argsMap)
// 			if errr != nil {
// 				return nil, errr
// 			}
//
// 		}
// 		toolResult, toolErr := tools.ExecTool(toolCall.Name, argsMap)
// 		if toolErr != nil {
// 			return nil, toolErr
// 		}
// 		toolMessage := Content{
// 			Role: "user",
// 			Parts: []Part{
// 				{
// 					Text: toolResult.(string),
// 				},
// 			},
// 		}
// 		c.MessageHistory = append(c.MessageHistory, toolMessage)
// 		return c.SendChatCompletionRequest(model, "tool", "")
// 	}
//
// 	/*
// 	   // Example response
// 	   x := map[string]any{
// 	     "candidates": [
// 	       {
// 	         "content": {
// 	           "parts": [
// 	             {
// 	               "text": "The command `CWD` stands for **Change Working Directory**. However, it's most commonly used in the context of:\n\n*   **FTP (File Transfer Protocol):** In an FTP session, `CWD` is a command that you send to the FTP server to change your current directory on that server.\n\n*   **Operating System Commands:**\n    *   In **Linux/macOS**, the command to display the current working directory is `pwd` (print working directory).\n    *   In **Windows**, the command to display the current working directory is `cd` (change directory) without any arguments. `cd` alone will print the current directory.\n\nI hope this clarifies your request.\n"
// 	             }
// 	           ],
// 	           "role": "model"
// 	         },
// 	         "finishReason": "STOP",
// 	         "avgLogprobs": -0.44389447063004889
// 	       }
// 	     ],
// 	     "usageMetadata": {
// 	       "promptTokenCount": 3,
// 	       "candidatesTokenCount": 147,
// 	       "totalTokenCount": 150,
// 	       "promptTokensDetails": [
// 	         {
// 	           "modality": "TEXT",
// 	           "tokenCount": 3
// 	         }
// 	       ],
// 	       "candidatesTokensDetails": [
// 	         {
// 	           "modality": "TEXT",
// 	           "tokenCount": 147
// 	         }
// 	       ]
// 	     },
// 	     "modelVersion": "gemini-2.0-flash",
// 	     "responseId": "GcWXaIWMC9nN1PIPiMWK0Ao"
// 	   }
// 	*/
// 	return respBytes, nil
// }
//
// func (c *Client) ExtractAnswerFromChatCompletionResponse(respBytes []byte) (string, error) {
// 	var resp ChatCompletionResponse
// 	if err := json.Unmarshal(respBytes, &resp); err != nil {
// 		return "", err
// 	}
// 	var answer string
// 	for _, part := range resp.Candidates[0].Content.Parts {
// 		if part.Text != "" {
// 			answer += part.Text
// 		}
// 	}
// 	return answer, nil
// }
