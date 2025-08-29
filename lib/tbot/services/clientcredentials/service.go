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

package clientcredentials

import (
	"cmp"
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

// ServiceBuilder creates a new client credential service with the given
// configuration.
//
// Note: when using the client credentials service to provide credentials to
// another service (e.g. the SPIFFE Workload API service) use NewSidecar instead.
func ServiceBuilder(cfg *UnstableConfig, credentialLifetime bot.CredentialLifetime) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &Service{
			botAuthClient:      deps.Client,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			credentialLifetime: credentialLifetime,
			cfg:                cfg,
			reloadCh:           deps.ReloadCh,
			identityGenerator:  deps.IdentityGenerator,
		}
		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

// NewSidecar creates a client credential service intended to provide credentials
// to another service.
func NewSidecar(deps bot.ServiceDependencies, credentialLifetime bot.CredentialLifetime) (*Service, *UnstableConfig) {
	cfg := &UnstableConfig{}
	svc := &Service{
		botAuthClient:      deps.Client,
		botIdentityReadyCh: deps.BotIdentityReadyCh,
		credentialLifetime: credentialLifetime,
		cfg:                cfg,
		reloadCh:           deps.ReloadCh,
		identityGenerator:  deps.IdentityGenerator,
		statusReporter:     readyz.NoopReporter(),
	}
	svc.log = deps.LoggerForService(svc)
	return svc, cfg
}

// Service produces credentials which can be used to connect to Teleport's API or SSH.
type Service struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient      *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	credentialLifetime bot.CredentialLifetime
	cfg                *UnstableConfig
	log                *slog.Logger
	statusReporter     readyz.Reporter
	reloadCh           <-chan struct{}
	identityGenerator  *identity.Generator
}

func (s *Service) String() string {
	return cmp.Or(
		s.cfg.Name,
		"client-credential-output",
	)
}

func (s *Service) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *Service) Run(ctx context.Context) error {
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service:         s.String(),
		Name:            "output-renewal",
		F:               s.generate,
		Interval:        s.credentialLifetime.RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
		StatusReporter:  s.statusReporter,
	})
	return trace.Wrap(err)
}

func (s *Service) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"Service/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	id, err := s.identityGenerator.Generate(ctx,
		identity.WithLifetime(s.credentialLifetime.TTL, s.credentialLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}

	s.cfg.SetOrUpdateFacade(id)
	return nil
}
