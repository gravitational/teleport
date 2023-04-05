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

type Destination struct {
	TTL   time.Duration
	Renew time.Duration
	Path  string
}

type Config struct {
	AuthServer string
	Oneshot    bool
	Dir        string

	// For the bots own identity rather than produced certs
	TTL   time.Duration
	Renew time.Duration
}

func NewBot(cfg Config, logger logrus.FieldLogger) *Bot {
	return &Bot{
		cfg:    cfg,
		logger: logger,
	}
}

type Bot struct {
	auth            auth.ClientI
	currentIdentity *identity.Identity
	logger          logrus.FieldLogger
	cfg             Config
	destinations    []*destWrapper
}

func (b *Bot) Run(ctx context.Context) error {
	b.logger.Info("Bot starting")

	// TODO: Joining/bot identity renewal.
	// Ugly current hack to steal identity from another bot for now.
	botStore := &DirectoryStore{
		Dir: b.cfg.Dir,
	}
	ident, err := identity.LoadIdentity(botStore, identity.BotKinds()...)
	if err != nil {
		return err
	}
	client, err := b.ClientForIdentity(ctx, ident)
	b.auth = client
	b.currentIdentity = ident

	// Set up CA watcher
	if !b.cfg.Oneshot {
		b.logger.Info("Watching for CA rotations")
		// TODO: Actually watch for ca rotations
	}

	// Convert configured destinations into wrapped and active destinations
	// that can be called.
	b.destinations = []*destWrapper{
		{
			bot: b,
			store: &DirectoryStore{
				Dir: "./identity-out",
			},
			destination: &IdentityDestination{},
			TTL:         10 * time.Minute,
			Roles:       []string{"access"},
		},
		{
			bot: b,
			store: &DirectoryStore{
				Dir: "./app-out",
			},
			destination: &ApplicationDestination{
				AppName: "httpbin",
			},
			TTL:   10 * time.Minute,
			Roles: []string{"access"},
		},
	}

	// If one-shot, fire off hard-coded destinations
	if b.cfg.Oneshot {
		for _, dest := range b.destinations {
			err := dest.Oneshot(ctx)
			if err != nil {
				return err
			}
		}
		b.logger.Info("Oneshot complete. Exiting.")
		return nil
	}

	// If not one-shot, spin up sockets and destinations.
	// TODO: Handle management of goroutines and synced closure/error states.
	block := make(chan struct{})
	go func() {
		for _, dest := range b.destinations {
			// TODO: Handle destination failing out?
			go dest.Run(ctx)
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
