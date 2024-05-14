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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

// botIdentityRenewalRetryLimit is the number of permissible consecutive
// failures in renewing the bot identity before the loop exits fatally.
const botIdentityRenewalRetryLimit = 7

// identityService is a [bot.Service] that handles renewing the bot's identity.
// It renews the bot's identity periodically and when receiving a broadcasted
// reload signal.
//
// It does not offer a [bot.OneShotService] implementation as the Bot's identity
// is renewed automatically during initialization.
type identityService struct {
	log               *slog.Logger
	reloadBroadcaster *channelBroadcaster
	cfg               *config.BotConfig
	resolver          reversetunnelclient.Resolver

	mu     sync.Mutex
	client *auth.Client
	facade *identity.Facade
}

// GetIdentity returns the current Bot identity.
func (s *identityService) GetIdentity() *identity.Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.facade.Get()
}

// GetClient returns the facaded client for the Bot identity for use by other
// components of `tbot`. Consumers should not call `Close` on the client.
func (s *identityService) GetClient() *auth.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// String returns a human-readable name of the service.
func (s *identityService) String() string {
	return "identity"
}

func hasTokenChanged(configTokenBytes, identityBytes []byte) bool {
	if len(configTokenBytes) == 0 || len(identityBytes) == 0 {
		return false
	}

	return !bytes.Equal(identityBytes, configTokenBytes)
}

// loadIdentityFromStore attempts to load a persisted identity from a store.
// It checks this loaded identity against the configured onboarding profile
// and ignores the loaded identity if there has been a configuration change.
func (s *identityService) loadIdentityFromStore(ctx context.Context, store bot.Destination) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "identityService/loadIdentityFromStore")
	defer span.End()
	s.log.InfoContext(ctx, "Loading existing bot identity from store.", "store", store)

	loadedIdent, err := identity.LoadIdentity(ctx, store, identity.BotKinds()...)
	if err != nil {
		if trace.IsNotFound(err) {
			s.log.InfoContext(ctx, "No existing bot identity found in store. Bot will join using configured token.")
			return nil, nil
		} else {
			return nil, trace.Wrap(err)
		}
	}

	// Determine if the token configured in the onboarding matches the
	// one used to produce the credentials loaded from disk.
	if s.cfg.Onboarding.HasToken() {
		if token, err := s.cfg.Onboarding.Token(); err == nil {
			sha := sha256.Sum256([]byte(token))
			configTokenHashBytes := []byte(hex.EncodeToString(sha[:]))
			if hasTokenChanged(loadedIdent.TokenHashBytes, configTokenHashBytes) {
				s.log.InfoContext(ctx, "Bot identity loaded from store does not match configured token. Bot will fetch identity using configured token.")
				// If the token has changed, do not return the loaded
				// identity.
				return nil, nil
			}
		} else {
			// we failed to get the newly configured token to compare to,
			// we'll assume the last good credentials written to disk should
			// still be used.
			s.log.ErrorContext(ctx, "There was an error loading the configured token. Bot identity loaded from store will be tried.", "error", err)
		}
	}
	s.log.InfoContext(ctx, "Loaded existing bot identity from store.", "identity", describeTLSIdentity(ctx, s.log, loadedIdent))

	return loadedIdent, nil
}

