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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
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

	client *fallableClient

	mu     sync.Mutex
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
func (s *identityService) GetClient() Client {
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
//
// If the persisted identity does not match the onboarding profile/join token,
// a nil identity will be returned. If the identity certificate has expired, the
// bool return value will be false.
func (s *identityService) loadIdentityFromStore(ctx context.Context, store bot.Destination) (*identity.Identity, bool) {
	ctx, span := tracer.Start(ctx, "identityService/loadIdentityFromStore")
	defer span.End()
	s.log.InfoContext(ctx, "Loading existing bot identity from store", "store", store)

	loadedIdent, err := identity.LoadIdentity(ctx, store, identity.BotKinds()...)
	if err != nil {
		if trace.IsNotFound(err) {
			s.log.InfoContext(ctx, "No existing bot identity found in store")
			return nil, false
		} else {
			s.log.WarnContext(
				ctx,
				"Failed to load existing bot identity from store",
				"error", err,
			)
			return nil, false
		}
	}

	// Determine if the token configured in the onboarding matches the
	// one used to produce the credentials loaded from disk.
	if s.cfg.Onboarding.HasToken() {
		if token, err := s.cfg.Onboarding.Token(); err == nil {
			sha := sha256.Sum256([]byte(token))
			configTokenHashBytes := []byte(hex.EncodeToString(sha[:]))
			if hasTokenChanged(loadedIdent.TokenHashBytes, configTokenHashBytes) {
				s.log.InfoContext(ctx, "Bot identity loaded from store does not match configured token")
				// If the token has changed, do not return the loaded
				// identity.
				return nil, false
			}
		} else {
			// we failed to get the newly configured token to compare to,
			// we'll assume the last good credentials written to disk should
			// still be used.
			s.log.WarnContext(
				ctx,
				"There was an error loading the configured token to compare to existing identity. Identity loaded from store will be tried",
				"error", err,
			)
		}
	}

	s.log.InfoContext(
		ctx,
		"Loaded existing bot identity from store",
		"identity", describeTLSIdentity(ctx, s.log, loadedIdent),
	)

	now := time.Now().UTC()
	valid := true
	if now.After(loadedIdent.X509Cert.NotAfter) {
		valid = false
		s.log.WarnContext(
			ctx,
			"Identity loaded from store is expired, it will not be used",
			"not_after", loadedIdent.X509Cert.NotAfter.Format(time.RFC3339),
			"current_time", now.Format(time.RFC3339),
		)
	} else if now.Before(loadedIdent.X509Cert.NotBefore) {
		valid = false
		s.log.WarnContext(
			ctx,
			"Identity loaded from store is not yet valid, it will not be used. Confirm that the system time is correct",
			"not_before", loadedIdent.X509Cert.NotBefore.Format(time.RFC3339),
			"current_time", now.Format(time.RFC3339),
		)
	}
	return loadedIdent, valid
}

// Initialize sets up the bot identity at startup. This process has a few
// steps to it.
//
// First, we attempt to load an existing identity from the configured storage.
// This is ignored if we know that the onboarding settings have changed.
//
// If the identity is found, and seems valid, we attempt to renew using this
// identity to give us a fresh set of certificates.
//
// If there is no identity, or the identity is invalid, we'll join using the
// configured onboarding settings.
func (s *identityService) Initialize(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Initialize")
	defer span.End()

	s.log.InfoContext(ctx, "Initializing bot identity")

	s.mu.Lock()
	defer s.mu.Unlock()

	s.client = &fallableClient{err: trace.Errorf("Failed to renew bot identity and initialize API client")}

	loadedIdent, valid := s.loadIdentityFromStore(ctx, s.cfg.Storage.Destination)
	if !valid {
		if !s.cfg.Onboarding.HasToken() {
			// There's no loaded identity to work with, and they've not
			// configured  a token to use to request an identity :(
			return trace.BadParameter(
				"no existing identity found on disk or join token configured",
			)
		}
		s.log.InfoContext(
			ctx,
			"Bot was unable to load a valid existing identity from the store, will attempt to join using configured token",
		)
	}

	var newIdentity *identity.Identity
	if loadedIdent == nil {
		// If there was no identity already on-disk or it did not match the
		// onboarding profile / join token, try to join from scratch.
		//
		// If this fails, tbot will exit because many services depend on there
		// being *some* identity - so there isn't really a way we can continue.
		var err error
		if newIdentity, err = botIdentityFromToken(ctx, s.log, s.cfg, nil); err != nil {
			return trace.Wrap(err, "joining with token")
		}
	} else {
		// If there was an identity on-disk from a previous run but renewing it
		// or re-joining fails, tbot will continue running with an "uninitialized
		// client" and possibly expired identity.
		//
		// In long-running mode, the Run method will retry the renewal process in
		// case connectivity to the auth server has been restored or something.
		//
		// In the meantime, some services may be able to continue operating with
		// cached credentials.
		s.facade = identity.NewFacade(s.cfg.FIPS, s.cfg.Insecure, loadedIdent)

		if valid {
			// If the identity is valid (not expired), try to renew it.
			var err error
			if newIdentity, err = renewIdentity(ctx, s.log, s.cfg, s.resolver, loadedIdent); err != nil {
				// We could technically try to create a client with the old identity
				// but if renewal failed, it's unlikely that dialing a connection
				// with the old certificate will succeed at this point.
				s.log.WarnContext(ctx, "Failed to renew bot identity. Some services may be able to run in a degraded mode, but many will fail", "error", err)
				return nil
			}
		} else {
			// If the identity has expired, try to join again from scratch.
			var err error
			if newIdentity, err = botIdentityFromToken(ctx, s.log, s.cfg, nil); err != nil {
				s.log.WarnContext(ctx, "Failed to re-join. Some services may be able to run in a degraded mode, but many will fail", "error", err)
				return nil
			}
		}
	}

	// We successfully renewed the bot identity!
	s.log.InfoContext(ctx, "Fetched new bot identity", "identity", describeTLSIdentity(ctx, s.log, newIdentity))

	// Save it to disk.
	if err := identity.SaveIdentity(ctx, newIdentity, s.cfg.Storage.Destination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err)
	}

	// Properly initialize the client.
	s.facade = identity.NewFacade(s.cfg.FIPS, s.cfg.Insecure, newIdentity)
	c, err := clientForFacade(ctx, s.log, s.cfg, s.facade, s.resolver)
	if err != nil {
		// It's unlikely for this to fail, because we've literally just been able
		// to create a client in order to renew the bot identity. If it does, tbot
		// will continue running with an "uninitialized client" and in long-running
		// mode, the Run method will try to fix this.
		s.log.WarnContext(ctx, "Failed to create API client. Some services may be able to run in a degraded mode, but many will fail", "error", err)
		return nil
	}
	s.client.setClient(c)

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

	if err := s.retryFailedInitialization(ctx); err != nil {
		return trace.Wrap(err, "retrying failed initialization")
	}

	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	storageDestination := s.cfg.Storage.Destination

	s.log.InfoContext(
		ctx,
		"Beginning bot identity renewal loop",
		"ttl", s.cfg.CredentialLifetime.TTL,
		"interval", s.cfg.CredentialLifetime.RenewalInterval,
	)

	err := runOnInterval(ctx, runOnIntervalConfig{
		service: s.String(),
		name:    "bot-identity-renewal",
		f: func(ctx context.Context) error {
			return s.renew(ctx, storageDestination)
		},
		interval:             s.cfg.CredentialLifetime.RenewalInterval,
		exitOnRetryExhausted: true,
		retryLimit:           botIdentityRenewalRetryLimit,
		log:                  s.log,
		reloadCh:             reloadCh,
		waitBeforeFirstRun:   true,
	})
	return trace.Wrap(err)
}

