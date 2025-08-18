package apc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/assagman/apc/internal/core"
	"github.com/assagman/apc/internal/environ"
	"github.com/assagman/apc/internal/logger"

	"github.com/assagman/apc/internal/providers/anthropic"
	"github.com/assagman/apc/internal/providers/cerebras"

	"github.com/assagman/apc/internal/providers/google"
	"github.com/assagman/apc/internal/providers/groq"
	"github.com/assagman/apc/internal/providers/openai"
	"github.com/assagman/apc/internal/providers/openrouter"
	"github.com/assagman/apc/internal/tools"
)

func LoadEnv(envFile string) error {
	if err := environ.LoadEnv(envFile); err != nil {
		return err
	}
	return nil
}

// Tool Manager

type APCTools struct {
	tools []tools.Tool
}

func (t *APCTools) EnableFsTools() error {
	fsTools, err := tools.GetFsTools()
	if err != nil {
		return err
	}
	t.tools = append(t.tools, fsTools...)
	return nil
}

func (t *APCTools) RegisterTool(name string, fn any) error {
	tool, err := tools.RegisterTool(name, fn)
	if err != nil {
		return err
	}
	t.tools = append(t.tools, tool)
	return nil
}

// Tool Manager

type APC struct {
	// public
	Provider core.IProvider
	Model    string
	// private
	chanWg sync.WaitGroup
}

