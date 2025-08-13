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

func (apc *APC) Complete(ctx context.Context, prompt string) (string, error) {
	answer, err := apc.Provider.SendUserPrompt(ctx, prompt)
	if err != nil {
		return "", err
	}
	return answer, nil
}
