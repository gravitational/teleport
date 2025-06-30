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
	"errors"
	"log/slog"
	"sync"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport/api/client"
	apiclient "github.com/gravitational/teleport/api/client"
	apitracing "github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/internal/diagnostics"
	"github.com/gravitational/teleport/lib/tbot/loop"
	"github.com/gravitational/teleport/lib/tbot/services/application"
	"github.com/gravitational/teleport/lib/tbot/services/example"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/utils"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot")

var clientMetrics = metrics.CreateGRPCClientMetrics(
	false,
	prometheus.Labels{},
)

type Bot struct {
	cfg     *config.BotConfig
	log     *slog.Logger
	modules modules.Modules

	mu       sync.Mutex
	started  bool
	identity getBotIdentityFn
	client   *client.Client
}

func New(cfg *config.BotConfig, log *slog.Logger) *Bot {
	if log == nil {
		log = slog.Default()
	}

	return &Bot{
		cfg:     cfg,
		log:     log,
		modules: modules.GetModules(),
	}
}

func (b *Bot) markStarted() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return trace.BadParameter("bot has already been started")
	}
	b.started = true

	return nil
}

type getBotIdentityFn func() *identity.Identity

// BotIdentity returns the bot's own identity. This will return nil if the bot
// has not been started.
func (b *Bot) BotIdentity() *identity.Identity {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.identity()
}

// Client returns the bot's API client. This will return nil if the bot has not
// been started.
func (b *Bot) Client() *apiclient.Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.client
}

