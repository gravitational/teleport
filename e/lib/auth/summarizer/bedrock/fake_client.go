package bedrock

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/jonboulle/clockwork"
)

type FakeClientFactory struct {
	Clock *clockwork.FakeClock
}

func (m *FakeClientFactory) NewFromConfig(cfg aws.Config) Client {
	return &fakeClient{
		clock: m.Clock,
	}
}

type fakeClient struct {
	clock *clockwork.FakeClock
}

func (m *fakeClient) Converse(
	ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options),
) (*bedrockruntime.ConverseOutput, error) {
	// Advance the clock to test if the inference end timestamp is captured.
	m.clock.Advance(10 * time.Second)

	content := params.Messages[0].Content[0].(*bedrocktypes.ContentBlockMemberText).Value

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
		}, nil
	case "no choices":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: nil,
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
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
		}, nil
	default:
		systemPrompt := params.System[0].(*bedrocktypes.SystemContentBlockMemberText).Value

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
		}, nil
	}
}
