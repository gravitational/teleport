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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"

	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// LogLeveler defines a struct that can retrieve and set log levels.
type LogLeveler interface {
	// GetLogLevel returns the current log level.
	GetLogLevel() slog.Level
	// SetLogLevel sets the log level.
	SetLogLevel(slog.Level)
}

// NewServeMux returns a http mux that handles all the debug service endpoints.
func NewServeMux(logger *slog.Logger, leveler LogLeveler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(PProfEndpointsPrefix+"cmdline", pprofMiddleware(logger, "cmdline", pprof.Cmdline))
	mux.HandleFunc(PProfEndpointsPrefix+"profile", pprofMiddleware(logger, "profile", pprof.Profile))
	mux.HandleFunc(PProfEndpointsPrefix+"symbol", pprofMiddleware(logger, "symbol", pprof.Symbol))
	mux.HandleFunc(PProfEndpointsPrefix+"trace", pprofMiddleware(logger, "trace", pprof.Trace))
	mux.HandleFunc(PProfEndpointsPrefix+"{profile}", func(w http.ResponseWriter, r *http.Request) {
		pprofMiddleware(logger, r.PathValue("profile"), pprof.Index)(w, r)
	})
	mux.Handle(GetLogLevelMethod+" "+LogLevelEndpoint, handleGetLog(logger, leveler))
	mux.Handle(SetLogLevelMethod+" "+LogLevelEndpoint, handleSetLog(logger, leveler))
	return mux
}

// handleGetLog returns the http get log level handler.
func handleGetLog(logger *slog.Logger, leveler LogLeveler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		level := leveler.GetLogLevel()
		logger.InfoContext(r.Context(), "Log level requested", "log_level", level)
		w.Write([]byte(logutils.MarshalText(level)))
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

		level, err := logutils.UnmarshalText(rawLevel)
		if err != nil {
			logger.WarnContext(r.Context(), "Failed to parse log level", "error", err)
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("Invalid log level."))
			return
		}

		currLevel := leveler.GetLogLevel()
		message := fmt.Sprintf("Log level already set to %q.", level)
		if level != currLevel {
			message = fmt.Sprintf("Changed log level from %q to %q.", currLevel, level)
			leveler.SetLogLevel(level)
			logger.InfoContext(r.Context(), "Changed log level.", "old", currLevel, "new", string(rawLevel))
		}

		w.Write([]byte(message))
	}
}

// pprofMiddleware logs pprof HTTP requests.
func pprofMiddleware(logger *slog.Logger, profile string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seconds := r.URL.Query().Get("seconds")
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
