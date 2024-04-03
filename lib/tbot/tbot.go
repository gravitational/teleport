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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot")

const componentTBot = "tbot"

// Service is a long-running sub-component of tbot.
type Service interface {
	// String returns a human-readable name for the service that can be used
	// in logging. It should identify the type of the service and any top
	// level configuration that could distinguish it from a same-type service.
	String() string
	// Run starts the service and blocks until the service exits. It should
	// return a nil error if the service exits successfully and an error
	// if it is unable to proceed. It should exit gracefully if the context
	// is canceled.
	Run(ctx context.Context) error
}

// OneShotService is a [Service] that offers a mode in which it runs a single
// time and then exits. This aligns with the `--oneshot` mode of tbot.
type OneShotService interface {
	Service
	// OneShot runs the service once and then exits. It should return a nil
	// error if the service exits successfully and an error if it is unable
	// to proceed. It should exit gracefully if the context is canceled.
	OneShot(ctx context.Context) error
}

type Bot struct {
	cfg     *config.BotConfig
	log     logrus.FieldLogger
	modules modules.Modules

	mu             sync.Mutex
	started        bool
	botIdentitySvc *identityService
}

func New(cfg *config.BotConfig, log logrus.FieldLogger) *Bot {
	if log == nil {
		log = utils.NewLogger()
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
	return b.botIdentitySvc.GetIdentity()
}

func (b *Bot) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "Bot/Run")
	defer span.End()

	if err := b.markStarted(); err != nil {
		return trace.Wrap(err)
	}
	unlock, err := b.preRunChecks(ctx)
	defer func() {
		b.log.Debug("Unlocking bot storage.")
		if unlock != nil {
			if err := unlock(); err != nil {
				b.log.WithError(err).Warn("Failed to release lock. Future starts of tbot may fail.")
			}
		}
	}()
	if err != nil {
		return trace.Wrap(err)
	}

	addr, _ := b.cfg.Address()
	resolver, err := reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: addr,
			Insecure:  b.cfg.Insecure,
		}),
		nil /* clock */)
	if err != nil {
		return trace.Wrap(err)
	}

	// Create an error group to manage all the services lifetimes.
	eg, egCtx := errgroup.WithContext(ctx)
	var services []Service

	// ReloadBroadcaster allows multiple entities to trigger a reload of
	// all services. This allows os signals and other events such as CA
	// rotations to trigger appropriate renewals.
	reloadBroadcaster := &channelBroadcaster{
		chanSet: map[chan struct{}]struct{}{},
	}
	// Trigger reloads from an configured reload channel.
	if b.cfg.ReloadCh != nil {
		// We specifically do not use the error group here as we do not want
		// this goroutine to block the bot from exiting.
		go func() {
			for {
				select {
				case <-egCtx.Done():
					return
				case <-b.cfg.ReloadCh:
					reloadBroadcaster.broadcast()
				}
			}
		}()
	}

	b.botIdentitySvc = &identityService{
		cfg:               b.cfg,
		reloadBroadcaster: reloadBroadcaster,
		resolver:          resolver,
		log: b.log.WithField(
			teleport.ComponentKey, teleport.Component(componentTBot, "identity"),
		),
	}
	// Initialize bot's own identity. This will load from disk, or fetch a new
	// identity, and perform an initial renewal if necessary.
	if err := b.botIdentitySvc.Initialize(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := b.botIdentitySvc.Close(); err != nil {
			b.log.WithError(err).Error("Failed to close bot identity service")
		}
	}()
	services = append(services, b.botIdentitySvc)

	authPingCache := &authPingCache{
		client: b.botIdentitySvc.GetClient(),
		log:    b.log,
	}
	proxyPingCache := &proxyPingCache{
		authPingCache: authPingCache,
		botCfg:        b.cfg,
		log:           b.log,
	}

	// Setup all other services
	if b.cfg.DiagAddr != "" {
		services = append(services, &diagnosticsService{
			diagAddr:     b.cfg.DiagAddr,
			pprofEnabled: b.cfg.Debug,
			log: b.log.WithField(
				teleport.ComponentKey, teleport.Component(componentTBot, "diagnostics"),
			),
		})
	}
	services = append(services, &outputsService{
		authPingCache:  authPingCache,
		proxyPingCache: proxyPingCache,
		getBotIdentity: b.botIdentitySvc.GetIdentity,
		botClient:      b.botIdentitySvc.GetClient(),
		cfg:            b.cfg,
		resolver:       resolver,
		log: b.log.WithField(
			teleport.ComponentKey, teleport.Component(componentTBot, "outputs"),
		),
		reloadBroadcaster: reloadBroadcaster,
	})
	services = append(services, &caRotationService{
		getBotIdentity: b.botIdentitySvc.GetIdentity,
		botClient:      b.botIdentitySvc.GetClient(),
		log: b.log.WithField(
			teleport.ComponentKey, teleport.Component(componentTBot, "ca-rotation"),
		),
		reloadBroadcaster: reloadBroadcaster,
	})

	// Append any services configured by the user
	for _, svcCfg := range b.cfg.Services {
		// Convert the service config into the actual service type.
		switch svcCfg := svcCfg.(type) {
		case *config.SPIFFEWorkloadAPIService:
			// Create a credential output for the SPIFFE Workload API service to
			// use as a source of an impersonated identity.
			svcIdentity := &config.UnstableClientCredentialOutput{}
			b.cfg.Outputs = append(b.cfg.Outputs, svcIdentity)

			svc := &SPIFFEWorkloadAPIService{
				botClient:             b.botIdentitySvc.GetClient(),
				svcIdentity:           svcIdentity,
				botCfg:                b.cfg,
				cfg:                   svcCfg,
				resolver:              resolver,
				rootReloadBroadcaster: reloadBroadcaster,
				trustBundleBroadcast: &channelBroadcaster{
					chanSet: map[chan struct{}]struct{}{},
				},
			}
			svc.log = b.log.WithField(
				teleport.ComponentKey, teleport.Component(componentTBot, "svc", svc.String()),
			)
			services = append(services, svc)
		case *config.DatabaseTunnelService:
			svc := &DatabaseTunnelService{
				getBotIdentity: b.botIdentitySvc.GetIdentity,
				proxyPingCache: proxyPingCache,
				botClient:      b.botIdentitySvc.GetClient(),
				resolver:       resolver,
				botCfg:         b.cfg,
				cfg:            svcCfg,
			}
			svc.log = b.log.WithField(
				teleport.ComponentKey, teleport.Component(componentTBot, "svc", svc.String()),
			)
			services = append(services, svc)
		case *config.ExampleService:
			services = append(services, &ExampleService{
				cfg: svcCfg,
			})
		default:
			return trace.BadParameter("unknown service type: %T", svcCfg)
		}
	}

	b.log.Info("Initialization complete. Starting services.")
	// Start services
	for _, svc := range services {
		svc := svc
		log := b.log.WithField("service", svc.String())

		if b.cfg.Oneshot {
			svc, ok := svc.(OneShotService)
			// We ignore services with no one-shot implementation
			if !ok {
				log.Debug("Service does not support oneshot mode, ignoring.")
				continue
			}
			eg.Go(func() error {
				log.Info("Running service in oneshot mode.")
				err := svc.OneShot(egCtx)
				if err != nil {
					log.WithError(err).Error("Service exited with error.")
					return trace.Wrap(err, "service(%s)", svc.String())
				}
				log.Info("Service finished.")
				return nil
			})
		} else {
			eg.Go(func() error {
				log.Info("Starting service.")
				err := svc.Run(egCtx)
				if err != nil {
					log.WithError(err).Error("Service exited with error.")
					return trace.Wrap(err, "service(%s)", svc.String())
				}
				log.Info("Service exited.")
				return nil
			})
		}
	}

	return eg.Wait()
}

