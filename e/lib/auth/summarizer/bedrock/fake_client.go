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

type FakeClientFactory struct {
	Clock *clockwork.FakeClock
}

func (m *FakeClientFactory) NewFromConfig(cfg aws.Config) Client {
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

		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: string(jsonStr),
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
	case "respond with json over multiple content blocks":
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

		blockCount := 3
		var contentBlocks []bedrocktypes.ContentBlock
		chunkSize := len(jsonStr) / blockCount
		for i := 0; i < blockCount; i++ {
			start := i * chunkSize
			end := start + chunkSize
			if i == blockCount-1 {
				end = len(jsonStr)
			}
			contentBlocks = append(contentBlocks, &bedrocktypes.ContentBlockMemberText{
				Value: string(jsonStr[start:end]),
			})
		}

		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: contentBlocks,
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil

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
	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced command analysis error"},
		}
	}

	ca := &schema.CommandAnalysis{
		Command:          "test-command",
		RiskLevel:        "low",
		RiskScore:        10,
		Category:         "other",
		TimelineTitle:    "Executed test command",
		TimelineSubtitle: "",
		ShortDescription: "Test command executed",
		Description:      "A test command was executed during the session.",
		ThreatCategory:   "none",
		Success:          true,
	}

	jsonStr, err := json.Marshal(ca)
	if err != nil {
		return nil, err
	}

	return &bedrockruntime.ConverseOutput{
		Output: &bedrocktypes.ConverseOutputMemberMessage{
			Value: bedrocktypes.Message{
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{
						Value: string(jsonStr),
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

func handleBedrockSessionAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced session analysis error"},
		}
	}

	sa := &schema.SessionAnalysis{
		ShortDescription:     "Test session with commands",
		SessionDescription:   "The user executed test commands during this session.",
		SuspiciousActivities: []string{},
		SecurityIncidents:    []string{},
		CompromiseIndicators: false,
		RiskLevel:            "low",
		RiskScore:            15,
		TooLarge:             false,
	}

	jsonStr, err := json.Marshal(sa)
	if err != nil {
		return nil, err
	}

	return &bedrockruntime.ConverseOutput{
		Output: &bedrocktypes.ConverseOutputMemberMessage{
			Value: bedrocktypes.Message{
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{
						Value: string(jsonStr),
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
