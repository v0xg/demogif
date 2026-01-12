package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"
	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/executor"
)

// OpenAIProvider implements the Provider interface using OpenAI
type OpenAIProvider struct {
	client *openai.Client
	model  string
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(model string) (*OpenAIProvider, error) {
	apiKey := os.Getenv("DEMOGIF_OPENAI_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("DEMOGIF_OPENAI_KEY or OPENAI_API_KEY environment variable required")
	}

	client := openai.NewClient(apiKey)

	if model == "" {
		model = "gpt-4o"
	}

	return &OpenAIProvider{
		client: client,
		model:  model,
	}, nil
}

// GenerateActions generates browser actions from the page map and user prompt
func (p *OpenAIProvider) GenerateActions(pageMap *crawler.PageMap, prompt string) ([]executor.Action, error) {
	pageMapJSON, err := json.MarshalIndent(pageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page map: %w", err)
	}

	userPrompt := buildUserPrompt(string(pageMapJSON), prompt)

	resp, err := p.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: p.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			MaxTokens: 1024,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	responseText := resp.Choices[0].Message.Content

	// Parse JSON response (extract JSON array if surrounded by text)
	actions, err := parseActionsJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response as JSON: %w\nResponse: %s", err, responseText)
	}

	return actions, nil
}

// ContinueActions generates the next batch of actions after a checkpoint
func (p *OpenAIProvider) ContinueActions(pageMap *crawler.PageMap, originalPrompt string, completedActions string) ([]executor.Action, error) {
	pageMapJSON, err := json.MarshalIndent(pageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page map: %w", err)
	}

	userPrompt := buildContinuePrompt(string(pageMapJSON), originalPrompt, completedActions)

	resp, err := p.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: p.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			MaxTokens: 1024,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	responseText := resp.Choices[0].Message.Content

	// Parse JSON response (extract JSON array if surrounded by text)
	actions, err := parseActionsJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response as JSON: %w\nResponse: %s", err, responseText)
	}

	return actions, nil
}
