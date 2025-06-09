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
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// fqdnResolver resolves fully-qualified domain names to possible VNet targets.
type fqdnResolver struct {
	cfg *fqdnResolverConfig
}

type fqdnResolverConfig struct {
	clientApplication  ClientApplication
	clusterConfigCache *ClusterConfigCache
	leafClusterCache   *leafClusterCache
}

func newFQDNResolver(cfg *fqdnResolverConfig) *fqdnResolver {
	return &fqdnResolver{
		cfg: cfg,
	}
}

// ResolveFQDN resolves queries for fully-qualified domain names to possible
// VNet target matches. It returns [errNoTCPHandler] if VNet should not handle
// this address and DNS queries should be forwarded upstream.
func (r *fqdnResolver) ResolveFQDN(ctx context.Context, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	profileNames, err := r.cfg.clientApplication.ListProfiles()
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
	}
	// First try to resolve a matching TCP or web app. A matching app is more
	// specific than the cluster match we do for SSH, and checking for a
	// matching app first maintains backward compatibility because apps were
	// supported before SSH.
	resp, err := r.tryResolveApp(ctx, profileNames, fqdn)
	switch {
	case err == nil:
		return resp, nil
	case !errors.Is(err, errNoMatch):
		return nil, err
	}
	resp, err = r.tryResolveSSH(ctx, profileNames, fqdn)
	switch {
	case err == nil:
		return resp, nil
	case !errors.Is(err, errNoMatch):
		return nil, err
	}
	return nil, errNoTCPHandler
}

var errNoMatch = errors.New("no match for queried FQDN")

func (r *fqdnResolver) tryResolveApp(ctx context.Context, profileNames []string, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	for _, profileName := range profileNames {
		clusterClient, err := r.clusterClientForAppFQDN(ctx, profileName, fqdn)
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

		return r.resolveAppInfoForCluster(ctx, clusterClient, profileName, leafClusterName, fqdn)
	}
	return nil, errNoMatch
}

