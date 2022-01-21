package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/gravitational/teleport"
	api "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/kr/pretty"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

const (
	authServerEnvVar = "TELEPORT_AUTH_SERVER"
	tokenEnvVar      = "TELEPORT_BOT_TOKEN"
)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		utils.FatalError(err)
		trace.DebugReport(err)
	}
}

func Run(args []string) error {
	var cf config.CLIConf
	utils.InitLogger(utils.LoggingForDaemon, logrus.InfoLevel)

	app := utils.InitCLIParser("tbot", "tbot: Teleport Credential Bot").Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout").Short('d').BoolVar(&cf.Debug)
	app.Flag("config", "tbot.yaml path").Short('c').StringVar(&cf.ConfigPath)

	configCmd := app.Command("config", "Parse and dump a config file")

	startCmd := app.Command("start", "Starts the renewal bot, writing certificates to the data dir at a set interval.")
	startCmd.Flag("auth-server", "Specify the Teleport auth server host").Short('a').Envar(authServerEnvVar).StringVar(&cf.AuthServer)
	startCmd.Flag("token", "A bot join token, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	startCmd.Flag("ca-pin", "A repeatable auth server CA hash to pin; used on first connect.").StringsVar(&cf.CAPins)
	startCmd.Flag("data-dir", "Directory to store internal bot data.").StringVar(&cf.DataDir)
	startCmd.Flag("destination-dir", "Directory to write generated certificates").StringVar(&cf.DestinationDir)
	startCmd.Flag("certificate-ttl", "TTL of generated certificates").Default("60m").DurationVar(&cf.CertificateTTL)
	startCmd.Flag("renew-interval", "Interval at which certificates are renewed; must be less than the certificate TTL.").Default("20m").DurationVar(&cf.RenewInterval)

	command, err := app.Parse(args)
	if err != nil {
		return trace.Wrap(err)
	}

	// While in debug mode, send logs to stdout.
	if cf.Debug {
		utils.InitLogger(utils.LoggingForDaemon, logrus.DebugLevel)
	}

	botConfig, err := config.FromCLIConf(&cf)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case configCmd.FullCommand():
		err = onConfig(botConfig)
	case startCmd.FullCommand():
		err = onStart(botConfig)
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.BadParameter("command %q not configured", command)
	}

	return err
}

func onConfig(botConfig *config.BotConfig) error {
	pretty.Println(botConfig)

	return nil
}

