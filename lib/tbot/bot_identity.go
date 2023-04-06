package tbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"time"
)

func (b *Bot) renewBotIdentityLoop(ctx context.Context) error {
	b.log.Infof(
		"Beginning bot's identity renewal loop: ttl=%s interval=%s",
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

	return trace.Wrap(b.renewBotIdentity(ctx, botDestination))
}

func (b *Bot) renewBotIdentity(ctx context.Context, botDestination bot.Destination) error {
	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(botDestination); err != nil {
		return trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	b.log.Debug("Attempting to renew bot's internal identity...")

	newIdentity, err := b.renewIdentityViaAuth(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	identStr, err := describeTLSIdentity(b.ident())
	if err != nil {
		return trace.Wrap(err, "Could not describe bot's internal identity at %s", botDestination)
	}

	b.log.Infof("Successfully renewed bots certificates, %s", identStr)

	// TODO: warn if duration < certTTL? would indicate TTL > server's max renewable cert TTL
	// TODO: error if duration < renewalInterval? next renewal attempt will fail

	// Immediately attempt to reconnect using the new identity (still
	// haven't persisted the known-good certs).
	newClient, err := b.AuthenticatedUserClientFromIdentity(ctx, newIdentity)
	if err != nil {
		return trace.Wrap(err)
	}

	b.setClient(newClient)
	b.setIdent(newIdentity)
	b.log.Debug("Auth client now using renewed credentials.")

	// Now that we're sure the new creds work, persist them.
	if err := identity.SaveIdentity(newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err)
	}

	// Next, generate impersonated certs
	expires := newIdentity.X509Cert.NotAfter
}

func (b *Bot) renewIdentityViaAuth(
	ctx context.Context,
) (*identity.Identity, error) {
	// If using the IAM join method we always go through the initial join flow
	// and fetch new nonrenewable certs
	var joinMethod types.JoinMethod
	if b.cfg.Onboarding != nil {
		joinMethod = b.cfg.Onboarding.JoinMethod
	}
	switch joinMethod {
	// When using join methods that are repeatable - renewDestinations fully rather than
	// renewing using existing credentials.
	case types.JoinMethodAzure,
		types.JoinMethodCircleCI,
		types.JoinMethodGitHub,
		types.JoinMethodGitLab,
		types.JoinMethodIAM:
		ident, err := b.getIdentityFromToken()
		return ident, trace.Wrap(err)
	default:
	}

	// Ask the auth server to generate a new set of certs with a new
	// expiration date.
	ident := b.ident()
	certs, err := b.Client().GenerateUserCerts(ctx, proto.UserCertsRequest{
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
	if b.cfg.Onboarding == nil {
		return nil, trace.BadParameter("onboarding config required via CLI or YAML")
	}
	if !b.cfg.Onboarding.HasToken() {
		return nil, trace.BadParameter("unable to start: no token present")
	}
	addr, err := utils.ParseAddr(b.cfg.AuthServer)
	if err != nil {
		return nil, trace.Wrap(err, "invalid auth server address %+v", b.cfg.AuthServer)
	}

	tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
	if err != nil {
		return nil, trace.Wrap(err, "unable to generate new keypairs")
	}

	b.log.Info("Attempting to generate new identity from token")

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
