package providers

import (
	"fmt"

	"github.com/assagman/apc/internal/providers/openrouter"
)

func GetClient(provider string) (Client, error) {
	switch provider {
	case "openrouter":
		return openrouter.New(), nil
	case "groq":
		return nil, nil
	default:
		return nil, fmt.Errorf("Unsupported provider: %s", provider)
	}
}
