package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrocktypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go"
	"github.com/gravitational/trace"

	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/structured"
	"github.com/gravitational/teleport/lib/session"
)

// structuredOutputSupportedFamilies lists Bedrock model-family tokens known to support the native structured output API.
// Support is curated from the individual Bedrock model cards: the Anthropic Claude 4.5 family plus Claude Sonnet 4.6 and
// Opus 4.6 support it over the Converse API; later Opus models (4.7, 4.8) do not and are listed as unsupported below. The
// structured outputs overview (linked below) documents the feature but defers the per-model support list to the model
// cards, so these tokens are maintained by hand.
//
// Tokens are matched as case-insensitive substrings of the configured model ID so the classification works for base
// model IDs (anthropic.claude-opus-4-6-v1), geo and global cross-region inference profiles (us./eu./global. prefixes),
// and system-defined inference profile ARNs.
//
// Application inference profile ARNs are opaque (they do not embed the model name), so they cannot be classified here
// and instead fall to the optimistic path (the native API is attempted once and falls back if rejected).
//
// Feature overview: https://docs.aws.amazon.com/bedrock/latest/userguide/structured-output.html
var structuredOutputSupportedFamilies = []string{
	"claude-haiku-4-5",
	"claude-sonnet-4-5",
	"claude-opus-4-5",
	"claude-sonnet-4-6",
	"claude-opus-4-6",
}

// structuredOutputUnsupportedFamilies lists Bedrock model-family tokens known not to support the native structured
// output API. Matching a model here only avoids a wasted native attempt; correctness does not depend on this list being
// exhaustive, because unknown models that fail the optimistic native attempt fall back to the prompt-based path anyway.
var structuredOutputUnsupportedFamilies = []string{
	"amazon.nova",
	"amazon.titan",
	"meta.llama",
	"claude-3",
	"claude-opus-4-1",
	"claude-opus-4-7",
	"claude-opus-4-8",
}

// classifyStructuredOutput reports whether the given Bedrock model ID is known to support, known not to support, or has
// unknown support for the native structured output API.
func classifyStructuredOutput(modelID string) structured.NativeSupport {
	id := strings.ToLower(modelID)
	for _, family := range structuredOutputSupportedFamilies {
		if strings.Contains(id, family) {
			return structured.SupportYes
		}
	}

	for _, family := range structuredOutputUnsupportedFamilies {
		if strings.Contains(id, family) {
			return structured.SupportNo
		}
	}

	return structured.SupportUnknown
}

// maxStructuredRetries is the number of times the prompt-based structured output path re-prompts the model after a
// response that cannot be parsed into the target schema, feeding the parse error back to the model each time.
const maxStructuredRetries = 1

// makeStructuredRequest produces a schema-conforming response and returns it as a T. Models known to support Bedrock's
// native structured output API use it, falling back to the prompt-based path if the native attempt fails in a way the
// prompt-based path could recover from. Models of unknown support are tried optimistically once and remembered as
// unsupported on failure. Models known not to support it use the prompt-based path directly.
func makeStructuredRequest[T any](ctx context.Context, p *InferenceProvider, sessionID session.ID, schemaName string, schema any, systemPrompt string, messages []bedrocktypes.Message) (T, *response, error) {
	var zero T

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return zero, nil, trace.Wrap(err)
	}
	schemaID := structured.SchemaFingerprint(schemaBytes)

	if p.structuredOutputCache.UseNative(p.structuredOutput, p.bedrockModelID, schemaID) {
		out, res, err := makeNativeStructuredRequest[T](ctx, p, sessionID, schemaName, schemaBytes, systemPrompt, messages)
		if err == nil {
			return out, res, nil
		}
		if !shouldFallbackToPrompt(err) {
			return zero, nil, trace.Wrap(err)
		}

		// For models of unknown support, remember that the native API is not usable for this schema so later requests (in
		// this session and, via the shared cache, in later ones until its TTL) skip straight to the prompt-based path. Only a
		// ValidationException is a durable "unsupported" signal; a transient bad response falls back for this request alone.
		if p.structuredOutput == structured.SupportUnknown && nativeUnsupportedByModel(err) {
			p.structuredOutputCache.MarkUnsupported(p.bedrockModelID, schemaID)
		}

		p.logger.WarnContext(ctx, "Native structured output request failed; falling back to prompt-based structured output",
			"session_id", sessionID,
			"model", p.bedrockModelID,
			"error", err,
		)
	}

	return makePromptStructuredRequest[T](ctx, p, sessionID, schemaBytes, systemPrompt, messages)
}

