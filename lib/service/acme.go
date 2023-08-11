/*
Copyright 2015-2020 Gravitational, Inc.

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
	"net"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

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
