package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/metrics"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	libmetrics "github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// TODO(bl-nero): add context window size detection and adaptive algorithm.
	// Nova Lite gives us 300k tokens, so we should never exceed this value in
	// practice if this model is used.
	defaultMaxSessionLength       = 300_000 // bytes
	maxCompletionTokens     int32 = 4000
	// labelApiErrorCode is a Prometheus metric label that carries the Bedrock
	// API error code.
	labelApiErrorCode = "api_error_code"
)

var (
	apiRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "bedrock_api_requests",
		Help:      "Number of requests to the Amazon Bedrock API",
	}, []string{metrics.LabelInferenceModelName})

	apiErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "bedrock_api_errors",
		Help:      "Number of errors returned by Amazon Bedrock API",
	}, []string{metrics.LabelInferenceModelName, labelApiErrorCode})

	apiRequestsInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "bedrock_api_requests_in_flight",
		Help:      "Number of Amazon Bedrock API requests currently in flight",
	}, []string{metrics.LabelInferenceModelName})
)

func init() {
	libmetrics.RegisterPrometheusCollectors(apiRequests, apiErrors, apiRequestsInFlight)
}

// ProviderConfig holds the configuration for the Amazon Bedrock inference
// provider.
type ProviderConfig struct {
	Spec *summarizerv1pb.BedrockProvider
	// ClientFactory is used to create Bedrock clients. Can be overridden for
	// testing. Defaults to a production implementation.
	ClientFactory ClientFactory
	// MaxSessionLength is the maximum length of a session recording that can be
	// summarized. If the session recording exceeds this length, an error will be
	// returned. If not set, defaults to 1MB.
	MaxSessionLength int64
	// ModelResourceName is the name of an inference model this configuration is
	// derived from.
	ModelResourceName string
	// AWSConfigCache is used to retrieve AWS OIDC tokens for Amazon Bedrock.
	AWSConfigCache *awsconfig.Cache
}

// ClientFactory is an interface for creating Bedrock clients.
type ClientFactory interface {
	NewFromConfig(cfg aws.Config) Client
}

type defaultClientFactory struct{}

func (defaultClientFactory) NewFromConfig(cfg aws.Config) Client {
	return bedrockruntime.NewFromConfig(cfg)
}

type Client interface {
	Converse(
		ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options),
	) (*bedrockruntime.ConverseOutput, error)
}

// InferenceProvider is an Amazon Bedrock inference provider that summarizes
// session recordings.
type InferenceProvider struct {
	bedrockModelID    string
	temperature       float32
	maxSessionLength  int64
	client            Client
	logger            *slog.Logger
	modelResourceName string
}

