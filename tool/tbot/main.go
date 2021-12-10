package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/teleport"
	api "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/renew"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTSH,
})

const (
	authServerEnvVar  = "TELEPORT_AUTH_SERVER"
	clusterNameEnvVar = "TELEPORT_CLUSTER_NAME"
	tokenEnvVar       = "TELEPORT_BOT_TOKEN"
)

// TODO: need to store the bot's host ID and the name of the cluster
// we're connecting to - should we just dump that in the store?

type CLIConf struct {
	Debug       bool
	AuthServer  string
	ClusterName string
	DataDir     string
	// CAPins is a list of pinned SKPI hashes of trusted auth server CAs, used
	// only on first connect.
	CAPins []string
	// CAPath is the path to the auth server CA certificate, if available. Used
	// only on first connect.
	CAPath string

	ProxyServer string

	Token string
	// RenewInterval is the interval at which certificates are renewed, as a
	// time.ParseDuration() string. It must be less than the certificate TTL.
	RenewInterval time.Duration
	// CertificateTTL is the requested TTL of certificates. It should be some
	// multiple of the renewal interval to allow for failed renewals.
	CertificateTTL time.Duration
}

func main() {
	if err := Run(os.Args[1:]); err != nil {
		utils.FatalError(err)
		trace.DebugReport(err)
	}
}

func Run(args []string) error {
	var cf CLIConf
	utils.InitLogger(utils.LoggingForDaemon, logrus.InfoLevel)

	app := utils.InitCLIParser("tbot", "tbot: Teleport Credential Bot").Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout").Short('d').BoolVar(&cf.Debug)

	startCmd := app.Command("start", "Starts the renewal bot, writing certificates to the data dir at a set interval.")
	startCmd.Flag("auth-server", "Specify the Teleport auth server host").Short('a').Envar(authServerEnvVar).Required().StringVar(&cf.AuthServer)
	startCmd.Flag("token", "A bot join token, if attempting to onboard a new bot; used on first connect.").Envar(tokenEnvVar).StringVar(&cf.Token)
	startCmd.Flag("ca-pin", "A repeatable auth server CA hash to pin; used on first connect.").StringsVar(&cf.CAPins)
	startCmd.Flag("data-dir", "Directory in which to write certificate files.").Required().StringVar(&cf.DataDir)
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

	log.Debugf("args: %+v", cf)

	switch command {
	case startCmd.FullCommand():
		err = onStart(&cf)
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.BadParameter("command %q not configured", command)
	}

	return err
}

