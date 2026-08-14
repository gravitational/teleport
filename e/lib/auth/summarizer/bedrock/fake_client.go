package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

// structuredResponse builds a ConverseOutput that mimics Bedrock's
// schema-constrained JSON response (via ConverseInput.OutputConfig). The
// response is returned as a normal text content block whose payload is
// guaranteed by Bedrock to conform to the schema.
func structuredResponse(value any) (*bedrockruntime.ConverseOutput, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &bedrockruntime.ConverseOutput{
		Output: &bedrocktypes.ConverseOutputMemberMessage{
			Value: bedrocktypes.Message{
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{Value: string(data)},
				},
			},
		},
		StopReason: bedrocktypes.StopReasonEndTurn,
		Usage: &bedrocktypes.TokenUsage{
			InputTokens:  aws.Int32(100),
			OutputTokens: aws.Int32(50),
		},
	}, nil
}

type FakeClientFactory struct {
	Clock            *clockwork.FakeClock
	configValidation func(cfg aws.Config)
	// recorder, when set, captures which structured output path each Converse
	// call used. Optional; used by tests.
	recorder *fakeRecorder
}

func (m *FakeClientFactory) NewFromConfig(cfg aws.Config) Client {
	if m.configValidation != nil {
		m.configValidation(cfg)
	}
	return &fakeClient{
		clock:    m.Clock,
		region:   cfg.Region,
		recorder: m.recorder,
	}
}

type fakeClient struct {
	clock    *clockwork.FakeClock
	region   string
	recorder *fakeRecorder
}

