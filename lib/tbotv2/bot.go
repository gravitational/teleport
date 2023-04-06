package tbotv2

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"time"
)

func NewBot(cfg Config, logger logrus.FieldLogger) *Bot {
	return &Bot{
		cfg:    cfg,
		logger: logger,
	}
}

type Bot struct {
	// TODO: Mutex auth/currentIdentity
	// TODO: Future: auth.ClientI with support for dynamic credentials.
	auth            auth.ClientI
	currentIdentity *identity.Identity
	logger          logrus.FieldLogger
	cfg             Config
}

func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Bot starting")

	b.logger.Info("Establishing bot identity")
	// TODO: Joining/bot identity renewal.
	// Ugly current hack to steal identity from another bot for now.
	b.logger.Info("Stealing identity from disk")
	ident, err := identity.LoadIdentity(b.cfg.Store, identity.BotKinds()...)
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := b.ClientForIdentity(ctx, ident)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()
	b.logger.Info("Successfully stole identity.")
	b.auth = client
	b.currentIdentity = ident

	// If one-shot, fire off hard-coded destinations.
	// In one-shot mode, we do not require the CA rotation watcher or the
	// bot identity renewer.
	if b.cfg.Oneshot {
		b.logger.Info("Oneshot mode enabled.")
		for _, dest := range b.cfg.Destinations {
			b.logger.WithField("destination", dest.String()).Info("Running destination")
			err := dest.Oneshot(ctx, b)
			if err != nil {
				return err
			}
		}
		b.logger.Info("Oneshots complete. Exiting.")
		return nil
	}

	// Set up CA watcher and bot identity renewer if not running on oneshot.
	b.logger.Info("Watching for CA rotations")
	// TODO: Actually watch for ca rotations
	b.logger.Info("Starting bot identity renewer")
	// TODO: Actually start bot identity renewer.

	// Wait for bot identity and CA rotation mechanisms to be happy
	b.logger.Info("Waiting for CA rotation watcher and bot identity to be healthy")

	// TODO: Emit readiness signal.
	b.logger.Info("CA rotation watcher and bot identity healthy")

	// TODO: Handle management of goroutines and synced closure/error states.
	block := make(chan struct{})
	go func() {
		for _, dest := range b.cfg.Destinations {
			b.logger.Info("Starting destination.")
			// TODO: Handle destination failing out?
			go dest.Run(ctx, b)
		}
	}()
	<-block

	return nil
}

type IdentityRequest struct {
	roles      []string
	routeToApp proto.RouteToApp
	ttl        time.Duration
}

func (b *Bot) GenerateIdentity(ctx context.Context, req IdentityRequest) (*identity.Identity, error) {
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.roles) == 0 {
		// TODO: Fallback to bot allowed roles.
		return nil, trace.BadParameter("roles must be specified")
	}

	upstreamReq := proto.UserCertsRequest{
		PublicKey:      publicKey,
		Username:       b.currentIdentity.X509Cert.Subject.CommonName,
		Expires:        time.Now().Add(req.ttl),
		RoleRequests:   req.roles,
		RouteToCluster: b.currentIdentity.ClusterName,
		RouteToApp:     req.routeToApp,
		// Make sure to specify this is an impersonated cert request. If unset,
		// auth cannot differentiate renewable vs impersonated requests when
		// len(roleRequests) == 0.
		UseRoleRequests: true,
	}

	certs, err := b.auth.GenerateUserCerts(ctx, upstreamReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned user certs will only contain the
	// Teleport User CA. We'll also need the host CA for future API calls.
	localCA, err := b.auth.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := tlsca.ParseCertificatePEMs(localCA.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Append the host CAs from the auth server.
	for _, cert := range caCerts {
		pemBytes, err := tlsca.MarshalCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs.TLSCACerts = append(certs.TLSCACerts, pemBytes)
	}

	// Do not trust SSH CA certs as returned by GenerateUserCerts() with an
	// impersonated identity. It only returns the SSH UserCA in this context,
	// but we also need the HostCA and can't directly set `includeHostCA` as
	// part of the UserCertsRequest.
	// Instead, copy the SSHCACerts from the primary identity.
	certs.SSHCACerts = b.currentIdentity.SSHCACertBytes

	newIdentity, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKey,
		PublicKeyBytes:  publicKey,
	}, certs, identity.DestinationKinds()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func (b *Bot) ListenForRotation(ctx context.Context) (chan struct{}, func(), error) {
	// TODO: Actually build in a CA rotation watcher
	ch := make(chan struct{})
	f := func() {
		close(ch)
	}
	return ch, f, nil
}

func (b *Bot) ClientForIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error) {
	if id.SSHCert == nil || id.X509Cert == nil {
		return nil, trace.BadParameter("auth client requires a fully formed identity")
	}

	tlsConfig, err := id.TLSConfig(utils.DefaultCipherSuites())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := id.SSHClientConfig(false)
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
		Log:         b.logger,
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}
