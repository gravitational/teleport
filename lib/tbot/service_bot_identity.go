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
	"math"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
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
	log               logrus.FieldLogger
	reloadBroadcaster *channelBroadcaster
	cfg               *config.BotConfig

	mu     sync.Mutex
	_ident *identity.Identity
}

func (s *identityService) String() string {
	return "identity"
}

func (s *identityService) setIdent(i *identity.Identity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s._ident = i
}

func (s *identityService) ident() *identity.Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._ident
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
	s.log.WithField("store", store).Info("Loading existing bot identity from store.")

	loadedIdent, err := identity.LoadIdentity(ctx, store, identity.BotKinds()...)
	if err != nil {
		if trace.IsNotFound(err) {
			s.log.Info("No existing bot identity found in store. Bot will join using configured token.")
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
				s.log.Info("Bot identity loaded from store does not match configured token. Bot will fetch identity using configured token.")
				// If the token has changed, do not return the loaded
				// identity.
				return nil, nil
			}
		} else {
			// we failed to get the newly configured token to compare to,
			// we'll assume the last good credentials written to disk should
			// still be used.
			s.log.
				WithError(err).
				Error("There was an error loading the configured token. Bot identity loaded from store will be tried.")
		}
	}
	s.log.WithField("identity", describeTLSIdentity(s.log, loadedIdent)).Info("Loaded existing bot identity from store.")

	return loadedIdent, nil
}

// Initialize attempts to loaad an existing identity from the bot's storage.
// If an identity is found, it is checked against the configured onboarding
// settings. It is then renewed and persisted.
//
// If no identity is found, or the identity is no longer valid, a new identity
// is requested using the configured onboarding settings.
func (s *identityService) Initialize(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Initialize")
	defer span.End()

	s.log.Info("Initializing bot identity.")
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
		if err := checkIdentity(s.log, loadedIdent); err != nil {
			return trace.Wrap(err)
		}
		authClient, err := clientForIdentity(ctx, s.log, s.cfg, loadedIdent)
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

	s.log.WithField("identity", describeTLSIdentity(s.log, newIdentity)).Info("Fetched new bot identity.")
	if err := identity.SaveIdentity(ctx, newIdentity, s.cfg.Storage.Destination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err)
	}

	testClient, err := clientForIdentity(ctx, s.log, s.cfg, newIdentity)
	if err != nil {
		return trace.Wrap(err)
	}
	defer testClient.Close()

	s.setIdent(newIdentity)

	// Attempt a request to make sure our client works so we can exit early if
	// we are in a bad state.
	if _, err := testClient.Ping(ctx); err != nil {
		return trace.Wrap(err, "unable to communicate with auth server")
	}
	s.log.Info("Identity initialized successfully.")

	return nil
}

func (s *identityService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Run")
	defer span.End()
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	s.log.Infof(
		"Beginning bot identity renewal loop: ttl=%s interval=%s",
		s.cfg.CertificateTTL,
		s.cfg.RenewalInterval,
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
			s.log.Infof(
				"Renewing bot identity. Attempt %d of %d.",
				attempt,
				botIdentityRenewalRetryLimit,
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
				s.log.WithError(err).Errorf(
					"Bot identity renewal attempt %d of %d failed. Retrying after %s.",
					attempt,
					botIdentityRenewalRetryLimit,
					backoffTime,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoffTime):
				}
			}
		}
		if err != nil {
			s.log.WithError(err).Errorf("%d bot identity renewals failed. All retry attempts exhausted. Exiting.", botIdentityRenewalRetryLimit)
			return trace.Wrap(err)
		}
		s.log.Infof("Renewed bot identity. Next bot identity renewal in approximately %s.", s.cfg.RenewalInterval)
	}
}

func (s *identityService) renew(
	ctx context.Context,
	botDestination bot.Destination,
) error {
	ctx, span := tracer.Start(ctx, "identityService/renew")
	defer span.End()

	currentIdentity := s.ident()
	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(ctx, botDestination); err != nil {
		return trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	var newIdentity *identity.Identity
	var err error
	if s.cfg.Onboarding.RenewableJoinMethod() {
		// When using a renewable join method, we use GenerateUserCerts to
		// request a new certificate using our current identity.
		authClient, err := clientForIdentity(ctx, s.log, s.cfg, currentIdentity)
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

	s.log.WithField("identity", describeTLSIdentity(s.log, newIdentity)).Info("Fetched new bot identity.")
	s.setIdent(newIdentity)

	if err := identity.SaveIdentity(ctx, newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err, "saving new identity")
	}
	s.log.WithField("identity", describeTLSIdentity(s.log, newIdentity)).Debug("Bot identity persisted.")

	return nil
}

// botIdentityFromAuth uses an existing identity to request a new from the auth
// server using GenerateUserCerts. This only works for renewable join types.
func botIdentityFromAuth(
	ctx context.Context,
	log logrus.FieldLogger,
	ident *identity.Identity,
	client auth.ClientI,
	ttl time.Duration,
) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "botIdentityFromAuth")
	defer span.End()
	log.Info("Fetching bot identity using existing bot identity.")

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
func botIdentityFromToken(ctx context.Context, log logrus.FieldLogger, cfg *config.BotConfig) (*identity.Identity, error) {
	_, span := tracer.Start(ctx, "botIdentityFromToken")
	defer span.End()

	log.Info("Fetching bot identity using token.")
	addr, err := utils.ParseAddr(cfg.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err, "invalid auth server address %+v", cfg.AuthServer)
	}

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
		AuthServers:        []utils.NetAddr{*addr},
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

	if params.JoinMethod == types.JoinMethodAzure {
		params.AzureParams = auth.AzureParams{
			ClientID: cfg.Onboarding.Azure.ClientID,
		}
	}

	certs, err := auth.Register(params)
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
