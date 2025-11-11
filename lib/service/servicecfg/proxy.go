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

package servicecfg

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxyConfig specifies the configuration for Teleport's Proxy Service
type ProxyConfig struct {
	// Enabled turns proxy role on or off for this process
	Enabled bool

	// DisableTLS is enabled if we don't want self-signed certs
	DisableTLS bool

	// DisableWebInterface allows turning off serving the Web UI interface
	DisableWebInterface bool

	// DisableWebService turns off serving web service completely, including web UI
	DisableWebService bool

	// DisableReverseTunnel disables reverse tunnel on the proxy
	DisableReverseTunnel bool

	// DisableDatabaseProxy disables database access proxy listener
	DisableDatabaseProxy bool

	// ReverseTunnelListenAddr is address where reverse tunnel dialers connect to
	ReverseTunnelListenAddr utils.NetAddr

	// PROXYProtocolMode controls behavior related to unsigned PROXY protocol headers.
	PROXYProtocolMode multiplexer.PROXYProtocolMode

	// PROXYAllowDowngrade controls whether or not pseudo IPv4 downgrading is allowed for
	// IPv6 sources communicating with IPv4 destinations.
	PROXYAllowDowngrade bool

	// WebAddr is address for web portal of the proxy
	WebAddr utils.NetAddr

	// SSHAddr is address of ssh proxy
	SSHAddr utils.NetAddr

	// MySQLAddr is address of MySQL proxy.
	MySQLAddr utils.NetAddr

	// MySQLServerVersion  allows to override the default MySQL Engine Version propagated by Teleport Proxy.
	MySQLServerVersion string

	// PostgresAddr is address of Postgres proxy.
	PostgresAddr utils.NetAddr

	// MongoAddr is address of Mongo proxy.
	MongoAddr utils.NetAddr

	// PeerAddress is the proxy peering address.
	PeerAddress utils.NetAddr

	// PeerPublicAddr is the public address the proxy advertises for proxy
	// peering clients.
	PeerPublicAddr utils.NetAddr

	Limiter limiter.Config

	// PublicAddrs is a list of the public addresses the proxy advertises
	// for the HTTP endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	PublicAddrs []utils.NetAddr

	// SSHPublicAddrs is a list of the public addresses the proxy advertises
	// for the SSH endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	SSHPublicAddrs []utils.NetAddr

	// TunnelPublicAddrs is a list of the public addresses the proxy advertises
	// for the tunnel endpoint. The hosts in PublicAddr are included in the
	// list of host principals on the TLS and SSH certificate.
	TunnelPublicAddrs []utils.NetAddr

	// PostgresPublicAddrs is a list of the public addresses the proxy
	// advertises for Postgres clients.
	PostgresPublicAddrs []utils.NetAddr

	// MySQLPublicAddrs is a list of the public addresses the proxy
	// advertises for MySQL clients.
	MySQLPublicAddrs []utils.NetAddr

	// MongoPublicAddrs is a list of the public addresses the proxy
	// advertises for Mongo clients.
	MongoPublicAddrs []utils.NetAddr

	// Kube specifies kubernetes proxy configuration
	Kube KubeProxyConfig

	// KeyPairs are the key and certificate pairs that the proxy will load.
	KeyPairs []KeyPairPath

	// KeyPairsReloadInterval is the interval between attempts to reload
	// x509 key pairs. If set to 0, then periodic reloading is disabled.
	KeyPairsReloadInterval time.Duration

	// ACME is ACME protocol support config
	ACME ACME

	// IdP is the identity provider config
	//
	//nolint:revive // Because we want this to be IdP.
	IdP IdP

	// DisableALPNSNIListener allows turning off the ALPN Proxy listener. Used in tests.
	DisableALPNSNIListener bool

	// UI provides config options for the web UI
	UI webclient.UIConfig

	// TrustXForwardedFor enables the service to take client source IPs from
	// the "X-Forwarded-For" headers for web APIs recevied from layer 7 load
	// balancers or reverse proxies.
	TrustXForwardedFor bool

	// ProxyGroupID is the reverse tunnel group ID, advertised as a label and
	// used by reverse tunnel agents in proxy peering mode. The empty group ID
	// is a valid group ID.
	ProxyGroupID string

	// ProxyGroupGeneration is the reverse tunnel group generation, advertised
	// as a label and used by reverse tunnel agents in proxy peering mode. Zero
	// is a valid generation.
	ProxyGroupGeneration uint64

	// AutomaticUpgradesChannels is a map of all version channels used by the
	// proxy built-in version server to retrieve target versions. This is part
	// of the automatic upgrades.
	AutomaticUpgradesChannels automaticupgrades.Channels

	// QUICProxyPeering will make it so that proxy peering will support inbound
	// QUIC connections and will use QUIC to connect to peer proxies that
	// advertise support for it.
	QUICProxyPeering bool
}

