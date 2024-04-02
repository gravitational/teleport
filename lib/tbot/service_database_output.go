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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

type ServiceDatabaseOutput struct {
	cfg    *config.DatabaseOutput
	botCfg *config.BotConfig
	log    logrus.FieldLogger

	resolver          reversetunnelclient.Resolver
	botClient         *auth.Client
	reloadBroadcaster *channelBroadcaster
	getBotIdentity    getBotIdentityFn
}

func (s *ServiceDatabaseOutput) generate(ctx context.Context, roles []string) error {
	ctx, span := tracer.Start(
		ctx,
		"ServiceDatabaseOutput/generate",
	)
	defer span.End()

	// Before we do anything, we test the output destination to make sure we
	// can write.
	if err := testDestination(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "testing destination")
	}

	impersonatedIdentity, err := generateIdentity(
		ctx,
		s.botClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateTTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	facade := identity.NewFacade(
		s.botCfg.FIPS, s.botCfg.Insecure, impersonatedIdentity,
	)
	impersonatedClient, err := clientForFacade(
		ctx, s.log, s.botCfg, facade, s.resolver,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := impersonatedClient.Close(); err != nil {
			s.log.WithError(err).Error("Failed to close client")
		}
	}()

	route, err := getRouteToDatabase(
		ctx,
		s.log,
		impersonatedClient,
		s.cfg.Service,
		s.cfg.Username,
		s.cfg.Database,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	routedIdentity, err := generateIdentity(
		ctx,
		s.botClient,
		impersonatedIdentity,
		roles,
		s.botCfg.CertificateTTL,
		func(req *proto.UserCertsRequest) {
			req.RouteToDatabase = route
		},
	)

	var dbCAs []types.CertAuthority   // TODO
	var hostCAs []types.CertAuthority // TODO
	key, err := config.NewClientKey(routedIdentity, dbCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Infof("Generated identity for database %q", s.cfg.Service)

	// Write Identity
	if err := identity.SaveIdentity(
		ctx, routedIdentity, s.cfg.Destination, identity.DestinationKinds()...,
	); err != nil {
		return trace.Wrap(err, "saving identity")
	}

	// Write IdentityFile
	cfg := identityfile.WriteConfig{
		OutputPath: config.IdentityFilePath,
		Writer: &config.BotConfigWriter{
			Ctx:  ctx,
			Dest: s.cfg.Destination,
		},
		Key:    key,
		Format: identityfile.FormatFile,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}
	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Debugf("Wrote identity file: %+v", files)

	// Write TLSCAs
	if err := s.cfg.Destination.Write(
		ctx, config.DatabaseCAPath, config.ConcatCACerts(dbCAs),
	); err != nil {
		return trace.Wrap(err)
	}
	if err := s.cfg.Destination.Write(
		ctx, config.HostCAPath, config.ConcatCACerts(hostCAs),
	); err != nil {
		return trace.Wrap(err)
	}

	// Write format specific files
	switch s.cfg.Format {
	case config.MongoDatabaseFormat:
		if err := s.renderMongo(ctx, key); err != nil {
			return trace.Wrap(err)
		}
	case config.CockroachDatabaseFormat:
		if err := s.renderCockroach(ctx, key); err != nil {
			return trace.Wrap(err)
		}
	case config.TLSDatabaseFormat:
		// This is a special case where we include the Host CA instead.
		key, err := config.NewClientKey(routedIdentity, hostCAs)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := s.renderTLS(ctx, key); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (s *ServiceDatabaseOutput) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ServiceDatabaseOutput/Run")
	defer span.End()

	// Determine the roles to use for the impersonated db access user. We fall
	// back to all the roles the bot has if none are configured.
	roles := s.cfg.Roles
	if len(roles) == 0 {
		roles, err := fetchDefaultRoles(ctx, s.botClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err, "fetching default roles")
		}
		s.log.WithField("roles", roles).Debug("No roles configured, using all roles available.")
	}

	return renewLoop(ctx, s.log, s.botCfg.RenewalInterval, s.reloadBroadcaster, func(ctx context.Context) error {
		return s.generate(ctx, roles)
	})
}

func (s *ServiceDatabaseOutput) OneShot(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ServiceDatabaseOutput/OneShot")
	defer span.End()
	// Determine the roles to use for the impersonated db access user. We fall
	// back to all the roles the bot has if none are configured.
	roles := s.cfg.Roles
	if len(roles) == 0 {
		roles, err := fetchDefaultRoles(ctx, s.botClient, s.getBotIdentity())
		if err != nil {
			return trace.Wrap(err, "fetching default roles")
		}
		s.log.WithField("roles", roles).Debug("No roles configured, using all roles available.")
	}
	return s.generate(ctx, roles)
}

func (s *ServiceDatabaseOutput) Describe() []config.FileDescription {
	fds := []config.FileDescription{
		{
			Name: config.IdentityFilePath,
		},
		{
			Name: config.HostCAPath,
		},
		{
			Name: config.DatabaseCAPath,
		},
	}
	switch s.cfg.Format {
	case config.MongoDatabaseFormat:
		fds = append(fds, []config.FileDescription{
			{
				Name: "mongo.crt",
			},
			{
				Name: "mongo.cas",
			},
		}...)
	case config.CockroachDatabaseFormat:
		fds = append(fds, []config.FileDescription{
			{
				Name:  "cockroach",
				IsDir: true,
			},
		}...)
	case config.TLSDatabaseFormat:
		fds = append(fds, []config.FileDescription{
			{
				Name: "tls.key",
			},
			{
				Name: "tls.crt",
			},
			{
				Name: "tls.cas",
			},
		}...)
	}

	return fds
}

func (s *ServiceDatabaseOutput) renderMongo(ctx context.Context, key *client.Key) error {
	ctx, span := tracer.Start(
		ctx,
		"ServiceDatabaseOutput/renderMongo",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: "mongo",
		Writer: &config.BotConfigWriter{
			Ctx:  ctx,
			Dest: s.cfg.Destination,
		},
		Key:    key,
		Format: identityfile.FormatMongo,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Debugf("Wrote MongoDB identity files: %+v", files)
	return nil
}

func (s *ServiceDatabaseOutput) renderCockroach(ctx context.Context, key *client.Key) error {
	ctx, span := tracer.Start(
		ctx,
		"ServiceDatabaseOutput/renderCockroach",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: "cockroach",
		Writer: &config.BotConfigWriter{
			Ctx:     ctx,
			Dest:    s.cfg.Destination,
			SubPath: "cockroach",
		},
		Key:    key,
		Format: identityfile.FormatCockroach,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Debugf("Wrote CockroachDB files: %+v", files)
	return nil
}

func (s *ServiceDatabaseOutput) renderTLS(ctx context.Context, key *client.Key) error {
	ctx, span := tracer.Start(
		ctx,
		"ServiceDatabaseOutput/renderTLS",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: "tls",
		Writer: &config.BotConfigWriter{
			Ctx:  ctx,
			Dest: s.cfg.Destination,
		},
		Key:    key,
		Format: identityfile.FormatTLS,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Debugf("Wrote TLS identity files: %+v", files)
	return nil
}

func (s *ServiceDatabaseOutput) String() string {
	return fmt.Sprintf("%s (%s)", config.DatabaseOutputType, s.cfg.Destination)
}

func (s *ServiceDatabaseOutput) GetDestination() bot.Destination {
	return s.cfg.Destination
}
