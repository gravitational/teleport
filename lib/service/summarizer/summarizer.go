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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

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
			case *apiEvents.DatabaseSessionStart:
				proto = e.DatabaseProtocol
				user = e.User
			case *apiEvents.DatabaseSessionQuery:
				sb.Write([]byte(e.DatabaseQuery))
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
		srm.Spec.Summary, err = generateSummaryUsingOllama(ctx, proto, sb.String())
		if err != nil {
			return trace.Wrap(err)
		}

	case "bedrock":
		srm.Spec.Summary, err = generateSummaryUsingBedrock(ctx, proto, sb.String())
		if err != nil {
			return trace.Wrap(err)
		}

	default:
		srm.Spec.Summary, err = generateSummaryUsingOpenAI(ctx, proto, sb.String())
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

func generateSummaryUsingBedrock(ctx context.Context, sessionType string, sessionContent string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		return "", trace.Wrap(err)
	}

	bedrockClient := bedrockruntime.NewFromConfig(cfg)

	claudeRequest := ClaudeRequest{
		Messages: []ClaudeMessage{
			{
				Role:    "user",
				Content: summarizationPrompt(sessionType, sessionContent),
			},
		},
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        1000,
		Temperature:      0.7,
	}

	requestBytes, err := json.Marshal(claudeRequest)
	if err != nil {
		return "", trace.Wrap(err)
	}

	modelID := "anthropic.claude-3-5-sonnet-20241022-v2:0"

	output, err := bedrockClient.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Body:        requestBytes,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	var response ClaudeResponse
	if err := json.Unmarshal(output.Body, &response); err != nil {
		return "", trace.Wrap(err)
	}

	if len(response.Content) > 0 {
		return response.Content[0].Text, nil
	}

	return "", trace.BadParameter("model returned no response")
}

func summarizationPrompt(sessionType string, sessionContent string) string {
	switch sessionType {
	case "postgres":
		return fmt.Sprintf(
			`You are a database security analyst specialized in reviewing
		 user session logs. Analyze the provided session details, including
		 queries executed, access patterns, data retrieved, and time of access.
		 Identify any potential security concerns such as unusual query
		 patterns, excessive privilege usage, access to sensitive tables.
		 Present your findings in a clear and concise summary in 3-5 sentences
		 highlighting both normal user activities and potential malicious
		 behaviors with specific recommendations for further investigation.
		 The session content starts below.

		 %s`, sessionContent)
	default:
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
}

// Claude message structure
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Claude request payload
type ClaudeRequest struct {
	Messages         []ClaudeMessage `json:"messages"`
	AnthropicVersion string          `json:"anthropic_version"`
	MaxTokens        int             `json:"max_tokens"`
	Temperature      float64         `json:"temperature,omitempty"`
	TopP             float64         `json:"top_p,omitempty"`
	TopK             int             `json:"top_k,omitempty"`
	StopSequences    []string        `json:"stop_sequences,omitempty"`
	System           string          `json:"system,omitempty"`
}

// Claude response structure
type ClaudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
