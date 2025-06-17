/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package client wraps the api/client package to implement tbot's connection
// logic.
//
// tbot supports two methods of dialing a gRPC connection to an auth server:
//
//  1. Via a direct TLS connection to the auth server itself
//  2. Via an SSH tunnel through a proxy server
//
// Both methods support client-side HTTP and SOCKS5 proxies, and method #2
// supports "TLS Routing" (via ALPN) and our "websocket upgrade" trick for
// traversing L7 load balancers that break ALPN.
//
// This is a subset of the methods offered by the api/client package. In the
// future, we may wish to add support for more of these methods, but many are
// unnecessarily complex for tbot's use-cases.
//
// If you start tbot with the `proxy_server` config option or `--proxy-server`
// flag, client.New will return a gRPC client *without* testing the connection.
//
// In other words: client.New will not return an error if the proxy or auth
// server is down, which is desirable because it allows us to start tbot in a
// "degraded" mode and retry the connection in the background.
//
// In previous versions of tbot, there was no way to explicitly provide the
// address of a proxy server. Instead, you could either put a proxy or auth
// server address in the `auth_server` option or `--auth-server` flag and tbot
// would try both connection methods.
//
// For backward compatibility, if you provide `auth_server` or `--auth-server`
// in major versions earlier than v19, client.New will test the connection and
// return an error if the proxy or auth server is unavailable. So in order for
// tbot to run in degraded mode, you must use `proxy_server` or `--proxy-server`.
//
// As tbot is also embedded in our Kubernetes operator and `tctl terraform env`
// which both support providing an auth server or proxy address using the same
// field, this package allows you to opt-in to the previous behavior by setting
// Config.AuthServerAddressMode to AllowProxyAsAuthServer.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// Address contains an address string tagged with whether it belongs to a proxy
// or auth server.
type Address struct {
	// Addr contains the address string.
	Addr string

	// Kind of address (i.e. auth server or proxy).
	Kind config.AddressKind
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return fmt.Sprintf("%s: %s", a.Kind, a.Addr)
}

// Validate the address string and kind.
func (a Address) Validate() error {
	if a.Addr == "" {
		return trace.BadParameter("address is required")
	}

	switch a.Kind {
	case config.AddressKindProxy, config.AddressKindAuth:
		return nil
	default:
		return trace.BadParameter("unsupported address type: %s", a.Kind)
	}
}

// Identity provides the TLS and SSH credentials required to dial a connection.
type Identity interface {
	// TLSConfig returns the bot's TLS configuration.
	TLSConfig() (*tls.Config, error)

	// SSHClientConfig returns the bot's SSH client configuration.
	SSHClientConfig() (*ssh.ClientConfig, error)
}

// Config contains options used to create the API client.
type Config struct {
	// Address that will be dialed to create the client connection.
	Address Address

	// AuthServerAddressMode controls the behavior when a proxy address is
	// given as an auth server address.
	AuthServerAddressMode config.AuthServerAddressMode

	// Identity that will provide the TLS and SSH credentials.
	Identity Identity

	// Resolver that will be used to find the address of a proxy server.
	Resolver reversetunnelclient.Resolver

	// Logger to which log messages will be written.
	Logger *slog.Logger

	// Insecure controls whether we will skip TLS host verification.
	Insecure bool

	// Metrics will record gRPC client metrics.
	Metrics *grpcprom.ClientMetrics
}

// CheckAndSetDefaults checks whether required config options have been provided
// and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Address.Validate(); err != nil {
		return err
	}
	if c.Identity == nil {
		return trace.BadParameter("identity is required")
	}
	if c.Resolver == nil {
		return trace.BadParameter("resolver is required")
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// New creates an API client. See the package documentation for more information.
func New(ctx context.Context, cfg Config) (*client.Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// If the address is known to be a proxy, dial without testing the
	// connection.
	if cfg.Address.Kind == config.AddressKindProxy {
		return dialViaProxy(ctx, cfg)
	}

	// If it is known to be an auth server (i.e. we do not support falling back
	// to treating it as a proxy), dial without testing the connection.
	if cfg.AuthServerAddressMode == config.AuthServerMustBeAuthServer {
		return dialDirectly(ctx, cfg)
	}

	// If the address is thought to be an auth server, try to dial it directly.
	clt, directErr := dialDirectly(ctx, cfg)
	if directErr == nil {
		// Send a ping to test the connection.
		if _, directErr = clt.Ping(ctx); directErr == nil {
			return clt, nil
		} else {
			_ = clt.Close()
		}
	}

	// If the direct connection failed, try to connect to it as a proxy.
	clt, proxyErr := dialViaProxy(ctx, cfg)
	if proxyErr == nil {
		// Send a ping to test the connection.
		if _, proxyErr = clt.Ping(ctx); proxyErr == nil {
			if cfg.AuthServerAddressMode == config.WarnIfAuthServerIsProxy {
				cfg.Logger.WarnContext(ctx,
					"Support for providing a proxy address via the 'auth_server' configuration option or '--auth-server' flag is deprecated and will be removed in v19. Use 'proxy_server' or '--proxy-server' instead.",
				)
			}
			return clt, nil
		} else {
			_ = clt.Close()
		}
	}

	return nil, trace.NewAggregate(
		trace.Wrap(directErr, "failed direct dial to auth server"),
		trace.Wrap(proxyErr, "failed dial to auth server through reverse tunnel"),
	)
}

func dialViaProxy(ctx context.Context, cfg Config) (*client.Client, error) {
	tlsConfig, err := cfg.Identity.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := cfg.Identity.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer, err := reversetunnelclient.NewTunnelAuthDialer(reversetunnelclient.TunnelAuthDialerConfig{
		Resolver:              cfg.Resolver,
		ClientConfig:          sshConfig,
		Log:                   cfg.Logger,
		InsecureSkipTLSVerify: cfg.Insecure,
		GetClusterCAs:         client.ClusterCAsFromCertPool(tlsConfig.RootCAs),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.New(ctx, client.Config{
		DialInBackground: true,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		Dialer:                   dialer,
		DialOpts:                 dialOpts(cfg),
		CircuitBreakerConfig:     circuitBreakerConfig(),
		InsecureAddressDiscovery: cfg.Insecure,
	})
}

func dialDirectly(ctx context.Context, cfg Config) (*client.Client, error) {
	tlsConfig, err := cfg.Identity.TLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(ctx, client.Config{
		DialInBackground: true,
		Addrs:            []string{cfg.Address.Addr},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		DialOpts:                 dialOpts(cfg),
		CircuitBreakerConfig:     circuitBreakerConfig(),
		InsecureAddressDiscovery: cfg.Insecure,
	})
}

func dialOpts(cfg Config) []grpc.DialOption {
	opts := []grpc.DialOption{
		metadata.WithUserAgentFromTeleportComponent(teleport.ComponentTBot),
	}
	if cfg.Metrics != nil {
		opts = append(opts,
			grpc.WithChainUnaryInterceptor(cfg.Metrics.UnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(cfg.Metrics.StreamClientInterceptor()),
		)
	}
	return opts
}

func circuitBreakerConfig() breaker.Config {
	cfg := breaker.DefaultBreakerConfig(clockwork.NewRealClock())
	cfg.TrippedErrorMessage = "Unable to communicate with the Teleport Auth Service"
	return cfg
}
