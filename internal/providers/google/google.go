package google

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

	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return "", nil
	}
	// logger.PrintV(p.History)

	var answer = ""
	for _, part := range finalResp.Candidates[0].Content.Parts {
		answer += part.Text
	}

	return answer, nil
}

func (p *Provider) SendToolResult(ctx context.Context, f FunctionCall) (*Response, error) {
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

	p.History = append(p.History, content)

	req := Request{
		SystemInstruction: p.GetSystemPrompt(),
		Tools:             p.GetTools(),
		Contents:          p.History,
	}

	resp, err := p.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	finalResp, err := p.HandleToolCalls(ctx, *resp)
	if err != nil {
		return nil, err
	}

	return finalResp, nil
}