func onStart(botConfig *config.BotConfig) error {
	// Start by loading the bot's primary destination.
	dest, err := botConfig.Storage.GetDestination()
	if err != nil {
		return trace.WrapWithMessage(err, "could not read bot storage destination from config")
	}

	addr, err := utils.ParseAddr(botConfig.AuthServer)
	if err != nil {
		return trace.WrapWithMessage(err, "invalid auth server address %+v", botConfig.AuthServer)
	}

	var authClient *auth.Client

	// First, attempt to load an identity from the given destination
	ident, err := identity.LoadIdentity(dest)
	if err == nil {
		log.Infof("successfully loaded identity %+v", ident)

		// TODO: we should cache the token; if --token is provided but
		// different, assume the user is attempting to start with a new
		// identity
		if botConfig.Onboarding != nil {
			log.Warn("note: onboard config ignored as identity was loaded from persistent storage")
		}

		authClient, err = authenticatedUserClientFromIdentity(ident, botConfig.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// If the identity can't be loaded, assume we're starting fresh and
		// need to generate our initial identity from a token

		// TODO: validate that errors from LoadIdentity are sanely typed; we
		// actually only want to ignore NotFound errors

		onboarding := botConfig.Onboarding
		if onboarding == nil {
			return trace.BadParameter("onboarding config required on first start via CLI or YAML")
		}

		// If no token is present, we can't continue.
		if onboarding.Token == "" {
			return trace.Errorf("unable to start: no identity could be loaded and no token present")
		}

		tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
		if err != nil {
			return trace.WrapWithMessage(err, "unable to generate new keypairs")
		}

		params := RegisterParams{
			Token:        onboarding.Token,
			Servers:      []utils.NetAddr{*addr}, // TODO: multiple servers?
			PrivateKey:   tlsPrivateKey,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
			CipherSuites: utils.DefaultCipherSuites(),
			CAPins:       onboarding.CAPins,
			CAPath:       onboarding.CAPath,
			Clock:        clockwork.NewRealClock(),
		}

		log.Info("attempting to generate new identity from token")
		ident, err = newIdentityViaAuth(params)
		if err != nil {
			return trace.Wrap(err)
		}

		// Attach the ssh public key.
		//ident.SSHPublicKeyBytes = sshPublicKey

		// TODO: consider `memory` dest type for testing / ephemeral use / etc

		log.Debug("attempting first connection using initial auth client")
		authClient, err = authenticatedUserClientFromIdentity(ident, botConfig.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Infof("storing new identity to destination: %+v", dest)
		if err := identity.SaveIdentity(ident, dest); err != nil {
			return trace.WrapWithMessage(err, "unable to save generated identity back to destination")
		}
	}

	log.Infof("Certificate is valid for principals: %+v", ident.Cert.ValidPrincipals)

	// TODO: handle cases where an identity exists on disk but we might _not_
	// want to use it:
	//  - identity has expired
	//  - user provided a new token
	//  - ???

	watcher, err := authClient.NewWatcher(context.Background(), types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindCertAuthority,
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go watchCARotations(watcher)

	defer watcher.Close()

	return renewLoop(botConfig, authClient, ident)
}

func watchCARotations(watcher types.Watcher) {
	for {
		select {
		case event := <-watcher.Events():
			fmt.Printf("CA event: %+v", event)
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				log.WithError(err).Warnf("error watching for CA rotations")
			}
			return
		}
	}
}

// newIdentityViaAuth contacts the auth server directly to exchange a token for
// a new set of user certificates.
func newIdentityViaAuth(params RegisterParams) (*identity.Identity, error) {
	var client *auth.Client
	var rootCAs []*x509.Certificate
	var err error

	// Build a client to the Auth Server. If a CA pin is specified require the
	// Auth Server is validated. Otherwise attempt to use the CA file on disk
	// but if it's not available connect without validating the Auth Server CA.
	switch {
	case len(params.CAPins) != 0:
		client, rootCAs, err = pinAuthClient(params)
	default:
		// TODO: need to handle an empty list of root CAs in this case. Should
		// we just trust on first connect and save the root CA?
		client, rootCAs, err = insecureAuthClient(params)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer client.Close()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Could not create an unauthenticated auth client")
	}

	// Note: GenerateInitialRenewableUserCerts will fetch _only_ the cluster's
	// user CA cert. However, to communicate with the auth server, we'll also
	// need to fetch the cluster's host CA cert, which we should have fetched
	// earlier while initializing the auth client.
	certs, err := client.GenerateInitialRenewableUserCerts(context.Background(), proto.RenewableCertsRequest{
		Token:     params.Token,
		PublicKey: params.PublicSSHKey,
	})
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Could not generate initial user certificates")
	}

	// Append any additional root CAs we received as part of the auth process
	// (i.e. the host CA cert)
	for _, cert := range rootCAs {
		pemBytes, err := tlsca.MarshalCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs.TLSCACerts = append(certs.TLSCACerts, pemBytes)
	}

	return identity.ReadIdentityFromKeyPair(params.PrivateKey, params.PublicSSHKey, certs)
}

func renewIdentityViaAuth(
	client *auth.Client,
	currentIdentity *identity.Identity,
	certTTL time.Duration,
) (*identity.Identity, error) {
	// TODO: enforce expiration > renewal period (by what margin?)

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(context.Background(), proto.UserCertsRequest{
		PublicKey: currentIdentity.SSHPublicKeyBytes,
		Username:  currentIdentity.XCert.Subject.CommonName,
		Expires:   time.Now().Add(certTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned user certs will only contain the
	// Teleport User CA. We'll also need the host CA for future API calls.
	localCA, err := client.GetClusterCACert()
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

	newIdentity, err := identity.ReadIdentityFromKeyPair(
		currentIdentity.KeyBytes,
		currentIdentity.SSHPublicKeyBytes,
		certs,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func renewLoop(cfg *config.BotConfig, client *auth.Client, ident *identity.Identity) error {
	// TODO: failures here should probably not just end the renewal loop, there
	// should be some retry / back-off logic.

	// TODO: what should this interval be? should it be user configurable?
	// Also, must be < the validity period.

	log.Infof("Beginning renewal loop: ttl=%s interval=%s", cfg.CertificateTTL, cfg.RenewInterval)
	if cfg.RenewInterval > cfg.CertificateTTL {
		log.Errorf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). The next renewal is likely to fail.",
			cfg.CertificateTTL,
			cfg.RenewInterval,
		)
	}

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	botDestination, err := cfg.Storage.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(cfg.RenewInterval)
	defer ticker.Stop()
	for {
		log.Info("Attempting to renew certificates...")
		newIdentity, err := renewIdentityViaAuth(client, ident, cfg.CertificateTTL)
		if err != nil {
			return trace.Wrap(err)
		}

		duration := time.Second * time.Duration(newIdentity.Cert.ValidBefore-newIdentity.Cert.ValidAfter)
		log.Infof(
			"Successfully fetched new certificates, now valid: after=%v, before=%v duration=%s",
			time.Unix(int64(newIdentity.Cert.ValidAfter), 0),
			time.Unix(int64(newIdentity.Cert.ValidBefore), 0),
			duration,
		)

		// TODO: warn if duration < certTTL? would indicate TTL > server's max renewable cert TTL
		// TODO: error if duration < renewalInterval? next renewal attempt will fail

		// Immediately attempt to reconnect using the new identity (still
		// haven't persisted the known-good certs).
		newClient, err := authenticatedUserClientFromIdentity(newIdentity, cfg.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Info("Auth client now using renewed credentials.")
		client = newClient
		ident = newIdentity

		// Now that we're sure the new creds work, persist them.
		if err := identity.SaveIdentity(newIdentity, botDestination); err != nil {
			return trace.Wrap(err)
		}

		// Next, generate impersonated certs
		// TODO: real role requests
		expires := time.Unix(int64(ident.Cert.ValidBefore), 0)
		for _, dest := range cfg.Destinations {
			destImpl, err := dest.GetDestination()
			if err != nil {
				return trace.Wrap(err)
			}

			impersonatedIdent, err := generateImpersonatedIdentity(client, ident, expires, dest.Roles)
			if err != nil {
				log.Warnf("could not generate impersonated certs: %+v", err)
			} else {
				log.Infof("impersonated identity: %+v", impersonatedIdent)

				if err := identity.SaveIdentity(impersonatedIdent, destImpl); err != nil {
					log.WithError(err).Warn("failed to save impersonated identity")
				}
			}

			// TODO: these auth api calls require an authenticated user so we can't
			// store them in the identity, at least initially; for now we'll just
			// reuse/abuse the hard-coded data dir. Unfortunately the SSH config file
			// requires path references to other values so the lack of exported
			// filesystem paths in our Destination impl is a problem.
			// TODO! reenable this for refactored destinations (need to at least
			// provide a unix permission hint)
			//if err := configtemplate.WriteSSHConfig(client, cf.DataDir, impersonatedIdent.Cert.ValidPrincipals); err != nil {
			//	return trace.Wrap(err)
			//}
		}

		log.Infof("Persisted new certificates to disk. Next renewal in approximately %s", cfg.RenewInterval)
		<-ticker.C
	}
}

func authenticatedUserClientFromIdentity(id *identity.Identity, authServer string) (*auth.Client, error) {
	tlsConfig, err := id.TLSConfig([]uint16{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr, err := utils.ParseAddr(authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return auth.NewClient(api.Config{
		Addrs: utils.NetAddrsToStrings([]utils.NetAddr{*addr}),
		Credentials: []api.Credentials{
			api.LoadTLS(tlsConfig),
		},
	})
}

func generateImpersonatedIdentity(
	client *auth.Client,
	currentIdentity *identity.Identity,
	expires time.Time,
	roleRequests []string,
) (*identity.Identity, error) {
	// TODO: enforce expiration > renewal period (by what margin?)

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(context.Background(), proto.UserCertsRequest{
		PublicKey:    currentIdentity.SSHPublicKeyBytes,
		Username:     currentIdentity.XCert.Subject.CommonName,
		Expires:      expires,
		RoleRequests: roleRequests,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned user certs will only contain the
	// Teleport User CA. We'll also need the host CA for future API calls.
	localCA, err := client.GetClusterCACert()
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

	newIdentity, err := identity.ReadIdentityFromKeyPair(
		currentIdentity.KeyBytes,
		currentIdentity.SSHPublicKeyBytes,
		certs,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func generateKeys() (private, sshpub, tlspub []byte, err error) {
	privateKey, publicKey, err := native.GenerateKeyPair("")
	if err != nil {
		return nil, nil, nil, err
	}

	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	return privateKey, publicKey, tlsPublicKey, nil
}
