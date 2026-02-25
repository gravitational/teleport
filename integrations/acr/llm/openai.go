// Package llm provides an OpenAI client for embeddings and chat completions.
package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// Default models for OpenAI.
const (
	DefaultEmbeddingModel  = openai.SmallEmbedding3
	DefaultCompletionModel = openai.GPT4oMini
)

// CompletionOptions configures a chat completion request.
type CompletionOptions struct {
	SystemPrompt string
	Temperature  float32
	MaxTokens    int
}

// Client wraps the OpenAI API for embeddings and chat completions.
type Client struct {
	client          *openai.Client
	embeddingModel  openai.EmbeddingModel
	completionModel string
}

// NewClient creates a new OpenAI client with the given API key.
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("llm: OpenAI API key is required")
	}
	return &Client{
		client:          openai.NewClient(apiKey),
		embeddingModel:  DefaultEmbeddingModel,
		completionModel: DefaultCompletionModel,
	}, nil
}

// CompleteJSON performs a chat completion requesting JSON output and
// unmarshals the response into result.
func (c *Client) CompleteJSON(ctx context.Context, prompt string, opts CompletionOptions, result any) error {
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.completionModel,
		Messages:    buildMessages(prompt, opts.SystemPrompt),
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		return fmt.Errorf("llm: chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("llm: no completion choices returned")
	}

	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), result); err != nil {
		return fmt.Errorf("llm: parsing JSON response: %w", err)
	}
	return nil
}

// Complete performs a chat completion and returns the text response.
func (c *Client) Complete(ctx context.Context, prompt string, opts CompletionOptions) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.completionModel,
		Messages:    buildMessages(prompt, opts.SystemPrompt),
		Temperature: opts.Temperature,
		MaxTokens:   opts.MaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("llm: chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm: no completion choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}

// EmbeddingModel returns the name of the embedding model in use.
func (c *Client) EmbeddingModel() string {
	return string(c.embeddingModel)
}

// CompletionModel returns the name of the completion model in use.
func (c *Client) CompletionModel() string {
	return c.completionModel
}

// buildMessages constructs the chat message slice, optionally prepending
// a system prompt.
func buildMessages(prompt, systemPrompt string) []openai.ChatCompletionMessage {
	msgs := make([]openai.ChatCompletionMessage, 0, 2)
	if systemPrompt != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: prompt,
	})
	return msgs
}

// toFloat64 converts a []float32 embedding to []float64.
func toFloat64(f32 []float32) []float64 {
	f64 := make([]float64, len(f32))
	for i, v := range f32 {
		f64[i] = float64(v)
	}
	return f64
}
