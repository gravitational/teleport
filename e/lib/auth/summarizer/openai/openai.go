package openai

import (
	"context"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"

	"github.com/gravitational/teleport"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// TODO(bl-nero): add context window size detection and adaptive algorithm.
	defaultMaxSessionLength       = 200_000 // bytes
	maxCompletionTokens     int64 = 2000
)

// ProviderConfig holds the configuration for the OpenAI inference provider.
type ProviderConfig struct {
	Spec    *summarizerv1pb.OpenAIProvider
	Backend services.Summarizer
	// ClientFactory is used to create OpenAI clients. Can be overridden for
	// testing. Defaults to a production implementation.
	ClientFactory ClientFactory
	// MaxSessionLength is the maximum length of a session recording that can be
	// summarized. If the session recording exceeds this length, an error will be
	// returned. If not set, defaults to 1MB.
	MaxSessionLength int64
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
	model            openai.ChatModel
	temperature      float64
	maxSessionLength int64
	client           Client
	logger           *slog.Logger
}

// NewProvider creates a new OpenAI inference provider.
func NewProvider(ctx context.Context, cfg ProviderConfig) (*InferenceProvider, error) {
	if cfg.Spec == nil {
		return nil, trace.BadParameter("provider spec is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend is required")
	}

	clientFactory := cfg.ClientFactory
	if clientFactory == nil {
		clientFactory = defaultClientFactory{}
	}

	maxSessionLength := cfg.MaxSessionLength
	if maxSessionLength <= 0 {
		maxSessionLength = defaultMaxSessionLength
	}

	apiKey, err := cfg.Backend.GetInferenceSecret(ctx, cfg.Spec.ApiKeySecretRef)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientOptions := []option.RequestOption{option.WithAPIKey(apiKey.GetSpec().GetValue())}
	baseURL := cfg.Spec.GetBaseUrl()
	if baseURL != "" {
		clientOptions = append(clientOptions, option.WithBaseURL(baseURL))
	}
	client := clientFactory.NewClient(clientOptions...)

	return &InferenceProvider{
		model:            cfg.Spec.GetOpenaiModelId(),
		temperature:      cfg.Spec.GetTemperature(),
		maxSessionLength: maxSessionLength,
		client:           client,
		logger:           slog.With(teleport.ComponentKey, "openai"),
	}, nil
}

// Summarize summarizes a session recording using OpenAI. Closes the reader
// when it's no longer needed.
func (p *InferenceProvider) Summarize(
	ctx context.Context, sessionID session.ID, sessionKind types.SessionKind, reader io.ReadCloser,
) (string, error) {
	defer reader.Close()
	p.logger.DebugContext(ctx, "Summarizing session", "session_id", sessionID)

	var systemPrompt string
	switch sessionKind {
	case types.SSHSessionKind, types.KubernetesSessionKind:
		systemPrompt = prompts.SSHPrompt
	case types.DatabaseSessionKind:
		systemPrompt = prompts.DatabasePrompt
	default:
		return "", trace.BadParameter("unsupported session kind: %v", sessionKind)
	}

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
		Model: p.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(string(transcript)),
		},
		MaxCompletionTokens: param.NewOpt(maxCompletionTokens),
	}

	if p.temperature > 0.0 {
		completionParams.Temperature = param.NewOpt(p.temperature)
	}

	p.logger.DebugContext(ctx, "Sending request to OpenAPI",
		"session_id", sessionID,
		"model", completionParams.Model,
		"temperature", completionParams.Temperature,
	)

	completion, err := p.client.NewChatCompletion(ctx, completionParams)
	if err != nil {
		return "", trace.Wrap(err)
	}
	p.logger.DebugContext(ctx, "Session summary generated",
		"session_id", sessionID,
		"prompt_tokens", completion.Usage.PromptTokens,
		"completion_tokens", completion.Usage.CompletionTokens,
	)
	return completion.Choices[0].Message.Content, nil
}
