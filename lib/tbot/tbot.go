/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tbot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

type Bot struct {
	cfg     *config.BotConfig
	log     logrus.FieldLogger
	modules modules.Modules

	// These are protected by getter/setters with mutex locks
	mu      sync.Mutex
	_ident  *identity.Identity
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

func (b *Bot) ident() *identity.Identity {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b._ident
}

func (b *Bot) setIdent(ident *identity.Identity) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b._ident = ident
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
	if err := b.markStarted(); err != nil {
		return trace.Wrap(err)
	}

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

	// One-shot mode just invokes the output of credentials to the destinations.
	// There's no retry logic here - this means we fail fast in the most common
	// oneshot use-cases like CI-CD where backing off over several minutes on
	// failure will just cost the customer money.
	if b.cfg.Oneshot {
		b.log.Info("One-shot mode enabled. Renewing destinations.")
		if err := b.renewDestinations(ctx); err != nil {
			return trace.Wrap(err)
		}

		b.log.Info("Renewed destinations. One-shot mode enabled so exiting.")
		return nil
	}

	// Warn about weird non-oneshot configuration.
	if b.cfg.RenewalInterval > b.cfg.CertificateTTL {
		b.log.Errorf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). This is likely an invalid configuration. Increase the certificate TTL or decrease the renewal interval.",
			b.cfg.CertificateTTL,
			b.cfg.RenewalInterval,
		)
	}

	reloadBroadcast := channelBroadcaster{
		chanSet: map[chan struct{}]struct{}{},
	}

	// If in daemon mode, we spin up all of our separate concurrent components.
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return trace.Wrap(b.caRotationLoop(egCtx, reloadBroadcast.broadcast))
	})
	eg.Go(func() error {
		reloadCh, unsubscribe := reloadBroadcast.subscribe()
		defer unsubscribe()
		return trace.Wrap(b.renewBotIdentityLoop(egCtx, reloadCh))
	})
	eg.Go(func() error {
		reloadCh, unsubscribe := reloadBroadcast.subscribe()
		defer unsubscribe()
		return trace.Wrap(b.renewDestinationsLoop(egCtx, reloadCh))
	})
	if b.cfg.ReloadCh != nil {
		eg.Go(func() error {
			for {
				select {
				case <-egCtx.Done():
					return nil
				case <-b.cfg.ReloadCh:
					reloadBroadcast.broadcast()
				}
			}
		})
	}

	if b.cfg.DiagAddr != "" {
		eg.Go(func() error {
			b.log.WithField("addr", b.cfg.DiagAddr).Info(
				"diag_addr configured, diagnostics service will be started.",
			)
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			// Only expose pprof when `-d` is provided.
			if b.cfg.Debug {
				b.log.Info("debug mode enabled, profiling endpoints will be served on the diagnostics service.")
				mux.HandleFunc("/debug/pprof/", pprof.Index)
				mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
				mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
				mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
				mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			}
			mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				msg := "404 - Not Found\n\nI'm a little tbot,\nshort and stout,\nthe page you seek,\nis not about."
				_, _ = w.Write([]byte(msg))
			}))
			srv := http.Server{
				Addr:    b.cfg.DiagAddr,
				Handler: mux,
			}
			go func() {
				<-egCtx.Done()
				if err := srv.Close(); err != nil {
					b.log.WithError(err).Warn("Failed to close HTTP server.")
				}
			}()
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				return err
			}
			return nil
		})
	}

	return eg.Wait()
}

// initialize returns an unlock function which must be deferred.
func (b *Bot) initialize(ctx context.Context) (func() error, error) {
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
	if err := checkDestinations(b.cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	// Start by loading the bot's primary storage.
	store, err := b.cfg.Storage.GetDestination()
	if err != nil {
		return nil, trace.Wrap(
			err, "could not read bot storage destination from config",
		)
	}

	if err := identity.VerifyWrite(store); err != nil {
		return nil, trace.Wrap(
			err, "Could not write to destination %s, aborting.", store,
		)
	}

	// Now attempt to lock the destination so we have sole use of it
	unlock, err := store.TryLock()
	if err != nil {
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return unlock, trace.WrapWithMessage(err, "Failed to acquire exclusive lock for tbot destination directory - is tbot already running?")
		}
		return unlock, trace.Wrap(err)
	}

	b.log.Info("Initializing bot identity.")
	var loadedIdent *identity.Identity
	if b.cfg.Onboarding.RenewableJoinMethod() {
		// Nil, nil will be returned if no identity can be found in store or
		// the identity in the store is no longer relevant.
		loadedIdent, err = b.loadIdentityFromStore(store)
		if err != nil {
			return unlock, trace.Wrap(err)
		}
	}

	var newIdentity *identity.Identity
	if b.cfg.Onboarding.RenewableJoinMethod() && loadedIdent != nil {
		// If using a renewable join method and we loaded an identity, let's
		// immediately renew it so we know that after initialisation we have the
		// full certificate TTL.
		if err := b.checkIdentity(loadedIdent); err != nil {
			return unlock, trace.Wrap(err)
		}
		authClient, err := b.AuthenticatedUserClientFromIdentity(ctx, loadedIdent)
		if err != nil {
			return unlock, trace.Wrap(err)
		}
		defer authClient.Close()
		newIdentity, err = botIdentityFromAuth(
			ctx, b.log, loadedIdent, authClient, b.cfg.CertificateTTL,
		)
		if err != nil {
			return unlock, trace.Wrap(err)
		}
	} else if b.cfg.Onboarding.HasToken() {
		// If using a non-renewable join method, or we weren't able to load an
		// identity from the store, let's get a new identity using the
		// configured token.
		newIdentity, err = botIdentityFromToken(b.log, b.cfg)
		if err != nil {
			return unlock, trace.Wrap(err)
		}
	} else {
		// There's no loaded identity to work with, and they've not configured
		// a token to use to request an identity :(
		return unlock, trace.BadParameter("no existing identity found on disk or join token configured")
	}

	b.log.WithField("identity", describeTLSIdentity(b.log, newIdentity)).Info("Fetched new bot identity.")
	if err := identity.SaveIdentity(newIdentity, store, identity.BotKinds()...); err != nil {
		return unlock, trace.Wrap(err)
	}

	testClient, err := b.AuthenticatedUserClientFromIdentity(ctx, newIdentity)
	if err != nil {
		return unlock, trace.Wrap(err)
	}
	defer testClient.Close()

	b.setIdent(newIdentity)

	// Attempt a request to make sure our client works so we can exit early if
	// we are in a bad state.
	if _, err := testClient.Ping(ctx); err != nil {
		return unlock, trace.Wrap(err, "unable to communicate with auth server")
	}
	b.log.Info("Bot initialization complete.")

	return unlock, nil
}

