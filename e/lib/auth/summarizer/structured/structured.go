package structured

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
)

const instructionText = "\n\nGenerate a JSON response that conforms to the provided JSON schema. " +
	"If a required field cannot be determined from the input, use the most conservative valid value the schema allows " +
	"(for example \"none\" or \"other\" for enums, 0 for numbers, false for booleans, or an empty array or string) " +
	"rather than null. " +
	"Respond with only the JSON object, with no surrounding prose, explanation, or Markdown code fences.\n\n" +
	"JSON schema:\n"

// Instruction is appended to a system prompt to ask a model that lacks a native structured output API to
// emit schema-conforming JSON.
func Instruction(schema []byte) string {
	return instructionText + string(schema)
}

// Conversation is an in-progress exchange with a model that [Complete] drives. Send issues the current
// conversation and returns the model's raw text response; Correct records a response that failed to parse,
// together with the reason, so the next Send re-prompts the model to correct itself.
type Conversation interface {
	// Send issues the current conversation to the model and returns its raw text response.
	Send(ctx context.Context) (string, error)
	// Correct appends the model's unusable response and the parse error to the conversation so the next Send
	// re-prompts the model to correct itself.
	Correct(modelResponse, parseError string)
}

// Complete drives conv until the model returns text that parses and validates as T, re-prompting on failure
// up to maxRetries times by feeding the parse error back through conv.Correct. A send error is returned
// unchanged; a response that never parses yields a [summarizererrors.BadResponseError] after the retries are
// spent.
func Complete[T any](ctx context.Context, conv Conversation, maxRetries int) (T, error) {
	var zero T

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		raw, err := conv.Send(ctx)
		if err != nil {
			return zero, trace.Wrap(err)
		}

		value, err := Parse[T](raw)
		if err == nil {
			return value, nil
		}
		lastErr = err

		conv.Correct(raw, err.Error())
	}

	return zero, trace.Wrap(summarizererrors.BadResponseError{
		Message: fmt.Sprintf("failed to unmarshal model response after %d retries: %v", maxRetries, lastErr),
	})
}

// Parse extracts a JSON object from a model's raw text — tolerating Markdown code fences and surrounding
// prose — unmarshals it into a fresh T, and validates it against T's schema. Validation enforces the required,
// enum, and additionalProperties constraints that struct unmarshaling ignores, matching the server-side
// validation a native structured output API performs.
func Parse[T any](raw string) (T, error) {
	value, candidate, err := unmarshalLoose[T](raw)
	if err != nil {
		return value, trace.Wrap(err)
	}

	if err := Validate[T]([]byte(candidate)); err != nil {
		return value, trace.Wrap(err)
	}

	return value, nil
}

// unmarshalLoose unmarshals a model's JSON response into a fresh T, tolerating responses that wrap the JSON
// in Markdown code fences or surrounding prose. It tries the raw string first, then a Markdown-stripped
// version, then the first complete JSON object embedded in prose, returning the parsed value and the raw JSON
// candidate it accepted so callers can validate that exact text against a schema. Each attempt decodes into a
// new value, so a partially-decoded failed attempt never leaks fields into the result.
func unmarshalLoose[T any](s string) (T, string, error) {
	var zero T

	candidates := []string{
		s,
		stripMarkdownCodeBlock(s),
		extractJSON(s),
	}

	lastErr := errors.New("model returned an empty response")
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		var out T
		if err := json.Unmarshal([]byte(candidate), &out); err != nil {
			lastErr = err
			continue
		}

		return out, candidate, nil
	}

	return zero, "", lastErr
}

// stripMarkdownCodeBlock removes Markdown code block formatting from a string. Models sometimes wrap JSON in
// Markdown code blocks, e.g.:
//
//	```json
//	{
//	  "key": "value"
//	}
//	```
func stripMarkdownCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
}

// extractJSON returns the first complete JSON object embedded in s. It scans from the first opening brace and
// decodes a single JSON value, so trailing prose, a trailing brace, or a second example object do not defeat
// recovery. Every structured output schema unmarshals into a struct, so the response is always a top-level
// object; arrays and other top-level values are ignored. It returns an empty string if no object is found.
func extractJSON(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}

	var obj json.RawMessage
	if err := json.NewDecoder(strings.NewReader(s[start:])).Decode(&obj); err != nil {
		return ""
	}

	return string(obj)
}
