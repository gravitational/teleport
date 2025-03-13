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
	"os"
	"sync/atomic"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/piv"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// profileOSConfigProvider implements targetOSConfigProvider for the MacOS
// daemon process. It reads all active profiles from the user's TELEPORT_HOME
// and uses the user certs found there to dial to each cluster to find the
// current vnet_config resource to compute the current target OS configuration.
type profileOSConfigProvider struct {
	clientStore        *client.Store
	clientCache        *clientcache.Cache
	clusterConfigCache *ClusterConfigCache
	// daemonClientCred are the credentials of the process that contacted the daemon.
	daemonClientCred daemon.ClientCred
	tunName          string
	tunIPv6          string
	dnsAddr          string
	homePath         string
	tunIPv4          string
}

func newProfileOSConfigProvider(tunName, ipv6Prefix, dnsAddr, homePath string, daemonClientCred daemon.ClientCred) (*profileOSConfigProvider, error) {
	if homePath == "" {
		// This runs as root so we need to be configured with the user's home path.
		return nil, trace.BadParameter("homePath must be passed from unprivileged process")
	}
	tunIPv6, err := tunIPv6ForPrefix(ipv6Prefix)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hwKeyService := piv.NewYubiKeyService(context.TODO(), &hardwarekey.CLIPrompt{})

	p := &profileOSConfigProvider{
		clientStore:        client.NewFSClientStore(homePath, hwKeyService),
		clusterConfigCache: NewClusterConfigCache(clockwork.NewRealClock()),
		daemonClientCred:   daemonClientCred,
		tunName:            tunName,
		tunIPv6:            tunIPv6,
		dnsAddr:            dnsAddr,
		homePath:           homePath,
	}
	clientCache, err := clientcache.New(clientcache.Config{
		NewClientFunc: p.getClient,
		RetryWithReloginFunc: func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
			// profileOSConfigProvider runs in the MacOS daemon process, there's no way for it to relogin.
			// Instead, osConfigurator depends on the user performing a relogin from another process.
			return trace.Wrap(fn())
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.clientCache = clientCache
	return p, nil
}

func (p *profileOSConfigProvider) targetOSConfig(ctx context.Context) (*osConfig, error) {
	var (
		dnsZones   []string
		cidrRanges []string
	)

	// Drop privileges to ensure that the user who spawned the daemon client has privileges necessary
	// to access p.homePath that it sent when starting the daemon.
	// Otherwise a client could make the daemon read a profile out of any directory.
	if err := doWithDroppedRootPrivileges(ctx, p.daemonClientCred, func() error {
		profileNames, err := profile.ListProfileNames(p.homePath)
		if err != nil {
			return trace.Wrap(err, "listing user profiles")
		}
		for _, profileName := range profileNames {
			profileDNSZones, profileCIDRRanges := p.getDNSZonesAndCIDRRangesForProfile(ctx, profileName)
			dnsZones = append(dnsZones, profileDNSZones...)
			cidrRanges = append(cidrRanges, profileCIDRRanges...)
		}
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	dnsZones = utils.Deduplicate(dnsZones)
	cidrRanges = utils.Deduplicate(cidrRanges)

	if p.tunIPv4 == "" && len(cidrRanges) > 0 {
		// Choose an IPv4 address for the TUN interface from the CIDR range of one arbitrary currently
		// logged-in cluster. Only one IPv4 address is needed.
		if err := p.setTunIPv4FromCIDR(cidrRanges[0]); err != nil {
			return nil, trace.Wrap(err, "setting TUN IPv4 address")
		}
	}

	return &osConfig{
		tunName:    p.tunName,
		tunIPv6:    p.tunIPv6,
		tunIPv4:    p.tunIPv4,
		dnsAddr:    p.dnsAddr,
		dnsZones:   dnsZones,
		cidrRanges: cidrRanges,
	}, nil
}

// getDNSZonesAndCIDRRangesForProfile returns DNS zones and CIDR ranges for the root cluster and its
// leaf clusters.
//
// It's important for this function to return any data it manages to collect. For example, if it
// manages to grab DNS zones and CIDR ranges of the root cluster but it fails to list leaf clusters,
// it should still return the zones and ranges of the root cluster. Hence the use of named return
// values.
func (p *profileOSConfigProvider) getDNSZonesAndCIDRRangesForProfile(ctx context.Context, profileName string) (dnsZones []string, cidrRanges []string) {
	shouldClearCacheForRoot := true
	defer func() {
		if shouldClearCacheForRoot {
			if err := p.clientCache.ClearForRoot(profileName); err != nil {
				log.ErrorContext(ctx, "Error while clearing client cache", "profile", profileName, "error", err)
			}
		}
	}()

	rootClient, err := p.clientCache.Get(ctx, profileName, "" /*leafClusterName*/)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to get root cluster client from cache, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)

		return
	}
	clusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, rootClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to load VNet configuration, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)

		return
	}

	dnsZones = append(dnsZones, clusterConfig.DNSZones...)
	cidrRanges = append(cidrRanges, clusterConfig.IPv4CIDRRange)

	leafClusters, err := getLeafClusters(ctx, rootClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to list leaf clusters, profile may be expired, not configuring VNet for leaf clusters of this cluster",
			"profile", profileName, "error", err)

		return
	}

	// getLeafClusters was the last call using the root client. Do not clear cache if any call to
	// a leaf cluster fails – it might fail because of a problem with the leaf cluster, not because of
	// an expired cert.
	shouldClearCacheForRoot = false

	for _, leafClusterName := range leafClusters {
		clusterClient, err := p.clientCache.Get(ctx, profileName, leafClusterName)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to create leaf cluster client, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := p.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to load VNet configuration, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		dnsZones = append(dnsZones, clusterConfig.DNSZones...)
		cidrRanges = append(cidrRanges, clusterConfig.IPv4CIDRRange)
	}

	return
}

