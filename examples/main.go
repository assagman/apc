package main

import (
	// "context"

	"fmt"

	"github.com/assagman/apc"
	// "github.com/assagman/apc/internal/http"
)

func main() {
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	// x, err := http.New().Get(ctx, "https://postman-echo.com/get", nil)
	// if err != nil {
	// 	fmt.Printf("%v\n", err)
	// }
	// fmt.Println(string(x))

	var err error
	orClient, err := apc.New("openrouter")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	groqClient, err := apc.New("groq")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	cerebrasClient, err := apc.New("cerebras")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	openaiClient, err := apc.New("openai")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	googleClient, err := apc.New("google")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	anthropicClient, err := apc.New("anthropic")
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	var respBytes []byte
	var answer string
	respBytes, err = orClient.SendChatCompletionRequest("moonshotai/kimi-k2", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = orClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", orClient.Name(), answer)

	respBytes, err = groqClient.SendChatCompletionRequest("openai/gpt-oss-120b", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = groqClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", groqClient.Name(), answer)

	respBytes, err = cerebrasClient.SendChatCompletionRequest("gpt-oss-120b", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = cerebrasClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", cerebrasClient.Name(), answer)

	respBytes, err = openaiClient.SendChatCompletionRequest("gpt-5", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = openaiClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", openaiClient.Name(), answer)

	respBytes, err = googleClient.SendChatCompletionRequest("gemini-2.5-flash", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = googleClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", googleClient.Name(), answer)

	respBytes, err = anthropicClient.SendChatCompletionRequest("claude-opus-4-1-20250805", "user", "show CWD")
	if err != nil {
		fmt.Printf("Failed to send chat completion request: %v\n", err)
		return
	}
	answer, err = anthropicClient.ExtractAnswerFromChatCompletionResponse(respBytes)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	fmt.Printf("%s : %s\n\n", anthropicClient.Name(), answer)
}
