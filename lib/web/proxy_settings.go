// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package web

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// NetworkConfigGetter is a helper interface that allows to fetch the current proxy configuration.
type NetworkConfigGetter interface {
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
}

// ProxySettings is a helper type that allows to fetch the current proxy configuration.
type ProxySettings struct {
	// cfg is the Teleport service configuration.
	ServiceConfig *servicecfg.Config
	// proxySSHAddr is the address of the proxy ssh service. It can be assigned during runtime when a user set the
	// proxy listener address to a random port (e.g. `127.0.0.1:0`).
	ProxySSHAddr string
	// accessPoint is the caching client connected to the auth server.
	AccessPoint NetworkConfigGetter
}

// GetProxySettings allows returns current proxy configuration.
func (p *ProxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	resp, err := p.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch p.ServiceConfig.Version {
	case defaults.TeleportConfigVersionV2, defaults.TeleportConfigVersionV3:
		return p.buildProxySettingsV2(resp.GetProxyListenerMode(), resp.GetSSHDialTimeout()), nil
	default:
		return p.buildProxySettings(resp.GetProxyListenerMode(), resp.GetSSHDialTimeout()), nil
	}
}

// buildProxySettings builds standard proxy configuration where proxy services are
// configured on different listeners. If the TLSRoutingEnabled flag is set and a proxy
// client support the TLSRouting dialer then the client will connect to the Teleport Proxy WebPort
// where incoming connections are routed to the proper proxy service based on TLS SNI ALPN routing information.
func (p *ProxySettings) buildProxySettings(proxyListenerMode types.ProxyListenerMode, sshDialTimeout time.Duration) *webclient.ProxySettings {
	proxySettings := webclient.ProxySettings{
		TLSRoutingEnabled: proxyListenerMode == types.ProxyListenerMode_Multiplex,
		Kube: webclient.KubeProxySettings{
			Enabled: p.ServiceConfig.Proxy.Kube.Enabled,
		},
		SSH: webclient.SSHProxySettings{
			ListenAddr:       p.ProxySSHAddr,
			TunnelListenAddr: p.ServiceConfig.Proxy.ReverseTunnelListenAddr.String(),
			WebListenAddr:    p.ServiceConfig.Proxy.WebAddr.String(),
			DialTimeout:      sshDialTimeout,
		},
	}

	p.setProxyPublicAddressesSettings(&proxySettings)

	if !p.ServiceConfig.Proxy.MySQLAddr.IsEmpty() {
		proxySettings.DB.MySQLListenAddr = p.ServiceConfig.Proxy.MySQLAddr.String()
	}

	if !p.ServiceConfig.Proxy.PostgresAddr.IsEmpty() {
		proxySettings.DB.PostgresListenAddr = p.ServiceConfig.Proxy.PostgresAddr.String()
	}

	if !p.ServiceConfig.Proxy.MongoAddr.IsEmpty() {
		proxySettings.DB.MongoListenAddr = p.ServiceConfig.Proxy.MongoAddr.String()
	}

	if p.ServiceConfig.Proxy.Kube.Enabled {
		proxySettings.Kube.ListenAddr = p.ServiceConfig.Proxy.Kube.ListenAddr.String()
	}
	return &proxySettings
}

// buildProxySettingsV2 builds the v2 proxy settings where teleport proxies can start only on a single listener.
func (p *ProxySettings) buildProxySettingsV2(proxyListenerMode types.ProxyListenerMode, sshDialTimeout time.Duration) *webclient.ProxySettings {
	multiplexAddr := p.ServiceConfig.Proxy.WebAddr.String()
	settings := p.buildProxySettings(proxyListenerMode, sshDialTimeout)
	if proxyListenerMode == types.ProxyListenerMode_Multiplex {
		settings.SSH.ListenAddr = multiplexAddr
		settings.SSH.TunnelListenAddr = multiplexAddr
		settings.SSH.WebListenAddr = multiplexAddr
		settings.Kube.ListenAddr = multiplexAddr
		settings.DB.MySQLListenAddr = multiplexAddr
		settings.DB.PostgresListenAddr = multiplexAddr
	}
	return settings
}

func (p *ProxySettings) setProxyPublicAddressesSettings(settings *webclient.ProxySettings) {
	if len(p.ServiceConfig.Proxy.PublicAddrs) > 0 {
		settings.SSH.PublicAddr = p.ServiceConfig.Proxy.PublicAddrs[0].String()
	}
	if len(p.ServiceConfig.Proxy.SSHPublicAddrs) > 0 {
		settings.SSH.SSHPublicAddr = p.ServiceConfig.Proxy.SSHPublicAddrs[0].String()
	}
	if len(p.ServiceConfig.Proxy.TunnelPublicAddrs) > 0 {
		settings.SSH.TunnelPublicAddr = p.ServiceConfig.Proxy.TunnelPublicAddrs[0].String()
	}
	if len(p.ServiceConfig.Proxy.Kube.PublicAddrs) > 0 {
		settings.Kube.PublicAddr = p.ServiceConfig.Proxy.Kube.PublicAddrs[0].String()
	}
	if len(p.ServiceConfig.Proxy.MySQLPublicAddrs) > 0 {
		settings.DB.MySQLPublicAddr = p.ServiceConfig.Proxy.MySQLPublicAddrs[0].String()
	}
	if len(p.ServiceConfig.Proxy.MongoPublicAddrs) > 0 {
		settings.DB.MongoPublicAddr = p.ServiceConfig.Proxy.MongoPublicAddrs[0].String()
	}
	settings.DB.PostgresPublicAddr = p.getPostgresPublicAddr()
}

// getPostgresPublicAddr returns the proxy PostgresPublicAddrs based on whether the Postgres proxy service
// was configured on separate listener. For backward compatibility if PostgresPublicAddrs was not provided.
// Proxy will reuse the PostgresPublicAddrs field to propagate postgres service address to legacy tsh clients.
func (p *ProxySettings) getPostgresPublicAddr() string {
	if len(p.ServiceConfig.Proxy.PostgresPublicAddrs) > 0 {
		return p.ServiceConfig.Proxy.PostgresPublicAddrs[0].String()
	}

	if p.ServiceConfig.Proxy.PostgresAddr.IsEmpty() {
		return ""
	}

	// DELETE IN 9.0.0
	// If the PostgresPublicAddrs address was not set propagate separate postgres service listener address
	// to legacy tsh clients reusing PostgresPublicAddrs field.
	var host string
	if len(p.ServiceConfig.Proxy.PublicAddrs) > 0 {
		// Get proxy host address from public address.
		host = p.ServiceConfig.Proxy.PublicAddrs[0].Host()
	} else {
		host = p.ServiceConfig.Proxy.WebAddr.Host()
	}
	return fmt.Sprintf("%s:%d", host, p.ServiceConfig.Proxy.PostgresAddr.Port(defaults.PostgresListenPort))
}
