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
	"crypto/sha256"
	"encoding/hex"
	"math"
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

type identityService struct {
	log               logrus.FieldLogger
	b                 *Bot
	reloadBroadcaster *channelBroadcaster
}

func (s *identityService) String() string {
	return "identity"
}

func (s *identityService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "identityService/Run")
	defer span.End()
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	s.log.Infof(
		"Beginning bot identity renewal loop: ttl=%s interval=%s",
		s.b.cfg.CertificateTTL,
		s.b.cfg.RenewalInterval,
	)

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	storageDestination := s.b.cfg.Storage.Destination

	ticker := time.NewTicker(s.b.cfg.RenewalInterval)
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
			s.b.log.Infof(
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
		s.log.Infof("Renewed bot identity. Next bot identity renewal in approximately %s.", s.b.cfg.RenewalInterval)
	}
}

func (s *identityService) renew(
	ctx context.Context,
	botDestination bot.Destination,
) error {
	ctx, span := tracer.Start(ctx, "identityService/renew")
	defer span.End()

	currentIdentity := s.b.ident()
	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(ctx, botDestination); err != nil {
		return trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	var newIdentity *identity.Identity
	var err error
	if s.b.cfg.Onboarding.RenewableJoinMethod() {
		// When using a renewable join method, we use GenerateUserCerts to
		// request a new certificate using our current identity.
		authClient, err := s.b.AuthenticatedUserClientFromIdentity(ctx, currentIdentity)
		if err != nil {
			return trace.Wrap(err, "creating auth client")
		}
		defer authClient.Close()
		newIdentity, err = botIdentityFromAuth(
			ctx, s.b.log, currentIdentity, authClient, s.b.cfg.CertificateTTL,
		)
		if err != nil {
			return trace.Wrap(err, "renewing identity with existing identity")
		}
	} else {
		// When using the non-renewable join methods, we rejoin each time rather
		// than using certificate renewal.
		newIdentity, err = botIdentityFromToken(ctx, s.b.log, s.b.cfg)
		if err != nil {
			return trace.Wrap(err, "renewing identity with token")
		}
	}

	s.b.log.WithField("identity", describeTLSIdentity(s.b.log, newIdentity)).Info("Fetched new bot identity.")
	s.b.setIdent(newIdentity)

	if err := identity.SaveIdentity(ctx, newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err, "saving new identity")
	}
	s.b.log.WithField("identity", describeTLSIdentity(s.b.log, newIdentity)).Debug("Bot identity persisted.")

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
