/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"math/rand"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
)

// Getter returns a list of registered apps and the local cluster name.
type Getter interface {
	// GetApplicationServers returns registered application servers.
	GetApplicationServers(context.Context, string) ([]types.AppServer, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// Match will match a list of applications with the passed in matcher function. Matcher
// functions that can match on public address and name are available. The
// resulting list is shuffled before it is returned.
func Match(ctx context.Context, authClient Getter, fn Matcher) ([]types.AppServer, error) {
	servers, err := authClient.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var as []types.AppServer
	for _, server := range servers {
		if fn(ctx, server) {
			as = append(as, server)
		}
	}

	rand.Shuffle(len(as), func(i, j int) {
		as[i], as[j] = as[j], as[i]
	})

	return as, nil
}

// MatchOne will match a single AppServer with the provided matcher function.
// If no AppServer are matched, it will return an error.
func MatchOne(ctx context.Context, authClient Getter, fn Matcher) (types.AppServer, error) {
	servers, err := authClient.GetApplicationServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, server := range servers {
		if fn(ctx, server) {
			return server, nil
		}
	}

	return nil, trace.NotFound("couldn't match any types.AppServer")
}

// Matcher allows matching on different properties of an application.
type Matcher func(context.Context, types.AppServer) bool

// MatchPublicAddr matches on the public address of an application.
func MatchPublicAddr(publicAddr string) Matcher {
	return func(_ context.Context, appServer types.AppServer) bool {
		return appServer.GetApp().GetPublicAddr() == publicAddr
	}
}

// MatchName matches on the name of an application.
func MatchName(name string) Matcher {
	return func(_ context.Context, appServer types.AppServer) bool {
		return appServer.GetApp().GetName() == name
	}
}

// MatchHealthy tries to establish a connection with the server using the
// `dialAppServer` function. The app server is matched if the function call
// doesn't return any error.
func MatchHealthy(proxyClient reversetunnel.Tunnel, clusterName string) Matcher {
	return func(ctx context.Context, appServer types.AppServer) bool {
		conn, err := dialAppServer(ctx, proxyClient, clusterName, appServer)
		if err != nil {
			return false
		}

		conn.Close()
		return true
	}
}

// MatchAll matches if all the Matcher functions return true.
func MatchAll(matchers ...Matcher) Matcher {
	return func(ctx context.Context, appServer types.AppServer) bool {
		for _, fn := range matchers {
			if !fn(ctx, appServer) {
				return false
			}
		}

		return true
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
func ResolveFQDN(ctx context.Context, clt Getter, tunnel reversetunnel.Tunnel, proxyDNSNames []string, fqdn string) (types.AppServer, string, error) {
	// Try and match FQDN to public address of application within cluster.
	servers, err := Match(ctx, clt, MatchPublicAddr(fqdn))
	if err == nil && len(servers) > 0 {
		clusterName, err := clt.GetClusterName()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return servers[0], clusterName.GetClusterName(), nil
	}

	// Extract the first subdomain from the FQDN and attempt to use this as the
	// application name. The rest of the FQDN must match one of the local
	// cluster's proxy DNS names.
	fqdnParts := strings.SplitN(fqdn, ".", 2)
	if len(fqdnParts) != 2 {
		return nil, "", trace.BadParameter("invalid FQDN: %v", fqdn)
	}
	if !slices.Contains(proxyDNSNames, fqdnParts[1]) {
		return nil, "", trace.BadParameter("FQDN %q is not a subdomain of the proxy", fqdn)
	}
	appName := fqdnParts[0]

	// Loop over all clusters and try and match application name to an
	// application within the cluster. This also includes the local cluster.
	clusterClients, err := tunnel.GetSites()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	for _, clusterClient := range clusterClients {
		authClient, err := clusterClient.CachingAccessPoint()
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		servers, err = Match(ctx, authClient, MatchName(appName))
		if err == nil && len(servers) > 0 {
			return servers[0], clusterClient.GetName(), nil
		}
	}

	return nil, "", trace.NotFound("failed to resolve %v to any application within any cluster", fqdn)
}
