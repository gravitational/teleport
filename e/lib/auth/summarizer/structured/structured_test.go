package structured

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	summarizererrors "github.com/gravitational/teleport/e/lib/auth/summarizer/errors/types"
)

// payload is a minimal schema type: name and tags are required, and additional properties are forbidden, so
// it exercises the required, type, and additionalProperties checks Parse layers on top of plain unmarshaling.
type payload struct {
	Name string   `json:"name" jsonschema:"required"`
	Tags []string `json:"tags" jsonschema:"required"`
}

func TestStripMarkdownCodeBlock(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"json fence", "```json\n{\"a\":1}\n```", "{\"a\":1}"},
		{"bare fence", "```\n{\"a\":1}\n```", "{\"a\":1}"},
		{"no fence", "{\"a\":1}", "{\"a\":1}"},
		{"surrounding whitespace", "  ```json\n{\"a\":1}\n```  ", "{\"a\":1}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, stripMarkdownCodeBlock(tc.in))
		})
	}
}

func TestExtractJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"object in prose", "Here is the result: {\"a\":1} hope that helps", "{\"a\":1}"},
		{"already json", "{\"a\":1}", "{\"a\":1}"},
		{"no json", "no json here", ""},
		{"object with nested array", "{\"a\":[1,2]}", "{\"a\":[1,2]}"},
		// Top-level arrays are not extracted: every schema is an object, and a stray bracket in prose must not be mistaken
		// for the payload.
		{"top-level array ignored", "prefix [1,2,3] suffix", ""},
		{"object after bracketed prose", "see results [below]: {\"a\":1}", "{\"a\":1}"},
		// Trailing prose containing braces, or a second example object, must not be swallowed into an invalid span.
		{"trailing prose with braces", "{\"a\":1} (use the {placeholder} for x)", "{\"a\":1}"},
		{"second object ignored", "{\"a\":1} e.g. {\"b\":2}", "{\"a\":1}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, extractJSON(tc.in))
		})
	}
}

func TestUnmarshalLoose(t *testing.T) {
	t.Parallel()

	type simple struct {
		A int `json:"a"`
	}

	t.Run("raw json", func(t *testing.T) {
		got, candidate, err := unmarshalLoose[simple](`{"a":1}`)
		require.NoError(t, err)
		require.Equal(t, `{"a":1}`, candidate)
		require.Equal(t, 1, got.A)
	})
	t.Run("markdown fenced", func(t *testing.T) {
		got, candidate, err := unmarshalLoose[simple]("```json\n{\"a\":2}\n```")
		require.NoError(t, err)
		require.Equal(t, `{"a":2}`, candidate)
		require.Equal(t, 2, got.A)
	})
	t.Run("embedded in prose", func(t *testing.T) {
		got, candidate, err := unmarshalLoose[simple](`Sure! Here you go: {"a":3}. Anything else?`)
		require.NoError(t, err)
		require.Equal(t, `{"a":3}`, candidate)
		require.Equal(t, 3, got.A)
	})
	t.Run("unrecoverable", func(t *testing.T) {
		_, _, err := unmarshalLoose[simple]("not json at all")
		require.Error(t, err)
	})
	t.Run("empty", func(t *testing.T) {
		_, _, err := unmarshalLoose[simple]("")
		require.Error(t, err)
	})

	// A failed parse must yield the zero value, never a partially-decoded struct: json.Unmarshal commits fields
	// before erroring on a bad one, so decoding into a fresh value per call is what keeps a failure clean.
	t.Run("failed parse yields the zero value", func(t *testing.T) {
		type pair struct {
			A int    `json:"a"`
			B string `json:"b"`
		}
		got, _, err := unmarshalLoose[pair](`{"b":"committed","a":"not-an-int"}`)
		require.Error(t, err)
		require.Equal(t, pair{}, got)
	})
}

func TestParse(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		got, err := Parse[payload](`{"name":"x","tags":[]}`)
		require.NoError(t, err)
		require.Equal(t, "x", got.Name)
	})

	t.Run("recovers JSON wrapped in prose", func(t *testing.T) {
		got, err := Parse[payload]("Here you go:\n```json\n{\"name\":\"y\",\"tags\":[\"a\"]}\n```")
		require.NoError(t, err)
		require.Equal(t, "y", got.Name)
		require.Equal(t, []string{"a"}, got.Tags)
	})

	// Decodable JSON that omits a required field must be rejected: it unmarshals into the struct (tags is just nil)
	// but violates the schema, which a native structured output API would have caught server-side.
	t.Run("rejects a decodable response missing a required field", func(t *testing.T) {
		_, err := Parse[payload](`{"name":"x"}`)
		require.ErrorContains(t, err, `missing required property "tags"`)
	})

	t.Run("rejects an unexpected property", func(t *testing.T) {
		_, err := Parse[payload](`{"name":"x","tags":[],"extra":1}`)
		require.ErrorContains(t, err, `property "extra" is not allowed`)
	})

	t.Run("rejects unparseable text", func(t *testing.T) {
		_, err := Parse[payload]("not json at all")
		require.Error(t, err)
	})
}

// fakeConversation replays a queue of model responses and records the parse errors fed back through Correct,
// so tests can drive Complete without a real model.
type fakeConversation struct {
	responses   []string
	sendErr     error
	sends       int
	corrections []string
}

func (c *fakeConversation) Send(ctx context.Context) (string, error) {
	if c.sendErr != nil {
		return "", c.sendErr
	}

	r := c.responses[c.sends]
	c.sends++

	return r, nil
}

func (c *fakeConversation) Correct(modelResponse, parseError string) {
	c.corrections = append(c.corrections, parseError)
}

func TestComplete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("succeeds on the first response", func(t *testing.T) {
		conv := &fakeConversation{responses: []string{`{"name":"x","tags":[]}`}}

		got, err := Complete[payload](ctx, conv, 1)
		require.NoError(t, err)
		require.Equal(t, "x", got.Name)
		require.Equal(t, 1, conv.sends)
		require.Empty(t, conv.corrections)
	})

	t.Run("re-prompts once and recovers", func(t *testing.T) {
		conv := &fakeConversation{responses: []string{
			`{"name":"x"}`, // missing required "tags"
			`{"name":"x","tags":[]}`,
		}}

		got, err := Complete[payload](ctx, conv, 1)
		require.NoError(t, err)
		require.Equal(t, "x", got.Name)
		require.Equal(t, 2, conv.sends)
		require.Len(t, conv.corrections, 1)
		require.Contains(t, conv.corrections[0], `missing required property "tags"`)
	})

	t.Run("returns BadResponseError after exhausting re-prompts", func(t *testing.T) {
		conv := &fakeConversation{responses: []string{`{"name":"x"}`, `{"name":"y"}`}}

		_, err := Complete[payload](ctx, conv, 1)
		require.Error(t, err)

		var badResp summarizererrors.BadResponseError
		require.ErrorAs(t, err, &badResp)
		require.Contains(t, badResp.Message, "after 1 retries")
		require.Equal(t, 2, conv.sends) // initial attempt + one re-prompt
	})

	t.Run("surfaces a send error without retrying", func(t *testing.T) {
		sentinel := errors.New("throttled")
		conv := &fakeConversation{sendErr: sentinel}

		_, err := Complete[payload](ctx, conv, 1)
		require.ErrorIs(t, err, sentinel)
		require.Empty(t, conv.corrections)
	})
}