func onStart(cf *CLIConf) error {
	// TODO: for now, destination is always dir
	dest, err := renew.NewDestination(&renew.DestinationSpec{
		Type:     renew.DestinationDir,
		Location: cf.DataDir,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	addr, err := utils.ParseAddr(cf.AuthServer)
	if err != nil {
		return trace.WrapWithMessage(err, "invalid auth server address %+v", cf.AuthServer)
	}

	var authClient *auth.Client

	// First, attempt to load an identity from the given destination
	ident, err := renew.LoadIdentity(dest)
	if err == nil {
		log.Infof("succesfully loaded identity %+v", ident)

		// TODO: we should cache the token; if --token is provided but
		// different, assume the user is attempting to start with a new
		// identity
		if cf.Token != "" {
			log.Warn("note: --token and --ca-pins ignored as identity was loaded from persistent storage")
		}

		authClient, err = authenticatedUserClientFromIdentity(ident, cf.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// If the identity can't be loaded, assume we're starting fresh and
		// need to generate our initial identity from a token

		// TODO: validate that errors from LoadIdentity are sanely typed; we
		// actually only want to ignore NotFound errors

		// If no token is present, we can't continue.
		if cf.Token == "" {
			return trace.Errorf("unable to start: no identity could be loaded and no token present")
		}

		tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
		if err != nil {
			return trace.WrapWithMessage(err, "unable to generate new keypairs")
		}

		params := RegisterParams{
			Token:        cf.Token,
			Servers:      []utils.NetAddr{*addr}, // TODO: multiple servers?
			PrivateKey:   tlsPrivateKey,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
			CipherSuites: utils.DefaultCipherSuites(),
			CAPins:       cf.CAPins,
			CAPath:       cf.CAPath,
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
		authClient, err = authenticatedUserClientFromIdentity(ident, cf.AuthServer)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Infof("storing new identity to destination: %+v", dest)
		if err := renew.SaveIdentity(ident, dest); err != nil {
			return trace.WrapWithMessage(err, "unable to save generated identity back to destination")
		}
	}

	log.Infof("Certificate is valid for principals: %+v", ident.Cert.ValidPrincipals)

	// TODO: handle cases where an identity exists on disk but we might _not_
	// want to use it:
	//  - identity has expired
	//  - user provided a new token
	//  - ???

	// TODO: these auth api calls require an authenticated user so we can't
	// store them in the identity, at least initially; for now we'll just
	// reuse/abuse the hard-coded data dir. Unfortunately the SSH config file
	// requires path references to other values so the lack of exported
	// filesystem paths in our Destination impl is a problem.
	if err := writeSSHConfig(authClient, cf.DataDir, ident.Cert.ValidPrincipals); err != nil {
		return trace.Wrap(err)
	}

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

	return renewLoop(authClient, ident, cf.AuthServer, dest, cf.CertificateTTL, cf.RenewInterval)
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
func newIdentityViaAuth(params RegisterParams) (*renew.Identity, error) {
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

	// Append any additional root CAs we receieved as part of the auth process
	// (i.e. the host CA cert)
	for _, cert := range rootCAs {
		pemBytes, err := tlsca.MarshalCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs.TLSCACerts = append(certs.TLSCACerts, pemBytes)
	}

	return renew.ReadIdentityFromKeyPair(params.PrivateKey, params.PublicSSHKey, certs)
}

func writeSSHConfig(client *auth.Client, dataDir string, validPrincipals []string) error {
	var (
		proxyHosts     []string
		firstProxyHost string
		firstProxyPort string
	)

	clusterName, err := client.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	proxies, err := client.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	for i, proxy := range proxies {
		host, _, err := utils.SplitHostPort(proxy.GetPublicAddr())
		if err != nil {
			log.Debugf("proxy %+v has no usable public address", proxy)
			continue
		}

		if i == 0 {
			firstProxyHost = host
			firstProxyPort = "3023" // TODO: need to resolve correct port somehow
		}

		proxyHosts = append(proxyHosts, host)
	}

	if len(proxyHosts) == 0 {
		return trace.BadParameter("auth server has no proxies with a valid public address")
	}

	proxyHostStr := strings.Join(proxyHosts, ",")

	knownHosts, err := fetchKnownHosts(client, clusterName.GetClusterName(), proxyHostStr)
	if err != nil {
		return trace.Wrap(err)
	}

	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return trace.Wrap(err)
	}

	knownHostsPath := filepath.Join(dataDir, "known_hosts")
	if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0600); err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Wrote known hosts configuration to %s", knownHostsPath)

	var sshConfigBuilder strings.Builder
	identityFilePath := filepath.Join(dataDir, renew.PrivateKeyKey)
	certificateFilePath := filepath.Join(dataDir, renew.SSHCertKey)
	sshConfigPath := filepath.Join(dataDir, "ssh_config")
	if err := sshConfigTemplate.Execute(&sshConfigBuilder, sshConfigParameters{
		ClusterName:         clusterName.GetClusterName(),
		ProxyHost:           firstProxyHost,
		ProxyPort:           firstProxyPort,
		KnownHostsPath:      knownHostsPath,
		IdentityFilePath:    identityFilePath,
		CertificateFilePath: certificateFilePath,
		SSHConfigPath:       sshConfigPath,
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(sshConfigPath, []byte(sshConfigBuilder.String()), 0600); err != nil {
		return trace.Wrap(err)
	}

	var principals string
	switch len(validPrincipals) {
	case 0:
		principals = "[user]"
	case 1:
		principals = validPrincipals[0]
	default:
		principals = fmt.Sprintf("[%s]", strings.Join(validPrincipals, "|"))
	}

	log.Infof("Wrote SSH configuration to %s", sshConfigPath)
	fmt.Printf("\nSSH usage example: ssh -F %s %s@[node].%s\n\n", sshConfigPath, principals, clusterName.GetClusterName())

	return nil
}

func fetchKnownHosts(client *auth.Client, clusterName, proxyHosts string) (string, error) {
	ca, err := client.GetCertAuthority(types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var sb strings.Builder
	for _, auth := range auth.AuthoritiesToTrustedCerts([]types.CertAuthority{ca}) {
		pubKeys, err := auth.SSHCertPublicKeys()
		if err != nil {
			return "", trace.Wrap(err)
		}

		for _, pubKey := range pubKeys {
			bytes := ssh.MarshalAuthorizedKey(pubKey)
			sb.WriteString(fmt.Sprintf(
				"@cert-authority %s,%s,*.%s %s type=host",
				proxyHosts, auth.ClusterName, auth.ClusterName, strings.TrimSpace(string(bytes)),
			))
		}
	}

	return sb.String(), nil
}

func renewIdentityViaAuth(
	client *auth.Client,
	currentIdentity *renew.Identity,
	certTTL time.Duration,
) (*renew.Identity, error) {
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

	newIdentity, err := renew.ReadIdentityFromKeyPair(
		currentIdentity.KeyBytes,
		currentIdentity.SSHPublicKeyBytes,
		certs,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func renewLoop(
	client *auth.Client,
	identity *renew.Identity,
	authServer string,
	dest renew.Destination,
	certTTL time.Duration,
	renewInterval time.Duration,
) error {
	// TODO: failures here should probably not just end the renewal loop, there
	// should be some retry / back-off logic.

	// TODO: what should this interval be? should it be user configurable?
	// Also, must be < the validity period.

	log.Infof("Beginning renewal loop: ttl=%s interval=%s", certTTL, renewInterval)
	if renewInterval > certTTL {
		log.Errorf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). The next renewal is likely to fail.",
			certTTL,
			renewInterval,
		)
	}

	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()
	for {
		log.Info("Attempting to renew certificates...")
		newIdentity, err := renewIdentityViaAuth(client, identity, certTTL)
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
		newClient, err := authenticatedUserClientFromIdentity(newIdentity, authServer)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Info("Auth client now using renewed credentials.")
		client = newClient
		identity = newIdentity

		// Now that we're sure the new creds work, persist them.
		if err := renew.SaveIdentity(newIdentity, dest); err != nil {
			return trace.Wrap(err)
		}

		log.Infof("Persisted new certificates to disk. Next renewal in approximately %s", renewInterval)
		<-ticker.C
	}
}

func authenticatedUserClientFromIdentity(id *renew.Identity, authServer string) (*auth.Client, error) {
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

var sshConfigTemplate = template.Must(template.New("ssh-config").Parse(`
# Begin generated Teleport configuration for {{ .ProxyHost }} from tbot config

# Common flags for all {{ .ClusterName }} hosts
Host *.{{ .ClusterName }} {{ .ProxyHost }}
    UserKnownHostsFile "{{ .KnownHostsPath }}"
    IdentityFile "{{ .IdentityFilePath }}"
    CertificateFile "{{ .CertificateFilePath }}"
    HostKeyAlgorithms ssh-rsa-cert-v01@openssh.com
    PubkeyAcceptedAlgorithms +ssh-rsa-cert-v01@openssh.com

# Flags for all {{ .ClusterName }} hosts except the proxy
Host *.{{ .ClusterName }} !{{ .ProxyHost }}
    Port 3022
    ProxyCommand ssh -F {{ .SSHConfigPath }} -l %r -p {{ .ProxyPort }} {{ .ProxyHost }} -s proxy:%h:%p@{{ .ClusterName }}

# End generated Teleport configuration
`))

type sshConfigParameters struct {
	ClusterName         string
	KnownHostsPath      string
	IdentityFilePath    string
	CertificateFilePath string
	ProxyHost           string
	ProxyPort           string
	SSHConfigPath       string
}

// func mainHostCerts() error {
// 	addr := utils.MustParseAddr(authServer)

// 	ds, err := renew.ParseDestinationSpec(dest)
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	store, err := renew.NewDestination(ds)
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	id, err := renew.LoadIdentity(store)
// 	if err != nil {
// 		log.Println("could not load identity, starting new registration", err)
// 		privateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
// 		if err != nil {
// 			return trace.Wrap(err)
// 		}
// 		hostID := uuid.New().String()
// 		id, err = auth.Register(auth.RegisterParams{
// 			Token: token,
// 			ID: auth.IdentityID{
// 				Role:     types.RoleNode,
// 				HostUUID: hostID,
// 				NodeName: nodeName,
// 			},
// 			Servers: []utils.NetAddr{*addr},
// 			CAPins:  []string{}, // TODO

// 			DNSNames:             nil,
// 			AdditionalPrincipals: nil,

// 			GetHostCredentials: client.HostCredentials,

// 			PrivateKey:   privateKey,
// 			PublicTLSKey: tlsPublicKey,
// 			PublicSSHKey: sshPublicKey,
// 		})
// 		if err != nil {
// 			return trace.WrapWithMessage(err, "could not register")
// 		}

// 		log.Println("registered with auth server, saving certs to disk!")

// 		if err := renew.SaveIdentity(id, store); err != nil {
// 			return trace.Wrap(err)
// 		}
// 	} else {
// 		// TODO: handle case where these certs are too old..
// 		log.Println("connecting to auth server with existing certificates")
// 	}

// 	tc, err := id.TLSConfig(nil)
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	client, err := api.New(context.Background(), api.Config{
// 		Addrs:                    []string{authServer},
// 		Credentials:              []api.Credentials{api.LoadTLS(tc)},
// 		InsecureAddressDiscovery: true,
// 	})
// 	if err != nil {
// 		return trace.Wrap(err)
// 	}

// 	if err := startServiceHeartbeat(client, id.ID.HostUUID); err != nil {
// 		return trace.Wrap(err)
// 	}

// 	// log.Println("generating user certs")
// 	// userCerts, err := client.GenerateUserCerts(context.Background(), proto.UserCertsRequest{
// 	// 	PublicKey: id.KeySigner.PublicKey().Marshal(),
// 	// 	Username:  "test3",
// 	// 	Expires:   time.Now().UTC().Add(4 * time.Hour),
// 	// 	Usage:     proto.UserCertsRequest_All, // TODO: allow pinning to a specific node with NodeName
// 	// })
// 	// if err != nil {
// 	// 	log.Fatalln("could not generate user certs", err)
// 	// }

// 	//log.Println("generated user certs!")
// 	//log.Println("SSH:", string(userCerts.SSH))

// 	// log.Println("waiting for signals: ^C to rotate, ^\\ to exit")
// 	// ch := make(chan os.Signal, 1)
// 	// signal.Notify(ch, os.Interrupt)

// 	// for {
// 	// 	select {
// 	// 	case <-ch:
// 	// 		log.Println("rotating due to signal")
// 	// 	}
// 	// }

// 	return nil
// }

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

// API client can't upsert core components like auth servers and proxies,
// so just nop those calls

type announcerAdapter struct{ *api.Client }

func (a announcerAdapter) UpsertAuthServer(s types.Server) error { return nil }
func (a announcerAdapter) UpsertProxy(s types.Server) error      { return nil }

// API client doesn't implement all of ClientI

type clientiAdapter struct{ *api.Client }
