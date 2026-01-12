package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/executor"
)

// ClaudeProvider implements the Provider interface using Anthropic's Claude
type ClaudeProvider struct {
	client *anthropic.Client
	model  string
}

// NewClaudeProvider creates a new Claude provider
func NewClaudeProvider(model string) (*ClaudeProvider, error) {
	apiKey := os.Getenv("DEMOGIF_ANTHROPIC_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("DEMOGIF_ANTHROPIC_KEY or ANTHROPIC_API_KEY environment variable required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	if model == "" {
		model = string(anthropic.ModelClaudeSonnet4_20250514)
	}

	return &ClaudeProvider{
		client: &client,
		model:  model,
	}, nil
}

// GenerateActions generates browser actions from the page map and user prompt
func (p *ClaudeProvider) GenerateActions(pageMap *crawler.PageMap, prompt string) ([]executor.Action, error) {
	pageMapJSON, err := json.MarshalIndent(pageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page map: %w", err)
	}

	userPrompt := buildUserPrompt(string(pageMapJSON), prompt)

	resp, err := p.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Claude API error: %w", err)
	}

	// Extract text content
	var responseText string
	for _, block := range resp.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("empty response from Claude")
	}

	// Parse JSON response (extract JSON array if surrounded by text)
	actions, err := parseActionsJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude response as JSON: %w\nResponse: %s", err, responseText)
	}

	return actions, nil
}

// ContinueActions generates the next batch of actions after a checkpoint
func (p *ClaudeProvider) ContinueActions(pageMap *crawler.PageMap, originalPrompt string, completedActions string) ([]executor.Action, error) {
	pageMapJSON, err := json.MarshalIndent(pageMap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal page map: %w", err)
	}

	userPrompt := buildContinuePrompt(string(pageMapJSON), originalPrompt, completedActions)

	resp, err := p.client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Claude API error: %w", err)
	}

	// Extract text content
	var responseText string
	for _, block := range resp.Content {
		if block.Type == "text" {
			responseText = block.Text
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("empty response from Claude")
	}

	// Parse JSON response (extract JSON array if surrounded by text)
	actions, err := parseActionsJSON(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Claude response as JSON: %w\nResponse: %s", err, responseText)
	}

	return actions, nil
}

// parseActionsJSON extracts and parses a JSON array from a response that may contain surrounding text
func parseActionsJSON(response string) ([]executor.Action, error) {
	// First try direct parsing
	var actions []executor.Action
	if err := json.Unmarshal([]byte(response), &actions); err == nil {
		return actions, nil
	}

	// Find JSON array in response (look for [ ... ])
	start := strings.Index(response, "[")
	if start == -1 {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	// Find matching closing bracket
	depth := 0
	end := -1
	for i := start; i < len(response); i++ {
		switch response[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end != -1 {
			break
		}
	}

	if end == -1 {
		return nil, fmt.Errorf("no matching closing bracket found")
	}

	jsonStr := response[start:end]
	if err := json.Unmarshal([]byte(jsonStr), &actions); err != nil {
		return nil, fmt.Errorf("failed to parse extracted JSON: %w", err)
	}

	return actions, nil
}
