/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package diagnostics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

// ServiceBuilder returns a builder for the diagnostics service.
func ServiceBuilder(cfg Config) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		return NewService(cfg, deps.StatusRegistry)
	}
	return bot.NewServiceBuilder("internal/diagnostics", "diagnostics", buildFn)
}

// Config contains configuration for the diagnostics service.
type Config struct {
	Address      string
	PProfEnabled bool
	Logger       *slog.Logger
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.Address == "" {
		return trace.BadParameter("Address is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// NewService creates a new diagnostics service.
func NewService(cfg Config, registry readyz.ReadOnlyRegistry) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if registry == nil {
		return nil, trace.BadParameter("registry is required")
	}
	return &Service{
		log:            cfg.Logger,
		diagAddr:       cfg.Address,
		pprofEnabled:   cfg.PProfEnabled,
		statusRegistry: registry,
	}, nil
}

// Service is a [bot.Service] that exposes diagnostics endpoints. It's only
// started when a --diag-addr is provided.
type Service struct {
	log            *slog.Logger
	diagAddr       string
	pprofEnabled   bool
	statusRegistry readyz.ReadOnlyRegistry
}

func (s *Service) String() string {
	return "diagnostics"
}

func (s *Service) Run(ctx context.Context) error {
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
		msg := "404 - Not Found\n\nI'm a little tbot,\nshort and stout,\nthe page you seek,\nis not about.\n\nYou can find out more information about the diagnostics service at https://goteleport.com/docs/reference/machine-id/diagnostics-service/"
		_, _ = w.Write([]byte(msg))
	}))
	mux.Handle("/livez", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	mux.Handle("/readyz", readyz.HTTPHandler(s.statusRegistry))
	mux.Handle("/readyz/", readyz.HTTPHandler(s.statusRegistry))
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

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
