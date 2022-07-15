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
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Bot struct {
	cfg        *config.BotConfig
	log        logrus.FieldLogger
	reloadChan chan struct{}

	// These are protected by getter/setters with mutex locks
	mu      sync.Mutex
	_client auth.ClientI
	_ident  *identity.Identity
	started bool
}

func New(cfg *config.BotConfig, log logrus.FieldLogger, reloadChan chan struct{}) *Bot {
	if log == nil {
		log = utils.NewLogger()
	}

	return &Bot{
		cfg:        cfg,
		log:        log,
		reloadChan: reloadChan,
	}
}

func (b *Bot) client() auth.ClientI {
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

func (b *Bot) Run(ctx context.Context) error {
	if err := b.markStarted(); err != nil {
		return trace.Wrap(err)
	}

	if err := b.initialize(ctx); err != nil {
		return trace.Wrap(err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return trace.Wrap(b.caRotationLoop(egCtx))
	})
	eg.Go(func() error {
		return trace.Wrap(b.renewLoop(egCtx))
	})

	return eg.Wait()
}

func (b *Bot) initialize(ctx context.Context) error {
	if b.cfg.AuthServer == "" {
		return trace.BadParameter("an auth or proxy server must be set via --auth-server or configuration")
	}

	// First, try to make sure all destinations are usable.
	if err := checkDestinations(b.cfg); err != nil {
		return trace.Wrap(err)
	}

	// Start by loading the bot's primary destination.
	dest, err := b.cfg.Storage.GetDestination()
	if err != nil {
		return trace.Wrap(err, "could not read bot storage destination from config")
	}

	configTokenHashBytes := []byte{}
	if b.cfg.Onboarding != nil && b.cfg.Onboarding.Token != "" {
		sha := sha256.Sum256([]byte(b.cfg.Onboarding.Token))
		configTokenHashBytes = []byte(hex.EncodeToString(sha[:]))
	}

	var authClient auth.ClientI

	// First, attempt to load an identity from storage.
	ident, err := identity.LoadIdentity(dest, identity.BotKinds()...)
	if err == nil && !hasTokenChanged(ident.TokenHashBytes, configTokenHashBytes) {
		identStr, err := describeTLSIdentity(ident)
		if err != nil {
			return trace.Wrap(err)
		}

		b.log.Infof("Successfully loaded bot identity, %s", identStr)

		if err := b.checkIdentity(ident); err != nil {
			return trace.Wrap(err)
		}

		if b.cfg.Onboarding != nil {
			b.log.Warn("Note: onboarding config ignored as identity was loaded from persistent storage")
		}

		authClient, err = b.authenticatedUserClientFromIdentity(ctx, ident, b.cfg.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// If the identity can't be loaded, assume we're starting fresh and
		// need to generate our initial identity from a token

		if ident != nil {
			// If ident is set here, we detected a token change above.
			b.log.Warnf("Detected a token change, will attempt to fetch a new identity.")
		} else if trace.IsNotFound(err) {
			// This is _probably_ a fresh start, so we'll log the true error
			// and try to fetch a fresh identity.
			b.log.Debugf("Identity %s is not found or empty and could not be loaded, will start from scratch: %+v", dest, err)
		} else {
			return trace.Wrap(err)
		}

		// Verify we can write to the destination.
		if err := identity.VerifyWrite(dest); err != nil {
			return trace.Wrap(err, "Could not write to destination %s, aborting.", dest)
		}

		// Get first identity
		ident, err = b.getIdentityFromToken()
		if err != nil {
			return trace.Wrap(err)
		}

		b.log.Debug("Attempting first connection using initial auth client")
		authClient, err = b.authenticatedUserClientFromIdentity(ctx, ident, b.cfg.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}

		// Attempt a request to make sure our client works.
		if _, err := authClient.Ping(ctx); err != nil {
			return trace.Wrap(err, "unable to communicate with auth server")
		}

		identStr, err := describeTLSIdentity(ident)
		if err != nil {
			return trace.Wrap(err)
		}
		b.log.Infof("Successfully generated new bot identity, %s", identStr)

		b.log.Debugf("Storing new bot identity to %s", dest)
		if err := identity.SaveIdentity(ident, dest, identity.BotKinds()...); err != nil {
			return trace.Wrap(err, "unable to save generated identity back to destination")
		}
	}

	b.setClient(authClient)
	b.setIdent(ident)

	return nil
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
