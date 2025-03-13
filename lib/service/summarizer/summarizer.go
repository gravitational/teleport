package summarizer

import (
	"context"
	"fmt"
	"io"
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
reader:
	for {
		select {
		case event := <-eventsCh:
			if event == nil {
				break reader
			}
			if event.GetType() == events.SessionPrintEvent {
				printEvent, ok := event.(*apiEvents.SessionPrint)
				if ok {
					sb.Write(printEvent.Data)
				}
			}
		case err := <-errCh:
			return trace.Wrap(err)
		}
	}
	srm := &sessionrecordingmetatadav1.SessionRecordingMetadata{
		Metadata: &v1.Metadata{Name: string(upload.SessionID)},
		Spec:     &sessionrecordingmetatadav1.SessionRecordingMetadataSpec{},
	}
	useBatchMode := os.Getenv("SUMMARIZATION_BATCH_MODE") != ""
	if useBatchMode {
		srm.Spec.BatchId, err = sendBatch(ctx, "ssh", sb.String())
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		srm.Spec.Summary, err = generateSummary(ctx, "ssh", sb.String())
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

// func

func generateSummary(ctx context.Context, sessionType string, sessionContent string) (string, error) {
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