func (s *identityService) retryFailedInitialization(ctx context.Context) error {
	s.mu.Lock()
	client := s.client
	facade := s.facade
	s.mu.Unlock()

	// Initialization succeeded, nothing to do.
	if client.err == nil {
		return nil
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(1 * time.Second),
		Max:    1 * time.Minute,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err, "creating retrier")
	}

	for {
		retry.Inc()

		select {
		case <-retry.After():
		case <-ctx.Done():
			return ctx.Err()
		}

		s.log.InfoContext(ctx, "Retrying failed bot identity initialization")

		oldIdent := facade.Get()
		now := time.Now().UTC()

		var newIdentity *identity.Identity
		if now.After(oldIdent.X509Cert.NotBefore) && now.Before(oldIdent.X509Cert.NotAfter) {
			// Old identity is still valid, try renewing it.
			if newIdentity, err = renewIdentity(ctx, s.log, s.cfg, s.resolver, oldIdent); err != nil {
				s.log.WarnContext(ctx, "Failed to renew bot identity", "error", err)
				continue
			}
		} else {
			// Old identity has expired, try re-joining.
			if newIdentity, err = botIdentityFromToken(ctx, s.log, s.cfg, nil); err != nil {
				s.log.WarnContext(ctx, "Failed to re-join", "error", err)
				continue
			}
		}

		s.log.InfoContext(ctx, "Fetched new bot identity", "identity", describeTLSIdentity(ctx, s.log, newIdentity))
		s.facade.Set(newIdentity)

		if err := identity.SaveIdentity(ctx, newIdentity, s.cfg.Storage.Destination, identity.BotKinds()...); err != nil {
			s.log.WarnContext(ctx, "Failed to save bot identity", "error", err)
			continue
		}

		newClient, err := clientForFacade(ctx, s.log, s.cfg, facade, s.resolver)
		if err != nil {
			s.log.WarnContext(ctx, "Failed to create API client", "error", err)
			continue
		}
		client.setClient(newClient)

		return nil
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

	newIdentity, err := renewIdentity(ctx, s.log, s.cfg, s.resolver, currentIdentity)
	if err != nil {
		return trace.Wrap(err, "renewing identity")
	}

	s.log.InfoContext(ctx, "Fetched new bot identity", "identity", describeTLSIdentity(ctx, s.log, newIdentity))
	s.facade.Set(newIdentity)

	if err := identity.SaveIdentity(ctx, newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err, "saving new identity")
	}
	s.log.DebugContext(ctx, "Bot identity persisted", "identity", describeTLSIdentity(ctx, s.log, newIdentity))

	return nil
}

