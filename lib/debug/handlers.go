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
	"strings"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// NewServeMux returns a http mux that handles all the debug service endpoints.
func NewServeMux(ctx context.Context, logger *slog.Logger, config *servicecfg.Config) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(PProfEndpointsPrefix, pprofMiddleware(ctx, logger, pprof.Index))
	mux.HandleFunc(PProfEndpointsPrefix+"cmdline", pprofMiddleware(ctx, logger, pprof.Cmdline))
	mux.HandleFunc(PProfEndpointsPrefix+"profile", pprofMiddleware(ctx, logger, pprof.Profile))
	mux.HandleFunc(PProfEndpointsPrefix+"symbol", pprofMiddleware(ctx, logger, pprof.Symbol))
	mux.HandleFunc(PProfEndpointsPrefix+"trace", pprofMiddleware(ctx, logger, pprof.Trace))
	mux.Handle(LogLevelEndpoint, newLogLevelHandler(ctx, logger, config))
	return mux
}

// logLevelHandler http handler of the set/get log level endpoints.
type logLevelHandler struct {
	ctx           context.Context
	logger        *slog.Logger
	processConfig *servicecfg.Config
}

func newLogLevelHandler(ctx context.Context, logger *slog.Logger, config *servicecfg.Config) *logLevelHandler {
	return &logLevelHandler{
		ctx:           ctx,
		logger:        logger,
		processConfig: config,
	}
}

func (h *logLevelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == GetLogLevelMethod {
		w.Write([]byte(logutils.MarshalText(h.processConfig.LoggerLevel.Level())))
		return
	}

	if r.Method != SetLogLevelMethod {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rawLevel, err := io.ReadAll(io.LimitReader(r.Body, 1024))
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("Unable to read request body."))
		return
	}

	level, err := logutils.UnmarshalText(rawLevel)
	if err != nil {
		h.logger.WarnContext(h.ctx, "Failed to parse log level", "error", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("Invalid log level."))
		return
	}

	currLevel := h.processConfig.LoggerLevel.Level()
	message := fmt.Sprintf("Log level already set to %q.", level)
	if level != currLevel {
		message = fmt.Sprintf("Changed log level from %q to %q.", currLevel, level)
		h.processConfig.SetLogLevel(level)
		h.logger.InfoContext(h.ctx, "Changed log level.", "old", currLevel, "new", string(rawLevel))
	}

	w.Write([]byte(message))
}

// pprofMiddleware logs pprof HTTP requests.
func pprofMiddleware(ctx context.Context, logger *slog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seconds := r.URL.Query().Get("seconds")
		if seconds == "" {
			seconds = "default"
		}

		logger.InfoContext(
			ctx,
			"Collecting pprof profile.",
			"profile", strings.TrimSuffix(r.URL.Path, PProfEndpointsPrefix),
			"seconds", seconds,
		)

		next(w, r)
	}
}
