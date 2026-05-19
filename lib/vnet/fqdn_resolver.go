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
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"
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

	var matchedClusters []clusterResolutionCandidate
	for candidate := range r.clusterResolutionCandidates(ctx, profileNames, fqdn) {
		// First check if there's a matching app in this cluster.
		result, err := r.resolveAppInfoForCluster(ctx, candidate, fqdn)
		switch {
		case err == nil:
			// Found a matching app, return it immediately.
			return result, nil
		case !errors.Is(err, errNoMatch):
			return nil, err
		}

		// SSH servers match at any subdomain of the cluster name. For example,
		// a query for "foo.bar.teleport.example.com" should match the cluster
		// "teleport.example.com" so that it may resolve for an SSH node named
		// "foo.bar".
		//
		// This doesn't return immediately so that an app found in a later
		// cluster still takes precedence.
		if isDescendantSubdomain(fqdn, candidate.clusterName) {
			matchedClusters = append(matchedClusters, candidate)
		}
	}

	if len(matchedClusters) == 0 {
		return nil, errNoTCPHandler
	}

	result, err := r.resolveClusterMatch(ctx, log, matchedClusters)
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, errNoMatch):
		return nil, errNoTCPHandler
	default:
		return nil, trace.Wrap(err)
	}
}

var errNoMatch = errors.New("no match for queried FQDN")

type clusterResolutionCandidate struct {
	client          ClusterClient
	profileName     string
	rootClusterName string
	leafClusterName string
	clusterName     string
}

// clusterResolutionCandidates yields all clusterResolutionCandidates for a
// queried FQDN. A clusterResolutionCandidate essentially identifies a specific
// Teleport cluster, and includes a client for the cluster and some metadata
// that may be helpful for deciding if the queried FQDN should really resolve
// to a match in that cluster.
//
// It does a first-pass filter to avoid yielding any clusters that definitely
// won't match for the queried FQDN, using only cached information without
// doing any actual queries to the cluster.
func (r *fqdnResolver) clusterResolutionCandidates(ctx context.Context, profileNames []string, fqdn string) iter.Seq[clusterResolutionCandidate] {
	return func(yield func(clusterResolutionCandidate) bool) {
		for _, profileName := range profileNames {
			for candidate := range r.clusterResolutionCandidatesInProfile(ctx, profileName, fqdn) {
				if !yield(candidate) {
					return
				}
			}
		}
	}
}

// clusterResolutionCandidatesInProfile yields all clusterResolutionCandidates
// in a specific profile for a queried FQDN.
//
// It does a first-pass filter to avoid yielding any clusters that definitely
// won't match for the queried FQDN, using only cached information without
// doing any actual queries to the cluster.
//
// A cluster candidate will be yielded in the following cases:
//
//  1. If fqdn is a direct (one-level) subdomain of the profileName it may
//     match an app in the root cluster or any leaf cluster.
//  2. If fqdn is any subdomain of the cluster name it may match an SSH node
//     in that cluster.
//  3. If fqdn is any subdomain of any of the app DNS zones for the cluster
//     (this includes the proxy public_addr and any configured custom DNS
//     zones)
func (r *fqdnResolver) clusterResolutionCandidatesInProfile(ctx context.Context, profileName string, fqdn string) iter.Seq[clusterResolutionCandidate] {
	return func(yield func(clusterResolutionCandidate) bool) {
		// A direct subdomain of the root cluster proxy public addr (which is
		// the profileName) may be a web app in a leaf cluster, which needs to
		// be reachable at <app-name>.<root-proxy-public-addr>. It also may be
		// an app in the root cluster, so we can/must yield every client before
		// checking configured DNS zones.
		shouldYieldAllCandidates := isDirectSubdomain(fqdn, profileName)

		shouldYieldCandidate := func(candidate clusterResolutionCandidate) bool {
			if shouldYieldAllCandidates {
				return true
			}

			if isDescendantSubdomain(fqdn, candidate.clusterName) {
				// This may match an SSH server, must yield the client.
				return true
			}

			clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, candidate.client)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get VNet config, apps in this cluster may not be resolved.", "profile", profileName, "leaf_cluster", candidate.leafClusterName, "error", err)
				return false
			}
			for _, zone := range clusterConfig.appDNSZones() {
				if isDescendantSubdomain(fqdn, zone) {
					return true
				}
			}

			return false
		}

		rootClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, "")
		if err != nil {
			log.ErrorContext(ctx, "Failed to get root cluster client, resources in this cluster will not be resolved.", "profile", profileName, "error", err)
			return
		}
		rootClusterName := rootClient.ClusterName()

		leafClusters, err := r.cfg.leafClusterCache.getLeafClusters(ctx, rootClient)
		if err != nil {
			// Good chance we're here because the user is not logged in to the profile.
			log.ErrorContext(ctx, "Failed to list leaf clusters, resources in leaves of this cluster will not be resolved.", "profile", profileName, "error", err)
			// Don't return so that the root cluster may still work even if
			// leaf cluster listing is broken.
			leafClusters = nil
		}

		// Prefix with an empty string to represent the root cluster.
		// GetCachedClient will return the root client if given an empty string for leafClusterName.
		leafClusters = append([]string{""}, leafClusters...)

		for _, leafClusterName := range leafClusters {
			clusterClient, err := r.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get cluster client, resources in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
				continue
			}

			candidate := clusterResolutionCandidate{
				client:          clusterClient,
				profileName:     profileName,
				rootClusterName: rootClusterName,
				leafClusterName: leafClusterName,
				clusterName:     clusterClient.ClusterName(),
			}

			if !shouldYieldCandidate(candidate) {
				continue
			}

			if !yield(candidate) {
				return
			}
		}
	}
}

