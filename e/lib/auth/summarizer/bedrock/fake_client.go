package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

// structuredResponse builds a ConverseOutput that mimics Bedrock's
// schema-constrained JSON response (via ConverseInput.OutputConfig). The
// response is returned as a normal text content block whose payload is
// guaranteed by Bedrock to conform to the schema.
func structuredResponse(value any) (*bedrockruntime.ConverseOutput, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &bedrockruntime.ConverseOutput{
		Output: &bedrocktypes.ConverseOutputMemberMessage{
			Value: bedrocktypes.Message{
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{Value: string(data)},
				},
			},
		},
		StopReason: bedrocktypes.StopReasonEndTurn,
		Usage: &bedrocktypes.TokenUsage{
			InputTokens:  aws.Int32(100),
			OutputTokens: aws.Int32(50),
		},
	}, nil
}

type FakeClientFactory struct {
	Clock            *clockwork.FakeClock
	configValidation func(cfg aws.Config)
}

func (m *FakeClientFactory) NewFromConfig(cfg aws.Config) Client {
	if m.configValidation != nil {
		m.configValidation(cfg)
	}
	return &fakeClient{
		clock:  m.Clock,
		region: cfg.Region,
	}
}

type fakeClient struct {
	clock  *clockwork.FakeClock
	region string
}

func (m *fakeClient) Converse(
	ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options),
) (*bedrockruntime.ConverseOutput, error) {
	// Advance the clock to test if the inference end timestamp is captured.
	m.clock.Advance(10 * time.Second)

	messageCount := len(params.Messages)
	if messageCount == 0 {
		return nil, errors.New("no content in the message")
	}

	if _, isImage := params.Messages[messageCount-1].Content[0].(*bedrocktypes.ContentBlockMemberImage); isImage {
		return handleBedrockDesktopScreenshotAnalysis(params)
	}

	content := params.Messages[messageCount-1].Content[0].(*bedrocktypes.ContentBlockMemberText).Value

	switch content {
	case "cause an error":
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "dummy", Message: "OMG"},
		}
	case "make the output too long":
		return &bedrockruntime.ConverseOutput{
			StopReason: bedrocktypes.StopReasonMaxTokens,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "no choices":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: nil,
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "respond with multiple content blocks":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: "block 1, ",
						},
						&bedrocktypes.ContentBlockMemberReasoningContent{
							Value: &bedrocktypes.ReasoningContentBlockMemberReasoningText{
								Value: bedrocktypes.ReasoningTextBlock{
									Text: aws.String("Hold on, I'm thinking."),
								},
							},
						},
						&bedrocktypes.ContentBlockMemberText{
							Value: "block 2",
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "respond with multiple empty content blocks":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: "",
						},
						&bedrocktypes.ContentBlockMemberReasoningContent{
							Value: &bedrocktypes.ReasoningContentBlockMemberReasoningText{
								Value: bedrocktypes.ReasoningTextBlock{
									Text: aws.String("Hold on, I'm thinking."),
								},
							},
						},
						&bedrocktypes.ContentBlockMemberText{
							Value: "",
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "json response for command analysis":
		return structuredResponse(map[string]any{
			"command":           "ls -al",
			"risk_level":        "low",
			"risk_score":        10,
			"category":          "file_operation",
			"timeline_title":    "Listed directory contents",
			"timeline_subtitle": "",
			"short_description": "Listed all files in the directory",
			"description":       "The command 'ls -al' was executed to list all files, including hidden ones, in the current directory.",
			"threat_category":   "none",
			"success":           true,
		})

	case "respond with region and Bedrock model ID":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: m.region + ", " + *params.ModelId,
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil

	default:
		systemPrompt := params.System[0].(*bedrocktypes.SystemContentBlockMemberText).Value

		// Handle structured responses for command analysis.
		if strings.Contains(systemPrompt, "analyzing a single command from a session recording") {
			return handleBedrockCommandAnalysis(content)
		}

		// Handle structured responses for session analysis.
		if strings.Contains(systemPrompt, "preparing a summary") && strings.Contains(systemPrompt, "terminal session") {
			return handleBedrockSessionAnalysis(content)
		}

		// Handle structured responses for desktop session synthesis.
		if strings.Contains(systemPrompt, "expert security analyst reviewing a Windows Desktop session") {
			return handleBedrockDesktopSessionAnalysis(content)
		}

		// Handle structured responses for prose embedding condensation.
		if strings.HasPrefix(systemPrompt, schema.GetProseEmbedding()) {
			return handleBedrockProseEmbedding(content)
		}

		// Handle simple summarization (legacy path).
		var responsePrefix string
		switch {
		case strings.Contains(systemPrompt, "Analyze this terminal session"):
			responsePrefix = "The user wrote: "
		case strings.Contains(systemPrompt, "Analyze this database session"):
			responsePrefix = "The user queried: "
		default:
			return nil, errors.New("unrecognized prompt")
		}

		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: responsePrefix + content,
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	}
}

func handleBedrockCommandAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	// Check that the magic refusal string was sanitized.
	if strings.Contains(content, "ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL") {
		return nil, errors.New("magic refusal string was not sanitized from prompt")
	}

	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") || strings.Contains(content, "trigger command error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced command analysis error"},
		}
	}

	return structuredResponse(map[string]any{
		"command":           "test-command",
		"risk_level":        "low",
		"risk_score":        10,
		"category":          "other",
		"timeline_title":    "Executed test command",
		"timeline_subtitle": "",
		"short_description": "Test command executed",
		"description":       "A test command was executed during the session.",
		"threat_category":   "none",
		"success":           true,
	})
}

func handleBedrockProseEmbedding(content string) (*bedrockruntime.ConverseOutput, error) {
	if strings.Contains(content, "trigger-api-error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "api_error", Message: "prose embedding API error"},
		}
	}

	if strings.Contains(content, "trigger-bad-json") {
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: "not valid json",
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	}

	return structuredResponse(map[string]any{
		"condensed_text": "A condensed description of the session for embedding generation.",
	})
}

func handleBedrockDesktopScreenshotAnalysis(params *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
	for _, msg := range params.Messages {
		for _, block := range msg.Content {
			img, ok := block.(*bedrocktypes.ContentBlockMemberImage)
			if !ok {
				continue
			}
			if img.Value.Format != bedrocktypes.ImageFormatPng {
				return nil, errors.New("expected PNG image format")
			}
			src, ok := img.Value.Source.(*bedrocktypes.ImageSourceMemberBytes)
			if !ok {
				return nil, errors.New("expected raw bytes image source")
			}
			if len(src.Value) == 0 {
				return nil, errors.New("image source bytes are empty")
			}
		}
	}

	systemPrompt := params.System[0].(*bedrocktypes.SystemContentBlockMemberText).Value
	if !strings.Contains(systemPrompt, "start_screenshot_index") {
		return nil, errors.New("system prompt missing screenshot index instructions")
	}

	return structuredResponse(map[string]any{
		"short_description":    "Spreadsheet open with financial data visible",
		"screenshot_context":   "Excel window showing a budget worksheet.",
		"sensitive_info_found": false,
		"risk_level":           "low",
		"notable_session_events": []map[string]any{
			{
				"start_screenshot_index": 0,
				"end_screenshot_index":   0,
				"category":               "data_access",
				"risk_level":             "low",
				"risk_score":             15,
				"threat_category":        "none",
				"timeline_title":         "Reviewed budget worksheet",
				"timeline_subtitle":      "",
				"short_description":      "Reviewed budget figures in a spreadsheet",
				"detailed_description":   "User scrolled through a budget worksheet in Excel.",
				"suspicious_flags":       []string{},
				"sensitive_items":        []string{},
				"suspicious_patterns":    []string{},
				"iocs":                   []string{},
				//nolint:misspell // ignore MITRE
				"mitre_attack_ids":     []string{},
				"has_sensitive_data":   false,
				"privilege_escalation": false,
				"data_exfiltration":    false,
				"persistence":          false,
				"applications":         []string{"Microsoft Excel"},
				"visible_urls":         []string{},
				"visible_file_paths":   []string{"/Users/test/budget.xlsx"},
				"active_window_title":  "budget.xlsx - Excel",
			},
		},
	})
}

func handleBedrockDesktopSessionAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	if !strings.Contains(content, "Desktop Session Events") {
		return nil, errors.New("desktop synthesis prompt missing events block")
	}

	return structuredResponse(map[string]any{
		"short_description":     "Routine spreadsheet work",
		"session_description":   "Reviewed a budget worksheet in Excel without any sensitive operations.",
		"suspicious_activities": []string{},
		"security_incidents":    []string{},
		"compromise_indicators": false,
		"risk_level":            "low",
		"risk_score":            15,
	})
}

func handleBedrockSessionAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced session analysis error"},
		}
	}

	return structuredResponse(map[string]any{
		"short_description":     "Test session with commands",
		"session_description":   "The user executed test commands during this session.",
		"suspicious_activities": []string{},
		"security_incidents":    []string{},
		"compromise_indicators": false,
		"risk_level":            "low",
		"risk_score":            15,
		"too_large":             false,
	})
}

func (m *fakeClient) InvokeModel(
	ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options),
) (*bedrockruntime.InvokeModelOutput, error) {
	var req struct {
		InputText string `json:"inputText"`
	}
	if err := json.Unmarshal(params.Body, &req); err != nil {
		return nil, err
	}

	switch req.InputText {
	case "cause an error":
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "InvokeModel",
			Err:           &smithy.GenericAPIError{Code: "dummy", Message: "OMG"},
		}
	case "invalid json response":
		return &bedrockruntime.InvokeModelOutput{
			Body: []byte("not valid json"),
		}, nil
	default:
		resp := struct {
			Embedding           []float32 `json:"embedding"`
			InputTextTokenCount int       `json:"inputTextTokenCount"`
		}{
			Embedding:           []float32{0.1, 0.2, 0.3},
			InputTextTokenCount: len(strings.Fields(req.InputText)),
		}
		body, err := json.Marshal(resp)
		if err != nil {
			return nil, err
		}
		return &bedrockruntime.InvokeModelOutput{Body: body}, nil
	}
}
