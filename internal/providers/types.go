package providers

type Client interface {
	Name() string
	SendChatCompletionRequest(model string, role string, content string) ([]byte, error)
	ExtractAnswerFromChatCompletionResponse(respBytes []byte) (string, error)
}