func (b *Bot) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "Bot/Run")
	defer func() { apitracing.EndSpan(span, err) }()

	if err := metrics.RegisterPrometheusCollectors(
		metrics.BuildCollector(),
		clientMetrics,
		loop.IterationsCounter,
		loop.IterationsSuccessCounter,
		loop.IterationsFailureCounter,
		loop.IterationTime,
	); err != nil {
		return trace.Wrap(err)
	}

	if err := b.markStarted(); err != nil {
		return trace.Wrap(err)
	}
	unlock, err := b.preRunChecks(ctx)
	defer func() {
		b.log.DebugContext(ctx, "Unlocking bot storage.")
		if unlock != nil {
			if err := unlock(); err != nil {
				b.log.WarnContext(
					ctx, "Failed to release lock. Future starts of tbot may fail.", "error", err,
				)
			}
		}
	}()
	if err != nil {
		return trace.Wrap(err)
	}

	alpnUpgradeCache := internal.NewALPNUpgradeCache(b.log)

	var botServices []bot.ServiceBuilder

	if b.cfg.DiagAddr != "" {
		diagService, err := diagnostics.NewService(diagnostics.Config{
			Address: b.cfg.DiagAddr,
			Logger: b.log.With(
				teleport.ComponentKey,
				teleport.Component(teleport.ComponentTBot, "diagnostics"),
			),
			PProfEnabled: b.cfg.Debug,
		})
		if err != nil {
			return trace.Wrap(err, "building diagnostics service")
		}
		botServices = append(botServices, bot.LiteralService(diagService))
	}

	// TODO: this is a bit hacky. Is it really the best way?
	botServices = append(botServices, func(deps bot.ServiceDependencies) (bot.Service, error) {
		b.mu.Lock()
		defer b.mu.Unlock()

		b.identity = deps.BotIdentity
		b.client = deps.Client

		return bot.NewNopService("client-fetcher"), nil
	})

	// We only want to create this service if it's needed by a dependent
	// service.
	var trustBundleCache *workloadidentity.TrustBundleCacheFacade
	setupTrustBundleCache := func() *workloadidentity.TrustBundleCacheFacade {
		if b.cfg.Oneshot {
			return nil
		}
		if trustBundleCache != nil {
			return trustBundleCache
		}
		trustBundleCache = workloadidentity.NewTrustBundleCacheFacade()
		botServices = append(botServices, trustBundleCache.BuildService)
		return trustBundleCache
	}

	var crlCache *workloadidentity.CRLCacheFacade
	setupCRLCache := func() *workloadidentity.CRLCacheFacade {
		if b.cfg.Oneshot {
			return nil
		}
		if crlCache != nil {
			return crlCache
		}
		crlCache = workloadidentity.NewCRLCacheFacade()
		botServices = append(botServices, crlCache.BuildService)
		return crlCache
	}

	// Append any services configured by the user
	for _, svcCfg := range b.cfg.Services {
		// Convert the service config into the actual service type.
		switch svcCfg := svcCfg.(type) {
		case *config.SPIFFEWorkloadAPIService:
			b.log.WarnContext(
				ctx,
				"The 'spiffe-workload-api' service is deprecated and will be removed in Teleport V19.0.0. See https://goteleport.com/docs/reference/workload-identity/configuration-resource-migration/ for further information.",
			)
			botServices = append(botServices, SPIFFEWorkloadAPIServiceBuilder(b.cfg, svcCfg, setupTrustBundleCache()))
		case *config.DatabaseTunnelService:
			botServices = append(botServices, DatabaseTunnelServiceBuilder(b.cfg, svcCfg))
		case *example.Config:
			botServices = append(botServices, example.ServiceBuilder(svcCfg))
		case *config.SSHMultiplexerService:
			botServices = append(botServices, SSHMultiplexerServiceBuilder(b.cfg, svcCfg, alpnUpgradeCache))
		case *config.KubernetesOutput:
			botServices = append(botServices, KubernetesOutputServiceBuilder(b.cfg, svcCfg))
		case *config.KubernetesV2Output:
			botServices = append(botServices, KubernetesV2OutputServiceBuilder(b.cfg, svcCfg))
		case *config.SPIFFESVIDOutput:
			botServices = append(botServices, SPIFFESVIDOutputServiceBuilder(b.cfg, svcCfg, setupTrustBundleCache()))
		case *config.SSHHostOutput:
			botServices = append(botServices, SSHHostOutputServiceBuilder(b.cfg, svcCfg))
		case *application.OutputConfig:
			botServices = append(botServices, application.OutputServiceBuilder(svcCfg, b.cfg.CredentialLifetime))
		case *config.DatabaseOutput:
			botServices = append(botServices, DatabaseOutputServiceBuider(b.cfg, svcCfg))
		case *config.IdentityOutput:
			botServices = append(botServices, IdentityOutputServiceBuilder(b.cfg, svcCfg, alpnUpgradeCache))
		case *config.UnstableClientCredentialOutput:
			botServices = append(botServices, ClientCredentialOutputServiceBuilder(b.cfg, svcCfg))
		case *application.TunnelConfig:
			botServices = append(botServices, application.TunnelServiceBuilder(svcCfg, b.cfg.ConnectionConfig(), b.cfg.CredentialLifetime))
		case *config.WorkloadIdentityX509Service:
			botServices = append(botServices, WorkloadIdentityX509ServiceBuilder(b.cfg, svcCfg, setupTrustBundleCache(), setupCRLCache()))
		case *config.WorkloadIdentityJWTService:
			botServices = append(botServices, WorkloadIdentityJWTServiceBuilder(b.cfg, svcCfg, setupTrustBundleCache()))
		case *config.WorkloadIdentityAPIService:
			botServices = append(botServices, WorkloadIdentityAPIServiceBuilder(b.cfg, svcCfg, setupTrustBundleCache(), setupCRLCache()))
		case *config.WorkloadIdentityAWSRAService:
			botServices = append(botServices, WorkloadIdentityAWSRAServiceBuilder(b.cfg, svcCfg))
		default:
			return trace.BadParameter("unknown service type: %T", svcCfg)
		}
	}

	bt, err := bot.New(bot.Config{
		Connection:         b.cfg.ConnectionConfig(),
		Onboarding:         b.cfg.Onboarding,
		InternalStorage:    b.cfg.Storage.Destination,
		CredentialLifetime: b.cfg.CredentialLifetime,
		FIPS:               b.cfg.FIPS,
		Logger:             b.log,
		ReloadCh:           b.cfg.ReloadCh,
		Services:           botServices,
		ClientMetrics:      clientMetrics,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if b.cfg.Oneshot {
		return bt.OneShot(ctx)
	} else {
		return bt.Run(ctx)
	}
}

// preRunChecks returns an unlock function which must be deferred.
// It performs any initial validation and locks the bot's storage before any
// more expensive initialization is performed.
func (b *Bot) preRunChecks(ctx context.Context) (_ func() error, err error) {
	ctx, span := tracer.Start(ctx, "Bot/preRunChecks")
	defer func() { apitracing.EndSpan(span, err) }()

	connCfg := b.cfg.ConnectionConfig()
	switch connCfg.AddressKind {
	case connection.AddressKindUnspecified:
		return nil, trace.BadParameter(
			"either a proxy or auth address must be set using --proxy-server, --auth-server or configuration",
		)
	}

	// Ensure they have provided a join method.
	if b.cfg.Onboarding.JoinMethod == types.JoinMethodUnspecified {
		return nil, trace.BadParameter("join method must be provided")
	}

	if b.cfg.FIPS {
		if !b.modules.IsBoringBinary() {
			b.log.ErrorContext(ctx, "FIPS mode enabled but FIPS compatible binary not in use. Ensure you are using the Enterprise FIPS binary to use this flag.")
			return nil, trace.BadParameter("fips mode enabled but binary was not compiled with boringcrypto")
		}
		b.log.InfoContext(ctx, "Bot is running in FIPS compliant mode.")
	}

	// First, try to make sure all destinations are usable.
	if err := checkDestinations(ctx, b.cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	// Start by loading the bot's primary storage.
	store := b.cfg.Storage.Destination
	if err := identity.VerifyWrite(ctx, store); err != nil {
		return nil, trace.Wrap(
			err, "Could not write to destination %s, aborting", store,
		)
	}

	// Now attempt to lock the destination so we have sole use of it
	unlock, err := store.TryLock()
	if err != nil {
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return unlock, trace.Wrap(
				err,
				"Failed to acquire exclusive lock for tbot destination directory - is tbot already running?",
			)
		}
		return unlock, trace.Wrap(err)
	}

	if !store.IsPersistent() {
		b.log.WarnContext(
			ctx,
			"Bot is configured with a non-persistent storage destination. If the bot is running in a non-ephemeral environment, this will impact the ability to provide a long-lived bot instance identity",
		)
	}

	return unlock, nil
}

// checkDestinations checks all destinations and tries to create any that
// don't already exist.
func checkDestinations(ctx context.Context, cfg *config.BotConfig) error {
	// Note: This is vaguely problematic as we don't recommend that users
	// store renewable certs under the same user as end-user certs. That said,
	//  - if the destination was properly created via tbot init this is a no-op
	//  - if users intend to follow that advice but miss a step, it should fail
	//    due to lack of permissions
	storageDest := cfg.Storage.Destination

	// Note: no subdirs to init for bot's internal storage.
	if err := storageDest.Init(ctx, []string{}); err != nil {
		return trace.Wrap(err)
	}

	// TODO: consider warning if ownership of all destinations is not expected.
	for _, initable := range cfg.GetInitables() {
		if err := initable.Init(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
