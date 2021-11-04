/*
Copyright 2021 Gravitational, Inc.

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

package service

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// proxySettings is a helper type that allows to fetch the current proxy configuration.
type proxySettings struct {
	// cfg is the Teleport service configuration.
	cfg *Config
	// proxySSHAddr is the address of the proxy ssh service. It can be assigned during runtime when a user set the
	// proxy listener address to a random port (e.g. `127.0.0.1:0`).
	proxySSHAddr utils.NetAddr
	// accessPoint is the caching client connected to the auth server.
	accessPoint auth.ProxyAccessPoint
}

// GetProxySettings allows returns current proxy configuration.
func (p *proxySettings) GetProxySettings(ctx context.Context) (*webclient.ProxySettings, error) {
	resp, err := p.accessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch p.cfg.Version {
	case defaults.TeleportConfigVersionV2:
		return p.buildProxySettingsV2(resp.GetProxyListenerMode()), nil
	default:
		return p.buildProxySettings(resp.GetProxyListenerMode()), nil
	}
}

// buildProxySettings builds standard proxy configuration where proxy services are
// configured on different listeners. If the TLSRoutingEnabled flag is set and a proxy
// client support the TLSRouting dialer then the client will connect to the Teleport Proxy WebPort
// where incoming connections are routed to the proper proxy service based on TLS SNI ALPN routing information.
func (p *proxySettings) buildProxySettings(proxyListenerMode types.ProxyListenerMode) *webclient.ProxySettings {
	proxySettings := webclient.ProxySettings{
		TLSRoutingEnabled: proxyListenerMode == types.ProxyListenerMode_Multiplex,
		Kube: webclient.KubeProxySettings{
			Enabled: p.cfg.Proxy.Kube.Enabled,
		},
		SSH: webclient.SSHProxySettings{
			ListenAddr:       p.proxySSHAddr.String(),
			TunnelListenAddr: p.cfg.Proxy.ReverseTunnelListenAddr.String(),
		},
	}

	p.setProxyPublicAddressesSettings(&proxySettings)

	if !p.cfg.Proxy.MySQLAddr.IsEmpty() {
		proxySettings.DB.MySQLListenAddr = p.cfg.Proxy.MySQLAddr.String()
	}
	if p.cfg.Proxy.Kube.Enabled {
		proxySettings.Kube.ListenAddr = p.getProxyKubeAddress(proxyListenerMode)
	}
	return &proxySettings
}

// buildProxySettingsV2 builds the v2 proxy settings where teleport proxies can start only on a single listener.
func (p *proxySettings) buildProxySettingsV2(proxyListenerMode types.ProxyListenerMode) *webclient.ProxySettings {
	multiplexAddr := p.cfg.Proxy.WebAddr.String()
	settings := p.buildProxySettings(proxyListenerMode)
	if proxyListenerMode == types.ProxyListenerMode_Multiplex {
		settings.SSH.ListenAddr = multiplexAddr
		settings.SSH.TunnelListenAddr = multiplexAddr
		settings.Kube.ListenAddr = multiplexAddr
		settings.DB.MySQLListenAddr = multiplexAddr
	}
	return settings
}

func (p *proxySettings) setProxyPublicAddressesSettings(settings *webclient.ProxySettings) {
	if len(p.cfg.Proxy.PublicAddrs) > 0 {
		settings.SSH.PublicAddr = p.cfg.Proxy.PublicAddrs[0].String()
	}
	if len(p.cfg.Proxy.SSHPublicAddrs) > 0 {
		settings.SSH.SSHPublicAddr = p.cfg.Proxy.SSHPublicAddrs[0].String()
	}
	if len(p.cfg.Proxy.TunnelPublicAddrs) > 0 {
		settings.SSH.TunnelPublicAddr = p.cfg.Proxy.TunnelPublicAddrs[0].String()
	}
	if len(p.cfg.Proxy.Kube.PublicAddrs) > 0 {
		settings.Kube.PublicAddr = p.cfg.Proxy.Kube.PublicAddrs[0].String()
	}
	if len(p.cfg.Proxy.PostgresPublicAddrs) > 0 {
		settings.DB.PostgresPublicAddr = p.cfg.Proxy.PostgresPublicAddrs[0].String()
	}
	if len(p.cfg.Proxy.MySQLPublicAddrs) > 0 {
		settings.DB.MySQLPublicAddr = p.cfg.Proxy.MySQLPublicAddrs[0].String()
	}
}

func (p *proxySettings) getProxyKubeAddress(mode types.ProxyListenerMode) string {
	if !p.cfg.Proxy.DisableALPNSNIListener && !p.cfg.Proxy.DisableTLS && mode == types.ProxyListenerMode_Multiplex {
		return p.cfg.Proxy.WebAddr.String()
	}
	return p.cfg.Proxy.Kube.ListenAddr.String()
}
