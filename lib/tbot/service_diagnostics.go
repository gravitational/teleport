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

package tbot

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
)

// diagnosticsService is a [bot.Service] that exposes diagnostics endpoints.
// It's only started when a --diag-addr is provided.
type diagnosticsService struct {
	log          *slog.Logger
	diagAddr     string
	pprofEnabled bool
}

func (s *diagnosticsService) String() string {
	return "diagnostics"
}

func (s *diagnosticsService) Run(ctx context.Context) error {
	s.log.InfoContext(
		ctx,
		"diagnostics service will be starting",
		"addr", s.diagAddr,
	)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	// Only expose pprof when `-d` is provided.
	if s.pprofEnabled {
		s.log.InfoContext(ctx, "debug mode enabled, profiling endpoints will be served on the diagnostics service.")
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		msg := "404 - Not Found\n\nI'm a little tbot,\nshort and stout,\nthe page you seek,\nis not about.\n\nYou can find out more information about the diagnostics service at https://goteleport.com/docs/machine-id/reference/diagnostics-service/"
		_, _ = w.Write([]byte(msg))
	}))
	mux.Handle("/livez", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	mux.Handle("/readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(noah): Eventually this should diverge from /livez and report
		// the readiness status from each sub-service, with an error status if
		// any of them are not ready.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	srv := http.Server{
		Addr:              s.diagAddr,
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
	}
	go func() {
		<-ctx.Done()
		if err := srv.Close(); err != nil {
			s.log.WarnContext(ctx, "Failed to close HTTP server", "error", err)
		}
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}
