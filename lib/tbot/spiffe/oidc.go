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

package spiffe

import (
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ProxiesGetter is a service that gets proxies.
type proxiesGetter interface {
	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// IssuerForCluster returns the issuer to use when creating JWT SVIDs based on
// the public address of the first proxy in the cluster.
// TODO(noah): It'd be nice to eventually make this configurable to allow for
// odd circumstances. It may also arise that it may be useful to have an
// "allowed" list of issuers and allow the client to select one.
func IssuerForCluster(cache proxiesGetter) (string, error) {
	proxies, err := cache.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	var proxy types.Server
	for _, p := range proxies {
		if p.GetPublicAddr() != "" {
			proxy = p
			break
		}
	}
	if proxy == nil {
		return "", trace.BadParameter("no proxies configured with public address")
	}

	return IssuerFromPublicAddress(proxy.GetPublicAddr())
}

// IssuerFromPublicAddress returns the issuer URL for the SPIFFE JWT issuer
// based on the provided public address.
func IssuerFromPublicAddress(addr string) (string, error) {
	parsed, err := url.Parse(addr)
	if err != nil {
		return "", trace.Wrap(err, "parsing proxy public addr (%s)", addr)
	}

	// Default to HTTPs if no explicit scheme configured
	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	if parsed.Port() == "443" {
		// Cut off redundant :443
		parsed.Host = parsed.Hostname()
	}

	// Append /workload-identity to distinguish this issuer from the other JWT
	// issuers within Teleport.
	parsed.Path = "/workload-identity"

	return parsed.String(), nil
}
