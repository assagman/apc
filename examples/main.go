package main

import (
	// "context"

	"fmt"

	"github.com/assagman/apc"
	// "github.com/assagman/apc/internal/providers"
	// "github.com/assagman/apc/internal/http"
)

var providerConfig = map[string]string{
	"openrouter": "z-ai/glm-4.5",
	"groq":       "moonshotai/kimi-k2-instruct",
	"cerebras":   "gpt-oss-120b",
	"openai":     "gpt-5-nano",
	"anthropic":  "claude-sonnet-4-20250514",
	"google":     "gemini-2.5-flash",
}

func TestAll(prompt string) {
	for providerName, modelName := range providerConfig {
		fmt.Println("---")
		client, err := apc.New(providerName)
		if err != nil {
			fmt.Printf("\n%v\n", err)
			fmt.Println("---")
			continue
		}
		fmt.Printf("%s —> ", providerName)
		respBytes, err := client.SendChatCompletionRequest(modelName, "user", prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			fmt.Println("---")
			continue
		}
		answer, err := client.ExtractAnswerFromChatCompletionResponse(respBytes)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			fmt.Println("---")
			continue
		}
		fmt.Printf("%s\n\n", answer)
		fmt.Println("---")
	}
}

func main() {
	TestAll("which module contains dynamic function execution implementation in the current Go project in CWD. Be concise, return module filename only")
}
