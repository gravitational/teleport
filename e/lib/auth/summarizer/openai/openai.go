package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/metrics"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	libmetrics "github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// TODO(bl-nero): add context window size detection and adaptive algorithm.
	defaultMaxSessionLength       = 200_000 // bytes
	maxCompletionTokens     int64 = 4000
	// labelApiErrorCode is a Prometheus metric label that carries the OpenAI API
	// error code.
	labelApiErrorCode = "api_error_code"
)

var (
	apiRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "openai_api_requests",
		Help:      "Number of requests to the OpenAI API",
	}, []string{metrics.LabelInferenceModelName})

	apiErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "openai_api_errors",
		Help:      "Number of errors returned by OpenAI API",
	}, []string{metrics.LabelInferenceModelName, labelApiErrorCode})

	apiRequestsInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: metrics.SummarizerSubsystem,
		Name:      "openai_api_requests_in_flight",
		Help:      "Number of OpenAI API requests currently in flight",
	}, []string{metrics.LabelInferenceModelName})
)

func init() {
	libmetrics.RegisterPrometheusCollectors(apiRequests, apiErrors, apiRequestsInFlight)
}

// ProviderConfig holds the configuration for the OpenAI inference provider.
type ProviderConfig struct {
	// ModelProvider is the OpenAI model specification.
	ModelProvider *summarizerv1pb.OpenAIProvider
	// SecretSpec is the inference secret containing the API key.
	SecretSpec *summarizerv1pb.InferenceSecretSpec
	// ClientFactory is used to create OpenAI clients. Can be overridden for
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

// ClientFactory is an interface for creating OpenAI clients.
type ClientFactory interface {
	NewClient(opts ...option.RequestOption) Client
}

// Client is an interface for the OpenAI client used to make requests.
type Client interface {
	NewChatCompletion(
		ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption,
	) (*openai.ChatCompletion, error)
}

type defaultClientFactory struct{}

func (defaultClientFactory) NewClient(opts ...option.RequestOption) Client {
	return &defaultClient{clt: openai.NewClient(opts...)}
}

type defaultClient struct {
	clt openai.Client
}

// NewChatCompletion makes a new chat completion request to the real OpenAI
// API.
func (c *defaultClient) NewChatCompletion(
	ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption,
) (*openai.ChatCompletion, error) {
	return c.clt.Chat.Completions.New(ctx, body, opts...)
}

// InferenceProvider is an OpenAI inference provider that summarizes session
// recordings.
type InferenceProvider struct {
	openAIModelName   openai.ChatModel
	temperature       float64
	maxSessionLength  int64
	client            Client
	logger            *slog.Logger
	modelResourceName string
	totalInputTokens  atomic.Uint64
	totalOutputTokens atomic.Uint64
}

// NewProvider creates a new OpenAI inference provider.
func NewProvider(ctx context.Context, cfg ProviderConfig) (*InferenceProvider, error) {
	if cfg.ModelProvider == nil {
		return nil, trace.BadParameter("provider spec is required")
	}
	if cfg.SecretSpec == nil {
		return nil, trace.BadParameter("secret spec is required")
	}
	if cfg.ModelResourceName == "" {
		return nil, trace.BadParameter("model resource name is required")
	}

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	maxSessionLength := cfg.MaxSessionLength
	if maxSessionLength <= 0 {
		maxSessionLength = defaultMaxSessionLength
	}

	clientOptions := []option.RequestOption{option.WithAPIKey(cfg.SecretSpec.GetValue())}
	baseURL := cfg.ModelProvider.GetBaseUrl()
	if baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(baseURL))
	}
	client := clientFactory.NewClient(clientOptions...)

	logger := slog.With(teleport.ComponentKey, "openai", "inference_model", cfg.ModelResourceName)
	return &InferenceProvider{
		openAIModelName:   cfg.ModelProvider.GetOpenaiModelId(),
		temperature:       cfg.ModelProvider.GetTemperature(),
		maxSessionLength:  maxSessionLength,
		client:            client,
		logger:            logger,
		modelResourceName: cfg.ModelResourceName,
	}, nil
}

