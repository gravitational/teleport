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
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

const message = "Adding diagnostic debugging handlers.\t To connect with profiler, use go tool pprof diag_addr."

var (
	logErr = &trace.BadParameterError{Message: "the quick brown fox jumped really high"}
	addr   = fakeAddr{addr: "127.0.0.1:1234"}

	fields = map[string]any{
		"local":        &addr,
		"remote":       &addr,
		"login":        "llama",
		"teleportUser": "user",
		"id":           1234,
	}
)

type fakeAddr struct {
	addr string
}

func (a fakeAddr) Network() string {
	return "tcp"
}

func (a fakeAddr) String() string {
	return a.addr
}

func (a fakeAddr) MarshalText() (text []byte, err error) {
	return []byte(a.addr), nil
}

func TestOutput(t *testing.T) {
	loc, err := time.LoadLocation("Africa/Cairo")
	require.NoError(t, err, "failed getting timezone")
	clock := clockwork.NewFakeClockAt(time.Now().In(loc))

	t.Run("text", func(t *testing.T) {
		// fieldsRegex matches all the key value pairs emitted after the message and before the caller. All fields are
		// in the following format key:value key2:value2 key3:value3.
		fieldsRegex := regexp.MustCompile(`(\w+):((?:"[^"]*"|\[[^]]*]|\S+))\s*`)
		// outputRegex groups the entire log output into 4 distinct matches. The regular expression is tailored toward
		// the output emitted by the test and is not a general purpose match for all output emitted by the loggers.
		// 1) the timestamp, component and level
		// 2) the message
		// 3) the fields
		// 4) the caller
		outputRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z)(\s+.*)(".*diag_addr\.")(.*)(\slog/formatter_test.go:\d{3})`)

		expectedFields := map[string]string{
			"local":        addr.String(),
			"remote":       addr.String(),
			"login":        "llama",
			"teleportUser": "user",
			"id":           "1234",
			"test":         "123",
			"animal":       `"llama\n"`,
			"error":        "[" + trace.DebugReport(logErr) + "]",
			"diag_addr":    addr.String(),
		}

		tests := []struct {
			name      string
			slogLevel slog.Level
		}{
			{
				name:      "trace",
				slogLevel: TraceLevel,
			},
			{
				name:      "debug",
				slogLevel: slog.LevelDebug,
			},
			{
				name:      "info",
				slogLevel: slog.LevelInfo,
			},
			{
				name:      "warn",
				slogLevel: slog.LevelWarn,
			},
			{
				name:      "error",
				slogLevel: slog.LevelError,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Create a slog logger using the custom handler which outputs to a local buffer.
				var slogOutput bytes.Buffer
				slogConfig := SlogTextHandlerConfig{
					Level:        test.slogLevel,
					EnableColors: true,
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.TimeKey {
							a.Value = slog.TimeValue(clock.Now().UTC())
						}
						return a
					},
				}
				slogLogger := slog.New(NewSlogTextHandler(&slogOutput, slogConfig)).With(teleport.ComponentKey, "test")

				// Add some fields and output the message at the desired log level via slog.
				l2 := slogLogger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				slogTestLogLineNumber := func() int {
					l2.With(teleport.ComponentFields, fields).Log(context.Background(), test.slogLevel, message, "diag_addr", &addr)
					return getCallerLineNumber() - 1 // Get the line number of this call, and assume the log call is right above it
				}()

				// Validate the logger output. The added complexity comes from the fact that
				// our custom slog handler does NOT sort the additional fields.
				slogMatches := outputRegex.FindStringSubmatch(slogOutput.String())
				require.NotEmpty(t, slogMatches, "slog output was in unexpected format: %s", slogOutput.String())

				// The first match is the timestamp: 2023-10-31T10:09:06+02:00
				slogTime, err := time.Parse(time.RFC3339, slogMatches[1])
				assert.NoError(t, err, "invalid slog timestamp found %s", slogMatches[1])
				assert.InDelta(t, clock.Now().Unix(), slogTime.Unix(), 10)

				// Match level, and component: DEBU [TEST]
				expectedLevel := formatLevel(test.slogLevel, true)
				expectedComponent := formatComponent(slog.StringValue("test"), defaultComponentPadding)
				expectedMatch := " " + expectedLevel + " " + expectedComponent + " "
				assert.Equal(t, expectedMatch, slogMatches[2], "level, and component to be identical")
				// Match the log message
				assert.Equal(t, `"Adding diagnostic debugging handlers.\t To connect with profiler, use go tool pprof diag_addr."`, slogMatches[3], "expected output messages to be identical")
				// The last matches are the caller information
				assert.Equal(t, fmt.Sprintf(" log/formatter_test.go:%d", slogTestLogLineNumber), slogMatches[5])

				// The third matches are the fields which will be key value pairs(animal:llama) separated by a space. Since
				// slog doesn't sort the fields, we can't assert equality and instead build a map of the key
				// value pairs to ensure they are all present and accounted for.
				slogFieldMatches := fieldsRegex.FindAllStringSubmatch(slogMatches[4], -1)

				// The first match is the key, the second match is the value
				slogFields := map[string]string{}
				for _, match := range slogFieldMatches {
					slogFields[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
				}

				require.Empty(t,
					cmp.Diff(
						expectedFields,
						slogFields,
						cmpopts.SortMaps(func(a, b string) bool { return a < b }),
					),
				)
			})
		}
	})

	t.Run("json", func(t *testing.T) {
		tests := []struct {
			name      string
			slogLevel slog.Level
		}{
			{
				name:      "trace",
				slogLevel: TraceLevel,
			},
			{
				name:      "debug",
				slogLevel: slog.LevelDebug,
			},
			{
				name:      "info",
				slogLevel: slog.LevelInfo,
			},
			{
				name:      "warn",
				slogLevel: slog.LevelWarn,
			},
			{
				name:      "error",
				slogLevel: slog.LevelError,
			},
		}

		expectedFields := map[string]any{
			"trace.fields": map[string]any{
				"teleportUser": "user",
				"id":           float64(1234),
				"local":        addr.String(),
				"login":        "llama",
				"remote":       addr.String(),
			},
			"test":      float64(123),
			"animal":    `llama`,
			"error":     logErr.Error(),
			"diag_addr": addr.String(),
			"component": "test",
			"message":   message,
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Create a slog logger using the custom formatter which outputs to a local buffer.
				var slogOutput bytes.Buffer
				slogLogger := slog.New(NewSlogJSONHandler(&slogOutput, SlogJSONHandlerConfig{Level: test.slogLevel})).With(teleport.ComponentKey, "test")

				// Add some fields and output the message at the desired log level via slog.
				l2 := slogLogger.With("test", 123).With("animal", "llama").With("error", trace.Wrap(logErr))
				slogTestLogLineNumber := func() int {
					l2.With(teleport.ComponentFields, fields).Log(context.Background(), test.slogLevel, message, "diag_addr", &addr)
					return getCallerLineNumber() - 1 // Get the line number of this call, and assume the log call is right above it
				}()

				var slogData map[string]any
				require.NoError(t, json.Unmarshal(slogOutput.Bytes(), &slogData), "invalid slog output format")

				slogCaller, ok := slogData["caller"].(string)
				delete(slogData, "caller")
				assert.True(t, ok, "caller was missing from slog output")
				assert.Equal(t, fmt.Sprintf("log/formatter_test.go:%d", slogTestLogLineNumber), slogCaller)

				slogLevel, ok := slogData["level"].(string)
				delete(slogData, "level")
				assert.True(t, ok, "level was missing from slog output")
				var expectedLevel string
				switch test.slogLevel {
				case TraceLevel:
					expectedLevel = "trace"
				case slog.LevelWarn:
					expectedLevel = "warning"
				default:
					expectedLevel = test.slogLevel.String()
				}
				assert.Equal(t, strings.ToLower(expectedLevel), slogLevel)

				slogTimestamp, ok := slogData["timestamp"].(string)
				delete(slogData, "timestamp")
				assert.True(t, ok, "time was missing from slog output")

				slogTime, err := time.Parse(time.RFC3339, slogTimestamp)
				assert.NoError(t, err, "invalid slog timestamp %s", slogTimestamp)

				assert.InDelta(t, clock.Now().Unix(), slogTime.Unix(), 10)

				require.Empty(t,
					cmp.Diff(
						expectedFields,
						slogData,
						cmpopts.SortMaps(func(a, b string) bool { return a < b }),
					),
				)
			})
		}
	})
}

func getCallerLineNumber() int {
	_, _, lineNumber, ok := runtime.Caller(1)
	if !ok {
		panic("failed to get the line number of the function calling this")
	}

	return lineNumber
}

func BenchmarkFormatter(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()

	b.Run("slog", func(b *testing.B) {
		b.Run("default_text", func(b *testing.B) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{
				AddSource: true,
				Level:     slog.LevelDebug,
			})).With(teleport.ComponentKey, "test")

			for b.Loop() {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(teleport.ComponentFields, fields).InfoContext(ctx, message, "diag_addr", &addr)
			}
		})

		b.Run("text", func(b *testing.B) {
			var output bytes.Buffer
			logger := slog.New(NewSlogTextHandler(&output, SlogTextHandlerConfig{Level: slog.LevelDebug, EnableColors: true})).With(teleport.ComponentKey, "test")

			for b.Loop() {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(teleport.ComponentFields, fields).InfoContext(ctx, message, "diag_addr", &addr)
			}
		})

		b.Run("default_json", func(b *testing.B) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
				AddSource: true,
				Level:     slog.LevelDebug,
			})).With(teleport.ComponentKey, "test")

			for b.Loop() {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(teleport.ComponentFields, fields).InfoContext(ctx, message, "diag_addr", &addr)
			}
		})

		b.Run("json", func(b *testing.B) {
			var output bytes.Buffer
			logger := slog.New(NewSlogJSONHandler(&output, SlogJSONHandlerConfig{Level: slog.LevelDebug})).With(teleport.ComponentKey, "test")

			for b.Loop() {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(teleport.ComponentFields, fields).InfoContext(ctx, message, "diag_addr", &addr)
			}
		})
	})
}

func TestConcurrentOutput(t *testing.T) {
	logger := slog.New(NewSlogTextHandler(os.Stdout, SlogTextHandlerConfig{
		EnableColors: true,
	})).With(teleport.ComponentKey, "test")

	var wg sync.WaitGroup
	ctx := context.Background()
	for i := range 1000 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			logger.InfoContext(ctx, "Teleport component entered degraded state",
				slog.Int("component", i),
				slog.Group("group",
					slog.String("test", "123"),
					slog.String("animal", "llama"),
				),
			)
		}(i)
	}
	wg.Wait()
}

// allPossibleSubsets returns all combinations of subsets for the
// provided slice, including the nil/empty set.
func allPossibleSubsets(in []string) [][]string {
	// include the empty set in the output
	subsets := [][]string{nil}
	length := len(in)

	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []string

		for object := range length {
			if (subsetBits>>object)&1 == 1 {
				subset = append(subset, in[object])
			}
		}
		subsets = append(subsets, subset)
	}
	return subsets
}

// TestExtraFields validates that the output is expected for the
// slog handler based on the configured extra fields.
func TestExtraFields(t *testing.T) {
	// Capture a fake time that all output will use.
	now := clockwork.NewFakeClock().Now()

	// Capture the caller information to be injected into all messages.
	pc, _, _, _ := runtime.Caller(0)

	const message = "testing 123"

	t.Run("text", func(t *testing.T) {
		// Test against every possible configured combination of allowed format fields.
		for _, configuredFields := range allPossibleSubsets(defaultFormatFields) {
			name := "not configured"
			if len(configuredFields) > 0 {
				name = strings.Join(configuredFields, " ")
			}

			t.Run(name, func(t *testing.T) {
				replaced := map[string]struct{}{}
				var slogHandler slog.Handler = NewSlogTextHandler(io.Discard, SlogTextHandlerConfig{
					ConfiguredFields: configuredFields,
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						replaced[a.Key] = struct{}{}
						return a
					},
				})

				record := slog.Record{
					Time:    now,
					Message: message,
					Level:   slog.LevelDebug,
					PC:      pc,
				}

				record.AddAttrs(slog.String(teleport.ComponentKey, "test"), slog.String("animal", "llama"), slog.String("vegetable", "carrot"))

				require.NoError(t, slogHandler.Handle(context.Background(), record))

				for k := range replaced {
					delete(replaced, k)
				}

				require.Empty(t, replaced, replaced)
			})
		}
	})

	t.Run("json", func(t *testing.T) {
		// Test against every possible configured combination of allowed format fields.
		// Note, the json handler limits the allowed fields to a subset of those allowed
		// by the text handler.
		for _, configuredFields := range allPossibleSubsets([]string{CallerField, ComponentField, TimestampField}) {
			name := "not configured"
			if len(configuredFields) > 0 {
				name = strings.Join(configuredFields, " ")
			}

			t.Run(name, func(t *testing.T) {
				var slogOutput bytes.Buffer
				var slogHandler slog.Handler = NewSlogJSONHandler(&slogOutput, SlogJSONHandlerConfig{ConfiguredFields: configuredFields})

				record := slog.Record{
					Time:    now,
					Message: message,
					Level:   slog.LevelDebug,
					PC:      pc,
				}

				record.AddAttrs(slog.String(teleport.ComponentKey, "test"), slog.String("animal", "llama"), slog.String("vegetable", "carrot"))

				require.NoError(t, slogHandler.Handle(context.Background(), record))

				var slogData map[string]any
				require.NoError(t, json.Unmarshal(slogOutput.Bytes(), &slogData))

				delete(slogData, "animal")
				delete(slogData, "vegetable")
				delete(slogData, "message")
				delete(slogData, "level")

				var expectedLen int
				expectedFields := configuredFields
				switch l := len(configuredFields); l {
				case 0:
					// The level field was removed above, but is included in the default fields
					expectedLen = len(defaultFormatFields) - 1
					expectedFields = defaultFormatFields
				default:
					expectedLen = l
				}
				require.Len(t, slogData, expectedLen, slogData)

				for _, f := range expectedFields {
					delete(slogData, f)
				}

				require.Empty(t, slogData, slogData)
			})
		}
	})
}

func TestValidateFields(t *testing.T) {
	tests := []struct {
		comment     string
		extraFields []string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			comment:     "invalid key (does not exist)",
			extraFields: []string{LevelField, "invalid key"},
			assertErr:   require.Error,
		},
		{
			comment:     "valid keys",
			extraFields: defaultFormatFields,
			assertErr:   require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			_, err := ValidateFields(tt.extraFields)
			tt.assertErr(t, err)
		})
	}
}
