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
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/vnet"
)

// vnetClientApplication implements [vnet.ClientApplication] in order to provide
// the necessary methods to list and log in to apps.
type vnetClientApplication struct {
	cf          *CLIConf
	clientStore *client.Store
	clientCache *clientcache.Cache
	loginMu     sync.Mutex
}

func newVnetClientApplication(cf *CLIConf) (*vnetClientApplication, error) {
	hwKeyService := keys.NewYubiKeyPIVService(context.TODO(), &keys.CLIPrompt{})

	clientStore := client.NewFSClientStore(cf.HomePath, hwKeyService)

	p := &vnetClientApplication{
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
func (p *vnetClientApplication) ListProfiles() ([]string, error) {
	return p.clientStore.ListProfiles()
}

// GetCachedClient returns a cached [*client.ClusterClient] for the given profile and leaf cluster.
// [leafClusterName] may be empty when requesting a client for the root cluster.
func (p *vnetClientApplication) GetCachedClient(ctx context.Context, profileName, leafClusterName string) (vnet.ClusterClient, error) {
	return p.clientCache.Get(ctx, profileName, leafClusterName)
}

// ReissueAppCert returns a new app certificate for the given app in the named profile and leaf cluster.
// It uses retryWithRelogin to issue the new app cert. A relogin may not be necessary if the app cert lifetime
// was shorter than the cluster cert lifetime, or if the user has already re-logged in to the cluster.
// If a cluster relogin is completed, the cluster client cache will be cleared for the root cluster and all
// leaf clusters of that root.
func (p *vnetClientApplication) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error) {
	appKey := appInfo.GetAppKey()
	tc, err := p.newTeleportClient(ctx, appKey.GetProfile(), appKey.GetLeafCluster())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	routeToApp := vnet.RouteToApp(appInfo, targetPort)

	var cert tls.Certificate
	err = p.retryWithRelogin(ctx, tc, func() error {
		var err error
		cert, err = p.reissueAppCert(ctx, tc, appKey.GetProfile(), appKey.GetLeafCluster(), routeToApp)
		return trace.Wrap(err, "reissuing app cert")
	})
	return cert, trace.Wrap(err)
}

// GetDialOptions returns ALPN dial options for the profile.
func (p *vnetClientApplication) GetDialOptions(ctx context.Context, profileName string) (*vnetv1.DialOptions, error) {
	profile, err := p.clientStore.GetProfile(profileName)
	if err != nil {
		return nil, trace.Wrap(err, "loading user profile")
	}
	dialOpts := &vnetv1.DialOptions{
		WebProxyAddr:            profile.WebProxyAddr,
		AlpnConnUpgradeRequired: profile.TLSRoutingConnUpgradeRequired,
		InsecureSkipVerify:      p.cf.InsecureSkipVerify,
	}
	if dialOpts.AlpnConnUpgradeRequired {
		dialOpts.RootClusterCaCertPool, err = p.getRootClusterCACertPoolPEM(ctx, profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return dialOpts, nil
}

// OnNewConnection gets called before each VNet connection. It's a noop as tsh doesn't need to do
// anything extra here.
func (p *vnetClientApplication) OnNewConnection(_ context.Context, _ *vnetv1.AppKey) error {
	return nil
}

// OnInvalidLocalPort gets called before VNet refuses to handle a connection to a multi-port TCP app
// because the provided port does not match any of the TCP ports in the app spec.
func (p *vnetClientApplication) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) {
	msg := fmt.Sprintf("%s: Connection refused, port not included in target ports of app %q.",
		net.JoinHostPort(appInfo.GetApp().GetPublicAddr(), strconv.Itoa(int(targetPort))), appInfo.GetAppKey().GetName())

	tcpPorts := appInfo.GetApp().GetTCPPorts()
	if len(tcpPorts) <= 10 {
		msg = fmt.Sprintf("%s Valid ports: %s.", msg, tcpPorts)
	}

	fmt.Println(msg)
}

// getRootClusterCACertPool returns a certificate pool for the root cluster of the given profile.
func (p *vnetClientApplication) getRootClusterCACertPoolPEM(ctx context.Context, profileName string) ([]byte, error) {
	tc, err := p.newTeleportClient(ctx, profileName, "")
	if err != nil {
		return nil, trace.Wrap(err, "creating new client")
	}
	certPool, err := tc.RootClusterCACertPoolPEM(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "loading root cluster CA cert pool")
	}
	return certPool, nil
}

func (p *vnetClientApplication) retryWithRelogin(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
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

func (p *vnetClientApplication) reissueAppCert(ctx context.Context, tc *client.TeleportClient, profileName, leafClusterName string, routeToApp *proto.RouteToApp) (tls.Certificate, error) {
	slog.InfoContext(ctx, "Reissuing cert for app.", "app_name", routeToApp.Name, "profile", profileName, "leaf_cluster", leafClusterName)

	profile, err := tc.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "loading client profile")
	}

	appCertParams := client.ReissueParams{
		RouteToCluster: leafClusterName,
		RouteToApp:     *routeToApp,
		AccessRequests: profile.ActiveRequests,
		RequesterName:  proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
	}

	// leafClusterName cannot be replaced with routeToApp.ClusterName here. That's because when
	// routeToApp points to an app in the root cluster, routeToApp.ClusterName uses the actual root
	// cluster name which is not necessarily equal to profileName.
	clusterClient, err := p.clientCache.Get(ctx, profileName, leafClusterName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting cached cluster client")
	}
	rootClient, err := p.clientCache.Get(ctx, profileName, "")
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting cached root client")
	}

	keyRing, err := appLogin(ctx, clusterClient, rootClient.AuthClient, appCertParams)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "logging in to app")
	}

	cert, err := keyRing.AppTLSCert(routeToApp.Name)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "getting TLS cert from key")
	}

	return cert, nil
}

func (p *vnetClientApplication) newTeleportClient(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
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
