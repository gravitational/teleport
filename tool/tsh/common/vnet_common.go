// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/vnet"
)

// vnetAppProvider implement [vnet.AppProvider] in order to provide the necessary methods to log in to apps
// and get clients able to list apps in all clusters in all current profiles.
type vnetAppProvider struct {
	cf          *CLIConf
	clientStore *client.Store
	clientCache *clientcache.Cache
	loginMu     sync.Mutex
}

func newVnetAppProvider(cf *CLIConf) (*vnetAppProvider, error) {
	clientStore := client.NewFSClientStore(cf.HomePath)

	p := &vnetAppProvider{
		cf:          cf,
		clientStore: clientStore,
	}

	clientCache, err := clientcache.New(clientcache.Config{
		NewClientFunc:        clientcache.NewClientFunc(p.newTeleportClient),
		RetryWithReloginFunc: clientcache.RetryWithReloginFunc(p.retryWithRelogin),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating client cache")
	}

	p.clientCache = clientCache
	return p, nil

}

// ListProfiles lists the names of all profiles saved for the user.
func (p *vnetAppProvider) ListProfiles() ([]string, error) {
	return p.clientStore.ListProfiles()
}

// GetCachedClient returns a cached [*client.ClusterClient] for the given profile and leaf cluster.
// [leafClusterName] may be empty when requesting a client for the root cluster.
func (p *vnetAppProvider) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (vnet.ClusterClient, error) {
	return p.clientCache.Get(ctx, profileName, leafClusterName)
}

// ReissueAppCert returns a new app certificate for the given app in the named profile and leaf cluster.
// It uses retryWithRelogin to issue the new app cert. A relogin may not be necessary if the app cert lifetime
// was shorter than the cluster cert lifetime, or if the user has already re-logged in to the cluster.
// If a cluster relogin is completed, the cluster client cache will be cleared for the root cluster and all
// leaf clusters of that root.
func (p *vnetAppProvider) ReissueAppCert(ctx context.Context, profileName, leafClusterName string, app types.Application) (tls.Certificate, error) {
	tc, err := p.newTeleportClient(ctx, profileName, leafClusterName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	var cert tls.Certificate
	err = p.retryWithRelogin(ctx, tc, func() error {
		var err error
		cert, err = p.reissueAppCert(ctx, tc, profileName, leafClusterName, app)
		return trace.Wrap(err, "reissuing app cert")
	})
	return cert, trace.Wrap(err)
}

// GetDialOptions returns ALPN dial options for the profile.
func (p *vnetAppProvider) GetDialOptions(ctx context.Context, profileName string) (*vnet.DialOptions, error) {
	profile, err := p.clientStore.GetProfile(profileName)
	if err != nil {
		return nil, trace.Wrap(err, "loading user profile")
	}
	dialOpts := &vnet.DialOptions{
		WebProxyAddr:            profile.WebProxyAddr,
		ALPNConnUpgradeRequired: profile.TLSRoutingConnUpgradeRequired,
		InsecureSkipVerify:      p.cf.InsecureSkipVerify,
	}
	if dialOpts.ALPNConnUpgradeRequired {
		dialOpts.RootClusterCACertPool, err = p.getRootClusterCACertPool(ctx, profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return dialOpts, nil
}

// OnNewConnection gets called before each VNet connection. It's a noop as tsh doesn't need to do
// anything extra here.
func (p *vnetAppProvider) OnNewConnection(ctx context.Context, profileName, leafClusterName string, app types.Application) error {
	return nil
}

// getRootClusterCACertPool returns a certificate pool for the root cluster of the given profile.
func (p *vnetAppProvider) getRootClusterCACertPool(ctx context.Context, profileName string) (*x509.CertPool, error) {
	tc, err := p.newTeleportClient(ctx, profileName, "")
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}
	certPool, err := tc.RootClusterCACertPool(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading root cluster CA cert pool")
	}
	return certPool, nil
}

func (p *vnetAppProvider) retryWithRelogin(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
	profileName, err := utils.Host(tc.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	// Make sure the release the login mutex if we end up acquiring it.
	didLock := false
	defer func() {
		if didLock {
			p.loginMu.Unlock()
		}
	}()

	opts = append(opts,
		client.WithBeforeLoginHook(func() error {
			// Multiple concurrent logins in tsh would be bad UX, especially when MFA is involved, so we only
			// allow one login at a time. If another login is already in progress this just returns an error
			// and no login will be attempted. Subsequent relogins can be attempted on the next client request
			// after the current one finishes.
			if p.loginMu.TryLock() {
				didLock = true
			} else {
				return fmt.Errorf("not attempting re-login to cluster %s, another login is current in progress.", tc.SiteName)
			}
			fmt.Printf("Login for cluster %s expired, attempting to log in again.\n", tc.SiteName)
			return nil
		}),
		client.WithAfterLoginHook(func() error {
			return trace.Wrap(p.clientCache.ClearForRoot(profileName), "clearing client cache after relogin")
		}),
		client.WithMakeCurrentProfile(false),
	)
	return client.RetryWithRelogin(ctx, tc, fn, opts...)
}

func (p *vnetAppProvider) reissueAppCert(ctx context.Context, tc *client.TeleportClient, profileName, leafClusterName string, app types.Application) (tls.Certificate, error) {
	slog.InfoContext(ctx, "Reissuing cert for app.", "app_name", app.GetName(), "profile", profileName, "leaf_cluster", leafClusterName)

	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: tc.SiteName,
		URI:         app.GetURI(),
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "loading client profile")
	}

	appCertParams := client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToApp:     routeToApp,
		AccessRequests: profile.ActiveRequests,
		RequesterName:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
		TTL:            tc.KeyTTL,
	}

	clusterClient, err := p.clientCache.Get(ctx, profileName, leafClusterName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting cached cluster client")
	}
	rootClient, err := p.clientCache.Get(ctx, profileName, "")
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting cached root client")
	}

	key, err := appLogin(ctx, tc, clusterClient, rootClient.AuthClient, appCertParams)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "logging in to app")
	}

	cert, err := key.AppTLSCert(app.GetName())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting TLS cert from key")
	}

	return cert, nil
}

func (p *vnetAppProvider) newTeleportClient(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
	cfg := &client.Config{
		ClientStore: p.clientStore,
	}
	if err := cfg.LoadProfile(p.clientStore, profileName); err != nil {
		return nil, trace.Wrap(err, "loading client profile")
	}
	if leafClusterName != "" {
		cfg.SiteName = leafClusterName
	}
	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}
	return tc, nil
}
