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
	"context"
	"log/slog"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/loop"
)

// ServiceBuilder creates a new client credential service with the given configuration.
func ServiceBuilder(credentialLifetime bot.CredentialLifetime, cfg *UnstableConfig) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		svc := &Service{
			botAuthClient:      deps.Client,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			credentialLifetime: credentialLifetime,
			cfg:                cfg,
			reloadCh:           deps.ReloadCh,
			identityGenerator:  deps.IdentityGenerator,
		}
		svc.log = deps.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, "svc", svc.String()),
		)
		return svc, nil
	}
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
	reloadCh           <-chan struct{}
	identityGenerator  *identity.Generator
}

func (s *Service) String() string {
	return "client-credential-output"
}

func (s *Service) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *Service) Run(ctx context.Context) error {
	err := loop.Run(ctx, loop.Config{
		Service:         s.String(),
		Name:            "output-renewal",
		Fn:              s.generate,
		Interval:        s.credentialLifetime.RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
	})
	return trace.Wrap(err)
}

func (s *Service) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"ClientCredentialService/generate",
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