// loadIdentityFromStore attempts to load a persisted identity from a store.
// It checks this loaded identity against the configured onboarding profile
// and ignores the loaded identity if there has been a configuration change.
func (b *Bot) loadIdentityFromStore(store bot.Destination) (*identity.Identity, error) {
	b.log.WithField("store", store).Info("Loading existing bot identity from store.")
	loadedIdent, err := identity.LoadIdentity(store, identity.BotKinds()...)
	if err != nil {
		if trace.IsNotFound(err) {
			b.log.Info("No existing bot identity found in store. Bot will join using configured token.")
			return nil, nil
		} else {
			return nil, trace.Wrap(err)
		}
	}

	// Determine if the token configured in the onboarding matches the
	// one used to produce the credentials loaded from disk.
	if b.cfg.Onboarding.HasToken() {
		if token, err := b.cfg.Onboarding.Token(); err == nil {
			sha := sha256.Sum256([]byte(token))
			configTokenHashBytes := []byte(hex.EncodeToString(sha[:]))
			if hasTokenChanged(loadedIdent.TokenHashBytes, configTokenHashBytes) {
				b.log.Info("Bot identity loaded from store does not match configured token. Bot will fetch identity using configured token.")
				// If the token has changed, do not return the loaded
				// identity.
				return nil, nil
			}
		} else {
			// we failed to get the newly configured token to compare to,
			// we'll assume the last good credentials written to disk should
			// still be used.
			b.log.
				WithError(err).
				Error("There was an error loading the configured token. Bot identity loaded from store will be tried.")
		}
	}
	b.log.WithField("identity", describeTLSIdentity(b.log, loadedIdent)).Info("Loaded existing bot identity from store.")

	return loadedIdent, nil
}

func hasTokenChanged(configTokenBytes, identityBytes []byte) bool {
	if len(configTokenBytes) == 0 || len(identityBytes) == 0 {
		return false
	}

	return !bytes.Equal(identityBytes, configTokenBytes)
}

// checkDestinations checks all destinations and tries to create any that
// don't already exist.
func checkDestinations(cfg *config.BotConfig) error {
	// Note: This is vaguely problematic as we don't recommend that users
	// store renewable certs under the same user as end-user certs. That said,
	//  - if the destination was properly created via tbot init this is a no-op
	//  - if users intend to follow that advice but miss a step, it should fail
	//    due to lack of permissions
	storage, err := cfg.Storage.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: consider warning if ownership of all destintions is not expected.

	// Note: no subdirs to init for bot's internal storage.
	if err := storage.Init([]string{}); err != nil {
		return trace.Wrap(err)
	}

	for _, dest := range cfg.Destinations {
		destImpl, err := dest.GetDestination()
		if err != nil {
			return trace.Wrap(err)
		}

		subdirs, err := dest.ListSubdirectories()
		if err != nil {
			return trace.Wrap(err)
		}

		if err := destImpl.Init(subdirs); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// checkIdentity performs basic startup checks on an identity and loudly warns
// end users if it is unlikely to work.
func (b *Bot) checkIdentity(ident *identity.Identity) error {
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
		b.log.Errorf(
			"Identity has expired. The renewal is likely to fail. (expires: %s, current time: %s)",
			validBefore.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
	} else if now.Before(validAfter) {
		b.log.Warnf(
			"Identity is not yet valid. Confirm that the system time is correct. (valid after: %s, current time: %s)",
			validAfter.Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
	}

	return nil
}

// AuthenticatedUserClientFromIdentity creates a new auth client from the given
// identity. Note that depending on the connection address given, this may
// attempt to connect via the proxy and therefore requires both SSH and TLS
// credentials.
func (b *Bot) AuthenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error) {
	if id.SSHCert == nil || id.X509Cert == nil {
		return nil, trace.BadParameter("auth client requires a fully formed identity")
	}

	tlsConfig, err := id.TLSConfig(b.cfg.CipherSuites())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := id.SSHClientConfig(b.cfg.FIPS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authAddr, err := utils.ParseAddr(b.cfg.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig := &authclient.Config{
		TLS:         tlsConfig,
		SSH:         sshConfig,
		AuthServers: []utils.NetAddr{*authAddr},
		Log:         b.log,
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}
