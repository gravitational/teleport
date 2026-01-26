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

package bot

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/webclient"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/carotation"
	"github.com/gravitational/teleport/lib/tbot/internal/heartbeat"
	"github.com/gravitational/teleport/lib/tbot/internal/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/bot")

// Bot runs a collection of services/outputs to generate and renew credentials
// on behalf of non-human actors (i.e. machines and workloads).
type Bot struct {
	cfg     Config
	started atomic.Bool
}

// New creates a Bot with the given configuration. Call Run to run the bot in
// long-running "daemon" mode, or OneShot to generate outputs once and then exit.
func New(cfg Config) (*Bot, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Bot{cfg: cfg}, nil
}

// Run the bot until the given context is canceled.
func (b *Bot) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "Bot/Run")
	defer func() { apitracing.EndSpan(span, err) }()

	if b.checkStarted(); err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	registry := readyz.NewRegistry()

	services, closer, err := b.buildServices(ctx, registry)
	defer closer()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, handle := range services {
		// If the service builder called ServiceDependencies.GetStatusReporter,
		// we take that as a promise that the service's Run method will report
		// statuses. Otherwise we will not include the service in heartbeats or
		// the `/readyz` endpoint.
		if handle.statusReporter.used {
			handle.statusReporter.reporter = registry.AddService(handle.serviceType, handle.name)
		}
	}

	b.cfg.Logger.InfoContext(ctx, "Initialization complete. Starting services")

	group, groupCtx := errgroup.WithContext(ctx)
	for _, handle := range services {
		handle := handle
		log := b.cfg.Logger.With("service", handle.name)

		group.Go(func() error {
			log.InfoContext(groupCtx, "Starting service")

			err := handle.service.Run(groupCtx)
			if err != nil {
				log.ErrorContext(ctx, "Service exited with error", "error", err)
				return trace.Wrap(err, "service(%s)", handle.name)
			}

			log.InfoContext(groupCtx, "Service exited")
			return nil
		})
	}
	return group.Wait()
}

// OneShot runs the bot in "one shot" mode to generate outputs once and then exits.
func (b *Bot) OneShot(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "Bot/OneShot")
	defer func() { apitracing.EndSpan(span, err) }()

	if b.checkStarted(); err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	registry := readyz.NewRegistry()

	services, closer, err := b.buildServices(ctx, registry)
	defer closer()
	if err != nil {
		return trace.Wrap(err)
	}

	// Filter out the services that don't support oneshot mode.
	oneShotServices := make([]*serviceHandle, 0, len(services))
	for _, handle := range services {
		handle := handle
		if _, ok := handle.service.(OneShotService); !ok {
			b.cfg.Logger.InfoContext(ctx,
				"Service does not support oneshot mode, will not run",
				"service", handle.name,
			)
			continue
		}

		// Add oneshot services to the registry.
		handle.statusReporter.reporter = registry.AddService(handle.serviceType, handle.name)
		oneShotServices = append(oneShotServices, handle)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, handle := range oneShotServices {
		handle := handle
		log := b.cfg.Logger.With("service", handle.name)

		group.Go(func() error {
			log.DebugContext(groupCtx, "Running service in oneshot mode")

			err := handle.service.(OneShotService).OneShot(groupCtx)
			if err != nil {
				log.ErrorContext(ctx, "Service exited with error", "error", err)
				handle.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
				return trace.Wrap(err, "service(%s)", handle.name)
			}

			log.InfoContext(groupCtx, "Service finished")
			handle.statusReporter.Report(readyz.Healthy)
			return nil
		})
	}
	return group.Wait()
}

func (b *Bot) checkStarted() error {
	if b.started.CompareAndSwap(false, true) {
		return nil
	}
	return trace.BadParameter("bot has already been started")
}

