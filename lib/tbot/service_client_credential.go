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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// ClientCredentialOutputService produces credentials which can be used to
// connect to Teleport's API or SSH.
type ClientCredentialOutputService struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient     *authclient.Client
	botCfg            *config.BotConfig
	cfg               *config.UnstableClientCredentialOutput
	getBotIdentity    getBotIdentityFn
	log               *slog.Logger
	reloadBroadcaster *channelBroadcaster
}

func (s *ClientCredentialOutputService) String() string {
	return "client-credential-output"
}

func (s *ClientCredentialOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *ClientCredentialOutputService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		name:       "output-renewal",
		f:          s.generate,
		interval:   s.botCfg.CertificateLifetime.RenewalInterval,
		retryLimit: renewalRetryLimit,
		log:        s.log,
		reloadCh:   reloadCh,
	})
	return trace.Wrap(err)
}

func (s *ClientCredentialOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"ClientCredentialOutputService/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return trace.Wrap(err, "fetching default roles")
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateLifetime.TTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}

	s.cfg.SetOrUpdateFacade(id)
	return nil
}
