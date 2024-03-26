// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"
	"golang.org/x/exp/slog/slogtest"
)

// TestSlogTextHandler validates that the SlogTextHandler fulfills
// the [slog.Handler] contract by exercising the handler with
// various scenarios from [slogtest.TestHandler].
func TestSlogTextHandler(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	now := clock.Now().UTC().Format(time.RFC3339)

	// Create a handler that doesn't report the caller and automatically sets
	// the time to whatever time the fake clock has in UTC time. Since the timestamp
	// is not important for this test overriding, it allows the regex to be simpler.
	var buf bytes.Buffer
	h := NewSlogTextHandler(&buf, SlogTextHandlerConfig{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(now)
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
	regex := fmt.Sprintf("^(?:(%s)?)\\s?([A-Z]{4})\\s+(\\w+)(?:\\s(.*))?$", now)
	lineRegex := regexp.MustCompile(regex)

	results := func() []map[string]any {
		var ms []map[string]any
		for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}

			var m map[string]any
			matches := lineRegex.FindSubmatch(line)
			if len(matches) == 0 {
				assert.Failf(t, "log output did not match regular expression", "regex: %s output: %s", regex, string(line))
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
		for _, line := range bytes.Split(buf.Bytes(), []byte{'\n'}) {
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