func NewProvider(ctx context.Context, cfg ProviderConfig) (*InferenceProvider, error) {
	if cfg.Spec == nil {
		return nil, trace.BadParameter("provider spec is required")
	}
	if cfg.ModelResourceName == "" {
		return nil, trace.BadParameter("model resource name is required")
	}
	if cfg.Spec.GetRegion() == "" {
		return nil, trace.BadParameter("region is required")
	}
	if cfg.AWSConfigCache == nil {
		return nil, trace.BadParameter("AWS config cache is required")
	}

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	maxSessionLength := cfg.MaxSessionLength
	if maxSessionLength <= 0 {
		maxSessionLength = defaultMaxSessionLength
	}

	awscfg, err := cfg.AWSConfigCache.GetConfig(
		ctx,
		cfg.Spec.GetRegion(),
		awsconfig.WithCredentialsMaybeIntegration(
			awsconfig.IntegrationMetadata{
				Name: cfg.Spec.GetIntegration(),
			},
		),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clientFactory.NewFromConfig(awscfg)

	logger := slog.With(teleport.ComponentKey, "bedrock", "inference_model", cfg.ModelResourceName)
	return &InferenceProvider{
		bedrockModelID:    cfg.Spec.GetBedrockModelId(),
		temperature:       cfg.Spec.GetTemperature(),
		maxSessionLength:  maxSessionLength,
		client:            client,
		logger:            logger,
		modelResourceName: cfg.ModelResourceName,
	}, nil
}

// Summarize summarizes a session recording using Bedrock. Closes the reader
// when it's no longer needed.
func (p *InferenceProvider) Summarize(
	ctx context.Context, sessionID session.ID, systemPrompt string, reader io.ReadCloser,
) (string, error) {
	defer reader.Close()
	p.logger.DebugContext(ctx, "Summarizing session", "session_id", sessionID)

	// Read up to maxSessionLength bytes from the stream. This call attempts to
	// read one more byte, as `ReadAtMost` also reports an error if we read
	// exactly the number of bytes left in the reader.
	transcript, err := utils.ReadAtMost(reader, p.maxSessionLength+1)
	if err != nil {
		if trace.IsLimitExceeded(err) {
			return "", trace.Wrap(
				err, "session transcript exceeds maximum length of %d bytes", p.maxSessionLength,
			)
		}
		return "", trace.Wrap(err)
	}

	// We have read enough, we may close the reader.
	reader.Close()

	convInput := bedrockruntime.ConverseInput{
		ModelId: &p.bedrockModelID,
		InferenceConfig: &bedrocktypes.InferenceConfiguration{
			MaxTokens: aws.Int32(maxCompletionTokens),
		},
		System: []bedrocktypes.SystemContentBlock{
			&bedrocktypes.SystemContentBlockMemberText{
				Value: systemPrompt,
			},
		},
		Messages: []bedrocktypes.Message{{
			Role: bedrocktypes.ConversationRoleUser,
			Content: []bedrocktypes.ContentBlock{
				&bedrocktypes.ContentBlockMemberText{
					Value: string(transcript),
				},
			},
		},
		},
	}

	res, err := p.makeRequest(ctx, sessionID, &convInput)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.logger.DebugContext(ctx, "Session summary generated",
		"session_id", sessionID,
		"session_length", len(transcript),
		"input_tokens", res.inputTokens,
		"output_tokens", res.outputTokens,
		"finish_reason", res.finishReason,
	)

	return res.result, nil
}

// SummarizeCommand summarizes a single command using Bedrock.
func (p *InferenceProvider) SummarizeCommand(ctx context.Context, sessionID session.ID, username, loginName, command string) (*schema.CommandAnalysis, error) {
	p.logger.DebugContext(ctx, "Summarizing command from session", "session_id", sessionID)

	systemPrompt := schema.SummarizeCommandSystemPrompt(username, loginName)

	res, err := p.makeStructuredRequest(ctx, sessionID, schema.CommandAnalysisSchema, systemPrompt, command)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var analysis schema.CommandAnalysis
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(res.result)), &analysis); err != nil {
		return nil, trace.Wrap(summarizererrors.BadResponseError{
			Message: fmt.Sprintf("failed to unmarshal model response: %v", err),
		})
	}

	p.logger.DebugContext(ctx, "Command summary generated",
		"session_id", sessionID,
		"command_length", len(command),
		"input_tokens", res.inputTokens,
		"output_tokens", res.outputTokens,
		"finish_reason", res.finishReason,
	)

	return &analysis, nil
}

