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
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const message = "Adding diagnostic debugging handlers.\t To connect with profiler, use `go tool pprof diag_addr`."

var (
	logErr = errors.New("the quick brown fox jumped really high")
	addr   = fakeAddr{addr: "127.0.0.1:1234"}

	fields = logrus.Fields{
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

func TestOutput(t *testing.T) {
	// Set a specific time zone so that the regex doesn't have to match
	// against any possible timezone. Note this mucks around with the global
	// time zone which prevents running this test in parallel.
	loc, err := time.LoadLocation("Africa/Cairo")
	require.NoError(t, err, "failed getting timezone")
	oldLoc := time.Local
	time.Local = loc
	t.Cleanup(func() {
		time.Local = oldLoc
	})

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
		outputRegex := regexp.MustCompile("(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\+\\d{2}:\\d{2})(\\s+.*)(\".*diag_addr`\\.\")(.*)(\\slog/formatter_test.go:\\d{3})")

		tests := []struct {
			name        string
			logrusLevel logrus.Level
			slogLevel   slog.Level
		}{
			{
				name:        "trace",
				logrusLevel: logrus.TraceLevel,
				slogLevel:   TraceLevel,
			},
			{
				name:        "debug",
				logrusLevel: logrus.DebugLevel,
				slogLevel:   slog.LevelDebug,
			},
			{
				name:        "info",
				logrusLevel: logrus.InfoLevel,
				slogLevel:   slog.LevelInfo,
			},
			{
				name:        "warn",
				logrusLevel: logrus.WarnLevel,
				slogLevel:   slog.LevelWarn,
			},
			{
				name:        "error",
				logrusLevel: logrus.ErrorLevel,
				slogLevel:   slog.LevelError,
			},
			{
				name:        "fatal",
				logrusLevel: logrus.FatalLevel,
				slogLevel:   slog.LevelError + 1,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Create a logrus logger using the custom formatter which outputs to a local buffer.
				var logrusOutput bytes.Buffer
				formatter := NewDefaultTextFormatter(true)
				formatter.timestampEnabled = true
				require.NoError(t, formatter.CheckAndSetDefaults())

				logrusLogger := logrus.New()
				logrusLogger.SetFormatter(formatter)
				logrusLogger.SetOutput(&logrusOutput)
				logrusLogger.ReplaceHooks(logrus.LevelHooks{})
				logrusLogger.SetLevel(test.logrusLevel)
				entry := logrusLogger.WithField(trace.Component, "test")

				// Create a slog logger using the custom handler which outputs to a local buffer.
				var slogOutput bytes.Buffer
				slogConfig := &SlogTextHandlerConfig{
					Level:        test.slogLevel,
					EnableColors: true,
					WithCaller:   true,
				}
				slogLogger := slog.New(NewSlogTextHandler(&slogOutput, slogConfig)).With(trace.Component, "test")

				// Add some fields and output the message at the desired log level via logrus.
				l := entry.WithField("test", 123).WithField("animal", "llama\n").WithField("error", logErr)
				l.WithField("diag_addr", &addr).WithField(trace.ComponentFields, fields).Log(test.logrusLevel, message)

				// Add some fields and output the message at the desired log level via slog.
				l2 := slogLogger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l2.With(trace.ComponentFields, fields).Log(context.Background(), test.slogLevel, message, "diag_addr", &addr)

				// Validate that both loggers produces the same output. The added complexity comes from the fact that
				// our custom slog handler does NOT sort the additional fields like our logrus formatter does.
				logrusMatches := outputRegex.FindStringSubmatch(logrusOutput.String())
				require.NotEmpty(t, logrusMatches, "logrus output was in unexpected format: %s", logrusOutput.String())
				slogMatches := outputRegex.FindStringSubmatch(slogOutput.String())
				require.NotEmpty(t, slogMatches, "slog output was in unexpected format: %s", slogOutput.String())

				// The first match is the timestamp: 2023-10-31T10:09:06+02:00
				logrusTime, err := time.Parse(time.RFC3339, logrusMatches[1])
				assert.NoError(t, err, "invalid logrus timestamp found %s", logrusMatches[1])

				slogTime, err := time.Parse(time.RFC3339, slogMatches[1])
				assert.NoError(t, err, "invalid slog timestamp found %s", slogMatches[1])

				assert.InDelta(t, logrusTime.UnixNano(), slogTime.UnixNano(), 10)

				// Match level, and component: DEBU [TEST]
				assert.Empty(t, cmp.Diff(logrusMatches[2], slogMatches[2]), "level, and component to be identical")
				// Match the log message: "Adding diagnostic debugging handlers.\t To connect with profiler, use `go tool pprof diag_addr`.\n"
				assert.Empty(t, cmp.Diff(logrusMatches[3], slogMatches[3]), "expected output messages to be identical")
				// The last matches are the caller information
				assert.Equal(t, " log/formatter_test.go:151", logrusMatches[5])
				assert.Equal(t, " log/formatter_test.go:155", slogMatches[5])

				// The third matches are the fields which will be key value pairs(animal:llama) separated by a space. Since
				// logrus sorts the fields and slog doesn't we can't just assert equality and instead build a map of the key
				// value pairs to ensure they are all present and accounted for.
				logrusFieldMatches := fieldsRegex.FindAllStringSubmatch(logrusMatches[4], -1)
				slogFieldMatches := fieldsRegex.FindAllStringSubmatch(slogMatches[4], -1)

				// The first match is the key, the second match is the value
				logrusFields := map[string]string{}
				for _, match := range logrusFieldMatches {
					logrusFields[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
				}

				slogFields := map[string]string{}
				for _, match := range slogFieldMatches {
					slogFields[strings.TrimSpace(match[1])] = strings.TrimSpace(match[2])
				}

				assert.Equal(t, slogFields, logrusFields)
			})
		}
	})

	t.Run("json", func(t *testing.T) {
		tests := []struct {
			name        string
			logrusLevel logrus.Level
			slogLevel   slog.Level
		}{
			{
				name:        "trace",
				logrusLevel: logrus.TraceLevel,
				slogLevel:   TraceLevel,
			},
			{
				name:        "debug",
				logrusLevel: logrus.DebugLevel,
				slogLevel:   slog.LevelDebug,
			},
			{
				name:        "info",
				logrusLevel: logrus.InfoLevel,
				slogLevel:   slog.LevelInfo,
			},
			{
				name:        "warn",
				logrusLevel: logrus.WarnLevel,
				slogLevel:   slog.LevelWarn,
			},
			{
				name:        "error",
				logrusLevel: logrus.ErrorLevel,
				slogLevel:   slog.LevelError,
			},
			{
				name:        "fatal",
				logrusLevel: logrus.FatalLevel,
				slogLevel:   slog.LevelError + 1,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Create a logrus logger using the custom formatter which outputs to a local buffer.
				var logrusOut bytes.Buffer
				formatter := &JSONFormatter{
					ExtraFields:   nil,
					callerEnabled: true,
				}
				require.NoError(t, formatter.CheckAndSetDefaults())

				logrusLogger := logrus.New()
				logrusLogger.SetFormatter(formatter)
				logrusLogger.SetOutput(&logrusOut)
				logrusLogger.ReplaceHooks(logrus.LevelHooks{})
				logrusLogger.SetLevel(test.logrusLevel)
				entry := logrusLogger.WithField(trace.Component, "test")

				// Create a slog logger using the custom formatter which outputs to a local buffer.
				var slogOutput bytes.Buffer
				slogLogger := slog.New(NewSlogJSONHandler(&slogOutput, test.slogLevel)).With(trace.Component, "test")

				// Add some fields and output the message at the desired log level via logrus.
				l := entry.WithField("test", 123).WithField("animal", "llama").WithField("error", logErr)
				l.WithField("diag_addr", &addr).Log(test.logrusLevel, message)

				// Add some fields and output the message at the desired log level via slog.
				l2 := slogLogger.With("test", 123).With("animal", "llama").With("error", logErr)
				l2.Log(context.Background(), test.slogLevel, message, "diag_addr", &addr)

				// The order of the fields emitted by the two loggers is different, so comparing the output directly
				// for equality won't work. Instead, a map is built with all the key value pairs, excluding the caller
				// and that map is compared to ensure all items are present and match.
				var logrusData map[string]any
				require.NoError(t, json.Unmarshal(logrusOut.Bytes(), &logrusData), "invalid logrus output format")

				var slogData map[string]any
				require.NoError(t, json.Unmarshal(slogOutput.Bytes(), &slogData), "invalid slog output format")

				logrusCaller, ok := logrusData["caller"].(string)
				delete(logrusData, "caller")
				assert.True(t, ok, "caller was missing from logrus output")
				assert.Equal(t, "log/formatter_test.go:264", logrusCaller)

				slogCaller, ok := slogData["caller"].(string)
				delete(slogData, "caller")
				assert.True(t, ok, "caller was missing from slog output")
				assert.Equal(t, "log/formatter_test.go:268", slogCaller)

				require.Empty(t,
					cmp.Diff(
						logrusData,
						slogData,
						cmpopts.SortMaps(func(a, b string) bool { return a < b }),
					),
				)
			})
		}
	})
}

