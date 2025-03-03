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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

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
func RegisterLogLevelHandlers(mux *http.ServeMux, logger *slog.Logger, leveler LogLeveler) {
	mux.Handle("GET /log-level", handleGetLog(logger, leveler))
	mux.Handle("PUT /log-level", handleSetLog(logger, leveler))
}

// handleGetLog returns the http get log level handler.
func handleGetLog(logger *slog.Logger, leveler LogLeveler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		level := leveler.GetLogLevel()
		logger.InfoContext(r.Context(), "Log level requested", "log_level", level)
		w.Write([]byte(marshalLogLevel(level)))
	}
}

// handleSetLog returns the http set log level handler.
func handleSetLog(logger *slog.Logger, leveler LogLeveler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawLevel, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("Unable to read request body."))
			return
		}

		level, err := unmarshalLogLevel(rawLevel)
		if err != nil {
			logger.WarnContext(r.Context(), "Failed to parse log level", "error", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("Invalid log level."))
			return
		}

		currLevel := leveler.GetLogLevel()
		message := fmt.Sprintf("Log level already set to %q.", level)
		if level != currLevel {
			message = fmt.Sprintf("Changed log level from %q to %q.", marshalLogLevel(currLevel), marshalLogLevel(level))
			leveler.SetLogLevel(level)
			logger.InfoContext(r.Context(), "Changed log level.", "old", marshalLogLevel(currLevel), "new", marshalLogLevel(level))
		}

		w.Write([]byte(message))
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

// marshalLogLevel marshals log level to its text representation.
func marshalLogLevel(level slog.Level) string {
	if level == logutils.TraceLevel {
		return logutils.TraceLevelText
	}

	return level.String()
}