func (m *fakeClient) Converse(
	ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options),
) (*bedrockruntime.ConverseOutput, error) {
	// Advance the clock to test if the inference end timestamp is captured.
	m.clock.Advance(10 * time.Second)

	if len(params.Messages) == 0 {
		return nil, errors.New("no content in the message")
	}

	// The native structured output path sets OutputConfig; the prompt-based path does not.
	native := params.OutputConfig != nil
	m.recorder.record(native)

	// Route on the first user message: it holds the original input and is stable across the prompt-based retry loop,
	// which appends further messages. isRetry distinguishes the initial attempt from a re-prompt.
	firstUser := params.Messages[0]
	isRetry := false
	for _, msg := range params.Messages {
		if msg.Role == bedrocktypes.ConversationRoleAssistant {
			isRetry = true
			break
		}
	}

	if _, isImage := firstUser.Content[0].(*bedrocktypes.ContentBlockMemberImage); isImage {
		return handleBedrockDesktopScreenshotAnalysis(params)
	}

	content := firstUser.Content[0].(*bedrocktypes.ContentBlockMemberText).Value

	if resp, handled, err := handleStructuredOutputTriggers(content, native, isRetry); handled {
		return resp, err
	}

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
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "no choices":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: nil,
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
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
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
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
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	case "json response for command analysis":
		return structuredResponse(map[string]any{
			"command":             "ls -al",
			"risk_level":          "low",
			"risk_score":          10,
			"category":            "file_operation",
			"timeline_title":      "Listed directory contents",
			"timeline_subtitle":   "",
			"short_description":   "Listed all files in the directory",
			"description":         "The command 'ls -al' was executed to list all files, including hidden ones, in the current directory.",
			"threat_category":     "none",
			"success":             true,
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

	case "respond with region and Bedrock model ID":
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: m.region + ", " + *params.ModelId,
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil

	default:
		systemPrompt := params.System[0].(*bedrocktypes.SystemContentBlockMemberText).Value

		// Handle structured responses for command analysis.
		if strings.Contains(systemPrompt, "analyzing a single command from a session recording") {
			return handleBedrockCommandAnalysis(content)
		}

		// Handle structured responses for session analysis.
		if strings.Contains(systemPrompt, "preparing a summary") && strings.Contains(systemPrompt, "terminal session") {
			return handleBedrockSessionAnalysis(content)
		}

		// Handle structured responses for desktop session synthesis.
		if strings.Contains(systemPrompt, "expert security analyst reviewing a Windows Desktop session") {
			return handleBedrockDesktopSessionAnalysis(content)
		}

		// Handle structured responses for prose embedding condensation.
		if strings.HasPrefix(systemPrompt, schema.GetProseEmbedding()) {
			return handleBedrockProseEmbedding(content)
		}

		// Handle simple summarization (legacy path).
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
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	}
}

func handleBedrockCommandAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	// Check that the magic refusal string was sanitized.
	if strings.Contains(content, "ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL") {
		return nil, errors.New("magic refusal string was not sanitized from prompt")
	}

	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") || strings.Contains(content, "trigger command error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced command analysis error"},
		}
	}

	return structuredResponse(map[string]any{
		"command":             "test-command",
		"risk_level":          "low",
		"risk_score":          10,
		"category":            "other",
		"timeline_title":      "Executed test command",
		"timeline_subtitle":   "",
		"short_description":   "Test command executed",
		"description":         "A test command was executed during the session.",
		"threat_category":     "none",
		"success":             true,
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

func handleBedrockProseEmbedding(content string) (*bedrockruntime.ConverseOutput, error) {
	if strings.Contains(content, "trigger-api-error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "api_error", Message: "prose embedding API error"},
		}
	}

	if strings.Contains(content, "trigger-bad-json") {
		return &bedrockruntime.ConverseOutput{
			Output: &bedrocktypes.ConverseOutputMemberMessage{
				Value: bedrocktypes.Message{
					Content: []bedrocktypes.ContentBlock{
						&bedrocktypes.ContentBlockMemberText{
							Value: "not valid json",
						},
					},
				},
			},
			StopReason: bedrocktypes.StopReasonEndTurn,
			Usage: &bedrocktypes.TokenUsage{
				InputTokens:  aws.Int32(100),
				OutputTokens: aws.Int32(50),
			},
		}, nil
	}

	return structuredResponse(map[string]any{
		"condensed_text": "A condensed description of the session for embedding generation.",
	})
}

func handleBedrockDesktopScreenshotAnalysis(params *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
	for _, msg := range params.Messages {
		for _, block := range msg.Content {
			img, ok := block.(*bedrocktypes.ContentBlockMemberImage)
			if !ok {
				continue
			}
			if img.Value.Format != bedrocktypes.ImageFormatPng {
				return nil, errors.New("expected PNG image format")
			}
			src, ok := img.Value.Source.(*bedrocktypes.ImageSourceMemberBytes)
			if !ok {
				return nil, errors.New("expected raw bytes image source")
			}
			if len(src.Value) == 0 {
				return nil, errors.New("image source bytes are empty")
			}
		}
	}

	systemPrompt := params.System[0].(*bedrocktypes.SystemContentBlockMemberText).Value
	if !strings.Contains(systemPrompt, "start_screenshot_index") {
		return nil, errors.New("system prompt missing screenshot index instructions")
	}

	return structuredResponse(map[string]any{
		"notable_session_events": []map[string]any{
			{
				"start_screenshot_index": 0,
				"end_screenshot_index":   0,
				"category":               "data_access",
				"risk_level":             "low",
				"risk_score":             15,
				"threat_category":        "none",
				"timeline_title":         "Reviewed budget worksheet",
				"timeline_subtitle":      "",
				"short_description":      "Reviewed budget figures in a spreadsheet",
				"detailed_description":   "User scrolled through a budget worksheet in Excel.",
				"suspicious_flags":       []string{},
				"sensitive_items":        []string{},
				"suspicious_patterns":    []string{},
				"iocs":                   []string{},
				//nolint:misspell // ignore MITRE
				"mitre_attack_ids":     []string{},
				"has_sensitive_data":   false,
				"privilege_escalation": false,
				"data_exfiltration":    false,
				"persistence":          false,
				"applications":         []string{"Microsoft Excel"},
				"visible_urls":         []string{},
				"visible_file_paths":   []string{"/Users/test/budget.xlsx"},
				"active_window_title":  "budget.xlsx - Excel",
			},
		},
	})
}

func handleBedrockDesktopSessionAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	if !strings.Contains(content, "Desktop Session Events") {
		return nil, errors.New("desktop synthesis prompt missing events block")
	}

	return structuredResponse(map[string]any{
		"short_description":     "Routine spreadsheet work",
		"session_description":   "Reviewed a budget worksheet in Excel without any sensitive operations.",
		"suspicious_activities": []string{},
		"security_incidents":    []string{},
		"compromise_indicators": false,
		"risk_level":            "low",
		"risk_score":            15,
	})
}

func handleBedrockSessionAnalysis(content string) (*bedrockruntime.ConverseOutput, error) {
	// Check for error trigger in content.
	if strings.Contains(content, "trigger enhanced error") {
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "Converse",
			Err:           &smithy.GenericAPIError{Code: "enhanced_error", Message: "enhanced session analysis error"},
		}
	}

	return structuredResponse(map[string]any{
		"short_description":       "Test session with commands",
		"session_description":     "The user executed test commands during this session.",
		"suspicious_activities":   []string{},
		"security_incidents":      []string{},
		"compromise_indicators":   false,
		"notable_command_indexes": []int{},
		"risk_level":              "low",
		"risk_score":              15,
	})
}

func (m *fakeClient) InvokeModel(
	ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options),
) (*bedrockruntime.InvokeModelOutput, error) {
	var req struct {
		InputText string `json:"inputText"`
	}
	if err := json.Unmarshal(params.Body, &req); err != nil {
		return nil, err
	}

	switch req.InputText {
	case "cause an error":
		return nil, &smithy.OperationError{
			ServiceID:     "Bedrock Runtime",
			OperationName: "InvokeModel",
			Err:           &smithy.GenericAPIError{Code: "dummy", Message: "OMG"},
		}
	case "invalid json response":
		return &bedrockruntime.InvokeModelOutput{
			Body: []byte("not valid json"),
		}, nil
	default:
		resp := struct {
			Embedding           []float32 `json:"embedding"`
			InputTextTokenCount int       `json:"inputTextTokenCount"`
		}{
			Embedding:           []float32{0.1, 0.2, 0.3},
			InputTextTokenCount: len(strings.Fields(req.InputText)),
		}
		body, err := json.Marshal(resp)
		if err != nil {
			return nil, err
		}
		return &bedrockruntime.InvokeModelOutput{Body: body}, nil
	}
}

