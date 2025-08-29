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

package database

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func OutputServiceBuilder(cfg *OutputConfig, defaultCredentialLifetime bot.CredentialLifetime) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &OutputService{
			botAuthClient:             deps.Client,
			botIdentityReadyCh:        deps.BotIdentityReadyCh,
			defaultCredentialLifetime: defaultCredentialLifetime,
			cfg:                       cfg,
			reloadCh:                  deps.ReloadCh,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
		}
		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

// OutputService generates the artifacts necessary to connect to a
// database using Teleport.
type OutputService struct {
	botAuthClient             *apiclient.Client
	botIdentityReadyCh        <-chan struct{}
	defaultCredentialLifetime bot.CredentialLifetime
	cfg                       *OutputConfig
	log                       *slog.Logger
	statusReporter            readyz.Reporter
	reloadCh                  <-chan struct{}
	identityGenerator         *identity.Generator
	clientBuilder             *client.Builder
}

func (s *OutputService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("database-output (%s)", s.cfg.Destination.String()),
	)
}

func (s *OutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *OutputService) Run(ctx context.Context) error {
	err := internal.RunOnInterval(ctx, internal.RunOnIntervalConfig{
		Service:         s.String(),
		Name:            "output-renewal",
		F:               s.generate,
		Interval:        cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval,
		RetryLimit:      internal.RenewalRetryLimit,
		Log:             s.log,
		ReloadCh:        s.reloadCh,
		IdentityReadyCh: s.botIdentityReadyCh,
		StatusReporter:  s.statusReporter,
	})
	return trace.Wrap(err)
}

func (s *OutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputService/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	// Check the ACLs. We can't fix them, but we can warn if they're
	// misconfigured. We'll need to precompute a list of keys to check.
	// Note: This may only log a warning, depending on configuration.
	if err := s.cfg.Destination.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
		return trace.Wrap(err)
	}
	// Ensure this destination is also writable. This is a hard fail if
	// ACLs are misconfigured, regardless of configuration.
	if err := identity.VerifyWrite(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "verifying destination")
	}

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	identityOpts := []identity.GenerateOption{
		identity.WithRoles(s.cfg.Roles),
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	}
	id, err := s.identityGenerator.GenerateFacade(ctx, identityOpts...)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()

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

	routedIdentity, err := s.identityGenerator.Generate(ctx, append(identityOpts,
		identity.WithCurrentIdentityFacade(id),
		identity.WithRouteToDatabase(route),
	)...)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.InfoContext(
		ctx,
		"Generated identity for database",
		"db_service", route.ServiceName,
	)

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(noah): It's likely the Database output does not really need to
	// output all these CAs - but - for backwards compat reasons, we output them.
	// Revisit this at a later date and make a call.
	userCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.UserCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	databaseCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.DatabaseCA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.render(ctx, routedIdentity, hostCAs, userCAs, databaseCAs), "rendering")
}

func (s *OutputService) render(
	ctx context.Context,
	routedIdentity *identity.Identity,
	hostCAs, userCAs, databaseCAs []types.CertAuthority,
) error {
	ctx, span := tracer.Start(
		ctx,
		"OutputService/render",
	)
	defer span.End()

	keyRing, err := internal.NewClientKeyRing(routedIdentity, hostCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := internal.WriteTLSCAs(ctx, s.cfg.Destination, hostCAs, userCAs, databaseCAs); err != nil {
		return trace.Wrap(err)
	}

	if err := internal.WriteIdentityFile(ctx, s.log, keyRing, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "writing identity file")
	}
	if err := identity.SaveIdentity(
		ctx, routedIdentity, s.cfg.Destination, identity.DestinationKinds()...,
	); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	switch s.cfg.Format {
	case MongoDatabaseFormat:
		if err := writeMongoDatabaseFiles(
			ctx, s.log, routedIdentity, databaseCAs, s.cfg.Destination,
		); err != nil {
			return trace.Wrap(err, "writing cockroach database files")
		}
	case CockroachDatabaseFormat:
		if err := writeCockroachDatabaseFiles(
			ctx, s.log, routedIdentity, databaseCAs, s.cfg.Destination,
		); err != nil {
			return trace.Wrap(err, "writing cockroach database files")
		}
	case TLSDatabaseFormat:
		if err := internal.WriteIdentityFileTLS(
			ctx, s.log, keyRing, s.cfg.Destination,
		); err != nil {
			return trace.Wrap(err, "writing tls database format files")
		}
	}

	return nil
}

func writeCockroachDatabaseFiles(
	ctx context.Context,
	log *slog.Logger,
	routedIdentity *identity.Identity,
	databaseCAs []types.CertAuthority,
	dest destination.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"writeCockroachDatabaseFiles",
	)
	defer span.End()

	// Cockroach format specifically uses database CAs rather than hostCAs
	keyRing, err := internal.NewClientKeyRing(routedIdentity, databaseCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: DefaultCockroachDirName,
		Writer:     internal.NewBotConfigWriter(ctx, dest, DefaultCockroachDirName),
		KeyRing:    keyRing,
		Format:     identityfile.FormatCockroach,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote CockroachDB files", "files", files)
	return nil
}

func writeMongoDatabaseFiles(
	ctx context.Context,
	log *slog.Logger,
	routedIdentity *identity.Identity,
	databaseCAs []types.CertAuthority,
	dest destination.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"writeMongoDatabaseFiles",
	)
	defer span.End()

	// Mongo format specifically uses database CAs rather than hostCAs
	keyRing, err := internal.NewClientKeyRing(routedIdentity, databaseCAs)
	if err != nil {
		return trace.Wrap(err)
	}

	cfg := identityfile.WriteConfig{
		OutputPath: DefaultMongoPrefix,
		Writer:     internal.NewBotConfigWriter(ctx, dest, ""),
		KeyRing:    keyRing,
		Format:     identityfile.FormatMongo,
		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote MongoDB identity files", "files", files)
	return nil
}

// chooseOneDatabase chooses one matched database by name, or tries to choose
// one database by unambiguous "discovered name".
func chooseOneDatabase(databases []types.Database, name string) (types.Database, error) {
	return internal.ChooseOneResource(databases, name, "database")
}
