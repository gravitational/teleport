package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

// fakeRecorder counts which structured output path each chat completion used, so tests can assert that the native
// json_schema attempt was made (or skipped) and how often the prompt-based json_object fallback ran.
type fakeRecorder struct {
	native int
	prompt int
}

type fakeClient struct {
	// recorder, when set, records the structured output path of each NewChatCompletion call.
	recorder *fakeRecorder
}

func (m *fakeClient) GenerateEmbeddings(ctx context.Context, input openai.EmbeddingNewParams, opts ...option.RequestOption) (*openai.CreateEmbeddingResponse, error) {
	text := input.Input.OfString.Value

	switch text {
	case "cause an error":
		return nil, errors.New("embeddings API error")
	default:
		return &openai.CreateEmbeddingResponse{
			Data: []openai.Embedding{
				{
					Embedding: []float64{0.1, 0.2, 0.3},
				},
			},
			Usage: openai.CreateEmbeddingResponseUsage{
				TotalTokens: int64(len(strings.Fields(text))),
			},
		}, nil
	}
}

func (m *fakeClient) NewChatCompletion(
	ctx context.Context, body openai.ChatCompletionNewParams, opts ...option.RequestOption,
) (*openai.ChatCompletion, error) {
	// The structured output path is distinguished by the response format: the native path requests json_schema, the
	// prompt-based fallback requests json_object.
	native := body.ResponseFormat.OfJSONSchema != nil
	if m.recorder != nil {
		if native {
			m.recorder.native++
		} else {
			m.recorder.prompt++
		}
	}

	firstUser := firstUserMessage(body)
	if firstUser == nil {
		return nil, errors.New("fake: no user message in request")
	}

	// Image requests (desktop screenshot analysis) carry their content as an array of parts.
	if len(firstUser.Content.OfArrayOfContentParts) > 0 {
		return handleOpenAIDesktopScreenshotAnalysis(body, firstUser)
	}

	// Dispatch on the first user message, which is stable across the prompt path's re-prompts (corrections are
	// appended after it), so a scenario behaves consistently whether it is the native or the fallback attempt.
	content := firstUser.Content.OfString.Value

	// "native-unsupported" simulates an endpoint that rejects json_schema but accepts json_object: the native attempt
	// errors and the prompt-based fallback succeeds, which is the signal makeStructuredRequest caches.
	if strings.Contains(content, "native-unsupported") {
		if native {
			return nil, errors.New("response_format json_schema is not supported by this endpoint")
		}
		return openAIStopCompletion(completeCommandAnalysisJSON())
	}

	switch content {
	case "no choices":
		return &openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{},
		}, nil
	case "cause an error":
		return openAIFinishCompletion(string(openai.CompletionChoiceFinishReasonContentFilter), "")
	case "make the output too long":
		return openAIFinishCompletion(string(openai.CompletionChoiceFinishReasonLength), "")
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
		return openAIStopCompletion(mustMarshal(ca))
	}

	// Check the system prompt to dispatch structured handlers for non-exact-string content. The prompt-based path
	// appends a schema instruction to the system prompt, so match with Contains rather than equality.
	systemContent := firstSystemMessage(body)
	if strings.Contains(systemContent, schema.GetProseEmbedding()) {
		return handleOpenAIProseEmbedding(content)
	}
	if strings.Contains(systemContent, "expert security analyst reviewing a Windows Desktop session") {
		return handleOpenAIDesktopSessionAnalysis(content)
	}

	return nil, errors.New("fake: unrecognized request")
}

// firstUserMessage returns the first user message in the request, or nil if there is none.
func firstUserMessage(body openai.ChatCompletionNewParams) *openai.ChatCompletionUserMessageParam {
	for i := range body.Messages {
		if u := body.Messages[i].OfUser; u != nil {
			return u
		}
	}
	return nil
}

// firstSystemMessage returns the text of the first system message in the request, or "" if there is none.
func firstSystemMessage(body openai.ChatCompletionNewParams) string {
	for i := range body.Messages {
		if s := body.Messages[i].OfSystem; s != nil {
			return s.Content.OfString.Value
		}
	}
	return ""
}

// openAIStopCompletion returns a single-choice completion that stopped normally with the given content.
func openAIStopCompletion(content string) (*openai.ChatCompletion, error) {
	return openAIFinishCompletion(string(openai.CompletionChoiceFinishReasonStop), content)
}

func openAIFinishCompletion(finishReason, content string) (*openai.ChatCompletion, error) {
	return &openai.ChatCompletion{
		Choices: []openai.ChatCompletionChoice{
			{
				FinishReason: finishReason,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: content,
				},
			},
		},
	}, nil
}

func mustMarshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// completeCommandAnalysisJSON returns a CommandAnalysis JSON document with every required field populated (arrays as
// empty slices, not null) and no server-only fields, so it passes schema validation on the prompt-based path. It is
// built from a map rather than by marshaling the struct because the struct's server-populated fields are tagged
// jsonschema:"-" but not json:"-", so marshaling them would emit properties the schema forbids.
func completeCommandAnalysisJSON() string {
	return mustMarshal(map[string]any{
		"command":             "ls -al",
		"category":            "file_operation",
		"success":             true,
		"risk_level":          "low",
		"risk_score":          10,
		"threat_category":     "none",
		"timeline_title":      "Listed directory contents",
		"timeline_subtitle":   "",
		"short_description":   "Listed all files in the directory",
		"description":         "The command 'ls -al' listed all files, including hidden ones, in the current directory.",
		"error_messages":      []string{},
		"suspicious_flags":    []string{},
		"sensitive_items":     []string{},
		"suspicious_patterns": []string{},
		"iocs":                []string{},
		//nolint:misspell // ignore MITRE
		"mitre_attack_ids":     []string{},
		"has_sensitive_data":   false,
		"privilege_escalation": false,
		"data_exfiltration":    false,
		"persistence":          false,
	})
}

func handleOpenAIDesktopScreenshotAnalysis(body openai.ChatCompletionNewParams, userMsg *openai.ChatCompletionUserMessageParam) (*openai.ChatCompletion, error) {
	for _, part := range userMsg.Content.OfArrayOfContentParts {
		if part.OfImageURL == nil {
			return nil, errors.New("expected image content part")
		}
		if !strings.HasPrefix(part.OfImageURL.ImageURL.URL, "data:image/png;base64,") {
			return nil, errors.New("expected image to be a base64 PNG data URL")
		}
		b64 := strings.TrimPrefix(part.OfImageURL.ImageURL.URL, "data:image/png;base64,")
		if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return nil, errors.New("image data URL is not valid base64")
		}
	}

	systemText := firstSystemMessage(body)
	if !strings.Contains(systemText, "start_screenshot_index") {
		return nil, errors.New("system prompt missing screenshot index instructions")
	}

	analysis := schema.DesktopScreenshotAnalysis{
		NotableSessionEvents: []schema.DesktopSessionEvent{
			{
				Category:             "data_access",
				StartScreenshotIndex: 0,
				EndScreenshotIndex:   0,
				RiskLevel:            "low",
				RiskScore:            15,
				ThreatCategory:       "none",
				TimelineTitle:        "Reviewed budget worksheet",
				TimelineSubtitle:     "",
				ShortDescription:     "Reviewed budget figures in a spreadsheet",
				DetailedDescription:  "User scrolled through a budget worksheet in Excel.",
				SuspiciousFlags:      []string{},
				SensitiveItems:       []string{},
				SuspiciousPatterns:   []string{},
				IOCs:                 []string{},
				MitreAttackIDs:       []string{},
				Applications:         []string{"Microsoft Excel"},
				VisibleURLs:          []string{},
				VisibleFilePaths:     []string{"/Users/test/budget.xlsx"},
				ActiveWindowTitle:    "budget.xlsx - Excel",
			},
		},
	}
	return openAIStopCompletion(mustMarshal(analysis))
}

func handleOpenAIDesktopSessionAnalysis(content string) (*openai.ChatCompletion, error) {
	if !strings.Contains(content, "Desktop Session Events") {
		return nil, errors.New("desktop synthesis prompt missing events block")
	}

	analysis := schema.DesktopSessionAnalysis{
		ShortDescription:     "Routine spreadsheet work",
		SessionDescription:   "Reviewed a budget worksheet in Excel without any sensitive operations.",
		SuspiciousActivities: []string{},
		SecurityIncidents:    []string{},
		CompromiseIndicators: false,
		RiskLevel:            "low",
		RiskScore:            15,
	}
	return openAIStopCompletion(mustMarshal(analysis))
}

func handleOpenAIProseEmbedding(content string) (*openai.ChatCompletion, error) {
	if strings.Contains(content, "trigger-api-error") {
		return nil, errors.New("prose embedding API error")
	}

	if strings.Contains(content, "trigger-bad-json") {
		return openAIStopCompletion("not valid json")
	}

	pe := &schema.ProseEmbedding{
		CondensedText: "A condensed description of the session for embedding generation.",
	}
	return openAIStopCompletion(mustMarshal(pe))
}