// WebPublicAddr returns the address for the web endpoint on this proxy that
// can be reached by clients.
func (c ProxyConfig) WebPublicAddr() (string, error) {
	// Use the port from the first public address if possible.
	if len(c.PublicAddrs) > 0 {
		publicAddr := c.PublicAddrs[0]
		u := url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(publicAddr.Host(), strconv.Itoa(publicAddr.Port(defaults.HTTPListenPort))),
		}
		return u.String(), nil
	}

	port := c.WebAddr.Port(defaults.HTTPListenPort)
	return c.getDefaultAddr(port), nil
}

func (c ProxyConfig) getDefaultAddr(port int) string {
	host := "<proxyhost>"
	// Try to guess the hostname from the HTTP public_addr.
	if len(c.PublicAddrs) > 0 {
		host = c.PublicAddrs[0].Host()
	}

	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	return u.String()
}

// KubeAddr returns the address for the Kubernetes endpoint on this proxy that
// can be reached by clients.
func (c ProxyConfig) KubeAddr() (string, error) {
	if !c.Kube.Enabled {
		return "", trace.NotFound("kubernetes support not enabled on this proxy")
	}
	if len(c.Kube.PublicAddrs) > 0 {
		return fmt.Sprintf("https://%s", c.Kube.PublicAddrs[0].Addr), nil
	}

	return c.getDefaultAddr(c.Kube.ListenAddr.Port(defaults.KubeListenPort)), nil
}

// PublicPeerAddr attempts to returns the public address the proxy advertises
// for proxy peering clients if available; otherwise, it falls back to trying to
// guess an appropriate public address based on the listen address.
func (c ProxyConfig) PublicPeerAddr() (*utils.NetAddr, error) {
	addr := &c.PeerPublicAddr
	if !addr.IsEmpty() && !addr.IsHostUnspecified() {
		return addr, nil
	}

	addr = &c.PeerAddress
	if addr.IsEmpty() {
		addr = defaults.ProxyPeeringListenAddr()
	}
	if !addr.IsHostUnspecified() {
		return addr, nil
	}

	ip, err := utils.GuessHostIP()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	port := addr.Port(defaults.ProxyPeeringListenPort)
	addr, err = utils.ParseAddr(fmt.Sprintf("%s:%d", ip.String(), port))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return addr, nil
}

// PeerListenAddr returns the proxy peering listen address that was configured,
// or the default one otherwise.
func (c ProxyConfig) PeerListenAddr() *utils.NetAddr {
	if c.PeerAddress.IsEmpty() {
		return defaults.ProxyPeeringListenAddr()
	}
	return &c.PeerAddress
}

// KubeProxyConfig specifies the Kubernetes configuration for Teleport's proxy service
type KubeProxyConfig struct {
	// Enabled turns kubernetes proxy role on or off for this process
	Enabled bool

	// ListenAddr is the address to listen on for incoming kubernetes requests.
	ListenAddr utils.NetAddr

	// ClusterOverride causes all traffic to go to a specific remote
	// cluster, used only in tests
	ClusterOverride string

	// PublicAddrs is a list of the public addresses the Teleport Kube proxy can be accessed by,
	// it also affects the host principals and routing logic
	PublicAddrs []utils.NetAddr

	// KubeconfigPath is a path to kubeconfig
	KubeconfigPath string

	// LegacyKubeProxy specifies that this proxy was configured using the
	// legacy kubernetes section.
	LegacyKubeProxy bool
}

// ACME configures ACME automatic certificate renewal
type ACME struct {
	// Enabled enables or disables ACME support
	Enabled bool
	// Email receives notifications from ACME server
	Email string
	// URI is ACME server URI
	URI string
}

// IdP configures identity providers.
//
//nolint:revive // Because we want this to be IdP.
type IdP struct {
	// SAMLIdP is configuration options for the SAML identity provider.
	SAMLIdP SAMLIdP
}

// SAMLIdP configures SAML identity providers
type SAMLIdP struct {
	// Enabled enables or disables the identity provider.
	Enabled bool
	// BaseURL is the base URL for the identity provider.
	BaseURL string
}

// KeyPairPath are paths to a key and certificate file.
type KeyPairPath struct {
	// PrivateKey is the path to a PEM encoded private key.
	PrivateKey string
	// Certificate is the path to a PEM encoded certificate.
	Certificate string
}
