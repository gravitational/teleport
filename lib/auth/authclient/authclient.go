package authclient

import (
	"context"
	"crypto/tls"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// Config holds configuration parameters for connecting to the auth service.
type Config struct {
	// TLS holds credentials for mTLS.
	TLS *tls.Config
	// SSH is client SSH config.
	SSH *ssh.ClientConfig
	// AuthAddrs is a list of possible auth or proxy server addresses.
	AuthServers []utils.NetAddr
	// Log sets the logger for the client to use.
	Log logrus.FieldLogger
}

// Connect creates a valid client connection to the auth service.  It may
// connect directly to the auth server, or tunnel through the proxy.
func Connect(ctx context.Context, cfg *Config) (auth.ClientI, error) {
	cfg.Log.Debugf("Connecting to auth servers: %v.", cfg.AuthServers)

	// Try connecting to the auth server directly over TLS.
	client, err := auth.NewClient(apiclient.Config{
		Addrs: utils.NetAddrsToStrings(cfg.AuthServers),
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(cfg.TLS),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed direct dial to auth server: %v", err)
	}

	// Check connectivity by calling something on the client.
	_, err = client.GetClusterName()
	if err != nil {
		directDialErr := trace.Wrap(err, "failed direct dial to auth server: %v", err)
		if cfg.SSH == nil {
			// No identity file was provided, don't try dialing via a reverse
			// tunnel on the proxy.
			return nil, trace.Wrap(directDialErr)
		}

		// If direct dial failed, we may have a proxy address in
		// cfg.AuthServers. Try connecting to the reverse tunnel
		// endpoint and make a client over that.
		//
		// TODO(nic): this logic should be implemented once and reused in IoT
		// nodes.

		resolver := reversetunnel.WebClientResolver(ctx, cfg.AuthServers, lib.IsInsecureDevMode())
		resolver, err = reversetunnel.CachingResolver(resolver, nil /* clock */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// reversetunnel.TunnelAuthDialer will take care of creating a net.Conn
		// within an SSH tunnel.
		dialer, err := reversetunnel.NewTunnelAuthDialer(reversetunnel.TunnelAuthDialerConfig{
			Resolver:     resolver,
			ClientConfig: cfg.SSH,
			Log:          cfg.Log,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		client, err = auth.NewClient(apiclient.Config{
			Dialer: dialer,
			Credentials: []apiclient.Credentials{
				apiclient.LoadTLS(cfg.TLS),
			},
		})
		if err != nil {
			tunnelClientErr := trace.Wrap(err, "failed dial to auth server through reverse tunnel: %v", err)
			return nil, trace.NewAggregate(directDialErr, tunnelClientErr)
		}
		// Check connectivity by calling something on the client.
		if _, err := client.GetClusterName(); err != nil {
			tunnelClientErr := trace.Wrap(err, "failed dial to auth server through reverse tunnel: %v", err)
			return nil, trace.NewAggregate(directDialErr, tunnelClientErr)
		}
	}
	return client, nil
}