func BenchmarkFormatter(b *testing.B) {
	b.ReportAllocs()
	b.Run("logrus", func(b *testing.B) {
		b.Run("text", func(b *testing.B) {
			formatter := NewDefaultTextFormatter(true)
			require.NoError(b, formatter.CheckAndSetDefaults())
			logger := logrus.New()
			logger.SetFormatter(formatter)
			logger.SetOutput(io.Discard)
			logger.ReplaceHooks(logrus.LevelHooks{})
			b.ResetTimer()

			entry := logger.WithField(trace.Component, "test")
			for i := 0; i < b.N; i++ {
				l := entry.WithField("test", 123).WithField("animal", "llama\n").WithField("error", logErr)
				l.WithField("diag_addr", &addr).WithField(trace.ComponentFields, fields).Info(message)
			}
		})

		b.Run("json", func(b *testing.B) {
			formatter := &JSONFormatter{}
			require.NoError(b, formatter.CheckAndSetDefaults())
			logger := logrus.New()
			logger.SetFormatter(formatter)
			logger.SetOutput(io.Discard)
			logger.ReplaceHooks(logrus.LevelHooks{})
			b.ResetTimer()

			entry := logger.WithField(trace.Component, "test")
			for i := 0; i < b.N; i++ {
				l := entry.WithField("test", 123).WithField("animal", "llama\n").WithField("error", logErr)
				l.WithField("diag_addr", &addr).WithField(trace.ComponentFields, fields).Info(message)
			}
		})
	})

	b.Run("slog", func(b *testing.B) {
		b.Run("default_text", func(b *testing.B) {
			logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
				AddSource:   true,
				Level:       slog.LevelDebug,
				ReplaceAttr: nil,
			})).With(trace.Component, "test")
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(trace.ComponentFields, fields).Info(message, "diag_addr", &addr)
			}
		})

		b.Run("text", func(b *testing.B) {
			logger := slog.New(NewSlogTextHandler(io.Discard, &SlogTextHandlerConfig{Level: slog.LevelDebug, EnableColors: true})).With(trace.Component, "test")
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(trace.ComponentFields, fields).Info(message, "diag_addr", &addr)
			}
		})

		b.Run("default_json", func(b *testing.B) {
			logger := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{
				AddSource:   true,
				Level:       slog.LevelDebug,
				ReplaceAttr: nil,
			})).With(trace.Component, "test")
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(trace.ComponentFields, fields).Info(message, "diag_addr", &addr)
			}
		})

		b.Run("json", func(b *testing.B) {
			logger := slog.New(NewSlogJSONHandler(io.Discard, slog.LevelDebug)).With(trace.Component, "test")
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				l := logger.With("test", 123).With("animal", "llama\n").With("error", logErr)
				l.With(trace.ComponentFields, fields).Info(message, "diag_addr", &addr)
			}
		})
	})
}
