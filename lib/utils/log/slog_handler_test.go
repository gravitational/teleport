/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

// TestSlogTextHandler validates that the SlogTextHandler fulfills
// the [slog.Handler] contract by exercising the handler with
// various scenarios from [slogtest.TestHandler].
func TestSlogTextHandler(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC()

	// Create a handler that doesn't report the caller and automatically sets
	// the time to whatever time the fake clock has in UTC time. Since the timestamp
	// is not important for this test overriding, it allows the regex to be simpler.
	var buf bytes.Buffer
	h := NewSlogTextHandler(&buf, SlogTextHandlerConfig{
		ConfiguredFields: []string{LevelField, ComponentField, TimestampField},
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.TimeValue(now)
			}
			return a
		},
	})

	// The regular expression matches a line of text output by the handler and captures several
	// groups to make interrogating the output simpler.
	// Group 1: timestamp which should match the replaced time with our fake clock
	// Group 2: verbosity level of output
	// Group 3: message contents
	// Group 4: additional attributes
	lineRegex := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z)?\s?([A-Z]{4})\s+(\w+)(?:\s(.*))?$`)

	results := func() []map[string]any {
		var ms []map[string]any
		for line := range bytes.SplitSeq(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}

			var m map[string]any
			matches := lineRegex.FindSubmatch(line)
			if len(matches) == 0 {
				assert.Failf(t, "log output did not match regular expression", "regex: %s output: %s", lineRegex.String(), string(line))
				ms = append(ms, m)
				continue
			}

			// Indexes mapped to the expected capture groups from the regular expression.
			const (
				timeMatch    = 1
				levelMatch   = 2
				messageMatch = 3
				fieldsMatch  = 4
			)

			// Entries that should always be in the output.
			m = map[string]any{
				slog.LevelKey:   matches[levelMatch],
				slog.MessageKey: string(matches[messageMatch]),
			}

			// The timestamp may be omitted if the record doesn't include a time.
			if len(matches[timeMatch]) > 0 {
				m[slog.TimeKey] = matches[timeMatch]
			}

			// Parse optional additional fields. Fields within a group will be in the form
			// Group1.Group2.Group3.key:value. This converts keys into sub-maps for groups.
			// The example group above would result in a map of Group1 -> Group2 -> Group3 -> key -> value
			// instead of a flat map that had keys Group1, Group2, Group3, and key.
			if len(matches[fieldsMatch]) > 0 {
				s := string(bytes.TrimSpace(matches[fieldsMatch]))
				for len(s) > 0 {
					field, rest, _ := strings.Cut(s, " ")

					k, value, found := strings.Cut(field, ":")
					assert.True(t, found, "no ':' in %s", field)

					keys := strings.Split(k, ".")
					mm := m
					for _, key := range keys[:len(keys)-1] {
						x, ok := mm[key]
						var m2 map[string]any
						if !ok {
							m2 = map[string]any{}
							mm[key] = m2
						} else {
							m2, ok = x.(map[string]any)
							if !ok {
								t.Fatalf("value in composite key %q is not map[string]any", key)
							}
						}
						mm = m2
					}
					mm[keys[len(keys)-1]] = value
					s = rest
				}
			}

			ms = append(ms, m)
		}

		return ms
	}

	require.NoError(t, slogtest.TestHandler(h, results))

}

// TestSlogJSONHandler validates that the SlogJSONHandler fulfills
// the [slog.Handler] contract by exercising the handler with
// various scenarios from [slogtest.TestHandler].
func TestSlogJSONHandler(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := NewSlogJSONHandler(&buf, SlogJSONHandlerConfig{Level: slog.LevelDebug})

	results := func() []map[string]any {
		var ms []map[string]any
		for line := range bytes.SplitSeq(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			assert.NoError(t, json.Unmarshal(line, &m), "unexpected non-json output: %s", line)

			// The conformance test expects [slog.TimeKey] to be present, but
			// because we convert that to "timestamp" to match the legacy output format,
			// we need to convert it back to [slog.TimeKey] to satisfy the test.
			if t, ok := m["timestamp"]; ok {
				m[slog.TimeKey] = t
				delete(m, "timestamp")
			}

			// The conformance test expects [slog.MessageKey] to be present, but
			// because we convert that to "message" to match the legacy output format,
			// we need to convert it back to [slog.MessageKey] to satisfy the test.
			if msg, ok := m["message"]; ok {
				m[slog.MessageKey] = msg
				delete(m, "message")
			}

			ms = append(ms, m)
		}
		return ms
	}
	require.NoError(t, slogtest.TestHandler(h, results))
}

func TestSlogJSONHandlerReservedKeysOverrideTypeDoesntPanic(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(NewSlogJSONHandler(&buf, SlogJSONHandlerConfig{Level: slog.LevelDebug}))

	logger.DebugContext(ctx, "Must not panic", "source", "not a slog.Source type", "time", "not a time.Time type", "level", true, "msg", 123) //nolint:sloglint // testing possible panics when using reserved keys

	logRecordMap := make(map[string]any)
	require.NoError(t, json.Unmarshal(buf.Bytes(), &logRecordMap))

	// Builtin fields must be present
	require.Contains(t, logRecordMap, "caller")
	require.Contains(t, logRecordMap["caller"], "slog_handler_test.go")

	require.Contains(t, logRecordMap, "message")
	require.Equal(t, "Must not panic", logRecordMap["message"])

	require.Contains(t, logRecordMap, "timestamp")

	// Some fields can appear twice in the output
	// See https://github.com/golang/go/issues/59365
	// Map does not accept two fields with the same name, so we must compare against the actual output.

	// Level is injected by the handler but was also defined as Attr, so it must appear twice.
	require.Contains(t, buf.String(), `"level":true`)
	require.Contains(t, buf.String(), `"level":"debug"`)

	// Fields that conflict with built-ins but have a different name, when not using the expected Attr Value's type should be present

	// source was injected but is not of slog.Source type, so, its value must be kept
	require.Contains(t, logRecordMap, "source")
	require.Equal(t, "not a slog.Source type", logRecordMap["source"])

	// time was injected but is not of time.Time type, so, its value must be kept
	require.Contains(t, logRecordMap, "time")
	require.Equal(t, "not a time.Time type", logRecordMap["time"])

	// msg was injected but is not a string, so, its value must be kept
	require.Contains(t, logRecordMap, "msg")
	require.InEpsilon(t, float64(123), logRecordMap["msg"], float64(0))
}

func TestSlogTextHandlerComponentPadding(t *testing.T) {
	tests := []struct {
		name      string
		component string
		padding   int
		want      string
	}{
		{name: "padded component",
			component: "foo",
			padding:   11,
			want:      "[FOO]       bar\n",
		},
		{name: "truncated component",
			component: "foobarbazquux",
			padding:   11,
			want:      "[FOOBARBAZ] bar\n",
		},
		{name: "no padding",
			component: "foobarbazquux",
			padding:   0,
			want:      "[FOOBARBAZQUUX] bar\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			h := NewSlogTextHandler(&buf, SlogTextHandlerConfig{
				Level:            slog.LevelDebug,
				Padding:          tt.padding,
				ConfiguredFields: []string{ComponentField},
			})
			// Override defaults set by NewSlogTextHandler.
			if tt.padding == 0 {
				h.cfg.Padding = 0
			}

			logger := slog.New(h)
			logger.DebugContext(t.Context(), "bar", teleport.ComponentKey, tt.component)

			require.Equal(t, tt.want, buf.String())
		})
	}
}

// TestSlogTextHandlerRawComponent checks if the value of the component field is forwarded from the
// handler to the writer without any changes. This allows certain writers, such as the os_log writer
// on macOS, to log the component in its full form since os_log provides dedicated fields for such
// metadata.
func TestSlogTextHandlerRawComponent(t *testing.T) {
	var buf bytes.Buffer
	out := &rawComponentWriter{}
	h := NewSlogTextHandler(&buf, SlogTextHandlerConfig{
		Level:            slog.LevelDebug,
		Padding:          6,
		ConfiguredFields: []string{ComponentField},
	})
	h.out = out

	logger := slog.New(h)
	logger.DebugContext(t.Context(), "bar", teleport.ComponentKey, "foobarbaz")
	require.Equal(t, "foobarbaz", out.lastRawComponent(),
		"raw component wasn't properly processed when teleport.ComponentKey was passed only directly with message")

	logger = logger.With(teleport.ComponentKey, "foobarbaz")
	logger.DebugContext(t.Context(), "bar")
	require.Equal(t, "foobarbaz", out.lastRawComponent(),
		"raw component wasn't properly processed when teleport.ComponentKey was passed to slog.Logger.With")

	logger.With("quux", "xuuq").DebugContext(t.Context(), "bar")
	require.Equal(t, "foobarbaz", out.lastRawComponent(),
		"raw component wasn't properly cloned when teleport.ComponentKey wasn't passed to slog.Logger.With")

	logger.DebugContext(t.Context(), "bar", teleport.ComponentKey, "bazbarfoo")
	require.Equal(t, "bazbarfoo", out.lastRawComponent(),
		"raw component wasn't properly processed when teleport.ComponentKey was meant to override existing component")
}

// rawComponentWriter is a writer that persists only rawComponent values that were passed to its
// Write method.
type rawComponentWriter struct {
	rawComponents []string
}

func (r *rawComponentWriter) Write(bytes []byte, rawComponent string, level slog.Level) error {
	r.rawComponents = append(r.rawComponents, rawComponent)
	return nil
}

func (r *rawComponentWriter) lastRawComponent() string {
	if len(r.rawComponents) == 0 {
		return ""
	}

	return r.rawComponents[len(r.rawComponents)-1]
}