// SummarizeMultipleCommands summarizes the result of multiple commands using Bedrock.
func (p *InferenceProvider) SummarizeMultipleCommands(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.SessionAnalysis, error) {
	p.logger.DebugContext(ctx, "Summarizing multiple commands from session", "session_id", sessionID)

	systemPrompt := schema.SummarizeMultipleCommandsSystemPrompt(username, loginName)

	res, err := p.makeStructuredRequest(ctx, sessionID, schema.SessionAnalysisSchema, systemPrompt, prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var analysis schema.SessionAnalysis
	if err := json.Unmarshal([]byte(stripMarkdownCodeBlock(res.result)), &analysis); err != nil {
		return nil, trace.Wrap(summarizererrors.BadResponseError{
			Message: fmt.Sprintf("failed to unmarshal model response: %v", err),
		})
	}

	p.logger.DebugContext(ctx, "Summary of multiple commands generated",
		"session_id", sessionID,
		"prompt_length", len(prompt),
		"input_tokens", res.inputTokens,
		"output_tokens", res.outputTokens,
		"finish_reason", res.finishReason,
	)

	return &analysis, nil
}

func (p *InferenceProvider) makeStructuredRequest(ctx context.Context, sessionID session.ID, schema any, systemPrompt, message string) (*response, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	jsonPrompt := "Generate a JSON response that compiles with the provided schema. If required fields are missing, return available fields with `null` for missing ones. Only respond with JSON matching the schema, no additional text or formatting.\n\nSchema:\n"

	convInput := bedrockruntime.ConverseInput{
		ModelId: &p.bedrockModelID,
		InferenceConfig: &bedrocktypes.InferenceConfiguration{
			MaxTokens: aws.Int32(maxCompletionTokens),
		},
		System: []bedrocktypes.SystemContentBlock{
			&bedrocktypes.SystemContentBlockMemberText{
				Value: systemPrompt + jsonPrompt,
			},
		},
		Messages: []bedrocktypes.Message{
			{
				Role: bedrocktypes.ConversationRoleUser,
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{
						Value: string(schemaBytes),
					},
				},
			},
			{
				Role: bedrocktypes.ConversationRoleUser,
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{
						Value: message,
					},
				},
			},
		},
	}

	return p.makeRequest(ctx, sessionID, &convInput)
}

type response struct {
	inputTokens  int32
	outputTokens int32
	finishReason string
	result       string
}

func (p *InferenceProvider) makeRequest(ctx context.Context, sessionID session.ID, convInput *bedrockruntime.ConverseInput) (*response, error) {
	if p.temperature > 0.0 {
		convInput.InferenceConfig.Temperature = &p.temperature
	}

	p.logger.DebugContext(ctx, "Sending request to Bedrock",
		"session_id", sessionID,
		"model", convInput.ModelId,
	)

	apiRequests.WithLabelValues(p.modelResourceName).Inc()
	reqInFlightMetric := apiRequestsInFlight.WithLabelValues(p.modelResourceName)
	reqInFlightMetric.Inc()
	defer reqInFlightMetric.Dec()

	resp, err := p.client.Converse(ctx, convInput)
	if err != nil {
		var apierr smithy.APIError
		if errors.As(err, &apierr) {
			apiErrors.With(prometheus.Labels{
				metrics.LabelInferenceModelName: p.modelResourceName,
				labelApiErrorCode:               apierr.ErrorCode(),
			}).Inc()
		}

		return nil, trace.Wrap(err)
	}

	res := &response{
		finishReason: string(resp.StopReason),
	}

	if resp.Usage != nil {
		res.inputTokens = aws.ToInt32(resp.Usage.InputTokens)
		res.outputTokens = aws.ToInt32(resp.Usage.OutputTokens)
	}

	switch resp.StopReason {
	case bedrocktypes.StopReasonEndTurn:
		msg, ok := resp.Output.(*bedrocktypes.ConverseOutputMemberMessage)
		if !ok {
			return nil, trace.Wrap(summarizererrors.BadResponseError{
				Message: fmt.Sprintf("expected ConverseOutputMemberMessage, got %T", resp.Output),
			})
		}
		if msg == nil {
			return nil, trace.Wrap(summarizererrors.BadResponseError{
				Message: "model did not return any output message",
			})
		}

		for _, block := range msg.Value.Content {
			text, ok := block.(*bedrocktypes.ContentBlockMemberText)
			if ok {
				res.result += text.Value
			}
		}

		if res.result == "" {
			return nil, trace.Wrap(summarizererrors.BadResponseError{
				Message: "model returned a message without content",
			})
		}

		return res, nil
	case bedrocktypes.StopReasonMaxTokens:
		return nil, trace.LimitExceeded("model response length limit exceeded")
	default:
		return nil, trace.Wrap(summarizererrors.BadResponseError{
			Message: fmt.Sprintf("model returned unexpected stop reason: %q", resp.StopReason),
		})
	}
}

// stripMarkdownCodeBlock removes Markdown code block formatting from a string. Sometimes, the model
// can return JSON wrapped in Markdown code blocks, e.g.:
//
//	```json
//	{
//	  "key": "value"
//	}
//	```
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
