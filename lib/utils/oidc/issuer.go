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

package oidc

import (
	"context"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ProxyGetter is a service that gets proxies.
type ProxiesGetter interface {
	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// IssuerForCluster returns the issuer URL using the Cluster state.
// Path is an optional element to append to the issuer to distinguish a
// separate CA within the same cluster.
func IssuerForCluster(ctx context.Context, clt ProxiesGetter, path string) (string, error) {
	proxies, err := clt.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	for _, p := range proxies {
		proxyPublicAddress := p.GetPublicAddr()
		if proxyPublicAddress != "" {
			return IssuerFromPublicAddress(proxyPublicAddress, path)
		}
	}

	return "", trace.BadParameter("failed to get Proxy Public Address")
}

// IssuerFromPublicAddress is the address for an OIDC Provider.
//
// It must match exactly what was introduced in AWS IAM console when adding the Identity Provider.
// PublicProxyAddr from `teleport.yaml/proxy` does not come with the desired format: it misses the protocol and has a port
// This method adds the `https` protocol and removes the port if it is the default one for https (443)
//
// Path is an optional element to append to the issuer to distinguish a
// separate CA within the same cluster.
func IssuerFromPublicAddress(addr string, path string) (string, error) {
	// Add protocol if not present.
	if !strings.HasPrefix(addr, "https://") && !strings.HasPrefix(addr, "http://") {
		addr = "https://" + addr
	}

	result, err := url.Parse(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if result.Port() == "443" {
		// Cut off redundant :443
		result.Host = result.Hostname()
	}

	if path != "" {
		result.Path = path
	}
	return result.String(), nil
}
