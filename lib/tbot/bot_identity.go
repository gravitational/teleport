package tbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

const botIdentityRenewalRetryLimit = 7

func (b *Bot) renewBotIdentityLoop(
	ctx context.Context,
	reloadChan <-chan struct{},
) error {
	b.log.Infof(
		"Beginning bot identity renewal loop: ttl=%s interval=%s",
		b.cfg.CertificateTTL,
		b.cfg.RenewalInterval,
	)
	if b.cfg.RenewalInterval > b.cfg.CertificateTTL {
		b.log.Errorf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). The next renewal is likely to fail.",
			b.cfg.CertificateTTL,
			b.cfg.RenewalInterval,
		)
	}

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	botDestination, err := b.cfg.Storage.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(b.cfg.RenewalInterval)
	jitter := retryutils.NewJitter()
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-reloadChan:
		}

		var err error
		var partiallyRenewedIdentity *identity.Identity
		for attempt := 1; attempt <= botIdentityRenewalRetryLimit; attempt++ {
			b.log.Infof(
				"Renewing bot identity. Attempt %d of %d.",
				attempt,
				botIdentityRenewalRetryLimit,
			)
			partiallyRenewedIdentity, err = b.renewBotIdentity(
				ctx, botDestination, partiallyRenewedIdentity,
			)
			if err == nil {
				break
			}

			if attempt != botIdentityRenewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				b.log.WithError(err).Errorf(
					"Bot identity renewal attempt %d of %d failed. Retrying after %s.",
					attempt,
					renewalRetryLimit,
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
			b.log.WithError(err).Errorf("%d bot identity renewals failed. All retry attempts exhausted. Exiting.", renewalRetryLimit)
			return trace.Wrap(err)
		}
		b.log.Infof("Renewed bot identity. Next bot identity renewal in approximately %s.", b.cfg.RenewalInterval)
	}
}

func (b *Bot) renewBotIdentity(
	ctx context.Context,
	botDestination bot.Destination,
	partiallyRenewedIdentity *identity.Identity,
) (*identity.Identity, error) {
	if partiallyRenewedIdentity != nil {
		// If we were able to fetch a new identity in the last attempt, we do
		// not want to try and fetch another one as this could cause a
		// generation lock-out. So instead, we only retry the saving and
		// creation of new client.
		if err := identity.SaveIdentity(
			partiallyRenewedIdentity,
			botDestination,
			identity.BotKinds()...,
		); err != nil {
			return partiallyRenewedIdentity, trace.Wrap(err)
		}

		newClient, err := b.AuthenticatedUserClientFromIdentity(ctx, partiallyRenewedIdentity)
		if err != nil {
			return partiallyRenewedIdentity, trace.Wrap(err)
		}

		b.setClient(newClient)
		b.setIdent(partiallyRenewedIdentity)
		b.log.WithField(
			"identity", describeTLSIdentity(b.log, partiallyRenewedIdentity),
		).Debug("Bot now using new identity.")
		return nil, nil
	}

	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(botDestination); err != nil {
		return nil, trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	var newIdentity *identity.Identity
	var err error
	if b.cfg.Onboarding.RenewableJoinMethod() {
		// When using a renewable join method, we use GenerateUserCerts to
		// request a new certificate using our current identity.
		newIdentity, err = b.renewIdentityViaAuth(ctx, b.ident(), b.Client())
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// When using the non-renewable join methods, we rejoin each time rather
		// than using certificate renewal.
		newIdentity, err = b.getIdentityFromToken()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	b.log.WithField("identity", describeTLSIdentity(b.log, newIdentity)).Info("Fetched new bot identity.")
	if err := identity.SaveIdentity(newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return newIdentity, trace.Wrap(err)
	}

	newClient, err := b.AuthenticatedUserClientFromIdentity(ctx, newIdentity)
	if err != nil {
		return newIdentity, trace.Wrap(err)
	}

	b.setClient(newClient)
	b.setIdent(newIdentity)
	b.log.WithField("identity", describeTLSIdentity(b.log, newIdentity)).Debug("Bot now using new identity.")

	// We only return the identity if it's been a partial success - otherwise,
	// the new identity is propagated by `b.setIdent`
	return nil, nil
}

func (b *Bot) renewIdentityViaAuth(
	ctx context.Context,
	ident *identity.Identity,
	client auth.ClientI,
) (*identity.Identity, error) {
	b.log.Info("Fetching bot identity using existing bot identity.")
	if ident == nil || client == nil {
		return nil, trace.BadParameter("renewIdentityWithAuth must be called with non-nil client and identity")
	}
	// Ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: ident.PublicKeyBytes,
		Username:  ident.X509Cert.Subject.CommonName,
		Expires:   time.Now().Add(b.cfg.CertificateTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newIdentity, err := identity.ReadIdentityFromStore(
		ident.Params(),
		certs,
		identity.BotKinds()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func (b *Bot) getIdentityFromToken() (*identity.Identity, error) {
	b.log.Info("Fetching bot identity using token.")
	addr, err := utils.ParseAddr(b.cfg.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err, "invalid auth server address %+v", b.cfg.AuthServer)
	}

	tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
	if err != nil {
		return nil, trace.Wrap(err, "unable to generate new keypairs")
	}

	token, err := b.cfg.Onboarding.Token()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	expires := time.Now().Add(b.cfg.CertificateTTL)
	params := auth.RegisterParams{
		Token: token,
		ID: auth.IdentityID{
			Role: types.RoleBot,
		},
		AuthServers:        []utils.NetAddr{*addr},
		PublicTLSKey:       tlsPublicKey,
		PublicSSHKey:       sshPublicKey,
		CAPins:             b.cfg.Onboarding.CAPins,
		CAPath:             b.cfg.Onboarding.CAPath,
		GetHostCredentials: client.HostCredentials,
		JoinMethod:         b.cfg.Onboarding.JoinMethod,
		Expires:            &expires,
		FIPS:               b.cfg.FIPS,
		CipherSuites:       b.cfg.CipherSuites(),
	}
	if params.JoinMethod == types.JoinMethodAzure {
		params.AzureParams = auth.AzureParams{
			ClientID: b.cfg.Onboarding.Azure.ClientID,
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
	}, certs, identity.BotKinds()...)
	return ident, trace.Wrap(err)
}
