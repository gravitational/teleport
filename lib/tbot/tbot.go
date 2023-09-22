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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

type Bot struct {
	cfg        *config.BotConfig
	log        logrus.FieldLogger
	reloadChan chan struct{}
	modules    modules.Modules

	// These are protected by getter/setters with mutex locks
	mu         sync.Mutex
	_client    auth.ClientI
	_ident     *identity.Identity
	_authPong  *proto.PingResponse
	_proxyPong *webclient.PingResponse
	_cas       map[types.CertAuthType][]types.CertAuthority
	started    bool
}

func New(cfg *config.BotConfig, log logrus.FieldLogger, reloadChan chan struct{}) *Bot {
	if log == nil {
		log = utils.NewLogger()
	}

	return &Bot{
		cfg:        cfg,
		log:        log,
		reloadChan: reloadChan,
		modules:    modules.GetModules(),

		_cas: map[types.CertAuthType][]types.CertAuthority{},
	}
}

// Config returns the current bot config
func (b *Bot) Config() *config.BotConfig {
	return b.cfg
}

// Client retrieves the current auth client.
func (b *Bot) Client() auth.ClientI {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b._client
}

func (b *Bot) setClient(client auth.ClientI) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Make sure the previous client is closed.
	if b._client != nil {
		_ = b._client.Close()
	}

	b._client = client
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

// certAuthorities returns cached CAs of the given type.
func (b *Bot) certAuthorities(caType types.CertAuthType) []types.CertAuthority {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b._cas[caType]
}

// clearCertAuthorities purges the CA cache. This should be run at least as
// frequently as CAs are rotated.
func (b *Bot) clearCertAuthorities() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b._cas = map[types.CertAuthType][]types.CertAuthority{}
}

