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

	services, closer, err := b.buildServices(ctx)
	defer closer()
	if err != nil {
		return trace.Wrap(err)
	}

	b.cfg.Logger.InfoContext(ctx, "Initialization complete. Starting services")

	group, groupCtx := errgroup.WithContext(ctx)
	for _, svc := range services {
		svc := svc
		log := b.cfg.Logger.With("service", svc.String())

		group.Go(func() error {
			log.InfoContext(groupCtx, "Starting service")

			err := svc.Run(groupCtx)
			if err != nil {
				log.ErrorContext(ctx, "Service exited with error", "error", err)
				return trace.Wrap(err, "service(%s)", svc.String())
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

	services, closer, err := b.buildServices(ctx)
	defer closer()
	if err != nil {
		return trace.Wrap(err)
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, service := range services {
		log := b.cfg.Logger.With("service", service.String())

		svc, ok := service.(OneShotService)
		if !ok {
			log.InfoContext(ctx, "Service does not support oneshot mode, will not run")
			continue
		}

		group.Go(func() error {
			log.DebugContext(groupCtx, "Running service in oneshot mode")

			err := svc.OneShot(groupCtx)
			if err != nil {
				log.ErrorContext(ctx, "Service exited with error", "error", err)
				return trace.Wrap(err, "service(%s)", svc.String())
			}

			log.InfoContext(groupCtx, "Service finished")
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

func (b *Bot) buildServices(ctx context.Context) ([]Service, func(), error) {
	startedAt := time.Now().UTC()
	services := make([]Service, 0, len(b.cfg.Services))

	var closers []func()
	closeFn := func() {
		for _, fn := range closers {
			fn()
		}
	}

	// Build internal services and dependencies.
	statusRegistry := readyz.NewRegistry()
	reloadBroadcaster := b.buildReloadBroadcaster(ctx)

	resolver, err := b.buildResolver(ctx)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building resolver")
	}

	clientBuilder, err := b.buildClientBuilder(resolver)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building client builder")
	}

	identityService, closeIdentityService, err := b.buildIdentityService(
		ctx,
		reloadBroadcaster,
		clientBuilder,
		statusRegistry,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building identity service")
	}
	services = append(services, identityService)
	closers = append(closers, closeIdentityService)

	heartbeatService, err := b.buildHeartbeatService(
		identityService,
		startedAt,
		statusRegistry,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building heartbeat service")
	}
	services = append(services, heartbeatService)

	caRotationService, err := b.buildCARotationService(
		reloadBroadcaster,
		identityService,
		statusRegistry,
	)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building CA rotation service")
	}
	services = append(services, caRotationService)

	proxyPinger, err := b.buildProxyPinger(identityService)
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building proxy pinger")
	}

	identityGenerator, err := identityService.GetGenerator()
	if err != nil {
		return nil, closeFn, trace.Wrap(err, "building identity generator")
	}

	// Build user services.
	for idx, buildService := range b.cfg.Services {
		reloadCh, unsubscribe := reloadBroadcaster.Subscribe()
		closers = append(closers, unsubscribe)

		service, err := buildService(ServiceDependencies{
			Client:             identityService.GetClient(),
			Resolver:           resolver,
			Logger:             b.cfg.Logger,
			ClientBuilder:      clientBuilder,
			IdentityGenerator:  identityGenerator,
			ProxyPinger:        proxyPinger,
			BotIdentity:        identityService.GetIdentity,
			BotIdentityReadyCh: identityService.Ready(),
			ReloadCh:           reloadCh,
			StatusRegistry:     statusRegistry,
		})
		if err != nil {
			return nil, closeFn, trace.Wrap(err, "building service [%d]", idx)
		}
		services = append(services, service)
	}
	return services, closeFn, nil
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
	statusRegistry *readyz.Registry,
) (*identity.Service, func(), error) {
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
			teleport.Component(teleport.ComponentTBot, "identity"),
		),
		ClientBuilder:  clientBuilder,
		ReloadCh:       reloadCh,
		StatusReporter: statusRegistry.AddService("identity"),
	})
	if err != nil {
		unsubscribe()
		return nil, nil, trace.Wrap(err, "building identity service")
	}

	close := func() {
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
		close()
		return nil, nil, trace.Wrap(err, "initializing identity service")
	}

	return identityService, close, nil
}

func (b *Bot) buildHeartbeatService(
	identityService *identity.Service,
	startedAt time.Time,
	statusRegistry *readyz.Registry,
) (*heartbeat.Service, error) {
	return heartbeat.NewService(heartbeat.Config{
		BotKind:            machineidv1.BotKind(b.cfg.Kind),
		Interval:           30 * time.Minute,
		RetryLimit:         5,
		Client:             machineidv1.NewBotInstanceServiceClient(identityService.GetClient().GetConnection()),
		BotIdentityReadyCh: identityService.Ready(),
		StartedAt:          startedAt,
		JoinMethod:         b.cfg.Onboarding.JoinMethod,
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey, teleport.Component(teleport.ComponentTBot, "heartbeat"),
		),
		StatusReporter: statusRegistry.AddService("heartbeat"),
	})
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
	statusRegistry *readyz.Registry,
) (*carotation.Service, error) {
	return carotation.NewService(carotation.Config{
		BroadcastFn:        reloadBroadcaster.Broadcast,
		Client:             identityService.GetClient(),
		GetBotIdentityFn:   identityService.GetIdentity,
		BotIdentityReadyCh: identityService.Ready(),
		Logger: b.cfg.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, "ca-rotation"),
		),
		StatusReporter: statusRegistry.AddService("ca-rotation"),
	})
}
