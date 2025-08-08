package mcpproxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// STDIOProxy presents a STDIO MCP transport to a named MCP server via Teleport.
// It is designed to handle direct invocation of `tbot mcp connect` command.
// It must be provided a path to a Teleport identity file produced by tbot
// with reissue enabled, as, the Proxy will automagically reissue the identity
// as needed.
//
// Right now, Teleport MCP proxying is effectively STDIO over TCP. Eventually,
// we'd maybe need to implement code here to talk to a Streaming HTTP MCP server
// over the TCP connection on behalf of a STDIO. Or, this may end up being
// implemented in the Teleport App access handler instead and we'd just pass
// the STDIO onwards as we currently do.
//
// WARNING: This is a hacky implementation to PoC. Frankly, presenting a
// reissuable identity file to `tsh mcp connect` would probably work as well?
// Not that we prefer such UX in 2025, but, this would be a better workaround
// for a customer with an imminent use-case.
func STDIOProxy(
	ctx context.Context,
	log *slog.Logger,
	mcpServerName string,
	proxyServerAddr string,
	identityFilePath string,
) error {
	creds, err := client.NewDynamicIdentityFileCreds(identityFilePath)
	if err != nil {
		return trace.Wrap(err, "failed to create dynamic identity file credentials")
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute * 1):
				log.DebugContext(ctx, "Refreshing Teleport identity credentials")
				if err := creds.Reload(); err != nil {
					log.ErrorContext(ctx, "Failed to reload Teleport identity credentials, retrying.")
				}
			}
		}
	}()

	// Do some gnarly stuff to extract the TLS identity
	botTLSConfig, err := creds.TLSConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	botCert, err := botTLSConfig.GetClientCertificate(nil)
	if err != nil {
		return trace.Wrap(err, "failed to get certificate from TLS config")
	}
	parsedCert, err := x509.ParseCertificate(botCert.Certificate[0])
	if err != nil {
		return trace.Wrap(err, "failed to parse certificate")
	}
	identity, err := tlsca.FromSubject(
		parsedCert.Subject, parsedCert.NotAfter,
	)
	if err != nil {
		return trace.Wrap(err, "failed to parse identity from certificate")
	}

	c, err := client.New(ctx, client.Config{
		Addrs:       []string{proxyServerAddr},
		Credentials: []client.Credentials{creds},
	})
	if err != nil {
		return trace.Wrap(err, "failed to create Teleport client")
	}

	// First fetch the app so we can issue a cert
	app, err := getApp(ctx, c, mcpServerName)
	if err != nil {
		return trace.Wrap(err, "failed to get MCP app")
	}
	if !app.IsMCP() {
		return trace.BadParameter("App %q is not an MCP app", mcpServerName)
	}
	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: identity.RouteToCluster,
	}
	// Now issue a cert for the app
	// TODO: In prod, this would definitely just use lib/tbot/identity/generator
	req := proto.UserCertsRequest{
		Username:       identity.Username,
		RouteToCluster: identity.RouteToCluster,
		RouteToApp:     routeToApp,
		Expires:        identity.Expires,
	}
	key, err := cryptosuites.GenerateKey(
		ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(c),
		cryptosuites.BotImpersonatedIdentity,
	)
	if err != nil {
		return trace.Wrap(err, "failed to generate key for user certs")
	}
	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return trace.Wrap(err)
	}
	req.SSHPublicKey = ssh.MarshalAuthorizedKey(sshPub)
	req.TLSPublicKey, err = keys.MarshalPublicKey(key.Public())
	if err != nil {
		return trace.Wrap(err)
	}

	certRes, err := c.GenerateUserCerts(
		ctx,
		req,
	)
	if err != nil {
		return trace.Wrap(err, "failed to generate user certs")
	}

	caCertRes, err := c.GetClusterCACert(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to get cluster CA cert")
	}
	caCerts, err := tlsca.ParseCertificatePEMs(caCertRes.TLSCA)
	if err != nil {
		return trace.Wrap(err)
	}

	certPool := x509.NewCertPool()
	for _, caCert := range caCerts {
		certPool.AddCert(caCert)

	}
	parsedAppCert, err := tlsca.ParseCertificatePEM(certRes.TLS)
	if err != nil {
		return trace.Wrap(err, "failed to parse app certificate")
	}

	// TODO: This config assumes no ALPN upgrade is required.
	conn, err := dialALPNMaybePing(
		ctx,
		proxyServerAddr,
		client.ALPNDialerConfig{
			TLSConfig: &tls.Config{
				NextProtos: []string{string(common.ProtocolMCP)},
				Certificates: []tls.Certificate{
					{
						PrivateKey:  key,
						Certificate: [][]byte{parsedAppCert.Raw},
					},
				},
			},
		},
	)
	if err != nil {
		return trace.Wrap(err, "failed to dial ALPN connection")
	}
	slog.Info("Proxying conn")
	return trace.Wrap(utils.ProxyConn(ctx, utils.CombinedStdio{}, conn))
}

func dialALPNMaybePing(ctx context.Context, addr string, cfg client.ALPNDialerConfig) (net.Conn, error) {
	tlsConn, err := client.DialALPN(ctx, addr, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if common.IsPingProtocol(common.Protocol(tlsConn.ConnectionState().NegotiatedProtocol)) {
		return pingconn.NewTLS(tlsConn), nil
	}
	return tlsConn, nil
}