// makeNativeStructuredRequest invokes Converse with OutputConfig so Bedrock validates the model's JSON response against
// the provided schema server-side, and returns it as a T.
func makeNativeStructuredRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaName string,
	schemaBytes []byte,
	systemPrompt string,
	messages []bedrocktypes.Message,
) (T, *response, error) {
	var out T

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
		Messages: messages,
		OutputConfig: &bedrocktypes.OutputConfig{
			TextFormat: &bedrocktypes.OutputFormat{
				Type: bedrocktypes.OutputFormatTypeJsonSchema,
				Structure: &bedrocktypes.OutputFormatStructureMemberJsonSchema{
					Value: bedrocktypes.JsonSchemaDefinition{
						Name:   new(schemaName),
						Schema: new(string(schemaBytes)),
					},
				},
			},
		},
	}

	res, err := p.makeRequest(ctx, sessionID, &convInput)
	if err != nil {
		return out, nil, trace.Wrap(err)
	}

	if err := json.Unmarshal([]byte(res.result), &out); err != nil {
		return out, nil, trace.Wrap(summarizererrorstypes.BadResponseError{
			Message: fmt.Sprintf("failed to unmarshal model response: %v", err),
		})
	}

	return out, res, nil
}

// promptConversation adapts a Bedrock Converse exchange to [structured.Conversation]: each Send issues the accumulated
// conversation and remembers the response so the caller can read its token usage, and each Correct appends the model's
// failed response and the parse error as a new assistant/user turn pair for the next attempt.
type promptConversation struct {
	p         *InferenceProvider
	sessionID session.ID
	input     bedrockruntime.ConverseInput
	last      *response
}

func (c *promptConversation) Send(ctx context.Context) (string, error) {
	res, err := c.p.makeRequest(ctx, c.sessionID, &c.input)
	if err != nil {
		return "", trace.Wrap(err)
	}

	c.last = res

	return res.result, nil
}

func (c *promptConversation) Correct(modelResponse, parseError string) {
	// Append the model's response and the parse error so the next attempt can correct itself. Bedrock's Converse API
	// requires alternating roles, so the assistant turn must sit between the user turns.
	c.input.Messages = append(c.input.Messages,
		bedrocktypes.Message{
			Role: bedrocktypes.ConversationRoleAssistant,
			Content: []bedrocktypes.ContentBlock{
				&bedrocktypes.ContentBlockMemberText{Value: modelResponse},
			},
		},
		bedrocktypes.Message{
			Role: bedrocktypes.ConversationRoleUser,
			Content: []bedrocktypes.ContentBlock{
				&bedrocktypes.ContentBlockMemberText{Value: fmt.Sprintf(
					"Your previous response could not be parsed as JSON matching the schema (%v). "+
						"Respond again with only the corrected JSON object and no other text.", parseError)},
			},
		},
	)
}

// makePromptStructuredRequest asks the model to produce schema-conforming JSON by describing the schema in the system
// prompt, then parses the response with lenient extraction and schema validation. If the response cannot be parsed, it
// re-prompts the model up to maxStructuredRetries times, feeding the parse error back so the model can correct itself,
// before returning a BadResponseError.
func makePromptStructuredRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaBytes []byte,
	systemPrompt string,
	messages []bedrocktypes.Message,
) (T, *response, error) {
	var zero T

	conv := &promptConversation{
		p:         p,
		sessionID: sessionID,
		input: bedrockruntime.ConverseInput{
			ModelId: &p.bedrockModelID,
			InferenceConfig: &bedrocktypes.InferenceConfiguration{
				MaxTokens: aws.Int32(maxCompletionTokens),
			},
			System: []bedrocktypes.SystemContentBlock{
				&bedrocktypes.SystemContentBlockMemberText{
					Value: systemPrompt + structured.Instruction(schemaBytes),
				},
			},
			Messages: messages,
		},
	}

	value, err := structured.Complete[T](ctx, conv, maxStructuredRetries)
	if err != nil {
		return zero, nil, trace.Wrap(err)
	}

	return value, conv.last, nil
}

// shouldFallbackToPrompt reports whether an error from a native structured output attempt is one that the prompt-based
// path could plausibly recover from. Schema-validation rejections (the model or schema is incompatible with the native
// API) and malformed responses warrant a fallback; transient or fatal errors (throttling, timeouts, access denial,
// exceeded length) are surfaced unchanged because re-issuing the request without OutputConfig would not help.
func shouldFallbackToPrompt(err error) bool {
	// A malformed or unusable response from the native path.
	if _, ok := errors.AsType[summarizererrorstypes.BadResponseError](err); ok {
		return true
	}

	// Bedrock rejected the request as invalid; the prompt-based path does not use OutputConfig, so it may still succeed.
	return nativeUnsupportedByModel(err)
}

// nativeUnsupportedByModel reports whether err is a durable signal that the model does not support the native structured
// output API for the requested schema (a ValidationException), as opposed to a transient failure that should not be
// cached as an "unsupported" verdict.
func nativeUnsupportedByModel(err error) bool {
	apiErr, ok := errors.AsType[smithy.APIError](err)

	return ok && apiErr.ErrorCode() == "ValidationException"
}
