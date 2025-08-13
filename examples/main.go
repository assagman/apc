package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

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
		client, err := apc.New(providerName, modelName, "")
		if err != nil {
			fmt.Printf("\n%v\n", err)
			fmt.Println("---")
			continue
		}
		fmt.Printf("%s —> ", providerName)
		answer, err := client.Complete(context.TODO(), prompt)
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

//
// func TestClient(providerName string, modelName string, prompt string) {
// 	client, err := apc.New(providerName)
// 	if err != nil {
// 		fmt.Printf("\n%v\n", err)
// 		fmt.Println("---")
// 	}
// 	respBytes, err := client.SendChatCompletionRequest(modelName, "user", prompt)
// 	if err != nil {
// 		fmt.Printf("\n%v\n", err)
// 		fmt.Println("---")
// 	}
// 	answer, err := client.ExtractAnswerFromChatCompletionResponse(respBytes)
// 	if err != nil {
// 		fmt.Printf("\n%v\n", err)
// 		fmt.Println("---")
// 	}
// 	fmt.Printf("%s\n\n", answer)
// 	fmt.Println("---")
// }

func TestLoop() {
	// client, err := apc.New("google", "gemini-2.5-flash", "Always write your response in bullet list")
	// client, err := apc.New("anthropic", "claude-sonnet-4-20250514", "Always write your response in bullet list")
	// client, err := apc.New("groq", "moonshotai/kimi-k2-instruct", "Always write your response in well markdown rendered format")
	// client, err := apc.New("openai", "gpt-5", "Always write your response in bullet list")
	// client, err := apc.New("openrouter", "moonshotai/kimi-k2", "Always write your response in json format")
	client, err := apc.New("cerebras", "qwen-3-235b-a22b-instruct-2507", "Always write your response in well markdown rendered format")
	if err != nil {
		fmt.Printf("\n%v\n", err)
		fmt.Println("---")
		return
	}
	for {
		var prompt string
		fmt.Print(">> Prompt: ")
		reader := bufio.NewReader(os.Stdin)
		prompt, err := reader.ReadString('!')
		prompt = strings.TrimRight(prompt, "!")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		answer, err := client.Complete(context.TODO(), prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			fmt.Println("---")
			continue
		}
		fmt.Printf("%s\n\n", answer)
	}
}

func main() {
	fmt.Println("Starting examples main")
	if err := apc.LoadEnv(".env"); err != nil {
		fmt.Println(err.Error())
	}

	// TestAll("Why ripgrep is faster than grep")
	TestLoop()

	// client, err := apc.New("openrouter", "google/gemini-2.5-flash", "")
	// client, err := apc.New("google", "gemini-2.5-flash", "")
	// client, err := apc.New("groq", "moonshotai/kimi-k2-instruct", "")
	// client, err := apc.New("openai", "gpt-5", "")
	// client, err := apc.New("anthropic", "claude-sonnet-4-20250514", "")
	// client, err := apc.New("cerebras", "gpt-oss-120b", "")
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return
	// }
	//
	// answer, err := client.Complete(context.Background(), "who is einstein?")
	// if err != nil {
	// 	fmt.Println(err.Error())
	// }
	// println(answer)
}
