package ai

import (
	"fmt"

	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/executor"
)

// Provider defines the interface for AI action generation
type Provider interface {
	GenerateActions(pageMap *crawler.PageMap, prompt string) ([]executor.Action, error)
}

// NewProvider creates a new AI provider based on the provider name
func NewProvider(name, model string) (Provider, error) {
	switch name {
	case "claude", "anthropic":
		return NewClaudeProvider(model)
	case "openai", "gpt":
		return NewOpenAIProvider(model)
	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: claude, openai)", name)
	}
}
