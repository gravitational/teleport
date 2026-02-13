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

package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// MatchUnshuffled will match a list of applications with the passed in matcher
// function. Matcher functions that can match on public address and name are
// available.
func MatchUnshuffled(ctx context.Context, cluster reversetunnelclient.Cluster, fn Matcher) ([]types.AppServer, error) {
	watcher, err := cluster.AppServerWatcher()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := watcher.CurrentResourcesWithFilter(ctx, fn)
	return servers, trace.Wrap(err)
}

// Matcher allows matching on different properties of an app server.
type Matcher func(readonly.AppServer) bool

// MatchPublicAddr matches on the public address of an application.
func MatchPublicAddr(publicAddr string) Matcher {
	return func(appServer readonly.AppServer) bool {
		return appServer.GetApp().GetPublicAddr() == publicAddr
	}
}

// MatchName matches on the name of an application.
func MatchName(name string) Matcher {
	return func(appServer readonly.AppServer) bool {
		return appServer.GetApp().GetName() == name
	}
}

// ResolveFQDN makes a best effort attempt to resolve FQDN to an application
// running a root or leaf cluster.
//
// Note: This function can incorrectly resolve application names. For example,
// if you have an application named "acme" within both the root and leaf
// cluster, this method will always return "acme" running within the root
// cluster. Always supply public address and cluster name to deterministically
// resolve an application.
func ResolveFQDN(ctx context.Context, clusterGetter reversetunnelclient.ClusterGetter, localClusterName string, proxyDNSNames []string, fqdn string) (types.AppServer, string, error) {
	clusterClient, err := clusterGetter.Cluster(ctx, localClusterName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Try and match FQDN to public address of application within cluster.
	servers, err := MatchUnshuffled(ctx, clusterClient, MatchPublicAddr(fqdn))
	if err == nil && len(servers) > 0 {
		return servers[rand.N(len(servers))], localClusterName, nil
	}

	proxyPublicAddr := utils.FindMatchingProxyDNS(fqdn, proxyDNSNames)
	if !strings.HasSuffix(fqdn, proxyPublicAddr) {
		return nil, "", trace.BadParameter("FQDN %q is not a subdomain of the proxy", fqdn)
	}
	appName := strings.TrimSuffix(fqdn, fmt.Sprintf(".%s", proxyPublicAddr))

	// Loop over all clusters and try and match application name to an
	// application within the cluster. This also includes the local cluster.
	clusterClients, err := clusterGetter.Clusters(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	for _, clusterClient := range clusterClients {
		servers, err = MatchUnshuffled(ctx, clusterClient, MatchName(appName))
		if err == nil && len(servers) > 0 {
			return servers[rand.N(len(servers))], clusterClient.GetName(), nil
		}
	}

	return nil, "", trace.NotFound("failed to resolve %v to any application within any cluster", fqdn)
}

// ResolveByName resolves an application in a specific Teleport cluster by name.
func ResolveByName(ctx context.Context, cluster reversetunnelclient.Cluster, appName string) (types.AppServer, error) {
	servers, err := MatchUnshuffled(ctx, cluster, MatchName(appName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(servers) == 0 {
		return nil, trace.BadParameter("unable to resolve requested app by name")
	}

	return servers[0], nil
}
