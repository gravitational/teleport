/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package authclient contains common code for creating an auth server client
// which may use SSH tunneling through a proxy.
package authclient

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/teleport/api/breaker"
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
	// AuthServers is a list of possible auth or proxy server addresses.
	AuthServers []utils.NetAddr
	// Log sets the logger for the client to use.
	Log logrus.FieldLogger
	// CircuitBreakerConfig is the configuration for the auth client circuit breaker.
	CircuitBreakerConfig breaker.Config
}

// Connect creates a valid client connection to the auth service.  It may
// connect directly to the auth server, or tunnel through the proxy.
func Connect(ctx context.Context, cfg *Config) (auth.ClientI, error) {
	cfg.Log.Debugf("Connecting to: %v.", cfg.AuthServers)

	// Try connecting to the auth server directly over TLS.
	client, err := auth.NewClient(apiclient.Config{
		Addrs: utils.NetAddrsToStrings(cfg.AuthServers),
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(cfg.TLS),
		},
		CircuitBreakerConfig: cfg.CircuitBreakerConfig,
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

		resolver := reversetunnel.WebClientResolver(cfg.AuthServers, lib.IsInsecureDevMode())
		resolver, err = reversetunnel.CachingResolver(ctx, resolver, nil /* clock */)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// reversetunnel.TunnelAuthDialer will take care of creating a net.Conn
		// within an SSH tunnel.
		dialer, err := reversetunnel.NewTunnelAuthDialer(reversetunnel.TunnelAuthDialerConfig{
			Resolver:              resolver,
			ClientConfig:          cfg.SSH,
			Log:                   cfg.Log,
			InsecureSkipTLSVerify: cfg.TLS.InsecureSkipVerify,
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