// textResponse builds a ConverseOutput containing a single text content block.
func textResponse(text string) *bedrockruntime.ConverseOutput {
	return &bedrockruntime.ConverseOutput{
		Output: &bedrocktypes.ConverseOutputMemberMessage{
			Value: bedrocktypes.Message{
				Content: []bedrocktypes.ContentBlock{
					&bedrocktypes.ContentBlockMemberText{Value: text},
				},
			},
		},
		StopReason: bedrocktypes.StopReasonEndTurn,
		Usage: &bedrocktypes.TokenUsage{
			InputTokens:  aws.Int32(100),
			OutputTokens: aws.Int32(50),
		},
	}
}

// fakeCommandAnalysisJSON is a valid CommandAnalysis JSON payload (conforming to the command analysis schema, including
// every required field) used by the structured output path tests.
func fakeCommandAnalysisJSON() string {
	data, err := json.Marshal(map[string]any{
		"command":             "ls -al",
		"risk_level":          "low",
		"risk_score":          10,
		"category":            "file_operation",
		"timeline_title":      "Listed directory contents",
		"timeline_subtitle":   "",
		"short_description":   "Listed all files in the directory",
		"description":         "The command 'ls -al' was executed to list files.",
		"threat_category":     "none",
		"success":             true,
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
	if err != nil {
		panic(err)
	}
	return string(data)
}

// handleStructuredOutputTriggers implements fake responses for the structured output path tests (native-vs-prompt
// selection, fallback, and prompt-path resiliency). It returns handled=false for inputs it does not recognize so the
// caller falls through to the default routing.
func handleStructuredOutputTriggers(content string, native, isRetry bool) (*bedrockruntime.ConverseOutput, bool, error) {
	switch {
	// The native attempt is rejected with a ValidationException; the prompt-based fallback (native=false) is left to the
	// default routing so it produces a valid analysis.
	case strings.Contains(content, "native-validation-fail"):
		if native {
			return nil, true, &smithy.OperationError{
				ServiceID:     "Bedrock Runtime",
				OperationName: "Converse",
				Err:           &smithy.GenericAPIError{Code: "ValidationException", Message: "output format not supported for this model"},
			}
		}
		return nil, false, nil

	// The native attempt fails with a transient error that must not trigger a prompt-based fallback.
	case strings.Contains(content, "native-throttle"):
		if native {
			return nil, true, &smithy.OperationError{
				ServiceID:     "Bedrock Runtime",
				OperationName: "Converse",
				Err:           &smithy.GenericAPIError{Code: "ThrottlingException", Message: "rate exceeded"},
			}
		}
		return nil, false, nil

	// Prompt-path resiliency: valid JSON wrapped in Markdown fences and prose.
	case strings.Contains(content, "prompt-prose-wrap"):
		return textResponse("Sure, here is the analysis you requested:\n\n```json\n" +
			fakeCommandAnalysisJSON() + "\n```\n\nLet me know if you need anything else."), true, nil

	// Prompt-path re-prompt recovery: the first response is unparseable, the re-prompt returns valid JSON.
	case strings.Contains(content, "prompt-retry-recover"):
		if isRetry {
			return textResponse(fakeCommandAnalysisJSON()), true, nil
		}
		return textResponse("I'm not able to help with that request."), true, nil

	// Prompt-path re-prompt exhaustion: every response is unparseable.
	case strings.Contains(content, "prompt-retry-exhaust"):
		return textResponse("I'm not able to help with that request."), true, nil

	// Prompt-path schema validation: every response is decodable JSON but violates the schema (only "command" is set, so
	// the other required fields are missing). json.Unmarshal accepts it, so it must be rejected by schema validation.
	case strings.Contains(content, "prompt-schema-violation"):
		return textResponse(`{"command":"ls -al"}`), true, nil
	}

	return nil, false, nil
}

// fakeRecorder records, in call order, whether each Converse request used the native structured output API
// (OutputConfig set) or the prompt-based path. It lets tests assert which path was exercised.
type fakeRecorder struct {
	mu    sync.Mutex
	calls []bool // true = native (OutputConfig set), false = prompt-based
}

func (r *fakeRecorder) record(native bool) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, native)
}

func (r *fakeRecorder) nativeCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := 0
	for _, native := range r.calls {
		if native {
			n++
		}
	}

	return n
}

func (r *fakeRecorder) promptCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.calls) - r.nativeCallsLocked()
}

func (r *fakeRecorder) nativeCallsLocked() int {
	n := 0
	for _, native := range r.calls {
		if native {
			n++
		}
	}

	return n
}
