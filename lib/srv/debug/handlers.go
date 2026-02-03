// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	debugpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/debug/v1"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// LogLeveler defines a struct that can retrieve and set log levels.
type LogLeveler interface {
	// GetLogLevel returns the current log level.
	GetLogLevel() slog.Level
	// SetLogLevel sets the log level.
	SetLogLevel(slog.Level)
}

// RegisterProfilingHandlers registers the debug profiling handlers (/debug/pprof/*)
// to a given multiplexer.
func RegisterProfilingHandlers(mux *http.ServeMux, logger *slog.Logger) {
	noWriteTimeout := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			rc := http.NewResponseController(w) //nolint:bodyclose // bodyclose gets really confused about NewResponseController
			if err := rc.SetWriteDeadline(time.Time{}); err == nil {
				// don't let the pprof handlers know about the WriteTimeout
				r = r.WithContext(context.WithValue(r.Context(), http.ServerContextKey, nil))
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/debug/pprof/cmdline", pprofMiddleware(logger, "cmdline", pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", pprofMiddleware(logger, "profile", noWriteTimeout(pprof.Profile)))
	mux.HandleFunc("/debug/pprof/symbol", pprofMiddleware(logger, "symbol", pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", pprofMiddleware(logger, "trace", noWriteTimeout(pprof.Trace)))
	mux.HandleFunc("/debug/pprof/{profile}", func(w http.ResponseWriter, r *http.Request) {
		pprofMiddleware(logger, r.PathValue("profile"), noWriteTimeout(pprof.Index))(w, r)
	})
}

// RegisterLogLevelHandlers registers log level handlers to a given multiplexer.
// This allows to dynamically change the process' log level.
func RegisterLogLevelHandlers(mux *http.ServeMux, svc *Service) {
	mux.Handle("GET /log-level", handleGetLog(svc))
	mux.Handle("PUT /log-level", handleSetLog(svc))
}

// handleGetLog returns the http get log level handler.
func handleGetLog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc.logger.InfoContext(r.Context(), "Log level requested")
		w.Write([]byte(svc.GetLogLevel()))
	}
}

// handleSetLog returns the http set log level handler.
func handleSetLog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawLevel, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("Unable to read request body."))
			return
		}

		msg, err := svc.SetLogLevel(string(rawLevel))
		if err != nil {
			svc.logger.WarnContext(r.Context(), "Failed to parse log level", "error", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("Invalid log level."))
			return
		}

		w.Write([]byte(msg))
	}
}

// pprofMiddleware logs pprof HTTP requests.
func pprofMiddleware(logger *slog.Logger, profile string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seconds := r.FormValue("seconds")
		if seconds == "" {
			seconds = "default"
		}

		logger.InfoContext(
			r.Context(),
			"Collecting pprof profile.",
			"profile", profile,
			"seconds", seconds,
		)

		next(w, r)
	}
}

// unmarshalLogLevel unmarshals log level text representation to slog.Level.
func unmarshalLogLevel(data []byte) (slog.Level, error) {
	var level slog.Level
	if contains := slices.Contains(logutils.SupportedLevelsText, strings.ToUpper(string(data))); !contains {
		return level, trace.BadParameter("%q log level not supported", string(data))
	}

	if strings.EqualFold(string(data), logutils.TraceLevelText) {
		return logutils.TraceLevel, nil
	}

	if err := level.UnmarshalText(data); err != nil {
		return level, trace.Wrap(err)
	}

	return level, nil
}

// RegisterLogStreamHandler registers the log streaming endpoint.
// Clients can connect to GET /log-stream and receive live log output
// as a chunked HTTP response.
func RegisterLogStreamHandler(mux *http.ServeMux, svc *Service) {
	mux.Handle("GET /log-stream", handleLogStream(svc))
}

func handleLogStream(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		levelStr := r.URL.Query().Get("level")
		ch, cleanup, err := svc.SubscribeLogs(levelStr)
		if err != nil {
			if trace.IsLimitExceeded(err) {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
			} else {
				http.Error(w, fmt.Sprintf("invalid level %q", levelStr), http.StatusBadRequest)
			}
			return
		}
		defer cleanup()

		svc.logger.InfoContext(r.Context(), "Log stream subscriber connected")
		defer svc.logger.InfoContext(r.Context(), "Log stream subscriber disconnected")

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		for {
			select {
			case entry, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(logEntryToMap(entry))
				if err != nil {
					return
				}
				data = append(data, '\n')
				if _, err := w.Write(data); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

// logEntryToMap converts a *debugpb.LogEntry to a map for JSON serialization
// in the HTTP log-stream endpoint.
func logEntryToMap(entry *debugpb.LogEntry) map[string]any {
	m := make(map[string]any, 8)
	if entry.Timestamp != nil {
		m["timestamp"] = entry.Timestamp.AsTime().UTC().Format(time.RFC3339Nano)
	}
	m["level"] = entry.Level
	m["message"] = entry.Message
	if entry.Caller != "" {
		m["caller"] = entry.Caller
	}
	if entry.Component != "" {
		m["component"] = entry.Component
	}
	if entry.TraceId != "" {
		m["trace_id"] = entry.TraceId
	}
	if entry.SpanId != "" {
		m["span_id"] = entry.SpanId
	}
	for k, v := range entry.Attributes {
		m[k] = v
	}
	return m
}

// marshalLogLevel marshals log level to its text representation.
func marshalLogLevel(level slog.Level) string {
	if level == logutils.TraceLevel {
		return logutils.TraceLevelText
	}

	return level.String()
}
