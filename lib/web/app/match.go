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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// Getter returns a list of registered apps and the local cluster name.
type Getter interface {
	// GetAppServers returns a list of app servers
	GetAppServers(context.Context, string, ...services.MarshalOption) ([]services.Server, error)

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error)
}

// Match will match an application with the passed in matcher function. Matcher
// functions that can match on public address and name are available.
//
// Note that in the situation multiple applications match, a random selection
// is returned. This is done on purpose to support HA to allow multiple
// application proxy nodes to be run and if one is down, at least the
// application can be accessible on the other.
//
// In the future this function should be updated to keep state on application
// servers that are down and to not route requests to that server.
func Match(ctx context.Context, authClient Getter, fn Matcher) (*types.App, services.Server, error) {
	servers, err := authClient.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var am []*types.App
	var sm []services.Server

	for _, server := range servers {
		for _, app := range server.GetApps() {
			if fn(app) {
				am = append(am, app)
				sm = append(sm, server)
			}
		}
	}

	if len(am) == 0 {
		return nil, nil, trace.NotFound("failed to match application")
	}
	index := rand.Intn(len(am))
	return am[index], sm[index], nil
}

// Matcher allows matching on different properties of an application.
type Matcher func(*types.App) bool

// MatchPublicAddr matches on the public address of an application.
func MatchPublicAddr(publicAddr string) Matcher {
	return func(app *types.App) bool {
		return app.PublicAddr == publicAddr
	}
}

// MatchName matches on the name of an application.
func MatchName(name string) Matcher {
	return func(app *types.App) bool {
		return app.Name == name
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
func ResolveFQDN(ctx context.Context, clt Getter, tunnel reversetunnel.Tunnel, proxyDNSNames []string, fqdn string) (*types.App, services.Server, string, error) {
	// Try and match FQDN to public address of application within cluster.
	application, server, err := Match(ctx, clt, MatchPublicAddr(fqdn))
	if err == nil {
		clusterName, err := clt.GetClusterName()
		if err != nil {
			return nil, nil, "", trace.Wrap(err)
		}
		return application, server, clusterName.GetClusterName(), nil
	}

	// Extract the first subdomain from the FQDN and attempt to use this as the
	// application name. The rest of the FQDN must match one of the local
	// cluster's proxy DNS names.
	fqdnParts := strings.SplitN(fqdn, ".", 2)
	if len(fqdnParts) != 2 {
		return nil, nil, "", trace.BadParameter("invalid FQDN: %v", fqdn)
	}
	if !utils.SliceContainsStr(proxyDNSNames, fqdnParts[1]) {
		return nil, nil, "", trace.BadParameter("FQDN %q is not a subdomain of the proxy", fqdn)
	}
	appName := fqdnParts[0]

	// Loop over all clusters and try and match application name to an
	// application within the cluster. This also includes the local cluster.
	clusterClients, err := tunnel.GetSites()
	if err != nil {
		return nil, nil, "", trace.Wrap(err)
	}
	for _, clusterClient := range clusterClients {
		authClient, err := clusterClient.CachingAccessPoint()
		if err != nil {
			return nil, nil, "", trace.Wrap(err)
		}

		application, server, err = Match(ctx, authClient, MatchName(appName))
		if err == nil {
			return application, server, clusterClient.GetName(), nil
		}
	}

	return nil, nil, "", trace.NotFound("failed to resolve %v to any application within any cluster", fqdn)
}
