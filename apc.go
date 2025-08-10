package apc

import (
	"fmt"

	"github.com/assagman/apc/internal/environ"
	"github.com/assagman/apc/internal/providers"
	"github.com/assagman/apc/internal/providers/anthropic"
	"github.com/assagman/apc/internal/providers/cerebras"
	"github.com/assagman/apc/internal/providers/google"
	"github.com/assagman/apc/internal/providers/groq"
	"github.com/assagman/apc/internal/providers/openai"
	"github.com/assagman/apc/internal/providers/openrouter"
)

var env = environ.LoadEnv()

const (
	R_SYSTEM = iota
	R_USER
	R_ASSISTANT
	R_TOOL
)

const (
	CT_TEXT = iota
	CT_IMAGE
	CT_AUDIO
)

func New(providerName string) (providers.Client, error) {
	switch providerName {
	case "openrouter":
		return openrouter.New(), nil
	case "groq":
		return groq.New(), nil
	case "cerebras":
		return cerebras.New(), nil
	case "openai":
		return openai.New(), nil
	case "google":
		return google.New(), nil
	case "anthropic":
		return anthropic.New(), nil
	default:
		return nil, fmt.Errorf("Unsupported provider: %s", providerName)
	}
}
