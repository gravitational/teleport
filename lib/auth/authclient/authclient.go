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
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
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
	// DialTimeout determines how long to wait for dialing to succeed before aborting.
	DialTimeout time.Duration
}

// Connect creates a valid client connection to the auth service.  It may
// connect directly to the auth server, or tunnel through the proxy.
func Connect(ctx context.Context, cfg *Config) (auth.ClientI, error) {
	cfg.Log.Debugf("Connecting to: %v.", cfg.AuthServers)

	directClient, err := connectViaAuthDirect(cfg)
	if err == nil {
		return directClient, nil
	}
	directErr := trace.Wrap(err, "failed direct dial to auth server: %v", err)

	// If it fails, we now want to try tunneling to the auth server through a
	// proxy, we can only do this with SSH credentials.
	if cfg.SSH == nil {
		return nil, trace.Wrap(directErr)
	}
	proxyTunnelClient, err := connectViaProxyTunnel(ctx, cfg)
	if err == nil {
		return proxyTunnelClient, nil
	}
	proxyTunnelErr := trace.Wrap(err, "failed dial to auth server through reverse tunnel: %v", err)

	return nil, trace.NewAggregate(
		directErr,
		proxyTunnelErr,
	)
}

func connectViaAuthDirect(cfg *Config) (auth.ClientI, error) {
	// Try connecting to the auth server directly over TLS.
	directDialClient, err := auth.NewClient(apiclient.Config{
		Addrs: utils.NetAddrsToStrings(cfg.AuthServers),
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(cfg.TLS),
		},
		CircuitBreakerConfig:     cfg.CircuitBreakerConfig,
		InsecureAddressDiscovery: cfg.TLS.InsecureSkipVerify,
		DialTimeout:              cfg.DialTimeout,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check connectivity by calling something on the client.
	if _, err := directDialClient.GetClusterName(); err != nil {
		// This client didn't work for us, so we close it.
		_ = directDialClient.Close()
		return nil, trace.Wrap(err)

	}
	return directDialClient, nil
}

func connectViaProxyTunnel(ctx context.Context, cfg *Config) (auth.ClientI, error) {
	// If direct dial failed, we may have a proxy address in
	// cfg.AuthServers. Try connecting to the reverse tunnel
	// endpoint and make a client over that.
	//
	// TODO(nic): this logic should be implemented once and reused in IoT
	// nodes.
	resolver := reversetunnel.WebClientResolver(&webclient.Config{
		Context:   ctx,
		ProxyAddr: cfg.AuthServers[0].String(),
		Insecure:  cfg.TLS.InsecureSkipVerify,
		Timeout:   cfg.DialTimeout,
	})

	resolver, err := reversetunnel.CachingResolver(ctx, resolver, nil /* clock */)
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
		ClusterCAs:            cfg.TLS.RootCAs,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tunnelClient, err := auth.NewClient(apiclient.Config{
		Dialer: dialer,
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(cfg.TLS),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Check connectivity by calling something on the client.
	if _, err := tunnelClient.GetClusterName(); err != nil {
		// This client didn't work for us, so we close it.
		_ = tunnelClient.Close()
		return nil, trace.Wrap(err)
	}
	return tunnelClient, nil
}
