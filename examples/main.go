package main

import (
	"fmt"

	"github.com/assagman/apc"
)

var providerConfig = map[string]string{
	"openrouter": "z-ai/glm-4.5",
	"groq":       "moonshotai/kimi-k2-instruct",
	"cerebras":   "gpt-oss-120b",
	"openai":     "gpt-5",
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

func TestClient(providerName string, modelName string, prompt string) {
	client, err := apc.New(providerName)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		fmt.Println("---")
	}
	respBytes, err := client.SendChatCompletionRequest(modelName, "user", prompt)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		fmt.Println("---")
	}
	answer, err := client.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		fmt.Println("---")
	}
	fmt.Printf("%s\n\n", answer)
	fmt.Println("---")
}

func main() {
	// TestAll("which module contains dynamic function execution implementation in the current Go project in CWD. Be concise, return module filename only")
	TestClient("google", "gemini-2.5-pro", "what is the purpose of generic in golang and howw to use them? be concise, provide 4 exxample.")
}