func renewIdentity(
	ctx context.Context,
	log *slog.Logger,
	botCfg *config.BotConfig,
	resolver reversetunnelclient.Resolver,
	oldIdentity *identity.Identity,
) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "renewIdentity")
	defer span.End()

	// Explicitly create a new client - this guarantees that requests will be
	// made with the most recent identity and that a connection associated with
	// an old identity will not be used.
	facade := identity.NewFacade(botCfg.FIPS, botCfg.Insecure, oldIdentity)
	client, err := clientForFacade(ctx, log, botCfg, facade, resolver)
	if err != nil {
		return nil, trace.Wrap(err, "creating auth client")
	}
	defer client.Close()

	if oldIdentity.TLSIdentity.Renewable {
		// When using a renewable join method, we use GenerateUserCerts to
		// request a new certificate using our current identity.
		newIdentity, err := botIdentityFromAuth(
			ctx, log, oldIdentity, &fallableClient{client: client}, botCfg.CredentialLifetime.TTL,
		)
		if err != nil {
			return nil, trace.Wrap(err, "renewing identity using GenerateUserCert")
		}
		return newIdentity, nil
	}

	newIdentity, err := botIdentityFromToken(ctx, log, botCfg, client)
	if err != nil {
		return nil, trace.Wrap(err, "renewing identity using Register")
	}
	return newIdentity, nil
}

