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
	"openrouter": "qwen/qwen3-coder",
	"groq":       "moonshotai/kimi-k2-instruct",
	"cerebras":   "gpt-oss-120b",
	"openai":     "gpt-5",
	"anthropic":  "claude-sonnet-4-20250514",
	"google":     "gemini-2.5-flash",
}

func TestAll(prompt string) {
	for providerName, modelName := range providerConfig {
		fmt.Println("---")
		client, err := apc.New(providerName, modelName, "Always response in json format", apc.APCTools{})
		if err != nil {
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s —> \n", providerName)
		answer, err := client.Complete(context.TODO(), prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s\n\n", answer)
	}
}

func TestAllGetName() {
	prompt := "get my name"
	apcTools := apc.APCTools{}
	err := apcTools.RegisterTool("ToolGetMyName", ToolGetMyName)
	if err != nil {
		fmt.Println(err)
		return
	}
	for providerName, modelName := range providerConfig {
		fmt.Println("---")
		client, err := apc.New(providerName, modelName, "Always response in json format", apcTools)
		if err != nil {
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s —> \n", providerName)
		answer, err := client.Complete(context.TODO(), prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s\n\n", answer)
	}
}

func TestAllGetCWD() {
	prompt := "get cwd"
	apcTools := apc.APCTools{}
	err := apcTools.EnableFsTools()
	if err != nil {
		fmt.Println(err)
		return
	}
	for providerName, modelName := range providerConfig {
		fmt.Println("---")
		client, err := apc.New(providerName, modelName, "Always response in json format", apcTools)
		if err != nil {
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s —> \n", providerName)
		answer, err := client.Complete(context.TODO(), prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("%s\n\n", answer)
	}
}

func ToolGetMyName() (string, error) {
	return "sercan", nil
}

func TestLoop(providerName string, modelName string) {
	apcTools := apc.APCTools{}
	err := apcTools.RegisterTool("ToolGetMyName", ToolGetMyName)
	if err != nil {
		fmt.Println(err)
		return
	}
	client, err := apc.New(providerName, modelName, "Always write your response in bullet list", apcTools)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		return
	}
	for {
		var prompt string
		fmt.Print(">> Prompt: ")
		reader := bufio.NewReader(os.Stdin)
		prompt, err := reader.ReadString('±')
		prompt = strings.TrimRight(prompt, "±")
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		answer, err := client.Complete(context.TODO(), prompt)
		if err != nil {
			fmt.Printf("failed:\n\n")
			fmt.Printf("\n%v\n", err)
			continue
		}
		fmt.Printf("[AI]:\n%s\n\n", answer)
	}
}

func TestEnablingTool(providerName string, modelName string, prompt string) {
	// fmt.Println("Session WITHOUT tools")
	// client, err := apc.New(providerName, modelName, "Always write your response in bullet list")
	// if err != nil {
	// 	fmt.Printf("\n%v\n", err)
	// 	return
	// }
	// answer, err := client.Complete(context.TODO(), prompt)
	// if err != nil {
	// 	fmt.Printf("failed:\n\n")
	// 	fmt.Printf("\n%v\n", err)
	// }
	// fmt.Printf("[AI]:\n%s\n\n", answer)
	// fmt.Println("[END]")
	//
	// fmt.Println("Session WITH tools")
	// if err := client.EnableFsTools(); err != nil {
	// 	fmt.Printf("[WARNING] Failed to enable fs tools. Error: %v", err)
	// 	return
	// }
	// answer, err = client.Complete(context.TODO(), prompt)
	// if err != nil {
	// 	fmt.Printf("failed:\n\n")
	// 	fmt.Printf("\n%v\n", err)
	// }
	// fmt.Printf("[AI]:\n%s\n\n", answer)
	// fmt.Println("[END]")
	//
	// fmt.Println("Session WITHOUT tools")
	// client.DisableFsTools()
	// answer, err = client.Complete(context.TODO(), prompt)
	// if err != nil {
	// 	fmt.Printf("failed:\n\n")
	// 	fmt.Printf("\n%v\n", err)
	// }
	// fmt.Printf("[AI]:\n%s\n\n", answer)
	// fmt.Println("[END]")
}

func main() {
	fmt.Println("Starting examples main")
	if err := apc.LoadEnv(".env"); err != nil {
		fmt.Println(err.Error())
		return
	}

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// TestLoop("openai", "gpt-4o")
	// TestLoop("groq", "moonshotai/kimi-k2-instruct")
	// TestLoop("cerebras", "gpt-oss-120b")
	// TestLoop("openrouter", "openai/gpt-4o")
	// TestLoop("anthropic", "claude-sonnet-4-20250514")
	// TestLoop("google", "gemini-2.5-flash")

	// TestAll("review fs.go module in tools package of the Golang project in CWD")
	// TestAll("Get cwd")
	// TestAll("Find the file containing IProvider definition and provide all functions of it")
	// TestAllGetName()
	TestAllGetCWD()

	// TestEnablingTool("google", "gemini-2.5-flash", "get cwd")
	// TestEnablingTool("anthropic", "claude-sonnet-4-20250514", "get cwd")

}