// GetTotalTokens returns the total number of input and output tokens used by this provider.
func (p *InferenceProvider) GetTotalTokens() (input uint64, output uint64) {
	return p.totalInputTokens.Load(), p.totalOutputTokens.Load()
}

// GetType returns the type of the inference provider.
func (p *InferenceProvider) GetType() string {
	return "openai"
}

// Summarize summarizes a session recording using OpenAI. Closes the reader
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

	completionParams := openai.ChatCompletionNewParams{
		Model: p.openAIModelName,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(string(transcript)),
		},
	}

	res, err := p.makeRequest(ctx, sessionID, completionParams)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.logger.DebugContext(ctx, "Session summary generated",
		"session_id", sessionID,
		"session_length", len(transcript),
		"prompt_tokens", res.promptTokens,
		"completion_tokens", res.completionTokens,
		"finish_reason", res.finishReason,
	)

	return res.result, nil
}

// SummarizeCommand summarizes a single command using OpenAI.
func (p *InferenceProvider) SummarizeCommand(ctx context.Context, sessionID session.ID, username, loginName, command string) (*schema.CommandAnalysis, error) {
	p.logger.DebugContext(ctx, "Summarizing command from session", "session_id", sessionID)

	systemPrompt := schema.SummarizeCommandSystemPrompt(username, loginName)

	res, err := p.makeStructuredRequest(ctx, sessionID, "CommandAnalysis", schema.CommandAnalysisSchema, systemPrompt, command)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.logger.DebugContext(ctx, "Command summary generated",
		"session_id", sessionID,
		"command_length", len(command),
		"prompt_tokens", res.promptTokens,
		"completion_tokens", res.completionTokens,
		"finish_reason", res.finishReason,
	)

	var analysis schema.CommandAnalysis
	if err := json.Unmarshal([]byte(res.result), &analysis); err != nil {
		return nil, trace.Wrap(summarizererrorstypes.BadResponseError{
			Message: fmt.Sprintf("failed to unmarshal model response: %v", err),
		})
	}

	return &analysis, nil
}

// SummarizeMultipleCommands summarizes the result of multiple commands using OpenAI.
func (p *InferenceProvider) SummarizeMultipleCommands(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.SessionAnalysis, error) {
	p.logger.DebugContext(ctx, "Summarizing multiple commands from session", "session_id", sessionID)

	systemPrompt := schema.SummarizeMultipleCommandsSystemPrompt(username, loginName)

	res, err := p.makeStructuredRequest(ctx, sessionID, "SessionAnalysis", schema.SessionAnalysisSchema, systemPrompt, prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.logger.DebugContext(ctx, "Summary of multiple commands generated",
		"session_id", sessionID,
		"prompt_length", len(prompt),
		"prompt_tokens", res.promptTokens,
		"completion_tokens", res.completionTokens,
		"finish_reason", res.finishReason,
	)

	var analysis schema.SessionAnalysis
	if err := json.Unmarshal([]byte(res.result), &analysis); err != nil {
		return nil, trace.Wrap(summarizererrorstypes.BadResponseError{
			Message: fmt.Sprintf("failed to unmarshal model response: %v", err),
		})
	}

	return &analysis, nil
}

func (p *InferenceProvider) makeStructuredRequest(ctx context.Context, sessionID session.ID, schemaName string, schema any, systemPrompt, message string) (*response, error) {
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:   schemaName,
		Schema: schema,
		Strict: openai.Bool(true),
	}

	completionParams := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(message),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
		Model: p.openAIModelName,
	}

	return p.makeRequest(ctx, sessionID, completionParams)
}

type response struct {
	promptTokens     int64
	completionTokens int64
	finishReason     string
	result           string
}

