package apc

import (
	"context"
	"fmt"

	"github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/internal/environ"
	"github.com/assagman/apc/internal/providers/anthropic"
	"github.com/assagman/apc/internal/providers/cerebras"
	"github.com/assagman/apc/internal/providers/google"
	"github.com/assagman/apc/internal/providers/groq"
	"github.com/assagman/apc/internal/providers/openai"
	"github.com/assagman/apc/internal/providers/openrouter"
)

func LoadEnv(envFile string) error {
	if err := environ.LoadEnv(envFile); err != nil {
		return err
	}
	return nil
}

type APC struct {
	Provider core.IProvider
	Model    string
}

func New(providerName string, model string, systemPrompt string) (*APC, error) {
	var provider core.IProvider
	var err error
	switch providerName {
	case "openrouter":
		provider, err = openrouter.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	case "groq":
		provider, err = groq.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	case "cerebras":
		provider, err = cerebras.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	case "openai":
		provider, err = openai.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	case "google":
		provider, err = google.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	case "anthropic":
		provider, err = anthropic.New(model, systemPrompt)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unsupported provider: %s", providerName)
	}
	apc := APC{
		Provider: provider,
		Model:    model,
	}

	return &apc, nil
}

// func (apc *APC) SetSystemPrompt(prompt string) {
// 	apc.SystemPrompt = prompt
//
// 	if apc.Provider.GetName() != "google" && apc.Provider.GetName() != "anthropic" {
// 		for _, msg := range apc.MessageHistory {
// 			println(msg.Role)
// 		}
//
// 		apc.AppendMessageHistory([]core.Message{{Role: "system", Content: prompt}})
// 	}
// }

func (apc *APC) Complete(ctx context.Context, prompt string) (string, error) {
	answer, err := apc.Provider.SendUserPrompt(ctx, prompt)
	if err != nil {
		return "", err
	}
	return answer, nil

	// if prompt != "<apctoolcompletion>" {
	// 	apc.AppendMessageHistory([]core.Message{{Role: "user", Content: prompt}})
	// }
	//
	// println(prompt)
	// println("= = = = = = = = = = = = = = = = = = = = = =")
	// reqBytes, err := apc.Provider.GetRequestBody(apc.MessageHistory)
	// if err != nil {
	// 	return "", err
	// }
	// println(string(reqBytes))
	// println("= = = = = = = = = = = = = = = = = = = = = =")
	//
	// httpClient := http.New()
	// respBytes, err := httpClient.Post(ctx, apc.Provider.GetEndpoint(), apc.Provider.GetHeaders(), reqBytes)
	// if err != nil {
	// 	return "", err
	// }
	// println(string(respBytes))
	// println("= = = = = = = = = = = = = = = = = = = = = =")
	//
	// answer, err := apc.Provider.ExtractModelAnswer(respBytes)
	// if err != nil {
	// 	return "", nil
	// }
	// var role string
	// if apc.Provider.GetName() == "google" {
	// 	role = "model"
	// } else {
	// 	role = "assistant"
	// }
	//
	// isToolCall, err := apc.Provider.IsToolCall(respBytes)
	// if err != nil {
	// 	return "", err
	// }
	// if isToolCall {
	// 	var toolMessages []core.Message
	// 	toolCall, err := apc.Provider.GetToolCall(respBytes)
	// 	if err != nil {
	// 		return "", err
	// 	}
	//
	// 	if toolCall != nil {
	// 		fmt.Printf("tool: %s\n", toolCall.Function.Name)
	//
	// 		var argsMap = make(map[string]any)
	// 		if toolCall.Function.Arguments != nil && string(toolCall.Function.Arguments) != "{}" {
	// 			var argsStr string
	//
	// 			// Unmarshal the RawMessage into the map
	// 			// fmt.Println("Unmarshalling arguments RawMessages to string")
	// 			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsStr)
	// 			if err != nil {
	// 				fmt.Printf("Failed to unmarshal toolCall.Function.Arguments. Value: %s\n", string(toolCall.Function.Arguments))
	// 				return "", err
	// 			}
	//
	// 			// Unmarshal the string into the map
	// 			// fmt.Println("Unmarshalling arguments strings to map")
	// 			errr := json.Unmarshal([]byte(argsStr), &argsMap)
	// 			if errr != nil {
	// 				return "", errr
	// 			}
	// 		}
	//
	// 		toolResult, toolErr := tools.ExecTool(toolCall.Function.Name, argsMap)
	// 		if toolErr != nil {
	// 			return "", toolErr
	// 		}
	//
	// 		var role string
	// 		if apc.Provider.GetName() == "google" || apc.Provider.GetName() == "anthropic" {
	// 			role = "user"
	// 		} else {
	// 			role = "tool"
	// 		}
	// 		toolMessages = append(toolMessages, core.Message{
	// 			Role:       role,
	// 			Content:    toolResult.(string),
	// 			ToolName:   toolCall.Function.Name,
	// 			ToolCallId: toolCall.Id,
	// 		})
	// 	}
	// 	apc.AppendMessageHistory(toolMessages)
	// 	return apc.Complete(ctx, "<apctoolcompletion>")
	// } else {
	// 	apc.AppendMessageHistory([]core.Message{{Role: role, Content: answer}})
	// }
	//
	// return apc.Provider.ExtractModelAnswer(respBytes)
}