// Initialize attempts to load an existing identity from the bot's storage.
// If an identity is found, it is checked against the configured onboarding
// settings. It is then renewed and persisted.
//
// If no identity is found, or the identity is no longer valid, a new identity
// is requested using the configured onboarding settings.
func (s *identityService) Initialize(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Initialize")
	defer span.End()

	s.log.InfoContext(ctx, "Initializing bot identity.")
	var loadedIdent *identity.Identity
	var err error
	if s.cfg.Onboarding.RenewableJoinMethod() {
		// Nil, nil will be returned if no identity can be found in store or
		// the identity in the store is no longer relevant.
		loadedIdent, err = s.loadIdentityFromStore(ctx, s.cfg.Storage.Destination)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var newIdentity *identity.Identity
	if s.cfg.Onboarding.RenewableJoinMethod() && loadedIdent != nil {
		// If using a renewable join method and we loaded an identity, let's
		// immediately renew it so we know that after initialisation we have the
		// full certificate TTL.
		if err := checkIdentity(ctx, s.log, loadedIdent); err != nil {
			return trace.Wrap(err)
		}
		facade := identity.NewFacade(s.cfg.FIPS, s.cfg.Insecure, loadedIdent)
		authClient, err := clientForFacade(ctx, s.log, s.cfg, facade, s.resolver)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()
		newIdentity, err = botIdentityFromAuth(
			ctx, s.log, loadedIdent, authClient, s.cfg.CertificateTTL,
		)
		if err != nil {
			return trace.Wrap(err)
		}
	} else if s.cfg.Onboarding.HasToken() {
		// If using a non-renewable join method, or we weren't able to load an
		// identity from the store, let's get a new identity using the
		// configured token.
		newIdentity, err = botIdentityFromToken(ctx, s.log, s.cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// There's no loaded identity to work with, and they've not configured
		// a token to use to request an identity :(
		return trace.BadParameter("no existing identity found on disk or join token configured")
	}

	s.log.InfoContext(ctx, "Fetched new bot identity", "identity", describeTLSIdentity(ctx, s.log, newIdentity))
	if err := identity.SaveIdentity(ctx, newIdentity, s.cfg.Storage.Destination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err)
	}

	// Create the facaded client we can share with other components of tbot.
	facade := identity.NewFacade(s.cfg.FIPS, s.cfg.Insecure, newIdentity)
	c, err := clientForFacade(ctx, s.log, s.cfg, facade, s.resolver)
	if err != nil {
		return trace.Wrap(err)
	}
	s.mu.Lock()
	s.client = c
	s.facade = facade
	s.mu.Unlock()

	s.log.InfoContext(ctx, "Identity initialized successfully")
	return nil
}

func (s *identityService) Close() error {
	c := s.GetClient()
	if c == nil {
		return nil
	}
	return trace.Wrap(c.Close())
}

func (s *identityService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Run")
	defer span.End()
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	s.log.InfoContext(
		ctx,
		"Beginning bot identity renewal loop",
		"ttl", s.cfg.CertificateTTL,
		"interval", s.cfg.RenewalInterval,
	)

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	storageDestination := s.cfg.Storage.Destination

	ticker := time.NewTicker(s.cfg.RenewalInterval)
	jitter := retryutils.NewJitter()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-reloadCh:
		}

		var err error
		for attempt := 1; attempt <= botIdentityRenewalRetryLimit; attempt++ {
			s.log.InfoContext(
				ctx,
				"Attempting to renewing bot identity",
				"attempt", attempt,
				"retry_limit", botIdentityRenewalRetryLimit,
			)
			err = s.renew(
				ctx, storageDestination,
			)
			if err == nil {
				break
			}

			if attempt != botIdentityRenewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				s.log.ErrorContext(
					ctx,
					"Bot identity renewal attempt failed. Waiting to retry.",
					"error", err,
					"attempt", attempt,
					"retry_limit", botIdentityRenewalRetryLimit,
					"backoff", backoffTime,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoffTime):
				}
			}
		}
		if err != nil {
			s.log.ErrorContext(
				ctx,
				"All bot identity renewal attempts exhausted. Exiting",
				"error", err,
				"retry_limit", botIdentityRenewalRetryLimit,
			)
			return trace.Wrap(err)
		}
		s.log.InfoContext(ctx, "Renewed bot identity. Waiting to renew again", "wait", s.cfg.RenewalInterval)
	}
}

