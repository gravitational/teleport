// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// localAppProvider implements [appProvider] in the VNet user process.
// Its methods get exposed by [clientApplicationService] so that
// [remoteAppProvider] can by implemented by calling these methods from the VNet
// admin process.
type localAppProvider struct {
	cfg *localAppProviderConfig
}

type localAppProviderConfig struct {
	clientApplication  ClientApplication
	clusterConfigCache *ClusterConfigCache
}

func newLocalAppProvider(cfg *localAppProviderConfig) *localAppProvider {
	return &localAppProvider{
		cfg: cfg,
	}
}

// ResolveAppInfo implements [appProvider.ResolveAppInfo].
func (p *localAppProvider) ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error) {
	profileNames, err := p.cfg.clientApplication.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err, "listing profiles")
	}
	for _, profileName := range profileNames {
		if fqdn == fullyQualify(profileName) {
			// This is a query for the proxy address, which we'll never want to handle.
			// The DNS request must be forwarded upstream so that the VNet
			// process can always dial the proxy address without recursively
			// querying the VNet DNS nameserver.
			return nil, errNoTCPHandler
		}

		clusterClient, err := p.clusterClientForAppFQDN(ctx, profileName, fqdn)
		if err != nil {
			if errors.Is(err, errNoMatch) {
				continue
			}
			// The user might be logged out from this one cluster (and retryWithRelogin isn't working). Log
			// the error but don't return it so that DNS resolution will be forwarded upstream instead of
			// failing, to avoid breaking e.g. web app access (we don't know if this is a web or TCP app yet
			// because we can't log in).
			log.ErrorContext(ctx, "Failed to get teleport client.", "error", err)
			continue
		}

		leafClusterName := ""
		clusterName := clusterClient.ClusterName()
		if clusterName != "" && clusterName != clusterClient.RootClusterName() {
			leafClusterName = clusterName
		}

		return p.resolveAppInfoForCluster(ctx, clusterClient, profileName, leafClusterName, fqdn)
	}
	// fqdn did not match any profile, forward the request upstream.
	return nil, errNoTCPHandler
}

func (p *localAppProvider) clusterClientForAppFQDN(ctx context.Context, profileName, fqdn string) (ClusterClient, error) {
	rootClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, "")
	if err != nil {
		log.ErrorContext(ctx, "Failed to get root cluster client, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	if isDescendantSubdomain(fqdn, profileName) {
		// The queried app fqdn is a subdomain of this cluster proxy address.
		return rootClient, nil
	}

	leafClusters, err := getLeafClusters(ctx, rootClient)
	if err != nil {
		// Good chance we're here because the user is not logged in to the profile.
		log.ErrorContext(ctx, "Failed to list leaf clusters, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	// Prefix with an empty string to represent the root cluster.
	allClusters := append([]string{""}, leafClusters...)
	for _, leafClusterName := range allClusters {
		clusterClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get cluster client, apps in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get VNet config, apps in the cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}
		for _, zone := range clusterConfig.DNSZones {
			if isDescendantSubdomain(fqdn, zone) {
				return clusterClient, nil
			}
		}
	}
	return nil, errNoMatch
}

var errNoMatch = errors.New("cluster does not match queried FQDN")

func (p *localAppProvider) resolveAppInfoForCluster(
	ctx context.Context,
	clusterClient ClusterClient,
	profileName, leafClusterName, fqdn string,
) (*vnetv1.AppInfo, error) {
	log := log.With("profile", profileName, "leaf_cluster", leafClusterName, "fqdn", fqdn)
	// An app public_addr could technically be full-qualified or not, match either way.
	expr := fmt.Sprintf(`(resource.spec.public_addr == "%s" || resource.spec.public_addr == "%s") && hasPrefix(resource.spec.uri, "tcp://")`,
		strings.TrimSuffix(fqdn, "."), fqdn)
	resp, err := apiclient.GetResourcePage[types.AppServer](ctx, clusterClient.CurrentCluster(), &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: expr,
		Limit:               1,
	})
	if err != nil {
		// Don't return an unexpected error so we can try to find the app in different clusters or forward the
		// request upstream.
		log.InfoContext(ctx, "Failed to list application servers", "error", err)
		return nil, errNoTCPHandler
	}
	if len(resp.Resources) == 0 {
		// Didn't find any matching app, forward the request upstream.
		return nil, errNoTCPHandler
	}
	// At this point we have found a matching app in the cluster, any error is
	// unexpected and is preventing access to the app and should be returned to
	// the user.
	app, ok := resp.Resources[0].GetApp().(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("expected *types.AppV3, got %T", resp.Resources[0].GetApp())
	}
	clusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster VNet config for matching app", "error", err)
		return nil, trace.Wrap(err, "getting cached cluster VNet config for matching app")
	}
	dialOpts, err := p.cfg.clientApplication.GetDialOptions(ctx, profileName)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster dial options", "error", err)
		return nil, trace.Wrap(err, "getting dial options for matching app")
	}
	appInfo := &vnetv1.AppInfo{
		AppKey: &vnetv1.AppKey{
			Profile:     profileName,
			LeafCluster: leafClusterName,
			Name:        app.GetName(),
		},
		Cluster:       clusterClient.ClusterName(),
		App:           app,
		Ipv4CidrRange: clusterConfig.IPv4CIDRRange,
		DialOptions:   dialOpts,
	}
	return appInfo, nil
}

// ReissueAppCert implements [appProvider.ReissueAppCert].
func (p *localAppProvider) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error) {
	return p.cfg.clientApplication.ReissueAppCert(ctx, appInfo, targetPort)
}

// OnNewConnection implements [appProvider.OnNewConnection].
func (p *localAppProvider) OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	return p.cfg.clientApplication.OnNewConnection(ctx, appKey)
}

// OnInvalidLocalPort implements [appProvider.OnInvalidLocalPort].
func (p *localAppProvider) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) {
	p.cfg.clientApplication.OnInvalidLocalPort(ctx, appInfo, targetPort)
}