func (r *fqdnResolver) clusterClientForAppFQDN(ctx context.Context, profileName, fqdn string) (ClusterClient, error) {
	rootClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, "")
	if err != nil {
		log.ErrorContext(ctx, "Failed to get root cluster client, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	if isDescendantSubdomain(fqdn, profileName) {
		// The queried FQDN is a subdomain of this cluster proxy address.
		return rootClient, nil
	}

	leafClusters, err := r.cfg.leafClusterCache.getLeafClusters(ctx, rootClient)
	if err != nil {
		// Good chance we're here because the user is not logged in to the profile.
		log.ErrorContext(ctx, "Failed to list leaf clusters, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	// Prefix with an empty string to represent the root cluster.
	allClusters := append([]string{""}, leafClusters...)
	for _, leafClusterName := range allClusters {
		clusterClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get cluster client, apps in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get VNet config, apps in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
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

func (r *fqdnResolver) resolveAppInfoForCluster(
	ctx context.Context,
	clusterClient ClusterClient,
	profileName, leafClusterName, fqdn string,
) (*vnetv1.ResolveFQDNResponse, error) {
	log := log.With("profile", profileName, "leaf_cluster", leafClusterName, "fqdn", fqdn)
	// An app public_addr could technically be fully-qualified or not, match either way.
	expr := fmt.Sprintf(`resource.spec.public_addr == "%s" || resource.spec.public_addr == "%s"`,
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
		return nil, errNoMatch
	}
	if len(resp.Resources) == 0 {
		// Didn't find any matching app, forward the request upstream.
		return nil, errNoMatch
	}
	// At this point we have found a matching app in the cluster, any error is
	// unexpected and is preventing access to the app and should be returned to
	// the user.
	app, ok := resp.Resources[0].GetApp().(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("expected *types.AppV3, got %T", resp.Resources[0].GetApp())
	}
	if !app.IsTCP() {
		log.InfoContext(ctx, "Query matched a web app")
		// If not a TCP app this must be a web app and we can return early.
		return &vnetv1.ResolveFQDNResponse{
			Match: &vnetv1.ResolveFQDNResponse_MatchedWebApp{
				MatchedWebApp: &vnetv1.MatchedWebApp{},
			},
		}, nil
	}
	clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster VNet config for matching app", "error", err)
		return nil, trace.Wrap(err, "getting cached cluster VNet config for matching app")
	}
	dialOpts, err := r.cfg.clientApplication.GetDialOptions(ctx, profileName)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster dial options", "error", err)
		return nil, trace.Wrap(err, "getting dial options for matching app")
	}
	log.InfoContext(ctx, "Query matched a TCP app")
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
	return &vnetv1.ResolveFQDNResponse{
		Match: &vnetv1.ResolveFQDNResponse_MatchedTcpApp{
			MatchedTcpApp: &vnetv1.MatchedTCPApp{
				AppInfo: appInfo,
			},
		},
	}, nil
}

// VNet SSH handles SSH hostnames matching "<hostname>.<cluster_name>." or
// "<hostname>.<leaf_cluster_name>.<cluster_name>.". tryResolveSSH checks if
// fqdn matches that pattern for any logged-in cluster and if so returns a
// match. We never actually query for whether or not a matching SSH node exists,
// we just attempt to dial it when the client connects to the assigned IP.
func (r *fqdnResolver) tryResolveSSH(ctx context.Context, profileNames []string, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	for _, profileName := range profileNames {
		log := log.With("profile", profileName)
		rootClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, "")
		if err != nil {
			log.ErrorContext(ctx, "Failed to get root cluster client, SSH nodes in this cluster will not be resolved", "error", err)
			continue
		}
		rootClusterName := rootClient.ClusterName()
		if !isDescendantSubdomain(fqdn, rootClusterName) {
			continue
		}
		leafClusters, err := r.cfg.leafClusterCache.getLeafClusters(ctx, rootClient)
		if err != nil {
			// Good chance we're here because the user is not logged in to the profile.
			log.ErrorContext(ctx, "Failed to list leaf clusters, SSH nodes in this cluster will not be resolved", "error", err)
			return nil, errNoMatch
		}
		rootDialOpts, err := r.cfg.clientApplication.GetDialOptions(ctx, profileName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get cluster dial options, SSH nodes in this cluster will not be resolved", "error", err)
			return nil, errNoMatch
		}
		for _, leafClusterName := range leafClusters {
			log := log.With("leaf_cluster", leafClusterName)
			if !isDescendantSubdomain(fqdn, leafClusterName+"."+rootClusterName) {
				continue
			}
			leafClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get cluster client, SSH nodes in this cluster will not be resolved", "error", err)
				return nil, errNoMatch
			}
			clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, leafClient)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get VNet config, SSH nodes in this cluster will not be resolved", "error", err)
				return nil, errNoMatch
			}
			log.InfoContext(ctx, "Query matched a leaf cluster subdomain and may later resolve to an app or SSH node")
			return &vnetv1.ResolveFQDNResponse{
				Match: &vnetv1.ResolveFQDNResponse_MatchedCluster{
					MatchedCluster: &vnetv1.MatchedCluster{
						WebProxyAddr:  rootDialOpts.GetWebProxyAddr(),
						Ipv4CidrRange: clusterConfig.IPv4CIDRRange,
						Profile:       profileName,
						RootCluster:   rootClusterName,
						LeafCluster:   leafClusterName,
					},
				},
			}, nil
		}
		// If it didn't match any leaf cluster assume it matches the root
		// cluster.
		clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, rootClient)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get VNet config, SSH nodes in this cluster will not be resolved", "error", err)
			return nil, errNoMatch
		}
		log.InfoContext(ctx, "Query matched a root cluster subdomain and may later resolve to an app or SSH node")
		return &vnetv1.ResolveFQDNResponse{
			Match: &vnetv1.ResolveFQDNResponse_MatchedCluster{
				MatchedCluster: &vnetv1.MatchedCluster{
					WebProxyAddr:  rootDialOpts.GetWebProxyAddr(),
					Ipv4CidrRange: clusterConfig.IPv4CIDRRange,
					Profile:       profileName,
					RootCluster:   rootClusterName,
				},
			},
		}, nil
	}
	return nil, errNoMatch
}

// isDescendantSubdomain checks if fqdn is a subdomain of zone.
func isDescendantSubdomain(fqdn, zone string) bool {
	return strings.HasSuffix(fqdn, "."+fullyQualify(zone))
}

// fullyQualify returns a fully-qualified domain name from [domain].
// Fully-qualified domain names always end with a ".".
func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}