// botIdentityFromAuth uses an existing identity to request a new from the auth
// server using GenerateUserCerts. This only works for renewable join types.
func botIdentityFromAuth(
	ctx context.Context,
	log *slog.Logger,
	ident *identity.Identity,
	client Client,
	ttl time.Duration,
) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "botIdentityFromAuth")
	defer span.End()
	log.InfoContext(ctx, "Fetching bot identity using existing bot identity")

	if ident == nil || client == nil {
		return nil, trace.BadParameter("renewIdentityWithAuth must be called with non-nil client and identity")
	}

	// Always generate a new key when refreshing the identity. This limits
	// usefulness of compromised keys to the lifetime of their associated cert,
	// and allows for new keys to follow any changes to the signature algorithm
	// suite.
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(client),
		cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPubKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub := ssh.MarshalAuthorizedKey(sshPubKey)
	tlsPub, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		SSHPublicKey: sshPub,
		TLSPublicKey: tlsPub,
		Username:     ident.X509Cert.Subject.CommonName,
		Expires:      time.Now().Add(ttl),
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling GenerateUserCerts")
	}

	newIdentity, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKeyPEM,
		PublicKeyBytes:  sshPub,
		TokenHashBytes:  ident.TokenHashBytes,
	}, certs)
	if err != nil {
		return nil, trace.Wrap(err, "reading renewed identity")
	}

	return newIdentity, nil
}

// botIdentityFromToken uses a join token to request a bot identity from an auth
// server using auth.Register.
//
// The authClient parameter is optional - if provided - this will be used for
// the request. This saves the overhead of trying to create a new client as
// part of the join process and allows us to preserve the bot instance id.
func botIdentityFromToken(
	ctx context.Context,
	log *slog.Logger,
	cfg *config.BotConfig,
	authClient *apiclient.Client,
) (*identity.Identity, error) {
	_, span := tracer.Start(ctx, "botIdentityFromToken")
	defer span.End()

	log.InfoContext(ctx, "Fetching bot identity using token")

	token, err := cfg.Onboarding.Token()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	expires := time.Now().Add(cfg.CredentialLifetime.TTL)
	params := join.RegisterParams{
		Token: token,
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		JoinMethod: cfg.Onboarding.JoinMethod,
		Expires:    &expires,

		// Below options are effectively ignored if AuthClient is not-nil
		Insecure:           cfg.Insecure,
		CAPins:             cfg.Onboarding.CAPins,
		CAPath:             cfg.Onboarding.CAPath,
		FIPS:               cfg.FIPS,
		GetHostCredentials: client.HostCredentials,
		CipherSuites:       cfg.CipherSuites(),
	}
	if authClient != nil {
		params.AuthClient = authClient
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

	switch params.JoinMethod {
	case types.JoinMethodAzure:
		params.AzureParams = join.AzureParams{
			ClientID: cfg.Onboarding.Azure.ClientID,
		}
	case types.JoinMethodTerraformCloud:
		params.TerraformCloudAudienceTag = cfg.Onboarding.Terraform.AudienceTag
	case types.JoinMethodGitLab:
		params.GitlabParams = join.GitlabParams{
			EnvVarName: cfg.Onboarding.Gitlab.TokenEnvVarName,
		}
	}

	if params.JoinMethod == types.JoinMethodBoundKeypair {
		joinSecret := cfg.Onboarding.BoundKeypair.InitialJoinSecret

		adapter := config.NewBoundkeypairDestinationAdapter(cfg.Storage.Destination)
		state, err := boundkeypair.LoadClientState(ctx, adapter)
		if trace.IsNotFound(err) && joinSecret != "" {
			return nil, trace.NotImplemented("no existing client state was found and join secrets are not yet supported")
		} else if err != nil {
			return nil, trace.Wrap(err, "loading bound keypair client state")
		}

		params.BoundKeypairParams = state.ToJoinParams(joinSecret)
	}

	result, err := join.Register(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeyPEM, err := keys.MarshalPrivateKey(result.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(result.PrivateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sha := sha256.Sum256([]byte(params.Token))
	tokenHash := hex.EncodeToString(sha[:])
	ident, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKeyPEM,
		PublicKeyBytes:  ssh.MarshalAuthorizedKey(sshPub),
		TokenHashBytes:  []byte(tokenHash),
	}, result.Certs)
	return ident, trace.Wrap(err)
}