func (b *Bot) buildServices(ctx context.Context, registry *readyz.Registry) ([]*serviceHandle, func(), error) {
	startedAt := time.Now().UTC()

	handles := make([]*serviceHandle, 0, len(b.cfg.Services))
	closeFn := func() {
		for _, h := range handles {
			if h.closeFn != nil {
				h.closeFn()
			}
		}
	}

	// Build internal services and dependencies.
	reloadBroadcaster := b.buildReloadBroadcaster(ctx)

	resolver, err := b.buildResolver(ctx)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building resolver")
	}

	clientBuilder, err := b.buildClientBuilder(resolver)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building client builder")
	}

	identityService, identityServiceHandle, err := b.buildIdentityService(
		ctx,
		reloadBroadcaster,
		clientBuilder,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building identity service")
	}
	handles = append(handles, identityServiceHandle)

	heartbeatService, err := b.buildHeartbeatService(
		identityService,
		startedAt,
		registry,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building heartbeat service")
	}
	handles = append(handles, heartbeatService)

	caRotationService, err := b.buildCARotationService(
		reloadBroadcaster,
		identityService,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building CA rotation service")
	}
	handles = append(handles, caRotationService)

	proxyPinger, err := b.buildProxyPinger(identityService)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building proxy pinger")
	}

	identityGenerator, err := identityService.GetGenerator()
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building identity generator")
	}

	// Build user services.
	for idx, builder := range b.cfg.Services {
		reloadCh, unsubscribe := reloadBroadcaster.Subscribe()

		handle := &serviceHandle{
			closeFn:        unsubscribe,
			statusReporter: &statusReporter{},
		}
		handle.serviceType, handle.name = builder.GetTypeAndName()

		var err error
		handle.service, err = builder.Build(ServiceDependencies{
			Client:             identityService.GetClient(),
			Resolver:           resolver,
			ClientBuilder:      clientBuilder,
			IdentityGenerator:  identityGenerator,
			ProxyPinger:        proxyPinger,
			BotIdentity:        identityService.GetIdentity,
			BotIdentityReadyCh: identityService.Ready(),
			ReloadCh:           reloadCh,
			StatusRegistry:     registry,
			GetStatusReporter: func() readyz.Reporter {
				handle.statusReporter.used = true
				return handle.statusReporter
			},
			Logger: b.cfg.Logger.With(
				teleport.ComponentKey,
				teleport.Component(teleport.ComponentTBot, "svc", handle.name),
			),
		})
		if err != nil {
			return nil, closeFn, trace.Wrap(err, "building service [%d]", idx)
		}

		handles = append(handles, handle)
	}
	return handles, closeFn, nil
}

func (b *Bot) buildReloadBroadcaster(ctx context.Context) *internal.ChannelBroadcaster {
	broadcaster := internal.NewChannelBroadcaster()
	if b.cfg.ReloadCh != nil {
		go func() {
			for {
				select {
				case <-b.cfg.ReloadCh:
					broadcaster.Broadcast()
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return broadcaster
}

func (b *Bot) buildResolver(ctx context.Context) (reversetunnelclient.Resolver, error) {
	if b.cfg.Connection.StaticProxyAddress {
		return reversetunnelclient.StaticResolver(
			b.cfg.Connection.Address,

			// If the user has indicated they want tbot to prefer using the proxy
			// address they have configured, we use a static resolver set to this
			// address. We also assume that they have TLS routing/multiplexing
			// enabled, since otherwise we'd need them to manually configure an
			// an entry for each kind of address.
			types.ProxyListenerMode_Multiplex,
		), nil
	}

	return reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: b.cfg.Connection.Address,
			Insecure:  b.cfg.Connection.Insecure,
		}),
		nil, /* clock */
	)
}

func (b *Bot) buildClientBuilder(resolver reversetunnelclient.Resolver) (*client.Builder, error) {
	return client.NewBuilder(client.BuilderConfig{
		Connection: b.cfg.Connection,
		Resolver:   resolver,
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, "client"),
		),
		Metrics: b.cfg.ClientMetrics,
	})
}