func (p *InferenceProvider) makeRequest(ctx context.Context, sessionID session.ID, completionParams openai.ChatCompletionNewParams) (*response, error) {
	completionParams.MaxCompletionTokens = param.NewOpt(maxCompletionTokens)

	if p.temperature > 0.0 {
		completionParams.Temperature = param.NewOpt(p.temperature)
	}

	p.logger.DebugContext(ctx, "Sending request to OpenAI",
		"session_id", sessionID,
		"model", completionParams.Model,
	)

	apiRequests.WithLabelValues(p.modelResourceName).Inc()
	reqInFlightMetric := apiRequestsInFlight.WithLabelValues(p.modelResourceName)
	reqInFlightMetric.Inc()
	defer reqInFlightMetric.Dec()

	completion, err := p.client.NewChatCompletion(ctx, completionParams)
	if err != nil {
		var apierr *openai.Error
		if errors.As(err, &apierr) {
			apiErrors.With(prometheus.Labels{
				metrics.LabelInferenceModelName: p.modelResourceName,
				labelApiErrorCode:               apierr.Code,
			}).Inc()
		}
		return nil, trace.Wrap(err)
	}

	if len(completion.Choices) == 0 {
		return nil, trace.Wrap(summarizererrorstypes.BadResponseError{
			Message: "model returned no choices",
		})
	}

	choice := completion.Choices[0]

	res := &response{
		promptTokens:     completion.Usage.PromptTokens,
		completionTokens: completion.Usage.CompletionTokens,
		finishReason:     choice.FinishReason,
		result:           choice.Message.Content,
	}

	p.totalInputTokens.Add(uint64(res.promptTokens))
	p.totalOutputTokens.Add(uint64(res.completionTokens))

	switch choice.FinishReason {
	case string(openai.CompletionChoiceFinishReasonStop):
		return res, nil
	case string(openai.CompletionChoiceFinishReasonLength):
		return res, trace.LimitExceeded("model response length limit exceeded")
	default:
		return res, trace.Wrap(summarizererrorstypes.BadResponseError{
			Message: fmt.Sprintf("model returned unexpected finish reason: %q", choice.FinishReason),
		})
	}
}

// FormatError formats OpenAI API errors into user-friendly messages.
func FormatError(err error, provider *summarizerv1pb.OpenAIProvider) string {
	// Check for OpenAI API errors
	var openaiErr *openai.Error
	if errors.As(err, &openaiErr) {
		switch openaiErr.Code {
		case "invalid_api_key":
			return "Invalid API key provided. Please verify your OpenAI API key is correct."
		case "insufficient_quota":
			return "OpenAI API quota exceeded. Please check your usage limits and billing status."
		case "rate_limit_exceeded":
			return "OpenAI API rate limit exceeded. Please try again in a few moments."
		case "model_not_found":
			return fmt.Sprintf("Model %q not found. Please verify the model ID is correct and accessible with your API key.", provider.GetOpenaiModelId())
		case "invalid_request_error":
			return fmt.Sprintf("Invalid request to OpenAI API: %s", openaiErr.Message)
		case "server_error", "service_unavailable":
			return "OpenAI API is currently unavailable. Please try again later."
		default:
			return fmt.Sprintf("OpenAI API error (%s): %s", openaiErr.Code, openaiErr.Message)
		}
	}

	// Check for network/connection errors
	if errors.Is(err, context.DeadlineExceeded) {
		return "Request to OpenAI API timed out. Please check your network connection and try again."
	}
	if errors.Is(err, context.Canceled) {
		return "Request to OpenAI API was canceled."
	}

	// Check for trace errors
	if trace.IsConnectionProblem(err) {
		baseURL := provider.GetBaseUrl()
		if baseURL != "" {
			return fmt.Sprintf("Failed to connect to OpenAI API at %s. Please verify the base URL is correct and accessible.", baseURL)
		}
		return "Failed to connect to OpenAI API. Please check your network connection."
	}
	if trace.IsAccessDenied(err) {
		return "Access denied by OpenAI API. Please verify your API key has the necessary permissions."
	}

	// Generic error
	return fmt.Sprintf("Failed to connect to OpenAI API: %v", err)
}
