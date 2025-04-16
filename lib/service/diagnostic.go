// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package service

import (
	"log/slog"
	"net/http"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/srv/debug"
)

type diagnosticHandlerConfig struct {
	enableMetrics    bool
	enableProfiling  bool
	enableHealth     bool
	enableLogLeveler bool
}

func (process *TeleportProcess) newDiagnosticHandler(config diagnosticHandlerConfig, logger *slog.Logger) (http.Handler, error) {
	if process.state == nil {
		return nil, trace.BadParameter("teleport process state machine has not yet been initialized (this is a bug)")
	}

	mux := http.NewServeMux()

	if config.enableMetrics {
		mux.Handle("/metrics", process.newMetricsHandler())
	}

	if config.enableProfiling {
		debug.RegisterProfilingHandlers(mux, logger)
	}

	if config.enableHealth {
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			roundtrip.ReplyJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		})
		mux.HandleFunc("/readyz", process.state.readinessHandler())
	}

	if config.enableLogLeveler {
		debug.RegisterLogLevelHandlers(mux, logger, process.Config)
	}

	return mux, nil
}

// newMetricsHandler creates a new metrics handler serving metrics both from the global prometheus registry and the
// in-process one.
func (process *TeleportProcess) newMetricsHandler() http.Handler {
	// We gather metrics both from the in-process registry (preferred metrics registration method)
	// and the global registry (used by some Teleport services and many dependencies).
	gatherers := prometheus.Gatherers{
		process.metricsRegistry,
		prometheus.DefaultGatherer,
	}

	metricsHandler := promhttp.InstrumentMetricHandler(
		process.metricsRegistry, promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			// Errors can happen if metrics are registered with identical names in both the local and the global registry.
			// In this case, we log the error but continue collecting metrics. The first collected metric will win
			// (the one from the local metrics registry takes precedence).
			// As we move more things to the local registry, especially in other tools like tbot, we will have less
			// conflicts in tests.
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog: promHTTPLogAdapter{
				ctx:    process.ExitContext(),
				Logger: process.logger.With(teleport.ComponentKey, teleport.ComponentMetrics),
			},
		}),
	)
	return metricsHandler
}