func (s *identityService) renew(
	ctx context.Context,
	botDestination bot.Destination,
) error {
	ctx, span := tracer.Start(ctx, "identityService/renew")
	defer span.End()

	currentIdentity := s.facade.Get()
	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(ctx, botDestination); err != nil {
		return trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	var newIdentity *identity.Identity
	var err error
	if s.cfg.Onboarding.RenewableJoinMethod() {
		// When using a renewable join method, we use GenerateUserCerts to
		// request a new certificate using our current identity.
		// We explicitly create a new client here to ensure that the latest
		// identity is being used!
		facade := identity.NewFacade(s.cfg.FIPS, s.cfg.Insecure, currentIdentity)
		authClient, err := clientForFacade(ctx, s.log, s.cfg, facade, s.resolver)
		if err != nil {
			return trace.Wrap(err, "creating auth client")
		}
		defer authClient.Close()
		newIdentity, err = botIdentityFromAuth(
			ctx, s.log, currentIdentity, authClient, s.cfg.CertificateTTL,
		)
		if err != nil {
			return trace.Wrap(err, "renewing identity with existing identity")
		}
	} else {
		// When using the non-renewable join methods, we rejoin each time rather
		// than using certificate renewal.
		newIdentity, err = botIdentityFromToken(ctx, s.log, s.cfg)
		if err != nil {
			return trace.Wrap(err, "renewing identity with token")
		}
	}

	s.log.InfoContext(ctx, "Fetched new bot identity", "identity", describeTLSIdentity(ctx, s.log, newIdentity))
	s.facade.Set(newIdentity)

	if err := identity.SaveIdentity(ctx, newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err, "saving new identity")
	}
	s.log.DebugContext(ctx, "Bot identity persisted", "identity", describeTLSIdentity(ctx, s.log, newIdentity))

	return nil
}

// botIdentityFromAuth uses an existing identity to request a new from the auth
// server using GenerateUserCerts. This only works for renewable join types.
func botIdentityFromAuth(
	ctx context.Context,
	log *slog.Logger,
	ident *identity.Identity,
	client *auth.Client,
	ttl time.Duration,
) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "botIdentityFromAuth")
	defer span.End()
	log.InfoContext(ctx, "Fetching bot identity using existing bot identity")

	if ident == nil || client == nil {
		return nil, trace.BadParameter("renewIdentityWithAuth must be called with non-nil client and identity")
	}
	// Ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: ident.PublicKeyBytes,
		Username:  ident.X509Cert.Subject.CommonName,
		Expires:   time.Now().Add(ttl),
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling GenerateUserCerts")
	}

	newIdentity, err := identity.ReadIdentityFromStore(
		ident.Params(),
		certs,
	)
	if err != nil {
		return nil, trace.Wrap(err, "reading renewed identity")
	}

	return newIdentity, nil
}

// botIdentityFromToken uses a join token to request a bot identity from an auth
// server using auth.Register.
func botIdentityFromToken(ctx context.Context, log *slog.Logger, cfg *config.BotConfig) (*identity.Identity, error) {
	_, span := tracer.Start(ctx, "botIdentityFromToken")
	defer span.End()

	log.InfoContext(ctx, "Fetching bot identity using token")

	tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
	if err != nil {
		return nil, trace.Wrap(err, "unable to generate new keypairs")
	}

	token, err := cfg.Onboarding.Token()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	expires := time.Now().Add(cfg.CertificateTTL)
	params := auth.RegisterParams{
		Token: token,
		ID: auth.IdentityID{
			Role: types.RoleBot,
		},
		PublicTLSKey:       tlsPublicKey,
		PublicSSHKey:       sshPublicKey,
		CAPins:             cfg.Onboarding.CAPins,
		CAPath:             cfg.Onboarding.CAPath,
		GetHostCredentials: client.HostCredentials,
		JoinMethod:         cfg.Onboarding.JoinMethod,
		Expires:            &expires,
		FIPS:               cfg.FIPS,
		CipherSuites:       cfg.CipherSuites(),
		Insecure:           cfg.Insecure,
	}

	addr, addrKind := cfg.Address()
	switch addrKind {
	case config.AddressKindAuth:
		parsed, err := utils.ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse addr")
		}
		params.AuthServers = []utils.NetAddr{*parsed}
	case config.AddressKindProxy:
		parsed, err := utils.ParseAddr(addr)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse addr")
		}
		params.ProxyServer = *parsed
	default:
		return nil, trace.BadParameter("unsupported address kind: %v", addrKind)
	}

	if params.JoinMethod == types.JoinMethodAzure {
		params.AzureParams = auth.AzureParams{
			ClientID: cfg.Onboarding.Azure.ClientID,
		}
	}

	certs, err := auth.Register(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sha := sha256.Sum256([]byte(params.Token))
	tokenHash := hex.EncodeToString(sha[:])
	ident, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: tlsPrivateKey,
		PublicKeyBytes:  sshPublicKey,
		TokenHashBytes:  []byte(tokenHash),
	}, certs)
	return ident, trace.Wrap(err)
}
