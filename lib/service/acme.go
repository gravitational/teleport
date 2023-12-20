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

package service

import (
	"context"
	"net"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
)

type hostPolicyCheckerConfig struct {
	// publicAddrs is a list of pubic addresses to support acme for
	publicAddrs []utils.NetAddr
	// clt is used to get the list of registered applications
	clt app.Getter
	// tun is a reverse tunnel
	tun reversetunnelclient.Tunnel
	// clusterName is a name of this cluster
	clusterName string
}

type hostPolicyChecker struct {
	dnsNames []string
	cfg      hostPolicyCheckerConfig
}

// checkHost approves getting certs for hosts specified in public_addr
// and their subdomains, if there is a valid application name registered
func (h *hostPolicyChecker) checkHost(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return trace.BadParameter(
			"with proxy_service.acme on, IP URL https://%v is not supported, use one of the domains in proxy_service.public_addr: %v",
			host, strings.Join(h.dnsNames, ","))
	}

	if slices.Contains(h.dnsNames, host) {
		return nil
	}

	_, _, err := app.ResolveFQDN(ctx, h.cfg.clt, h.cfg.tun, h.dnsNames, host)
	if err == nil {
		return nil
	}
	if trace.IsNotFound(err) {
		return trace.BadParameter(
			"acme can't get a cert for %v, there is no app with this name", host)
	}

	return trace.BadParameter(
		"acme can't get a cert for domain %v, add it to the proxy_service.public_addr, or use one of the domains: %v",
		host, strings.Join(h.dnsNames, ","))
}

func newHostPolicyChecker(cfg hostPolicyCheckerConfig) (*hostPolicyChecker, error) {
	dnsNames, err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &hostPolicyChecker{
		dnsNames: dnsNames,
		cfg:      cfg,
	}, nil
}

func (h *hostPolicyCheckerConfig) CheckAndSetDefaults() ([]string, error) {
	if h.clt == nil {
		return nil, trace.BadParameter("missing parameter clt")
	}

	if h.tun == nil {
		return nil, trace.BadParameter("missing parameter tun")
	}

	dnsNames := make([]string, 0, len(h.publicAddrs))
	for _, addr := range h.publicAddrs {
		host, err := utils.DNSName(addr.Addr)
		if err != nil {
			continue
		}
		dnsNames = append(dnsNames, host)
	}

	if len(dnsNames) == 0 {
		return nil, trace.BadParameter(
			"acme is enabled, set at least one valid DNS name in public_addr section of proxy_service")
	}
	return dnsNames, nil
}