func (b *Bot) buildIdentityService(
	ctx context.Context,
	reloadBroadcaster *internal.ChannelBroadcaster,
	clientBuilder *client.Builder,
) (*identity.Service, *serviceHandle, error) {
	handle := &serviceHandle{
		serviceType:    "internal/identity",
		name:           "identity",
		statusReporter: &statusReporter{used: true},
	}

	reloadCh, unsubscribe := reloadBroadcaster.Subscribe()
	identityService, err := identity.NewService(identity.Config{
		Connection:      b.cfg.Connection,
		Onboarding:      b.cfg.Onboarding,
		Destination:     b.cfg.InternalStorage,
		TTL:             b.cfg.CredentialLifetime.TTL,
		RenewalInterval: b.cfg.CredentialLifetime.RenewalInterval,
		FIPS:            b.cfg.FIPS,
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, handle.name),
		),
		ClientBuilder:  clientBuilder,
		ReloadCh:       reloadCh,
		StatusReporter: handle.statusReporter,
	})
	if err != nil {
		unsubscribe()
		return nil, nil, trace.Wrap(err, "building identity service")
	}

	handle.service = identityService
	handle.closeFn = func() {
		if err := identityService.Close(); err != nil {
			b.cfg.Logger.ErrorContext(
				ctx,
				"Failed to close bot identity service",
				"error", err,
			)
		}
		unsubscribe()
	}

	if err := identityService.Initialize(ctx); err != nil {
		handle.closeFn()
		return nil, nil, trace.Wrap(err, "initializing identity service")
	}
	return identityService, handle, nil
}

func (b *Bot) buildHeartbeatService(
	identityService *identity.Service,
	startedAt time.Time,
	statusRegistry *readyz.Registry,
) (*serviceHandle, error) {
	handle := &serviceHandle{
		serviceType:    "internal/heartbeat",
		name:           "heartbeat",
		statusReporter: &statusReporter{used: true},
	}

	var err error
	handle.service, err = heartbeat.NewService(heartbeat.Config{
		Interval:           30 * time.Minute,
		RetryLimit:         5,
		Client:             machineidv1.NewBotInstanceServiceClient(identityService.GetClient().GetConnection()),
		BotIdentityReadyCh: identityService.Ready(),
		StartedAt:          startedAt,
		JoinMethod:         b.cfg.Onboarding.JoinMethod,
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, handle.name),
		),
		StatusReporter: handle.statusReporter,
		StatusRegistry: statusRegistry,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return handle, nil
}

func (b *Bot) buildProxyPinger(identityService *identity.Service) (connection.ProxyPinger, error) {
	return internal.NewCachingProxyPinger(internal.CachingProxyPingerConfig{
		Connection: b.cfg.Connection,
		Client:     identityService.GetClient(),
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, "proxy-pinger"),
		),
	})
}

func (b *Bot) buildCARotationService(
	reloadBroadcaster *internal.ChannelBroadcaster,
	identityService *identity.Service,
) (*serviceHandle, error) {
	handle := &serviceHandle{
		serviceType:    "internal/ca-rotation",
		name:           "ca-rotation",
		statusReporter: &statusReporter{used: true},
	}

	var err error
	handle.service, err = carotation.NewService(carotation.Config{
		BroadcastFn:        reloadBroadcaster.Broadcast,
		Client:             identityService.GetClient(),
		GetBotIdentityFn:   identityService.GetIdentity,
		BotIdentityReadyCh: identityService.Ready(),
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, handle.name),
		),
		StatusReporter: handle.statusReporter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return handle, nil
}

// serviceHandle contains a built service, its type/name, close function, and a
// pointer to its status reporter - so we can automatically report statuses for
// oneshot services.
type serviceHandle struct {
	serviceType, name string
	service           Service
	statusReporter    *statusReporter
	closeFn           func()
}

// statusReporter wraps readyz.Reporter to break a circular dependency where:
//
//   - We need a status reporter to build a service
//   - We must register the service in order to get the status reporter
//   - We may not want to register the service in one-shot mode if it does not
//     implement OneShotService
//   - We can only know whether the service implements OneShotService after
//     we've built it
//
// This wrapper allows us to defer the actual registration until we know whether
// the service implements OneShotService.
type statusReporter struct {
	used     bool
	reporter readyz.Reporter
}

func (r *statusReporter) Report(status readyz.Status) {
	if r.reporter != nil {
		r.reporter.Report(status)
	}
}

func (r *statusReporter) ReportReason(status readyz.Status, reason string) {
	if r.reporter != nil {
		r.reporter.ReportReason(status, reason)
	}
}
