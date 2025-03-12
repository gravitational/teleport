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
	"github.com/jonboulle/clockwork"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// ClientApplication is the common interface implemented by each VNet client
// application: Connect and tsh. It provides methods to list user profiles, get
// cluster clients, issue app certificates, and report metrics and errors -
// anything that uses the user's credentials or a Teleport client.
// The name "client application" refers to a user-facing client application, in
// constrast to the MacOS daemon or Windows service.
type ClientApplication interface {
	// ListProfiles lists the names of all profiles saved for the user.
	ListProfiles() ([]string, error)

	// GetCachedClient returns a [*client.ClusterClient] for the given profile and leaf cluster.
	// [leafClusterName] may be empty when requesting a client for the root cluster. Returned clients are
	// expected to be cached, as this may be called frequently.
	GetCachedClient(ctx context.Context, profileName, leafClusterName string) (ClusterClient, error)

	// ReissueAppCert issues a new cert for the target app.
	ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error)

	// GetDialOptions returns ALPN dial options for the profile.
	GetDialOptions(ctx context.Context, profileName string) (*vnetv1.DialOptions, error)

	// OnNewConnection gets called whenever a new connection is about to be established through VNet.
	// By the time OnNewConnection, VNet has already verified that the user holds a valid cert for the
	// app.
	//
	// The connection won't be established until OnNewConnection returns. Returning an error prevents
	// the connection from being made.
	OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error

	// OnInvalidLocalPort gets called before VNet refuses to handle a connection to a multi-port TCP app
	// because the provided port does not match any of the TCP ports in the app spec.
	OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16)
}

// ClusterClient is an interface defining the subset of [client.ClusterClient]
// methods used by via [ClientApplication].
type ClusterClient interface {
	CurrentCluster() authclient.ClientI
	ClusterName() string
	RootClusterName() string
}

// localAppProvider implements wraps a [ClientApplication] to implement
// appProvider.
type localAppProvider struct {
	ClientApplication
	clusterConfigCache *ClusterConfigCache
}

func newLocalAppProvider(clientApp ClientApplication, clock clockwork.Clock) *localAppProvider {
	return &localAppProvider{
		ClientApplication:  clientApp,
		clusterConfigCache: NewClusterConfigCache(clock),
	}
}

// ResolveAppInfo implements [appProvider.ResolveAppInfo].
func (p *localAppProvider) ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error) {
	profileNames, err := p.ClientApplication.ListProfiles()
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
	rootClient, err := p.ClientApplication.GetCachedClient(ctx, profileName, "")
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
		clusterClient, err := p.ClientApplication.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get cluster client, apps in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
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

func getLeafClusters(ctx context.Context, rootClient ClusterClient) ([]string, error) {
	var leafClusters []string
	nextPage := ""
	for {
		remoteClusters, nextPage, err := rootClient.CurrentCluster().ListRemoteClusters(ctx, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, rc := range remoteClusters {
			leafClusters = append(leafClusters, rc.GetName())
		}
		if nextPage == "" {
			return leafClusters, nil
		}
	}
}

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
	clusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster VNet config for matching app", "error", err)
		return nil, trace.Wrap(err, "getting cached cluster VNet config for matching app")
	}
	dialOpts, err := p.ClientApplication.GetDialOptions(ctx, profileName)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get cluster dial options", "error", err)
		return nil, trace.Wrap(err, "getting dial options for matching app")
	}
	appInfo := &vnetv1.AppInfo{
		AppKey: &vnetv1.AppKey{
			Profile:     profileName,
			LeafCluster: leafClusterName,
		},
		Cluster:       clusterClient.ClusterName(),
		App:           app,
		Ipv4CidrRange: clusterConfig.IPv4CIDRRange,
		DialOptions:   dialOpts,
	}
	return appInfo, nil
}

// getTargetOSConfiguration returns the configuration values that should be
// configured in the OS, including DNS zones that should be handled by the VNet
// DNS nameserver and the IPv4 CIDR ranges that should be routed to the VNet TUN
// interface. This is not all of the OS configuration values, only the ones that
// must be communicated from the client application to the admin process.
func (p *localAppProvider) getTargetOSConfiguration(ctx context.Context) (*vnetv1.GetTargetOSConfigurationResponse, error) {
	profiles, err := p.ClientApplication.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err, "listing profiles")
	}
	var targetOSConfig vnetv1.TargetOSConfiguration
	for _, profileName := range profiles {
		profileTargetConfig := p.targetOSConfigurationForProfile(ctx, profileName)
		targetOSConfig.DnsZones = append(targetOSConfig.DnsZones, profileTargetConfig.DnsZones...)
		targetOSConfig.Ipv4CidrRanges = append(targetOSConfig.Ipv4CidrRanges, profileTargetConfig.Ipv4CidrRanges...)
	}
	return &vnetv1.GetTargetOSConfigurationResponse{
		TargetOsConfiguration: &targetOSConfig,
	}, nil
}

// targetOSConfigurationForProfile does not return errors, it is better to
// configure VNet for any working profiles and log errors for failures.
func (p *localAppProvider) targetOSConfigurationForProfile(ctx context.Context, profileName string) *vnetv1.TargetOSConfiguration {
	targetOSConfig := &vnetv1.TargetOSConfiguration{}
	rootClusterClient, err := p.GetCachedClient(ctx, profileName, "" /*leafClusterName*/)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to get root cluster client from cache, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	rootClusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, rootClusterClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to load VNet configuration, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	targetOSConfig.DnsZones = rootClusterConfig.DNSZones
	targetOSConfig.Ipv4CidrRanges = []string{rootClusterConfig.IPv4CIDRRange}

	leafClusterNames, err := getLeafClusters(ctx, rootClusterClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to list leaf clusters, profile may be expired, not configuring VNet for leaf clusters of this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	for _, leafClusterName := range leafClusterNames {
		leafClusterClient, err := p.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to create leaf cluster client, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			return targetOSConfig
		}
		leafClusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, leafClusterClient)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to load VNet configuration, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			return targetOSConfig
		}
		targetOSConfig.DnsZones = append(targetOSConfig.DnsZones, leafClusterConfig.DNSZones...)
		targetOSConfig.Ipv4CidrRanges = append(targetOSConfig.Ipv4CidrRanges, leafClusterConfig.IPv4CIDRRange)
	}
	return targetOSConfig
}
