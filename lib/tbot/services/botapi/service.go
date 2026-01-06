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

package botapi

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	apiclient "github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/tbot/services/botapi/botapiconfig"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

type Spawner interface {
	AddDynamicService(ctx context.Context, cfg config.ServiceConfig) error
}

func ServiceBuilder(
	cfg *botapiconfig.Config,
	spawner Spawner,
) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &Service{
			cfg:                cfg,
			log:                deps.Logger,
			botClient:          deps.Client,
			getBotIdentity:     deps.BotIdentity,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			statusReporter:     deps.GetStatusReporter(),
			spawner:            spawner,
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(botapiconfig.ServiceType, cfg.Name, buildFn)
}

type Service struct {
	cfg                *botapiconfig.Config
	log                *slog.Logger
	botClient          *apiclient.Client
	getBotIdentity     func() *identity.Identity
	botIdentityReadyCh <-chan struct{}
	statusReporter     readyz.Reporter

	spawner Spawner
}

func (s *Service) String() string {
	return cmp.Or(
		s.cfg.Name,
		"bot-api",
	)
}

func (s *Service) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "BotAPIService/Run")
	defer span.End()

	l := s.cfg.Listener
	if l == nil {
		s.log.DebugContext(ctx, "Opening listener for bot api.", "listen", s.cfg.Listen)
		var err error
		l, err = internal.CreateListener(ctx, s.log, s.cfg.Listen)
		if err != nil {
			return trace.Wrap(err, "opening listener")
		}
		defer func() {
			if err := l.Close(); err != nil && !utils.IsUseOfClosedNetworkError(err) {
				s.log.ErrorContext(ctx, "Failed to close listener", "error", err)
			}
		}()
	}

	if s.botIdentityReadyCh != nil {
		select {
		case <-s.botIdentityReadyCh:
		default:
			s.log.InfoContext(ctx, "Waiting for internal bot identity to be renewed before running")
			select {
			case <-s.botIdentityReadyCh:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if s.spawner == nil {
		return trace.BadParameter("a spawner implementation is required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/spawn", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body config.ServiceConfigs
		if err := yaml.NewDecoder(r.Body).Decode(&body); err != nil {
			s.log.ErrorContext(r.Context(), "invalid dynamic service config", "error", err)
			http.Error(w, fmt.Sprintf("invalid service config: %v", err), http.StatusBadRequest)
			return
		}

		for i, cfg := range body {
			s.log.DebugContext(r.Context(), "attempting to spawn new dynamic service", "name", cfg.GetName(), "type", cfg.Type())
			if err := s.spawner.AddDynamicService(r.Context(), cfg); err != nil {
				s.log.ErrorContext(r.Context(), "unable to add dynamic service", "error", err)
				http.Error(w, fmt.Sprintf("unable to add dynamic service [%d]: %v", i, err), http.StatusInternalServerError)
				return
			}

			s.log.InfoContext(r.Context(), "spawned new dynamic service", "name", cfg.GetName(), "type", cfg.Type())
		}

		w.WriteHeader(http.StatusOK)
	}))

	srv := http.Server{
		Handler:           mux,
		ReadTimeout:       apidefaults.DefaultIOTimeout,
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		WriteTimeout:      apidefaults.DefaultIOTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			// Use the main context which controls the service being stopped as
			// the base context for all incoming requests.
			return ctx
		},
	}

	var errCh = make(chan error, 1)
	go func() {
		s.log.DebugContext(ctx, "Starting bot api request handler goroutine")
		errCh <- srv.Serve(l)
	}()
	s.log.InfoContext(
		ctx, "Listening for bot api requests",
		"address", l.Addr().String(),
	)
	s.statusReporter.Report(readyz.Healthy)

	select {
	case <-ctx.Done():
		return trace.Wrap(srv.Close(), "closing http server")
	case err := <-errCh:
		s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
		return trace.Wrap(err, "bot api failed")
	}
}
