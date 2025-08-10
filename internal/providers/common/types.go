package common

import "github.com/assagman/apc/internal/tools"

type Message struct {
	Role        string           `json:"role"`
	Content     string           `json:"content"`
	Refusal     string           `json:"refusal,omitempty"`
	Annotations []string         `json:"anotations,omitempty"`
	ToolCalls   []tools.ToolCall `json:"tool_calls,omitempty"`   // tool call request returned FROM AI
	ToolCallId  string           `json:"tool_call_id,omitempty"` // tool call request returned TO AI
	Name        string           `json:"name,omitempty"`         // tool call request returned TO AI
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Tools    []*tools.Tool `json:"tools"`
}