// create new instance of APC
//
// providerName: openrouter, groq, cerebras, openai, google, anthropic
// model: model name supported by the provider
// systemPrompt: top-level system instructions for the chat
func New(providerName string, model string, systemPrompt string, apcTools APCTools) (*APC, error) {
	var provider core.IProvider
	var err error
	switch providerName {
	case "openrouter":
		provider, err = openrouter.New(model, systemPrompt, apcTools.tools)
		if err != nil {
			return nil, err
		}
	case "groq":
		provider, err = groq.New(model, systemPrompt, apcTools.tools)
		if err != nil {
			return nil, err
		}
	case "cerebras":
		provider, err = cerebras.New(model, systemPrompt, apcTools.tools)
		if err != nil {
			return nil, err
		}
	case "openai":
		provider, err = openai.New(model, systemPrompt, apcTools.tools)
		if err != nil {
			return nil, err
		}
	case "google":
		provider, err = google.New(model, systemPrompt, apcTools.tools)
		if err != nil {
			return nil, err
		}
	case "anthropic":
		provider, err = anthropic.New(model, systemPrompt, apcTools.tools)
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

func (apc *APC) ProcessUserPrompt(userPromptChan <-chan string, msgHistoryChan chan<- []core.GenericMessage) {
	defer apc.chanWg.Done()

	for prompt := range userPromptChan {
		logger.Info("[ProcessUserPrompt] ✅ Got user prompt")
		msgHistoryChan <- []core.GenericMessage{apc.Provider.ConstructUserPromptMessage(prompt)}
	}
}

func (apc *APC) ProcessMessage(msgHistoryChan <-chan []core.GenericMessage, reqChan chan<- core.GenericRequest, errChan chan<- error) {
	defer apc.chanWg.Done()

	for messages := range msgHistoryChan {
		var err error
		for _, msg := range messages {
			err = apc.Provider.AppendMessageHistory(msg)
			if err != nil {
				break
			}

		}
		if err != nil {
			errChan <- err
			continue
		}
		isSenderRole := false
		for _, msg := range messages {
			isSenderRole, err = apc.Provider.IsSenderRole(msg)
			if err != nil {
				break
			}
		}
		if err != nil {
			errChan <- err
			continue
		}
		if isSenderRole {
			req, err := apc.Provider.NewRequest()
			if err != nil {
				errChan <- err
			}
			reqChan <- req
		}
	}
}
func (apc *APC) ProcessRequest(ctx context.Context, reqChan <-chan core.GenericRequest, respChan chan<- core.GenericResponse, errChan chan<- error) {
	defer apc.chanWg.Done()

	for req := range reqChan {
		logger.Info("[ProcessRequest] 📤 Sending request...")
		resp, err := apc.Provider.SendRequest(ctx, req)
		if err != nil {
			errChan <- err
			continue
		}
		logger.Info("[ProcessRequest] ✅ Request sent")
		respChan <- resp
	}
}

func (apc *APC) ProcessToolCall(ctx context.Context, toolCallChan <-chan []tools.ToolCall, msgHistoryChan chan<- []core.GenericMessage, errChan chan<- error) {
	defer apc.chanWg.Done()

	for toolCalls := range toolCallChan {
		tooCallCounter := 1
		toolMessages := make([]core.GenericMessage, 0)
		for _, toolCall := range toolCalls {
			logger.Info("[ProcessToolCall] ⚡ Call tool `%s` [%d/%d]", toolCall.Function.Name, tooCallCounter, len(toolCalls))
			isToolCallValid, err := apc.Provider.IsToolCallValid(toolCall)
			if err != nil {
				errChan <- err
			}
			if isToolCallValid {
				var argsStr string
				var argsMap = make(map[string]any)
				if toolCall.Function.Arguments != nil && string(toolCall.Function.Arguments) != "{}" {
					var err error
					if toolCall.Function.Arguments[0] == '"' { // string
						err = json.Unmarshal([]byte(toolCall.Function.Arguments), &argsStr)
						if err != nil {
							errChan <- fmt.Errorf("Failed to unmarshal toolCall.Function.Arguments to argStr. Value: %s\n, err: %v", string(toolCall.Function.Arguments), err)
							continue
						}
						err = json.Unmarshal([]byte(argsStr), &argsMap)
						if err != nil {
							errChan <- fmt.Errorf("Failed to unmarshal argStr to argsMap. Value: %s\n, err: %v", string(toolCall.Function.Arguments), err)
							continue
						}
					} else { // object ready
						err = json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap)
						if err != nil {
							errChan <- fmt.Errorf("Failed to unmarshal argStr to argsMap. Value: %s\n, err: %v", string(toolCall.Function.Arguments), err)
							continue
						}
					}
				}

				var toolResultStr string
				toolResult, toolErr := tools.ExecTool(toolCall.Function.Name, argsMap)
				if toolErr != nil {
					toolResultStr = toolErr.Error()
					logger.Warning("[ProcessToolCall] Tool `%s` returned err: %s", toolCall.Function.Name, toolResultStr)
				} else {
					var ok bool
					toolResultStr, ok = toolResult.(string)
					if !ok {
						errChan <- fmt.Errorf("Failed to cast toolResult to string")
						continue
					}
					logger.Info("[ProcessToolCall] ✅ Tool call successful `%s` [%d/%d]", toolCall.Function.Name, tooCallCounter, len(toolCalls))
				}

				toolMsg := apc.Provider.ConstructToolMessage(toolCall, toolResultStr)
				toolMessages = append(toolMessages, toolMsg)
			}
			tooCallCounter += 1
		}
		if len(toolMessages) > 0 {
			msgHistoryChan <- toolMessages
		} else {
			logger.Warning("\t\t\t[ProcessToolCall] ⚠️ No tool message constructed")
		}
	}
}

func (apc *APC) ProcessResponse(ctx context.Context, respChan <-chan core.GenericResponse, msgHistoryChan chan<- []core.GenericMessage, toolCallChan chan<- []tools.ToolCall, errChan chan<- error, outChan chan<- string) {
	defer apc.chanWg.Done()

	for resp := range respChan {
		logger.Info("[ProcessResponse] 📦 Got response")
		msg, err := apc.Provider.GetMessageFromResponse(resp)
		if err != nil {
			errChan <- err
		}
		msgHistoryChan <- []core.GenericMessage{msg}

		isToolCall, err := apc.Provider.IsToolCall(resp)
		if err != nil {
			errChan <- err
			continue
		}
		if !isToolCall {
			answer, err := apc.Provider.GetAnswerFromResponse(resp)
			if err != nil {
				errChan <- err
				continue
			}
			outChan <- answer
		} else {
			toolCalls, err := apc.Provider.GetToolCallsFromResponse(resp)
			if err != nil {
				errChan <- err
			}
			toolCallChan <- toolCalls
		}
		logger.Info("[ProcessResponse] ✅ Response processed successfully")
	}
}

func (apc *APC) Complete(ctx context.Context, userPrompt string) (string, error) {
	userPromptChan := make(chan string, 1)
	toolCallChan := make(chan []tools.ToolCall, 1)
	msgHistoryChan := make(chan []core.GenericMessage, 1)
	reqChan := make(chan core.GenericRequest, 1)
	respChan := make(chan core.GenericResponse, 1)
	outChan := make(chan string, 1)
	errChan := make(chan error, 1)

	apc.chanWg.Add(6)
	go apc.ProcessUserPrompt(userPromptChan, msgHistoryChan)
	go apc.ProcessMessage(msgHistoryChan, reqChan, errChan)
	go apc.ProcessRequest(ctx, reqChan, respChan, errChan)
	go apc.ProcessToolCall(ctx, toolCallChan, msgHistoryChan, errChan)
	go apc.ProcessResponse(ctx, respChan, msgHistoryChan, toolCallChan, errChan, outChan)

	var answer string
	var err error
	go func() {
		defer apc.chanWg.Done()

		for e := range errChan {
			fmt.Printf("%v", e)
			err = e
			close(userPromptChan)
			close(msgHistoryChan)
			close(reqChan)
			close(respChan)
			close(toolCallChan)
			close(outChan)
			break
		}
	}()

	userPromptChan <- userPrompt
	answer = <-outChan
	close(errChan)

	if err != nil {
		logger.PrintV(apc.Provider.GetMessageHistory())
		return "", err
	}

	close(userPromptChan)
	close(msgHistoryChan)
	close(reqChan)
	close(respChan)
	close(toolCallChan)
	close(outChan)

	// fmt.Printf("%+v", apc.waitGroup)
	apc.chanWg.Wait()
	return answer, nil
}
