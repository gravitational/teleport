package bedrock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/metrics"
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

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	maxSessionLength := cfg.MaxSessionLength
	if maxSessionLength <= 0 {
		maxSessionLength = defaultMaxSessionLength
	}

	awscfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Spec.GetRegion()),
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

	maxTokens := maxCompletionTokens
	convInput := bedrockruntime.ConverseInput{
		ModelId: &p.bedrockModelID,
		InferenceConfig: &bedrocktypes.InferenceConfiguration{
			MaxTokens: &maxTokens,
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

	resp, err := p.client.Converse(ctx, &convInput)
	if err != nil {
		var apierr smithy.APIError
		if errors.As(err, &apierr) {
			apiErrors.With(prometheus.Labels{
				metrics.LabelInferenceModelName: p.modelResourceName,
				labelApiErrorCode:               apierr.ErrorCode(),
			}).Inc()
		}

		return "", trace.Wrap(err)
	}

	switch resp.StopReason {
	case bedrocktypes.StopReasonEndTurn:
		msg, ok := resp.Output.(*bedrocktypes.ConverseOutputMemberMessage)
		if !ok {
			return "", trace.Wrap(summarizererrors.BadResponseError{
				Message: fmt.Sprintf("expected ConverseOutputMemberMessage, got %T", resp.Output),
			})
		}
		if msg == nil {
			return "", trace.Wrap(summarizererrors.BadResponseError{
				Message: "model did not return any output message",
			})
		}

		result := ""
		for _, block := range msg.Value.Content {
			text, ok := block.(*bedrocktypes.ContentBlockMemberText)
			if ok {
				result += text.Value
			}
		}

		if result == "" {
			return "", trace.Wrap(summarizererrors.BadResponseError{
				Message: "model returned a message without content",
			})
		}

		return result, nil
	case bedrocktypes.StopReasonMaxTokens:
		return "", trace.LimitExceeded("model response length limit exceeded")
	default:
		return "", trace.Wrap(summarizererrors.BadResponseError{
			Message: fmt.Sprintf("model returned unexpected stop reason: %q", resp.StopReason),
		})
	}
}
