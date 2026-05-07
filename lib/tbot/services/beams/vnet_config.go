// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package beams

import (
	"context"
	"net/netip"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/vnet"
)

// VNetServiceType identifies the Beams VNet service.
const VNetServiceType = "beams/vnet"

// VNetServiceConfig holds the configuration for the Beams VNet service.
type VNetServiceConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`

	// DelegationSessionID identifies the delegation session the generated
	// credentials will be associated with, enabling the bot to act on a (human)
	// user's behalf.
	DelegationSessionID string `yaml:"delegation_session_id,omitempty"`

	// UpstreamNameservers allows you to override the upstream nameservers that
	// will be used when an FQDN does not belong to a VNet-accessible app.
	//
	// By default, the `/etc/resolv.conf` file will be read to determine these.
	UpstreamNameservers StaticUpstreamNameservers `yaml:"upstream_nameservers,omitempty"`
}

// Type satisfies the config.ServiceConfig interface.
func (VNetServiceConfig) Type() string { return VNetServiceType }

// CheckAndSetDefaults satisfies the config.ServiceConfig interface.
func (c *VNetServiceConfig) CheckAndSetDefaults(scoped bool) error {
	if scoped {
		return trace.BadParameter("service type %q is not supported in scoped mode", VNetServiceType)
	}
	if c.DelegationSessionID == "" {
		return trace.BadParameter("delegation_session_id: is required")
	}
	for idx, ns := range c.UpstreamNameservers {
		if _, err := netip.ParseAddrPort(ns); err != nil {
			return trace.BadParameter("upstream_nameservers[%d]: must be a valid `ip:port` pair", idx)
		}
	}
	return nil
}

// GetCredentialLifetime satisfies the config.ServiceConfig interface.
func (c *VNetServiceConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return c.CredentialLifetime
}

// GetName satisfies the config.ServiceConfig interface.
func (c *VNetServiceConfig) GetName() string {
	return c.Name
}

// SetName satisfies the config.ServiceConfig interface.
func (c *VNetServiceConfig) SetName(name string) {
	c.Name = name
}

// VNetServiceOpt can be used to customize the VNetService.
type VNetServiceOpt func(*VNetService)

// WithInsecure disables server certificate verification when dialing the ALPN
// proxy.
func WithInsecure() VNetServiceOpt {
	return func(svc *VNetService) {
		svc.insecure = true
	}
}

// WithDefaultCredentialLifetime sets the default credential lifetime.
func WithDefaultCredentialLifetime(lifetime bot.CredentialLifetime) VNetServiceOpt {
	return func(svc *VNetService) {
		svc.defaultCredentialLifetime = lifetime
	}
}

// WithTUNDevice overrides the service's TUN device in tests.
func WithTUNDevice(tun vnet.TUNDevice) VNetServiceOpt {
	return func(svc *VNetService) {
		svc.createTUN = func() (vnet.TUNDevice, error) {
			return tun, nil
		}
	}
}

// WithConfigureHost overrides the function that will be called to configure
// the host network, in tests.
func WithConfigureHost(fn vnet.EmbeddedConfigureHostFunc) VNetServiceOpt {
	return func(svc *VNetService) {
		svc.configureHost = func(ctx context.Context, _ vnet.TUNDevice, cfg *vnet.EmbeddedVNetHostConfig) error {
			return fn(ctx, cfg)
		}
	}
}

// StaticUpstreamNameservers is a fixed list of upstream nameserver addresses.
type StaticUpstreamNameservers []string

// UpstreamNameservers satisfies the dns.UpstreamNameserversSource interface.
func (s StaticUpstreamNameservers) UpstreamNameservers(context.Context) ([]string, error) {
	return s, nil
}
