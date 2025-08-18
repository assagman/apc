package tools

import "encoding/json"

/* LLM PROVIDER GENERIC API TYPES */

type Function struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}
type ToolCall struct {
	Id       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Property struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	// Enum        []string `json:"enum,omitempty"`
}

type ToolFunctionParameters struct {
	Type       string              `json:"type"` // object
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  ToolFunctionParameters `json:"parameters"`
}

type Tool struct {
	Type     string             `json:"type"` // always = "function"
	Function FunctionDefinition `json:"function"`
}

/* LLM PROVIDER GENERIC API TYPES */
