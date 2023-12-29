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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot")

const componentTBot = "tbot"

type Bot struct {
	cfg     *config.BotConfig
	log     logrus.FieldLogger
	modules modules.Modules

	mu      sync.Mutex
	started bool
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

func (b *Bot) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "Bot/Run")
	defer span.End()

	if err := b.markStarted(); err != nil {
		return trace.Wrap(err)
	}

	// Create an error group to manage all the services lifetimes.
	eg, egCtx := errgroup.WithContext(ctx)
	services := []bot.Service{}

	// ReloadBroadcaster allows multiple entities to trigger a reload of
	// all services. This allows os signals and other events such as CA
	// rotations to trigger appropriate renewals.
	reloadBroadcaster := &channelBroadcaster{
		chanSet: map[chan struct{}]struct{}{},
	}
	// Trigger reloads from an configured reload channel.
	if b.cfg.ReloadCh != nil {
		eg.Go(func() error {
			for {
				select {
				case <-egCtx.Done():
					return nil
				case <-b.cfg.ReloadCh:
					reloadBroadcaster.broadcast()
				}
			}
		})
	}

	// Perform any pre-start initialization
	unlock, err := b.initialize(ctx)
	defer func() {
		if unlock != nil {
			if err := unlock(); err != nil {
				b.log.WithError(err).Warn("Failed to release lock. Future starts of tbot may fail.")
			}
		}
	}()
	if err != nil {
		return trace.Wrap(err)
	}
	identitySvc := &identityService{
		cfg:               b.cfg,
		reloadBroadcaster: reloadBroadcaster,
		log: b.log.WithField(
			trace.Component, teleport.Component(componentTBot, "identity"),
		),
	}
	services = append(services, identitySvc)
	// Initialize bot's own identity. This will load from disk, or fetch a new
	// identity, and perform an initial renewal if necessary.
	if err := identitySvc.Init(ctx); err != nil {
		return trace.Wrap(err)
	}

	// Setup all other services
	if b.cfg.DiagAddr != "" {
		services = append(services, &diagnosticsService{
			diagAddr:     b.cfg.DiagAddr,
			pprofEnabled: b.cfg.Debug,
			log: b.log.WithField(
				trace.Component, teleport.Component(componentTBot, "diagnostics"),
			),
		})
	}
	services = append(services, &outputsService{
		identitySrc:       identitySvc,
		reloadBroadcaster: reloadBroadcaster,
		log: b.log.WithField(
			trace.Component, teleport.Component(componentTBot, "outputs"),
		),
	})
	services = append(services, &caRotationService{
		log: b.log.WithField(
			trace.Component, teleport.Component(componentTBot, "ca-rotation-watcher"),
		),
		reloadBroadcaster: reloadBroadcaster,
		identitySrc:       identitySvc,
	})
	// Append any services configured by the user
	services = append(services, b.cfg.Services...)

	// Start services
	for _, svc := range services {
		svc := svc
		if b.cfg.Oneshot {
			svc, ok := svc.(bot.OneShotService)
			// We ignore services with no one-shot implementation
			if ok {
				eg.Go(func() error {
					b.log.WithField("service", svc.String()).Info("Running service as oneshot")
					err := svc.OneShot(egCtx)
					if err != nil {
						b.log.WithError(err).WithField("service", svc.String()).Error("Service exited with error")
						return trace.Wrap(err, "running service %q", svc.String())
					}
					b.log.WithField("service", svc.String()).Info("Service exited")
					return nil
				})
			}

		} else {
			eg.Go(func() error {
				b.log.WithField("service", svc.String()).Info("Starting service")
				err := svc.Run(egCtx)
				if err != nil {
					b.log.WithError(err).WithField("service", svc.String()).Error("Service exited with error")
					return trace.Wrap(err, "running service %q", svc.String())
				}
				b.log.WithField("service", svc.String()).Info("Service exited")
				return nil
			})
		}
	}

	return eg.Wait()
}

// initialize returns an unlock function which must be deferred.
func (b *Bot) initialize(ctx context.Context) (func() error, error) {
	ctx, span := tracer.Start(ctx, "Bot/initialize")
	defer span.End()

	if b.cfg.AuthServer == "" {
		return nil, trace.BadParameter(
			"an auth or proxy server must be set via --auth-server or configuration",
		)
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
			err, "Could not write to destination %s, aborting.", store,
		)
	}

	// Now attempt to lock the destination so we have sole use of it
	unlock, err := store.TryLock()
	if err != nil {
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return unlock, trace.WrapWithMessage(
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

	// TODO: consider warning if ownership of all destintions is not expected.
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

// clientForIdentity creates a new auth client from the given
// identity. Note that depending on the connection address given, this may
// attempt to connect via the proxy and therefore requires both SSH and TLS
// credentials.
func clientForIdentity(
	ctx context.Context,
	log logrus.FieldLogger,
	cfg *config.BotConfig,
	id *identity.Identity,
) (auth.ClientI, error) {
	ctx, span := tracer.Start(ctx, "Bot/clientForIdentity")
	defer span.End()

	if id.SSHCert == nil || id.X509Cert == nil {
		return nil, trace.BadParameter("auth client requires a fully formed identity")
	}

	// TODO(noah): Eventually we'll want to reuse this facade across the bot
	// rather than recreating it. Right now the blocker to that is handling the
	// generation field on the certificate.
	facade := identity.NewFacade(cfg.FIPS, cfg.Insecure, id)
	tlsConfig, err := facade.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshConfig, err := facade.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authAddr, err := utils.ParseAddr(cfg.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig := &authclient.Config{
		TLS:         tlsConfig,
		SSH:         sshConfig,
		AuthServers: []utils.NetAddr{*authAddr},
		Log:         log,
		Insecure:    cfg.Insecure,
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}