func (p *profileOSConfigProvider) setTunIPv4FromCIDR(cidrRange string) error {
	if p.tunIPv4 != "" {
		return nil
	}
	ip, err := tunIPv4ForCIDR(cidrRange)
	if err != nil {
		return trace.Wrap(err, "setting TUN IPv4 address for range %s", cidrRange)
	}
	p.tunIPv4 = ip
	return nil
}

func (p *profileOSConfigProvider) getClient(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
	// This runs in the root process, so obviously we don't have access to the client cache in the user
	// process. This loads cluster profiles and credentials from TELEPORT_HOME.
	clientConfig := &client.Config{
		ClientStore: p.clientStore,
	}
	if err := clientConfig.LoadProfile(p.clientStore, profileName); err != nil {
		return nil, trace.Wrap(err, "loading client profile")
	}
	if leafClusterName != "" {
		clientConfig.SiteName = leafClusterName
	}
	tc, err := client.NewClient(clientConfig)
	return tc, trace.Wrap(err)
}

var hasDroppedPrivileges atomic.Bool

// doWithDroppedRootPrivileges drops the privileges of the current process to those of the client
// process that called the VNet daemon.
func doWithDroppedRootPrivileges(ctx context.Context, clientCred daemon.ClientCred, fn func() error) (err error) {
	if !hasDroppedPrivileges.CompareAndSwap(false, true) {
		// At the moment of writing, the VNet daemon wasn't expected to do multiple things in parallel
		// with dropped privileges. If you run into this error, consider if employing a mutex is going
		// to be enough or if a more elaborate refactoring is required.
		return trace.CompareFailed("privileges are being temporarily dropped already")
	}
	defer hasDroppedPrivileges.Store(false)

	rootEgid := os.Getegid()
	rootEuid := os.Geteuid()

	log.InfoContext(ctx, "Temporarily dropping root privileges.", "egid", clientCred.Egid, "euid", clientCred.Euid)

	if err := syscall.Setegid(clientCred.Egid); err != nil {
		panic(trace.Wrap(err, "setting egid"))
	}
	if err := syscall.Seteuid(clientCred.Euid); err != nil {
		panic(trace.Wrap(err, "setting euid"))
	}

	defer func() {
		if err := syscall.Seteuid(rootEuid); err != nil {
			panic(trace.Wrap(err, "reverting euid"))
		}
		if err := syscall.Setegid(rootEgid); err != nil {
			panic(trace.Wrap(err, "reverting egid"))
		}

		log.InfoContext(ctx, "Restored root privileges.", "egid", rootEgid, "euid", rootEuid)
	}()

	return trace.Wrap(fn())
}
