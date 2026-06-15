package openai

import (
	"context"
	"encoding/base64"
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

	if last := body.Messages[messageCount-1].OfUser; last != nil && len(last.Content.OfArrayOfContentParts) > 0 {
		return handleOpenAIDesktopScreenshotAnalysis(body)
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

	// Check system prompt to dispatch structured handlers for non-exact-string content.
	systemContent := *body.Messages[0].GetContent().AsAny().(*string)
	if systemContent == schema.GetProseEmbedding() {
		return handleOpenAIProseEmbedding(content)
	}
	if strings.Contains(systemContent, "expert security analyst reviewing a Windows Desktop session") {
		return handleOpenAIDesktopSessionAnalysis(content)
	}

	return nil, nil
}

func handleOpenAIDesktopScreenshotAnalysis(body openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	userMsg := body.Messages[len(body.Messages)-1].OfUser
	if userMsg == nil {
		return nil, errors.New("expected user message with image content")
	}

	for _, part := range userMsg.Content.OfArrayOfContentParts {
		if part.OfImageURL == nil {
			return nil, errors.New("expected image content part")
		}
		if !strings.HasPrefix(part.OfImageURL.ImageURL.URL, "data:image/png;base64,") {
			return nil, errors.New("expected image to be a base64 PNG data URL")
		}
		b64 := strings.TrimPrefix(part.OfImageURL.ImageURL.URL, "data:image/png;base64,")
		if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return nil, errors.New("image data URL is not valid base64")
		}
	}

	systemContent := body.Messages[0].OfSystem
	if systemContent == nil {
		return nil, errors.New("expected system message")
	}
	systemText := systemContent.Content.OfString.Value
	if !strings.Contains(systemText, "start_screenshot_index") {
		return nil, errors.New("system prompt missing screenshot index instructions")
	}

	analysis := schema.DesktopScreenshotAnalysis{
		NotableSessionEvents: []schema.DesktopSessionEvent{
			{
				Category:             "data_access",
				StartScreenshotIndex: 0,
				EndScreenshotIndex:   0,
				RiskLevel:            "low",
				RiskScore:            15,
				ThreatCategory:       "none",
				TimelineTitle:        "Reviewed budget worksheet",
				TimelineSubtitle:     "",
				ShortDescription:     "Reviewed budget figures in a spreadsheet",
				DetailedDescription:  "User scrolled through a budget worksheet in Excel.",
				SuspiciousFlags:      []string{},
				SensitiveItems:       []string{},
				SuspiciousPatterns:   []string{},
				IOCs:                 []string{},
				MitreAttackIDs:       []string{},
				Applications:         []string{"Microsoft Excel"},
				VisibleURLs:          []string{},
				VisibleFilePaths:     []string{"/Users/test/budget.xlsx"},
				ActiveWindowTitle:    "budget.xlsx - Excel",
			},
		},
	}
	jsonStr, err := json.Marshal(analysis)
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

func handleOpenAIDesktopSessionAnalysis(content string) (*openai.ChatCompletion, error) {
	if !strings.Contains(content, "Desktop Session Events") {
		return nil, errors.New("desktop synthesis prompt missing events block")
	}

	analysis := schema.DesktopSessionAnalysis{
		ShortDescription:     "Routine spreadsheet work",
		SessionDescription:   "Reviewed a budget worksheet in Excel without any sensitive operations.",
		SuspiciousActivities: []string{},
		SecurityIncidents:    []string{},
		CompromiseIndicators: false,
		RiskLevel:            "low",
		RiskScore:            15,
	}
	jsonStr, err := json.Marshal(analysis)
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

func handleOpenAIProseEmbedding(content string) (*openai.ChatCompletion, error) {
	if strings.Contains(content, "trigger-api-error") {
		return nil, errors.New("prose embedding API error")
	}

	if strings.Contains(content, "trigger-bad-json") {
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{
				{
					FinishReason: string(openai.CompletionChoiceFinishReasonStop),
					Message: openai.ChatCompletionMessage{
						Role:    "assistant",
						Content: "not valid json",
					},
				},
			},
		}, nil
	}

	pe := &schema.ProseEmbedding{
		CondensedText: "A condensed description of the session for embedding generation.",
	}
	jsonStr, err := json.Marshal(pe)
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
