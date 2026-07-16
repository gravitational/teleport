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
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
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

// MatchAppServerForRoute matches an app server against routing information,
// typically from a certificate. It matches on whichever of name and public
// address are provided:
//
//   - When both are set both must match. This is what disambiguates multiple apps
//     that share a public address: the app name uniquely identifies an app within
//     the cluster, and the public address is also verified as a safety check.
//   - An empty field is not checked, so this both supports name-only and addr-only
//     resolution.
//   - If both are empty, nothing matches.
func MatchAppServerForRoute(name, publicAddr string) Matcher {
	return func(appServer readonly.AppServer) bool {
		app := appServer.GetApp()
		if publicAddr != "" && !appMatchesPublicAddr(app, publicAddr) {
			return false
		}
		if name != "" && app.GetName() != name {
			return false
		}
		return name != "" || publicAddr != ""
	}
}

// MatchPublicAddr matches on the public address of an application.
func MatchPublicAddr(publicAddr string) Matcher {
	return func(appServer readonly.AppServer) bool {
		return appMatchesPublicAddr(appServer.GetApp(), publicAddr)
	}
}

// appMatchesPublicAddr reports whether the app should answer for the requested
// public address.
// Scoped apps match on the computed subdomain only,
// so the app resolves regardless of which proxy the FQDN was assembled under.
// Unscoped apps require an exact public_addr match.
func appMatchesPublicAddr(app readonly.Application, publicAddr string) bool {
	if scope := app.GetScope(); scope != "" {
		return scopedapp.ScopedAppPublicAddrValid(scope, app.GetName(), publicAddr)
	}

	return app.GetPublicAddr() == publicAddr
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
// canAccess, when specified, reports whether the requesting user is permitted to
// access a given application. It is used to disambiguate when an FQDN matches
// more than one application. When no match is accessible, or canAccess is nil,
// it falls back to a plain best-effort pick and leaves the final access decision
// to the application service.
//
// Note: This function can incorrectly resolve application names. For example,
// if you have an application named "acme" within both the root and leaf
// cluster, this method will always return "acme" running within the root
// cluster. Always supply public address and cluster name to deterministically
// resolve an application.
func ResolveFQDN(
	ctx context.Context,
	clusterGetter reversetunnelclient.ClusterGetter,
	localClusterName string,
	proxyDNSNames []string,
	fqdn string,
	canAccess func(types.Application) bool,
) (types.AppServer, string, error) {
	clusterClient, err := clusterGetter.Cluster(ctx, localClusterName)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Try and match FQDN to public address of application within cluster.
	servers, err := MatchUnshuffled(ctx, clusterClient, MatchPublicAddr(fqdn))
	if err == nil && len(servers) > 0 {
		srv := pickAppServer(servers, canAccess)
		// A scoped app is addressed as <hash>.<proxy> and its hashed subdomain matches regardless of the
		// suffix, so confirm the FQDN actually sits under a configured proxy DNS name before trusting a
		// scoped match, otherwise "<hash>.somewebsite.com" resolves.
		if srv.GetApp().GetScope() != "" {
			host := strings.Split(fqdn, ":")[0]
			// In the case where FindMatchingProxyDNS finds an actual found proxy match, this check is redundant.
			// In the case where it finds no matches, FindMatchingProxyDNS returns the first element of proxyDNSNames,
			// we must recheck to make sure that the proxy found matches, and reject if it doesn't.
			proxyMatch := strings.Split(utils.FindMatchingProxyDNS(fqdn, proxyDNSNames), ":")[0]
			if host != proxyMatch && !strings.HasSuffix(host, "."+proxyMatch) {
				return nil, "", trace.BadParameter("FQDN %q is not a subdomain of the proxy", fqdn)
			}
		}
		return srv, localClusterName, nil
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
			return pickAppServer(servers, canAccess), clusterClient.GetName(), nil
		}
	}

	return nil, "", trace.NotFound("failed to resolve %v to any application within any cluster", fqdn)
}

// pickAppServer chooses one app server from a set of matches. When canAccess is
// provided it prefers servers the user is allowed to access.
//
// If no apps are accessible or canAccess is nil it falls back to a random match,
// leaving the final access decision to the app_service.
func pickAppServer(servers []types.AppServer, canAccess func(types.Application) bool) types.AppServer {
	if canAccess != nil {
		accessible := make([]types.AppServer, 0, len(servers))
		for _, s := range servers {
			if canAccess(s.GetApp()) {
				accessible = append(accessible, s)
			}
		}
		if len(accessible) > 0 {
			return accessible[rand.N(len(accessible))]
		}
	}
	return servers[rand.N(len(servers))]
}
