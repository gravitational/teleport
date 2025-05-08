/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"runtime"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Config describes gateway configuration
type Config struct {
	// URI is the gateway URI
	URI uri.ResourceURI
	// TargetName is the remote resource name
	TargetName string
	// TargetURI is the remote resource URI
	TargetURI uri.ResourceURI
	// TargetUser is the target user name
	TargetUser string
	// TargetGroups is a list of target groups
	TargetGroups []string
	// TargetSubresourceName points at a subresource of the remote resource, for example a database
	// name on a database server. It is used only for generating the CLI command.
	TargetSubresourceName string

	// Port is the gateway port
	LocalPort string
	// LocalAddress is the local address
	LocalAddress string
	// Protocol is the gateway protocol
	Protocol string
	// Cert is used by the local proxy to connect to the Teleport proxy.
	Cert tls.Certificate
	// Insecure
	Insecure bool
	// ClusterName is the Teleport cluster name.
	ClusterName string
	// Username is the username of the profile.
	Username string
	// WebProxyAddr
	WebProxyAddr string
	// Logger is a component logger
	Logger *slog.Logger
	// TCPPortAllocator creates listeners on the given ports. This interface lets us avoid occupying
	// hardcoded ports in tests.
	TCPPortAllocator TCPPortAllocator
	// Clock is used by Gateway.localProxy to check cert expiration.
	Clock clockwork.Clock
	// OnExpiredCert is called when a new downstream connection is accepted by the
	// gateway but cannot be proxied because the cert used by the gateway has expired.
	//
	// Returns a fresh valid cert.
	//
	// Handling of the connection is blocked until OnExpiredCert returns.
	OnExpiredCert OnExpiredCertFunc
	// TLSRoutingConnUpgradeRequired indicates that ALPN connection upgrades
	// are required for making TLS routing requests.
	TLSRoutingConnUpgradeRequired bool
	// RootClusterCACertPoolFunc is callback function to fetch Root cluster CAs
	// when ALPN connection upgrade is required.
	RootClusterCACertPoolFunc alpnproxy.GetClusterCACertPoolFunc
	// KubeconfigsDir is the directory containing kubeconfigs for kube gateways.
	KubeconfigsDir string
	// ClearCertsOnTargetSubresourceNameChange is useful in situations where TargetSubresourceName is
	// used to generate a cert. In that case, after TargetSubresourceName is changed, the gateway will
	// clear the cert from the local proxy and the middleware is going to request a new cert on the
	// next connection.
	ClearCertsOnTargetSubresourceNameChange bool
}

// OnExpiredCertFunc is the type of a function that is called when a new downstream connection is
// accepted by the gateway but cannot be proxied because the cert used by the gateway has expired.
//
// Handling of the connection is blocked until the function returns.
type OnExpiredCertFunc func(context.Context, Gateway) (tls.Certificate, error)

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if !c.TargetURI.IsDB() && !c.TargetURI.IsKube() && !c.TargetURI.IsApp() {
		return trace.BadParameter("unsupported gateway target %v", c.TargetURI)
	}

	if len(c.Cert.Certificate) == 0 {
		return trace.BadParameter("missing cert")
	}

	if c.URI.String() == "" {
		c.URI = uri.NewGatewayURI(uuid.NewString())
	}

	if c.LocalAddress == "" {
		c.LocalAddress = "localhost"
		// SQL Server Management Studio won't connect to localhost:12345, so use 127.0.0.1:12345 instead.
		if runtime.GOOS == constants.WindowsOS && c.Protocol == defaults.ProtocolSQLServer {
			c.LocalAddress = "127.0.0.1"
		}
	}

	if c.LocalPort == "" {
		c.LocalPort = "0"
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	if c.TargetName == "" {
		return trace.BadParameter("missing target name")
	}

	if c.TargetURI.String() == "" {
		return trace.BadParameter("missing target URI")
	}

	if c.TCPPortAllocator == nil {
		c.TCPPortAllocator = NetTCPPortAllocator{}
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	if c.RootClusterCACertPoolFunc == nil {
		if !c.Insecure {
			return trace.BadParameter("missing RootClusterCACertPoolFunc")
		}
		c.RootClusterCACertPoolFunc = func(_ context.Context) (*x509.CertPool, error) {
			return x509.NewCertPool(), nil
		}
	}

	c.Logger = c.Logger.With(
		"resource", c.TargetURI.String(),
		"gateway", c.URI.String(),
	)
	return nil
}

// RouteToDatabase returns tlsca.RouteToDatabase based on the config of the gateway.
//
// The tlsca.RouteToDatabase.Database field is skipped, as it's an optional field and gateways can
// change their Config.TargetSubresourceName at any moment.
func (c *Config) RouteToDatabase() tlsca.RouteToDatabase {
	return tlsca.RouteToDatabase{
		ServiceName: c.TargetName,
		Protocol:    c.Protocol,
		Username:    c.TargetUser,
	}
}

func (c *Config) makeListener() (net.Listener, error) {
	listener, err := c.TCPPortAllocator.Listen(c.LocalAddress, c.LocalPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// retrieve automatically assigned port number
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.LocalPort = port
	return listener, nil
}