// preRunChecks returns an unlock function which must be deferred.
// It performs any initial validation and locks the bot's storage before any
// more expensive initialization is performed.
func (b *Bot) preRunChecks(ctx context.Context) (func() error, error) {
	ctx, span := tracer.Start(ctx, "Bot/preRunChecks")
	defer span.End()

	switch _, addrKind := b.cfg.Address(); addrKind {
	case config.AddressKindUnspecified:
		return nil, trace.BadParameter(
			"either a proxy or auth address must be set using --proxy, --auth-server or configuration",
		)
	case config.AddressKindAuth:
		// TODO(noah): DELETE IN V17.0.0
		b.log.Warn("We recently introduced the ability to explicitly configure the address of the Teleport Proxy using --proxy-server. We recommend switching to this if you currently provide the address of the Proxy to --auth-server.")
	}

	// Ensure they have provided a join method.
	if b.cfg.Onboarding.JoinMethod == types.JoinMethodUnspecified {
		return nil, trace.BadParameter("join method must be provided")
	}

	if b.cfg.FIPS {
		if !b.modules.IsBoringBinary() {
			b.log.Error("FIPS mode enabled but FIPS compatible binary not in use. Ensure you are using the Enterprise FIPS binary to use this flag.")
			return nil, trace.BadParameter("fips mode enabled but binary was not compiled with boringcrypto")
		}
		b.log.Info("Bot is running in FIPS compliant mode.")
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
	for _, output := range cfg.Outputs {
		if err := output.Init(ctx); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// checkIdentity performs basic startup checks on an identity and loudly warns
// end users if it is unlikely to work.
func checkIdentity(log logrus.FieldLogger, ident *identity.Identity) error {
	var validAfter time.Time
	var validBefore time.Time

	if ident.X509Cert != nil {
		validAfter = ident.X509Cert.NotBefore
		validBefore = ident.X509Cert.NotAfter
	} else if ident.SSHCert != nil {
		validAfter = time.Unix(int64(ident.SSHCert.ValidAfter), 0)
		validBefore = time.Unix(int64(ident.SSHCert.ValidBefore), 0)
	} else {
		return trace.BadParameter("identity is invalid and contains no certificates")
	}

	now := time.Now().UTC()
	if now.After(validBefore) {
		log.Errorf(
			"Identity has expired. The renewal is likely to fail. (expires: %s, current time: %s)",
			validBefore.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
	} else if now.Before(validAfter) {
		log.Warnf(
			"Identity is not yet valid. Confirm that the system time is correct. (valid after: %s, current time: %s)",
			validAfter.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
	}

	return nil
}

// clientForFacade creates a new auth client from the given
// facade. Note that depending on the connection address given, this may
// attempt to connect via the proxy and therefore requires both SSH and TLS
// credentials.
func clientForFacade(
	ctx context.Context,
	log logrus.FieldLogger,
	cfg *config.BotConfig,
	facade *identity.Facade,
	resolver reversetunnelclient.Resolver) (*auth.Client, error) {
	ctx, span := tracer.Start(ctx, "clientForFacade")
	defer span.End()

	tlsConfig, err := facade.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshConfig, err := facade.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr, _ := cfg.Address()
	parsedAddr, err := utils.ParseAddr(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig := &authclient.Config{
		TLS: tlsConfig,
		SSH: sshConfig,
		// TODO(noah): It'd be ideal to distinguish the proxy addr and auth addr
		// here to avoid pointlessly hitting the address as an auth server.
		AuthServers: []utils.NetAddr{*parsedAddr},
		Log:         log,
		Insecure:    cfg.Insecure,
		Resolver:    resolver,
		DialOpts:    []grpc.DialOption{metadata.WithUserAgentFromTeleportComponent(teleport.ComponentTBot)},
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}

type authPingCache struct {
	client *auth.Client
	log    logrus.FieldLogger

	mu          sync.RWMutex
	cachedValue *proto.PingResponse
}

func (a *authPingCache) ping(ctx context.Context) (proto.PingResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cachedValue != nil {
		return *a.cachedValue, nil
	}

	a.log.Debug("Pinging auth server.")
	res, err := a.client.Ping(ctx)
	if err != nil {
		a.log.WithError(err).Error("Failed to ping auth server.")
		return proto.PingResponse{}, trace.Wrap(err)
	}
	a.cachedValue = &res
	a.log.WithField("pong", res).Debug("Successfully pinged auth server.")

	return *a.cachedValue, nil
}

type proxyPingCache struct {
	authPingCache *authPingCache
	botCfg        *config.BotConfig
	log           logrus.FieldLogger

	mu          sync.RWMutex
	cachedValue *webclient.PingResponse
}

func (p *proxyPingCache) ping(ctx context.Context) (*webclient.PingResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cachedValue != nil {
		return p.cachedValue, nil
	}

	// Determine the Proxy address to use.
	addr, addrKind := p.botCfg.Address()
	switch addrKind {
	case config.AddressKindAuth:
		// If the address is an auth address, ping auth to determine proxy addr.
		authPong, err := p.authPingCache.ping(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		addr = authPong.ProxyPublicAddr
	case config.AddressKindProxy:
		// If the address is a proxy address, use it directly.
	default:
		return nil, trace.BadParameter("unsupported address kind: %v", addrKind)
	}

	p.log.WithField("addr", addr).Debug("Pinging proxy.")
	res, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr,
		Insecure:  p.botCfg.Insecure,
	})
	if err != nil {
		p.log.WithError(err).Error("Failed to ping proxy.")
		return nil, trace.Wrap(err)
	}
	p.log.WithField("pong", res).Debug("Successfully pinged proxy.")
	p.cachedValue = res

	return p.cachedValue, nil
}
