package openai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

type fakeClient struct{}

func (m *fakeClient) GenerateEmbeddings(ctx context.Context, input openai.EmbeddingNewParams, opts ...option.RequestOption) (*openai.CreateEmbeddingResponse, error) {
	text := input.Input.OfString.Value

	switch text {
	case "cause an error":
		return nil, errors.New("embeddings API error")
	default:
		return &openai.CreateEmbeddingResponse{
			Data: []openai.Embedding{
				{
					Embedding: []float64{0.1, 0.2, 0.3},
				},
			},
			Usage: openai.CreateEmbeddingResponseUsage{
				TotalTokens: int64(len(strings.Fields(text))),
			},
		}, nil
	}
}

func (m *fakeClient) NewChatCompletion(
	ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption,
) (*openai.ChatCompletion, error) {
	messageCount := len(body.Messages)
	if messageCount == 0 {
		return nil, errors.New("no content in the message")
	}

	content := *body.Messages[messageCount-1].GetContent().AsAny().(*string)

	switch content {
	case "no choices":
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{},
		}, nil
	case "cause an error":
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{
				{
					FinishReason: string(openai.CompletionChoiceFinishReasonContentFilter),
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: "",
					},
				},
			},
		}, nil
	case "make the output too long":
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{
				{
					FinishReason: string(openai.CompletionChoiceFinishReasonLength),
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: "",
					},
				},
			},
		}, nil
	case "json response for command analysis":
		ca := &schema.CommandAnalysis{
			Command:          "ls -al",
			RiskLevel:        "low",
			Category:         "file_operation",
			TimelineTitle:    "Listed directory contents",
			TimelineSubtitle: "",
			ShortDescription: "Listed all files in the directory",
			Description:      "The command 'ls -al' was executed to list all files, including hidden ones, in the current directory.",
			ThreatCategory:   "none",
		}

		jsonStr, err := json.Marshal(ca)
		if err != nil {
			return nil, err
		}

		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{
				{
					FinishReason: string(openai.CompletionChoiceFinishReasonStop),
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: string(jsonStr),
					},
				},
			},
		}, nil
	}

	return nil, nil
}
