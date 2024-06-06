/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// SSHProxyService
type SSHProxyService struct {
	cfg         *config.SSHProxyService
	svcIdentity *config.UnstableClientCredentialOutput
	botCfg      *config.BotConfig
	log         *slog.Logger
	resolver    reversetunnelclient.Resolver

	// client holds the impersonated client for the service
	client *authclient.Client
}

func (s *SSHProxyService) Run(ctx context.Context) error {
	return trace.NotImplemented("SSHProxyService.Run is not implemented")
}

func (s *SSHProxyService) String() string {
	return config.SSHProxyServiceType
}
