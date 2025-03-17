package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	sessionrecordingmetatadav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
	apiEvents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

func WrapSessionHandler(wrapped events.MultipartHandler, streamer player.Streamer) *Summarizer {
	return &Summarizer{wrapped: wrapped}
}

type Summarizer struct {
	wrapped                  events.MultipartHandler
	Streamer                 player.Streamer
	SessionRecordingMetadata services.SessionRecordingMetadata
}

func (s *Summarizer) Upload(ctx context.Context, sessionID session.ID, readCloser io.Reader) (string, error) {
	return s.wrapped.Upload(ctx, sessionID, readCloser)
}

func (s *Summarizer) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	return s.wrapped.Download(ctx, sessionID, writer)
}

func (s *Summarizer) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	return s.wrapped.CreateUpload(ctx, sessionID)
}

func (s *Summarizer) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	err := s.wrapped.CompleteUpload(ctx, upload, parts)
	if err != nil {
		return trace.Wrap(err)
	}
	eventsCh, errCh := s.Streamer.StreamSessionEvents(ctx, upload.SessionID, 0)
	sb := strings.Builder{}
	var proto string
	var user string
reader:
	for {
		select {
		case event := <-eventsCh:
			if event == nil {
				break reader
			}
			switch e := event.(type) {
			case *apiEvents.SessionStart:
				proto = e.Protocol
				user = e.User
			case *apiEvents.SessionPrint:
				sb.Write(e.Data)
			}
		case err := <-errCh:
			return trace.Wrap(err)
		}
	}
	srm := &sessionrecordingmetatadav1.SessionRecordingMetadata{
		Metadata: &v1.Metadata{Name: string(upload.SessionID)},
		Spec: &sessionrecordingmetatadav1.SessionRecordingMetadataSpec{
			User: user,
			Kind: proto,
		},
	}
	mode := os.Getenv("SUMMARIZATION_MODE")
	switch mode {
	case "openapi-batch":
		srm.Spec.BatchId, err = sendBatch(ctx, proto, sb.String())
		if err != nil {
			return trace.Wrap(err)
		}

	case "openai":
		srm.Spec.Summary, err = generateSummaryUsingOpenAI(ctx, proto, sb.String())
		if err != nil {
			return trace.Wrap(err)
		}

	case "ollama":
	default:
		srm.Spec.Summary, err = generateSummaryUsingOllama(ctx, proto, sb.String())
		if err != nil {
			return trace.Wrap(err)
		}

	}
	_, err = s.SessionRecordingMetadata.CreateSessionRecordingMetadata(ctx, srm)
	return trace.Wrap(err)
}

func (s *Summarizer) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return s.wrapped.ReserveUploadPart(ctx, upload, partNumber)
}

func (s *Summarizer) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	return s.wrapped.UploadPart(ctx, upload, partNumber, partBody)
}

func (s *Summarizer) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	return s.wrapped.ListParts(ctx, upload)
}

func (s *Summarizer) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	return s.wrapped.ListUploads(ctx)
}

func (s *Summarizer) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	return s.wrapped.GetUploadMetadata(sessionID)
}

func sendBatch(ctx context.Context, sessionType string, sessionContent string) (string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	prompt := summarizationPrompt(sessionType, sessionContent)

	response, err := client.CreateBatchWithUploadFile(ctx, openai.CreateBatchWithUploadFileRequest{
		Endpoint:         openai.BatchEndpointChatCompletions,
		CompletionWindow: "24h",
		UploadBatchFileRequest: openai.UploadBatchFileRequest{
			Lines: []openai.BatchLineItem{
				openai.BatchChatCompletionRequest{
					CustomID: "request",
					Method:   "POST",
					URL:      openai.BatchEndpointChatCompletions,
					Body: openai.ChatCompletionRequest{
						Model: openai.GPT4o,
						Messages: []openai.ChatCompletionMessage{
							{
								Role:    openai.ChatMessageRoleUser,
								Content: prompt,
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return response.ID, nil
}

func generateSummaryUsingOpenAI(ctx context.Context, sessionType string, sessionContent string) (string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	prompt := summarizationPrompt(sessionType, sessionContent)

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Choices[0].Message.Content, nil
}

func generateSummaryUsingOllama(ctx context.Context, sessionType string, sessionContent string) (string, error) {
	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434/api/generate"
	}

	prompt := summarizationPrompt(sessionType, sessionContent)

	requestBody := OllamaRequest{
		Model:       "deepseek-coder",
		Prompt:      prompt,
		Stream:      false,
		Temperature: 0.2,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Send request to Ollama
	resp, err := http.Post(ollamaURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Parse response
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", trace.Wrap(err)
	}

	return ollamaResp.Response, nil
}

// OllamaRequest represents the request body for Ollama API
type OllamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
}

// OllamaResponse represents the response from Ollama API
type OllamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func summarizationPrompt(sessionType string, sessionContent string) string {
	return fmt.Sprintf(
		`Summarize this Teleport session recording:
Type: %s

Session Content:
%s

Please provide a concise summary of what commands were executed, their purpose, and any notable details in 3-5 sentences.
If this session is a Kubernetes session, note the relevant pod/cluster/namespace if present.`,
		sessionType,
		sessionContent,
	)
}
