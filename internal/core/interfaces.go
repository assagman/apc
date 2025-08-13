package core

import (
	"context"
)

type IProvider interface {
	GetApiKey() string
	GetEndpoint() string
	GetHeaders() map[string]string
	SendUserPrompt(ctx context.Context, userPrompt string) (string, error)
}
