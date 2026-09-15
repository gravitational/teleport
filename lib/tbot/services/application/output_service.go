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

package application

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func OutputServiceBuilder(cfg *OutputConfig, defaultCredentialLifetime bot.CredentialLifetime) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &OutputService{
			botAuthClient:      deps.Client,
			getBotIdentity:     deps.BotIdentity,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			effectiveLifetime:  cmp.Or(cfg.CredentialLifetime, defaultCredentialLifetime),
			cfg:                cfg,
			reloadCh:           deps.ReloadCh,
			identityGenerator:  deps.IdentityGenerator,
			clientBuilder:      deps.ClientBuilder,
			log:                deps.Logger,
			statusReporter:     deps.GetStatusReporter(),
			scoped:             deps.Scoped,
		}
		return svc, nil
	}
	return bot.NewServiceBuilder(OutputServiceType, cfg.Name, buildFn)
}

// OutputService generates the artifacts necessary to connect to a
// HTTP or TCP application using Teleport.
type OutputService struct {
	botAuthClient      *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	effectiveLifetime  bot.CredentialLifetime
	cfg                *OutputConfig
	getBotIdentity     func() *identity.Identity
	log                *slog.Logger
	reloadCh           <-chan struct{}
	statusReporter     readyz.Reporter
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
	scoped             bool
}

func (s *OutputService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("application-output (%s)", s.cfg.Destination.String()),
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
		Interval:        s.effectiveLifetime.RenewalInterval,
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

	var routedIdentity *identity.Identity
	var err error

	if s.scoped {
		routedIdentity, err = s.generateScopedIdentity(ctx)
	} else {
		routedIdentity, err = s.generateUnscopedIdentity(ctx)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(noah): It's likely the Application output does not really need to
	// output these CAs - but - for backwards compat reasons, we output them.
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

func (s *OutputService) generateUnscopedIdentity(ctx context.Context) (*identity.Identity, error) {
	identityOpts := []identity.GenerateOption{
		identity.WithLifetime(s.effectiveLifetime.TTL, s.effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	}
	if s.cfg.DelegationSessionID != "" {
		identityOpts = append(identityOpts, identity.WithDelegation(s.cfg.DelegationSessionID))
	}
	id, err := s.identityGenerator.GenerateFacade(ctx, identityOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	app, err := getAppLegacy(ctx, impersonatedClient, s.cfg.AppName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: s.getBotIdentity().ClusterName,
		Scope:       app.GetScope(),
	}

	routedIdentity, err := s.identityGenerator.Generate(ctx, append(identityOpts,
		identity.WithCurrentIdentityFacade(id),
		identity.WithRouteToApp(routeToApp),
	)...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.log.InfoContext(
		ctx,
		"Generated identity for app",
		"app_name", routeToApp.Name,
	)

	return routedIdentity, nil
}

func (s *OutputService) generateScopedIdentity(ctx context.Context) (*identity.Identity, error) {
	qn, err := scopes.ParseQualifiedName(s.cfg.AppName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := qn.StrongValidate(); err != nil {
		return nil, err
	}

	app, err := getApp(ctx, s.botAuthClient, qn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: s.getBotIdentity().ClusterName,
		Scope:       app.GetScope(),
	}

	routedIdentity, err := s.identityGenerator.GenerateScoped(
		ctx, s.effectiveLifetime.TTL, s.effectiveLifetime.RenewalInterval, identity.UsageApp(routeToApp),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.log.InfoContext(
		ctx,
		"Generated scoped identity for app",
		"app_name", routeToApp.Name,
	)

	return routedIdentity, nil
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

	if err := internal.WriteIdentityFile(ctx, s.log, keyRing, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "writing identity file")
	}
	if err := identity.SaveIdentity(
		ctx, routedIdentity, s.cfg.Destination, identity.DestinationKinds()...,
	); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	if s.cfg.SpecificTLSExtensions {
		if err := internal.WriteIdentityFileTLS(ctx, s.log, keyRing, s.cfg.Destination); err != nil {
			return trace.Wrap(err, "writing specific tls extension files")
		}
	}

	return trace.Wrap(internal.WriteTLSCAs(ctx, s.cfg.Destination, hostCAs, userCAs, databaseCAs))
}
