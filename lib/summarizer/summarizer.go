package summarizer

import (
	"context"
	"errors"
	"os"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type InferenceProvider interface {
	Summarize(
		ctx context.Context,
		sessionKind string,
		transcript []byte,
	) (*SummaryResult, error)
}

type SummaryResult struct {
	Content     string
	ModelId     string
	ModelParams any
}

type ErrorWithCode struct {
	message      string
	Code         string
	ProviderCode string
}

func (err ErrorWithCode) Error() string { return err.message }

const systemPrompt = `You are a cybersecurity analyst tasked with examining SSH
user session logs to identify and categorize user activities. Analyze all
commands executed, file operations, network connections, and system
modifications during the session. Highlight any potentially malicious
activities such as privilege escalation attempts, unauthorized file access,
suspicious network connections, data exfiltration, or the use of known attack
tools and techniques. Provide a clear summary of legitimate activities versus
concerning behaviors, and assign a risk level (low, medium, high) based on the
observed actions. Focus on identifying patterns that deviate from normal
administrative tasks or suggest unauthorized access attempts.`

type OpenAPIInferenceProvider struct {
	apiKey string
	model  openai.ChatModel
}

type Summarizer interface {
	Summarize(ctx context.Context, sessionID session.ID) error
}

type summarizer struct {
	cfg SummarizerConfig
}

type SummarizerConfig struct {
	Streamer player.Streamer
}

func NewSummarizer(cfg SummarizerConfig) Summarizer {
	return &summarizer{
		cfg: cfg,
	}
}

func (s *summarizer) Summarize(ctx context.Context, sessionID session.ID) error {
	eventsCh, errCh := s.cfg.Streamer.StreamSessionEvents(ctx, sessionID, 0)
	var kind string
	var participants []string
	var transcript []byte
reader:
	for {
		select {
		case event := <-eventsCh:
			if event == nil {
				break reader
			}
			switch e := event.(type) {
			case *events.SessionStart:
				kind = e.Protocol
				participants = append(participants, e.User)
			case *events.SessionJoin:
				participants = append(participants, e.User)
			case *events.SessionPrint:
				transcript = append(transcript, e.Data...)
			case *events.DatabaseSessionStart:
				kind = e.DatabaseProtocol
				participants = append(participants, e.User)
			case *events.DatabaseSessionQuery:
				transcript = append(transcript, []byte(e.DatabaseQuery)...)
			}
		case err := <-errCh:
			return trace.Wrap(err)
		}
	}
	provider := NewProvider()
	summary, err := provider.Summarize(ctx, kind, transcript)
	if err != nil {
		return err
	}
}

func NewProvider() InferenceProvider {
	return &OpenAPIInferenceProvider{
		apiKey: os.Getenv("OPENAI_API_KEY"),
		model:  openai.ChatModelGPT4o,
	}
}

func (p *OpenAPIInferenceProvider) Summarize(ctx context.Context, sessionKind string, transcript []byte) (*SummaryResult, error) {
	client := openai.NewClient(option.WithAPIKey(p.apiKey))
	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: p.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(string(transcript)),
		},
	})
	if err != nil {
		var apierr *openai.Error
		if errors.As(err, &apierr) {
			return nil, ErrorWithCode{
				message:      apierr.Error(),
				Code:         apierr.Code, // TODO: translate codes.
				ProviderCode: apierr.Code,
			}
		}
		return nil, err
	}
	return &SummaryResult{
		Content: completion.Choices[0].Message.Content,
		ModelId: p.model,
	}, nil
}
