package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/openai/openai-go/v3"

	summarizererrorstypes "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/structured"
	"github.com/gravitational/teleport/lib/session"
)

// maxStructuredRetries is the number of times the prompt-based (json_object) structured output path re-prompts the
// model after a response that cannot be parsed into the target schema, feeding the parse error back each time.
const maxStructuredRetries = 1

// makeStructuredRequest produces a schema-conforming response and returns it as a T. It first attempts OpenAI's native
// json_schema response format; on any failure it falls back to the prompt-based json_object path for that request.
//
// OpenAI-compatible endpoints return inconsistent errors, so rather than classifying which failures mean "json_schema
// unsupported", the fallback triggers on any native error. If the json_object fallback then succeeds, that is the
// evidence the endpoint does not support json_schema for this schema, so the verdict is remembered (per endpoint +
// schema, until the cache TTL) to skip the wasted native attempt on later requests.
func makeStructuredRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaName string,
	schema any,
	systemPrompt string,
	userMessages []openai.ChatCompletionMessageParamUnion,
) (T, *response, error) {
	var zero T

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return zero, nil, trace.Wrap(err)
	}
	schemaID := structured.SchemaFingerprint(schemaBytes)

	if p.structuredOutputCache.UseNative(structured.SupportUnknown, p.nativeSupportKey, schemaID) {
		out, res, err := makeNativeStructuredRequest[T](ctx, p, sessionID, schemaName, schema, systemPrompt, userMessages)
		if err == nil {
			return out, res, nil
		}

		p.logger.WarnContext(ctx, "Native structured output request failed; falling back to prompt-based structured output",
			"session_id", sessionID,
			"model", p.openAIModelName,
			"error", err,
		)

		out, res, err = makePromptStructuredRequest[T](ctx, p, sessionID, schemaBytes, systemPrompt, userMessages)
		if err != nil {
			return zero, nil, trace.Wrap(err)
		}

		// The native attempt failed but the json_object fallback succeeded: remember that this endpoint does not support
		// json_schema for this schema, so later requests skip straight to the prompt-based path until the verdict expires.
		p.structuredOutputCache.MarkUnsupported(p.nativeSupportKey, schemaID)

		return out, res, nil
	}

	return makePromptStructuredRequest[T](ctx, p, sessionID, schemaBytes, systemPrompt, userMessages)
}

// makeStructuredTextRequest delegates to makeStructuredRequest with a single-text user message.
func makeStructuredTextRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaName string,
	schema any,
	systemPrompt,
	message string,
) (T, *response, error) {
	return makeStructuredRequest[T](ctx, p, sessionID, schemaName, schema, systemPrompt, []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(message),
	})
}

// makeNativeStructuredRequest sends the request with OpenAI's native json_schema response format, which validates the
// model's JSON against the schema server-side, and returns it as a T.
func makeNativeStructuredRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaName string,
	schema any,
	systemPrompt string,
	userMessages []openai.ChatCompletionMessageParamUnion,
) (T, *response, error) {
	var out T

	completionParams := openai.ChatCompletionNewParams{
		Model:    p.openAIModelName,
		Messages: append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}, userMessages...),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   schemaName,
					Schema: schema,
					Strict: openai.Bool(true),
				},
			},
		},
	}

	res, err := p.makeRequest(ctx, sessionID, completionParams)
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

// promptConversation adapts an OpenAI json_object exchange to [structured.Conversation]: each Send issues the
// accumulated conversation and remembers the response so the caller can read its token usage, and each Correct appends
// the model's failed response and the parse error as a new assistant/user turn pair for the next attempt.
type promptConversation struct {
	p         *InferenceProvider
	sessionID session.ID
	params    openai.ChatCompletionNewParams
	last      *response
}

func (c *promptConversation) Send(ctx context.Context) (string, error) {
	res, err := c.p.makeRequest(ctx, c.sessionID, c.params)
	if err != nil {
		return "", trace.Wrap(err)
	}

	c.last = res

	return res.result, nil
}

func (c *promptConversation) Correct(modelResponse, parseError string) {
	c.params.Messages = append(c.params.Messages,
		openai.AssistantMessage(modelResponse),
		openai.UserMessage(fmt.Sprintf(
			"Your previous response could not be parsed as JSON matching the schema (%v). "+
				"Respond again with only the corrected JSON object and no other text.", parseError)),
	)
}

// makePromptStructuredRequest asks the model to produce schema-conforming JSON in json_object mode, describing the
// schema in the system prompt, then parses the response with lenient extraction and schema validation. If the response
// cannot be parsed, it re-prompts the model up to maxStructuredRetries times, feeding the parse error back so the model
// can correct itself, before returning a BadResponseError.
func makePromptStructuredRequest[T any](
	ctx context.Context,
	p *InferenceProvider,
	sessionID session.ID,
	schemaBytes []byte,
	systemPrompt string,
	userMessages []openai.ChatCompletionMessageParamUnion,
) (T, *response, error) {
	var zero T

	conv := &promptConversation{
		p:         p,
		sessionID: sessionID,
		params: openai.ChatCompletionNewParams{
			Model: p.openAIModelName,
			Messages: append([]openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(systemPrompt + structured.Instruction(schemaBytes)),
			}, userMessages...),
			ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: &openai.ResponseFormatJSONObjectParam{},
			},
		},
	}

	value, err := structured.Complete[T](ctx, conv, maxStructuredRetries)
	if err != nil {
		return zero, nil, trace.Wrap(err)
	}

	return value, conv.last, nil
}