// GetCertAuthorities returns the possibly cached CAs of the given type and
// requests them from the server if unavailable.
func (b *Bot) GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error) {
	if cas := b.certAuthorities(caType); len(cas) > 0 {
		return cas, nil
	}

	cas, err := b.Client().GetCertAuthorities(ctx, caType, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b._cas[caType] = cas
	return cas, nil
}

// authPong returns the last ping response from the auth server. It may be nil
// if no ping has succeeded.
func (b *Bot) authPong() *proto.PingResponse {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b._authPong
}

// AuthPing pings the auth server and returns the (possibly cached) response.
func (b *Bot) AuthPing(ctx context.Context) (*proto.PingResponse, error) {
	if authPong := b.authPong(); authPong != nil {
		return authPong, nil
	}

	pong, err := b.Client().Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b._authPong = &pong

	return &pong, nil
}

// proxyPong returns the last proxy ping response. It may be nil if no proxy
// ping has succeeded.
func (b *Bot) proxyPong() *webclient.PingResponse {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b._proxyPong
}

// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
// Note that it relies on the auth server being configured with a sane proxy
// public address.
func (b *Bot) ProxyPing(ctx context.Context) (*webclient.PingResponse, error) {
	if proxyPong := b.proxyPong(); proxyPong != nil {
		return proxyPong, nil
	}

	// Note: this relies on the auth server's proxy address. We could
	// potentially support some manual parameter here in the future if desired.
	authPong, err := b.AuthPing(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyPong, err := webclient.Ping(&webclient.Config{
		Context:   ctx,
		ProxyAddr: authPong.ProxyPublicAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b._proxyPong = proxyPong

	return proxyPong, nil
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

	// Maintain a context that we can cancel if the bot is running in one shot.
	ctx, cancel := context.WithCancel(ctx)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return trace.Wrap(b.caRotationLoop(egCtx))
	})
	eg.Go(func() error {
		err := b.renewLoop(egCtx)
		if err != nil {
			return trace.Wrap(err)
		}
		// If `renewLoop` exits with nil, the bot is running in "one-shot", so
		// we should indicate to other long-running processes that they can
		// finish up.
		cancel()
		return nil
	})

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

	// Start by loading the bot's primary destination.
	dest, err := b.cfg.Storage.GetDestination()
	if err != nil {
		return nil, trace.Wrap(
			err, "could not read bot storage destination from config",
		)
	}

	// Now attempt to lock the destination so we have sole use of it
	unlock, err := dest.TryLock()
	if err != nil {
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return unlock, trace.WrapWithMessage(err, "Failed to acquire exclusive lock for tbot destination directory - is tbot already running?")
		}
		return unlock, trace.Wrap(err)
	}

	var authClient auth.ClientI

	fetchNewIdentity := true
	// First, attempt to load an identity from storage.
	ident, err := identity.LoadIdentity(dest, identity.BotKinds()...)
	if err == nil {
		if b.cfg.Onboarding != nil && b.cfg.Onboarding.HasToken() {
			// try to grab the token to see if it's changed, as we'll need to fetch a new identity if it has
			if token, err := b.cfg.Onboarding.Token(); err == nil {
				sha := sha256.Sum256([]byte(token))
				configTokenHashBytes := []byte(hex.EncodeToString(sha[:]))

				fetchNewIdentity = hasTokenChanged(ident.TokenHashBytes, configTokenHashBytes)
			} else {
				// we failed to get the token, we'll continue on trying to use the existing identity
				b.log.WithError(err).Error("There was an error loading the token")

				fetchNewIdentity = false

				b.log.Info("Using the last good identity")
			}
		}

		if !fetchNewIdentity {
			identStr, err := describeTLSIdentity(ident)
			if err != nil {
				return unlock, trace.Wrap(err)
			}

			b.log.Infof("Successfully loaded bot identity, %s", identStr)

			if err := b.checkIdentity(ident); err != nil {
				return unlock, trace.Wrap(err)
			}

			if b.cfg.Onboarding != nil {
				b.log.Warn("Note: onboarding config ignored as identity was loaded from persistent storage")
			}

			authClient, err = b.AuthenticatedUserClientFromIdentity(ctx, ident)
			if err != nil {
				return unlock, trace.Wrap(err)
			}
		}
	}

	if fetchNewIdentity {
		if ident != nil {
			// If ident is set here, we detected a token change above.
			b.log.Warnf("Detected a token change, will attempt to fetch a new identity.")
		} else if trace.IsNotFound(err) {
			// This is _probably_ a fresh start, so we'll log the true error
			// and try to fetch a fresh identity.
			b.log.Debugf("Identity %s is not found or empty and could not be loaded, will start from scratch: %+v", dest, err)
		} else {
			return unlock, trace.Wrap(err)
		}

		// Verify we can write to the destination.
		if err := identity.VerifyWrite(dest); err != nil {
			return unlock, trace.Wrap(
				err, "Could not write to destination %s, aborting.", dest,
			)
		}

		// Get first identity
		ident, err = b.getIdentityFromToken()
		if err != nil {
			return unlock, trace.Wrap(err)
		}

		b.log.Debug("Attempting first connection using initial auth client")
		authClient, err = b.AuthenticatedUserClientFromIdentity(ctx, ident)
		if err != nil {
			return unlock, trace.Wrap(err)
		}

		// Attempt a request to make sure our client works.
		if _, err := authClient.Ping(ctx); err != nil {
			return unlock, trace.Wrap(err, "unable to communicate with auth server")
		}

		identStr, err := describeTLSIdentity(ident)
		if err != nil {
			return unlock, trace.Wrap(err)
		}
		b.log.Infof("Successfully generated new bot identity, %s", identStr)

		b.log.Debugf("Storing new bot identity to %s", dest)
		if err := identity.SaveIdentity(ident, dest, identity.BotKinds()...); err != nil {
			return unlock, trace.Wrap(
				err, "unable to save generated identity back to destination",
			)
		}
	}

	b.setClient(authClient)
	b.setIdent(ident)

	return unlock, nil
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