func (r *fqdnResolver) resolveAppInfoForCluster(
	ctx context.Context,
	candidate clusterResolutionCandidate,
	fqdn string,
) (*vnetv1.ResolveFQDNResponse, error) {
	log := log.With("profile", candidate.profileName, "leaf_cluster", candidate.leafClusterName, "fqdn", fqdn)

	// An app public_addr could technically be fully-qualified or not, match either way.
	expr := fmt.Sprintf(`resource.spec.public_addr == "%s" || resource.spec.public_addr == "%s"`,
		strings.TrimSuffix(fqdn, "."), fqdn)

	// If candidate is a leaf cluster and fqdn possibly points to a leaf
	// cluster app, also query for the app by name.
	//
	// Apps in leaf clusters are reachable through the root cluster at
	// <app-name>.<root-proxy-addr>. They can only be queried in the leaf
	// cluster by name. The root proxy address is always the profile name, so
	// any queried FQDN that is a direct subdomain of the profile name may be
	// for a web app in a leaf cluster.
	var potentialAppName string
	if candidate.leafClusterName != "" && isDirectSubdomain(fqdn, candidate.profileName) {
		potentialAppName = strings.TrimSuffix(fqdn, "."+fullyQualify(candidate.profileName))
		expr += fmt.Sprintf(` || name == "%s"`, potentialAppName)
	}

	resp, err := apiclient.GetResourcePage[types.AppServer](ctx, candidate.client.CurrentCluster(), &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: expr,
		Limit:               10,
	})
	if err != nil {
		// Don't return an unexpected error so we can try to find the app in different clusters or forward the
		// request upstream.
		log.InfoContext(ctx, "Failed to list application servers", "error", err)
		return nil, errNoMatch
	}
	if len(resp.Resources) == 0 {
		log.DebugContext(ctx, "Found no matching app servers")
		// Didn't find any matching app, forward the request upstream.
		return nil, errNoMatch
	}

	selectBestAppMatch := func() (*types.AppV3, error) {
		var matchedWebApp *types.AppV3
		for _, resource := range resp.Resources {
			app, ok := resource.GetApp().(*types.AppV3)
			if !ok {
				return nil, trace.BadParameter("expected *types.AppV3, got %T", resource.GetApp())
			}
			matchedByPublicAddr := fullyQualify(app.GetPublicAddr()) == fqdn
			matchedByName := app.GetName() == potentialAppName
			if !matchedByName && !matchedByPublicAddr {
				// This shouldn't happen if the backend query worked correctly.
				log.WarnContext(ctx, "Query returned an app that did not match by name or public_addr",
					"name", app.GetName(), "public_addr", app.GetPublicAddr())
				continue
			}
			if app.IsTCP() {
				if matchedByPublicAddr {
					// Greedily prefer to match an arbitrary TCP app by public addr.
					return app, nil
				}
				// Skip TCP apps that only matched by name, VNet only handles
				// TCP apps that match a public addr.
			} else {
				matchedWebApp = app
			}
		}
		if matchedWebApp != nil {
			return matchedWebApp, nil
		}
		// Didn't find any matching app.
		return nil, errNoMatch
	}

	app, err := selectBestAppMatch()
	if err != nil {
		return nil, err
	}

	// At this point we have found a matching app in the cluster, any error is
	// unexpected and is preventing access to the app and should be returned to
	// the user.
	if !app.IsTCP() {
		log.InfoContext(ctx, "Query matched a web app")
		// If not a TCP app this must be a web app and we can return early.
		return &vnetv1.ResolveFQDNResponse{
			Match: &vnetv1.ResolveFQDNResponse_MatchedWebApp{
				MatchedWebApp: &vnetv1.MatchedWebApp{},
			},
		}, nil
	}
	clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, candidate.client)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster VNet config for matching app", "error", err)
		return nil, trace.Wrap(err, "getting cached cluster VNet config for matching app")
	}
	dialOpts, err := r.cfg.clientApplication.GetDialOptions(ctx, candidate.profileName)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster dial options", "error", err)
		return nil, trace.Wrap(err, "getting dial options for matching app")
	}
	log.InfoContext(ctx, "Query matched a TCP app")
	appInfo := &vnetv1.AppInfo{
		AppKey: &vnetv1.AppKey{
			Profile:     candidate.profileName,
			LeafCluster: candidate.leafClusterName,
			Name:        app.GetName(),
		},
		Cluster:       candidate.clusterName,
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

// VNet SSH handles SSH hostnames matching "<hostname>.<cluster_name>.", where
// the <cluster-name> may be the name of a root or leaf cluster.
// We never actually query for whether or not a matching SSH node exists, we
// just attempt to dial it when the client connects to the assigned IP.
func (r *fqdnResolver) resolveClusterMatch(ctx context.Context, log *slog.Logger, matchedClusters []clusterResolutionCandidate) (*vnetv1.ResolveFQDNResponse, error) {
	// Longer cluster name matches are preferred, this is so that
	// node.leaf.example.com will preferentially resolve to a match in
	// leaf.example.com if there is another cluster named example.com
	slices.SortFunc(matchedClusters, func(a, b clusterResolutionCandidate) int {
		return cmp.Compare(len(b.clusterName), len(a.clusterName))
	})
	for _, matchedCluster := range matchedClusters {
		rootDialOpts, err := r.cfg.clientApplication.GetDialOptions(ctx, matchedCluster.profileName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get cluster dial options, SSH nodes in this cluster will not be resolved", "error", err)
			continue
		}

		clusterConfig, err := r.cfg.clusterConfigCache.GetClusterConfig(ctx, matchedCluster.client)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get VNet config, SSH nodes in this cluster will not be resolved", "error", err)
			continue
		}

		log.InfoContext(ctx, "Query matched a cluster subdomain and may later resolve to an app or SSH node",
			"cluster_name", matchedCluster.clusterName)
		return &vnetv1.ResolveFQDNResponse{
			Match: &vnetv1.ResolveFQDNResponse_MatchedCluster{
				MatchedCluster: &vnetv1.MatchedCluster{
					WebProxyAddr:  rootDialOpts.GetWebProxyAddr(),
					Ipv4CidrRange: clusterConfig.IPv4CIDRRange,
					Profile:       matchedCluster.profileName,
					RootCluster:   matchedCluster.rootClusterName,
					LeafCluster:   matchedCluster.leafClusterName,
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

// isDirectSubdomain checks if fqdn is a direct single-level subdomain of zone.
func isDirectSubdomain(fqdn, zone string) bool {
	trimmed := strings.TrimSuffix(fqdn, "."+fullyQualify(zone))
	if trimmed == fqdn {
		return false
	}
	return !strings.ContainsRune(trimmed, '.')
}

// fullyQualify returns a fully-qualified domain name from [domain].
// Fully-qualified domain names always end with a ".".
func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}
