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
	"runtime"
	"sync"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport"
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
	"github.com/gravitational/teleport/lib/tbot/services/application"
	"github.com/gravitational/teleport/lib/tbot/services/awsra"
	"github.com/gravitational/teleport/lib/tbot/services/clientcredentials"
	"github.com/gravitational/teleport/lib/tbot/services/database"
	"github.com/gravitational/teleport/lib/tbot/services/example"
	identitysvc "github.com/gravitational/teleport/lib/tbot/services/identity"
	"github.com/gravitational/teleport/lib/tbot/services/k8s"
	"github.com/gravitational/teleport/lib/tbot/services/ssh"
	workloadidentitysvc "github.com/gravitational/teleport/lib/tbot/services/workloadidentity"
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
	client   *apiclient.Client
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

// getBotIdentity returns the bot's own identity. This will return nil if the
// bot has not been started.
func (b *Bot) getBotIdentity() *identity.Identity {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.identity()
}

// getClient returns the bot's API client. This will return nil if the bot has
// not been started.
func (b *Bot) getClient() *apiclient.Client {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.client
}

func (b *Bot) Run(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "Bot/Run")
	defer func() { apitracing.EndSpan(span, err) }()
	b.log.InfoContext(
		ctx, "Initializing tbot",
		"version", versionLogValue(),
	)

	if err := metrics.RegisterPrometheusCollectors(
		metrics.BuildCollector(),
		clientMetrics,
		internal.LoopIterationsCounter,
		internal.LoopIterationsSuccessCounter,
		internal.LoopIterationsFailureCounter,
		internal.LoopIterationTime,
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

	var services []bot.ServiceBuilder

	if b.cfg.DiagAddr != "" {
		services = append(services,
			diagnostics.ServiceBuilder(diagnostics.Config{
				Address: b.cfg.DiagAddr,
				Logger: b.log.With(
					teleport.ComponentKey,
					teleport.Component(teleport.ComponentTBot, "diagnostics"),
				),
				PProfEnabled: b.cfg.Debug,
			}),
		)
	}

	// This faux service allows us to get the bot's internal identity and client
	// for tests, without exposing them on the core bot.Bot struct.
	if b.cfg.Testing {
		services = append(services,
			bot.NewServiceBuilder("internal/client-fetcher", "client-fetcher",
				func(deps bot.ServiceDependencies) (bot.Service, error) {
					b.mu.Lock()
					defer b.mu.Unlock()

					b.identity = deps.BotIdentity
					b.client = deps.Client

					return bot.NewNopService("client-fetcher"), nil
				}),
		)
	}

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
		services = append(services, trustBundleCache.Builder())
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
		services = append(services, crlCache.Builder())
		return crlCache
	}

	// Append any services configured by the user
	for _, svcCfg := range b.cfg.Services {
		// Convert the service config into the actual service type.
		switch svcCfg := svcCfg.(type) {
		case *database.TunnelConfig:
			services = append(services, database.TunnelServiceBuilder(svcCfg, b.cfg.ConnectionConfig(), b.cfg.CredentialLifetime))
		case *example.Config:
			services = append(services, example.ServiceBuilder(svcCfg))
		case *ssh.MultiplexerConfig:
			services = append(services, ssh.MultiplexerServiceBuilder(svcCfg, alpnUpgradeCache, b.cfg.ConnectionConfig(), b.cfg.CredentialLifetime, clientMetrics))
		case *k8s.OutputV1Config:
			services = append(services, k8s.OutputV1ServiceBuilder(svcCfg, k8s.WithDefaultCredentialLifetime(b.cfg.CredentialLifetime)))
		case *k8s.OutputV2Config:
			services = append(services, k8s.OutputV2ServiceBuilder(svcCfg, k8s.WithDefaultCredentialLifetime(b.cfg.CredentialLifetime)))
		case *k8s.ArgoCDOutputConfig:
			services = append(services, k8s.ArgoCDServiceBuilder(
				svcCfg,
				k8s.WithDefaultCredentialLifetime(b.cfg.CredentialLifetime),
				k8s.WithInsecure(b.cfg.ConnectionConfig().Insecure),
				k8s.WithALPNUpgradeCache(alpnUpgradeCache),
			))
		case *ssh.HostOutputConfig:
			services = append(services, ssh.HostOutputServiceBuilder(svcCfg, b.cfg.CredentialLifetime))
		case *application.OutputConfig:
			services = append(services, application.OutputServiceBuilder(svcCfg, b.cfg.CredentialLifetime))
		case *database.OutputConfig:
			services = append(services, database.OutputServiceBuilder(svcCfg, b.cfg.CredentialLifetime))
		case *identitysvc.OutputConfig:
			services = append(services, identitysvc.OutputServiceBuilder(svcCfg, alpnUpgradeCache, b.cfg.CredentialLifetime, b.cfg.Insecure, b.cfg.FIPS))
		case *clientcredentials.UnstableConfig:
			services = append(services, clientcredentials.ServiceBuilder(svcCfg, b.cfg.CredentialLifetime))
		case *application.TunnelConfig:
			services = append(services, application.TunnelServiceBuilder(svcCfg, b.cfg.ConnectionConfig(), b.cfg.CredentialLifetime))
		case *application.ProxyServiceConfig:
			services = append(services, application.ProxyServiceBuilder(svcCfg, b.cfg.ConnectionConfig(), b.cfg.CredentialLifetime, alpnUpgradeCache))
		case *workloadidentitysvc.X509OutputConfig:
			services = append(services, workloadidentitysvc.X509OutputServiceBuilder(svcCfg, setupTrustBundleCache(), setupCRLCache(), b.cfg.CredentialLifetime))
		case *workloadidentitysvc.JWTOutputConfig:
			services = append(services, workloadidentitysvc.JWTOutputServiceBuilder(svcCfg, setupTrustBundleCache(), b.cfg.CredentialLifetime))
		case *workloadidentitysvc.WorkloadAPIConfig:
			services = append(services, workloadidentitysvc.WorkloadAPIServiceBuilder(svcCfg, setupTrustBundleCache(), setupCRLCache(), b.cfg.CredentialLifetime))
		case *awsra.Config:
			services = append(services, awsra.ServiceBuilder(svcCfg))
		default:
			return trace.BadParameter("unknown service type: %T", svcCfg)
		}
	}

	bt, err := bot.New(bot.Config{
		Kind:               bot.KindTbot,
		Connection:         b.cfg.ConnectionConfig(),
		Onboarding:         b.cfg.Onboarding,
		InternalStorage:    b.cfg.Storage.Destination,
		CredentialLifetime: b.cfg.CredentialLifetime,
		FIPS:               b.cfg.FIPS,
		Logger:             b.log,
		ReloadCh:           b.cfg.ReloadCh,
		Services:           services,
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

	if b.cfg.JoinURI != "" {
		parsed, err := config.ParseJoinURI(b.cfg.JoinURI)
		if err != nil {
			return nil, trace.Wrap(err, "parsing joining URI")
		}

		if err := parsed.ApplyToConfig(b.cfg); err != nil {
			return nil, trace.Wrap(err, "applying joining URI to bot config")
		}
	}

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

func versionLogValue() slog.Value {
	return slog.GroupValue(
		slog.String("teleport", teleport.Version),
		slog.String("teleport_git", teleport.Gitref),
		slog.String("go", runtime.Version()),
	)
}
