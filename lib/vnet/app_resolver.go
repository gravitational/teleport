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

package vnet

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// AppProvider is an interface providing the necessary methods to log in to apps and get clients able to list
// apps in all clusters in all current profiles. This should be the minimum necessary interface that needs to
// be implemented differently for Connect and `tsh vnet`.
type AppProvider interface {
	// ListProfiles lists the names of all profiles saved for the user.
	ListProfiles() ([]string, error)

	// GetCachedClient returns a [*client.ClusterClient] for the given profile and leaf cluster.
	// [leafClusterName] may be empty when requesting a client for the root cluster. Returned clients are
	// expected to be cached, as this may be called frequently.
	GetCachedClient(ctx context.Context, profileName, leafClusterName string) (ClusterClient, error)

	// ReissueAppCert returns a new app certificate for the given app in the named profile and leaf cluster.
	// Implementations may trigger a re-login to the cluster, but if they do, they MUST clear all cached
	// clients for that cluster so that new working clients will be returned from [GetCachedClient].
	ReissueAppCert(ctx context.Context, profileName, leafClusterName string, routeToApp proto.RouteToApp) (tls.Certificate, error)

	// GetDialOptions returns ALPN dial options for the profile.
	GetDialOptions(ctx context.Context, profileName string) (*DialOptions, error)

	// OnNewConnection gets called whenever a new connection is about to be established through VNet.
	// By the time OnNewConnection, VNet has already verified that the user holds a valid cert for the
	// app.
	//
	// The connection won't be established until OnNewConnection returns. Returning an error prevents
	// the connection from being made.
	OnNewConnection(ctx context.Context, profileName, leafClusterName string, routeToApp proto.RouteToApp) error
}

// ClusterClient is an interface defining the subset of [client.ClusterClient] methods used by [AppProvider].
type ClusterClient interface {
	CurrentCluster() authclient.ClientI
	ClusterName() string
}

// DialOptions holds ALPN dial options for dialing apps.
type DialOptions struct {
	// WebProxyAddr is the address to dial.
	WebProxyAddr string
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// SNI is a ServerName value set for upstream TLS connection.
	SNI string
	// RootClusterCACertPool overrides the x509 certificate pool used to verify the server.
	RootClusterCACertPool *x509.CertPool
	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool
}

// TCPAppResolver implements [TCPHandlerResolver] for Teleport TCP apps.
type TCPAppResolver struct {
	appProvider        AppProvider
	clusterConfigCache *ClusterConfigCache
	log                *slog.Logger
	clock              clockwork.Clock
}

// NewTCPAppResolver returns a new *TCPAppResolver which will resolve full-qualified domain names to
// TCPHandlers that will proxy TCP connection to Teleport TCP apps.
//
// It uses [appProvider] to list and retrieve cluster clients which are expected to be cached to avoid
// repeated/unnecessary dials to the cluster. These clients are then used to list TCP apps that should be
// handled.
//
// [appProvider] is also used to get app certificates used to dial the apps.
func NewTCPAppResolver(appProvider AppProvider, opts ...tcpAppResolverOption) (*TCPAppResolver, error) {
	r := &TCPAppResolver{
		appProvider: appProvider,
		log:         log.With(teleport.ComponentKey, "VNet.AppResolver"),
	}
	for _, opt := range opts {
		opt(r)
	}
	r.clock = cmp.Or(r.clock, clockwork.NewRealClock())
	r.clusterConfigCache = cmp.Or(r.clusterConfigCache, NewClusterConfigCache(r.clock))
	return r, nil
}

type tcpAppResolverOption func(*TCPAppResolver)

// withClock is a functional option to override the default clock (for tests).
func withClock(clock clockwork.Clock) tcpAppResolverOption {
	return func(r *TCPAppResolver) {
		r.clock = clock
	}
}

// WithClusterConfigCache is a functional option to override the cluster config cache.
func WithClusterConfigCache(clusterConfigCache *ClusterConfigCache) tcpAppResolverOption {
	return func(r *TCPAppResolver) {
		r.clusterConfigCache = clusterConfigCache
	}
}

// ResolveTCPHandler resolves a fully-qualified domain name to a [TCPHandlerSpec] for a Teleport TCP app that should
// be used to handle all future TCP connections to [fqdn].
// Avoid using [trace.Wrap] on [ErrNoTCPHandler] to prevent collecting a full stack trace on every unhandled
// query.
func (r *TCPAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (*TCPHandlerSpec, error) {
	profileNames, err := r.appProvider.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err, "listing profiles")
	}
	for _, profileName := range profileNames {
		if fqdn == fullyQualify(profileName) {
			// This is a query for the proxy address, which we'll never want to handle.
			return nil, ErrNoTCPHandler
		}

		clusterClient, err := r.clusterClientForAppFQDN(ctx, profileName, fqdn)
		if err != nil {
			if errors.Is(err, errNoMatch) {
				continue
			}
			// The user might be logged out from this one cluster (and retryWithRelogin isn't working). Log
			// the error but don't return it so that DNS resolution will be forwarded upstream instead of
			// failing, to avoid breaking e.g. web app access (we don't know if this is a web or TCP app yet
			// because we can't log in).
			r.log.ErrorContext(ctx, "Failed to get teleport client.", "error", err)
			continue
		}

		leafClusterName := ""
		if clusterClient.ClusterName() != profileName {
			leafClusterName = clusterClient.ClusterName()
		}

		return r.resolveTCPHandlerForCluster(ctx, clusterClient, profileName, leafClusterName, fqdn)
	}
	// fqdn did not match any profile, forward the request upstream.
	return nil, ErrNoTCPHandler
}

var errNoMatch = errors.New("cluster does not match queried FQDN")

func (r *TCPAppResolver) clusterClientForAppFQDN(ctx context.Context, profileName, fqdn string) (ClusterClient, error) {
	rootClient, err := r.appProvider.GetCachedClient(ctx, profileName, "")
	if err != nil {
		r.log.ErrorContext(ctx, "Failed to get root cluster client, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	if isDescendantSubdomain(fqdn, profileName) {
		// The queried app fqdn is a subdomain of this cluster proxy address.
		return rootClient, nil
	}

	leafClusters, err := getLeafClusters(ctx, rootClient)
	if err != nil {
		// Good chance we're here because the user is not logged in to the profile.
		r.log.ErrorContext(ctx, "Failed to list leaf clusters, apps in this cluster will not be resolved.", "profile", profileName, "error", err)
		return nil, errNoMatch
	}

	// Prefix with an empty string to represent the root cluster.
	allClusters := append([]string{""}, leafClusters...)
	for _, leafClusterName := range allClusters {
		clusterClient, err := r.appProvider.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			r.log.ErrorContext(ctx, "Failed to get cluster client, apps in this cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := r.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
		if err != nil {
			r.log.ErrorContext(ctx, "Failed to get VnetConfig, apps in the cluster will not be resolved.", "profile", profileName, "leaf_cluster", leafClusterName, "error", err)
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

// resolveTCPHandlerForCluster takes a cluster client and resolves [fqdn] to a [TCPHandlerSpec] if a matching
// app is found in that cluster.
// Avoid using [trace.Wrap] on [ErrNoTCPHandler] to prevent collecting a full stack trace on every unhandled
// query.
func (r *TCPAppResolver) resolveTCPHandlerForCluster(
	ctx context.Context,
	clusterClient ClusterClient,
	profileName, leafClusterName, fqdn string,
) (*TCPHandlerSpec, error) {
	log := r.log.With("profile", profileName, "leaf_cluster", leafClusterName, "fqdn", fqdn)
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
		log.InfoContext(ctx, "Failed to list application servers.", "error", err)
		return nil, ErrNoTCPHandler
	}
	if len(resp.Resources) == 0 {
		// Didn't find any matching app, forward the request upstream.
		return nil, ErrNoTCPHandler
	}
	app := resp.Resources[0].GetApp()
	appHandler, err := r.newTCPAppHandler(ctx, profileName, leafClusterName, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterConfig, err := r.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &TCPHandlerSpec{
		IPv4CIDRRange: clusterConfig.IPv4CIDRRange,
		TCPHandler:    appHandler,
	}, nil
}

type tcpAppHandler struct {
	log              *slog.Logger
	appProvider      AppProvider
	clock            clockwork.Clock
	profileName      string
	leafClusterName  string
	app              types.Application
	portToLocalProxy map[uint16]*alpnproxy.LocalProxy
	// mu guards access to portToLocalProxy.
	mu sync.Mutex
}

func (r *TCPAppResolver) newTCPAppHandler(
	ctx context.Context,
	profileName string,
	leafClusterName string,
	app types.Application,
) (*tcpAppHandler, error) {
	return &tcpAppHandler{
		appProvider:      r.appProvider,
		clock:            r.clock,
		profileName:      profileName,
		leafClusterName:  leafClusterName,
		app:              app,
		portToLocalProxy: make(map[uint16]*alpnproxy.LocalProxy),
		log: r.log.With(teleport.ComponentKey, "VNet.AppHandler",
			"profile", profileName, "leaf_cluster", leafClusterName, "fqdn", app.GetPublicAddr()),
	}, nil
}

// getOrInitializeLocalProxy returns a separate local proxy for each port for multi-port apps. For
// single-port apps, it returns the same local proxy no matter the port.
func (h *tcpAppHandler) getOrInitializeLocalProxy(ctx context.Context, localPort uint16) (*alpnproxy.LocalProxy, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Connections to single-port apps need to go through a local proxy that has a cert with TargetPort
	// set to 0. This ensures that the old behavior is kept for such apps, where the client can dial
	// the public address of an app on any port and be routed to the port from the URI.
	//
	// https://github.com/gravitational/teleport/blob/master/rfd/0182-multi-port-tcp-app-access.md#vnet-with-single-port-apps
	if len(h.app.GetTCPPorts()) == 0 {
		localPort = 0
	}
	// TODO(ravicious): For multi-port apps, check if localPort is valid and surface the error in UI.
	// https://github.com/gravitational/teleport/blob/master/rfd/0182-multi-port-tcp-app-access.md#incorrect-port

	lp, ok := h.portToLocalProxy[localPort]
	if ok {
		return lp, nil
	}

	dialOpts, err := h.appProvider.GetDialOptions(ctx, h.profileName)
	if err != nil {
		return nil, trace.Wrap(err, "getting dial options for profile %q", h.profileName)
	}
	clusterClient, err := h.appProvider.GetCachedClient(ctx, h.profileName, h.leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	routeToApp := proto.RouteToApp{
		Name:       h.app.GetName(),
		PublicAddr: h.app.GetPublicAddr(),
		// ClusterName must not be set to "" when targeting an app from a root cluster. Otherwise the
		// connection routed through a local proxy will just get lost somewhere in the cluster (with no
		// clear error being reported) and hang forever.
		ClusterName: clusterClient.ClusterName(),
		URI:         h.app.GetURI(),
		TargetPort:  uint32(localPort),
	}

	appCertIssuer := &appCertIssuer{
		appProvider:     h.appProvider,
		profileName:     h.profileName,
		leafClusterName: h.leafClusterName,
		routeToApp:      routeToApp,
	}
	certChecker := client.NewCertChecker(appCertIssuer, h.clock)
	middleware := &localProxyMiddleware{
		certChecker:     certChecker,
		appProvider:     h.appProvider,
		routeToApp:      routeToApp,
		profileName:     h.profileName,
		leafClusterName: h.leafClusterName,
	}

	localProxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOpts.WebProxyAddr,
		Protocols:               []alpncommon.Protocol{alpncommon.ProtocolTCP},
		ParentContext:           ctx,
		SNI:                     dialOpts.SNI,
		RootCAs:                 dialOpts.RootClusterCACertPool,
		ALPNConnUpgradeRequired: dialOpts.ALPNConnUpgradeRequired,
		Middleware:              middleware,
		InsecureSkipVerify:      dialOpts.InsecureSkipVerify,
		Clock:                   h.clock,
	}

	h.log.DebugContext(ctx, "Creating local proxy", "target_port", localPort)
	newLP, err := alpnproxy.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}

	h.portToLocalProxy[localPort] = newLP

	return newLP, nil
}

// HandleTCPConnector handles an incoming TCP connection from VNet by passing it to the local alpn proxy,
// which is set up with middleware to automatically handler certificate renewal and re-logins.
func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	lp, err := h.getOrInitializeLocalProxy(ctx, localPort)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

// appCertIssuer implements [client.CertIssuer].
type appCertIssuer struct {
	appProvider     AppProvider
	profileName     string
	leafClusterName string
	routeToApp      proto.RouteToApp
	group           singleflight.Group
}

func (i *appCertIssuer) CheckCert(cert *x509.Certificate) error {
	// appCertIssuer does not perform any additional certificate checks.
	return nil
}

func (i *appCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	cert, err, _ := i.group.Do("", func() (any, error) {
		return i.appProvider.ReissueAppCert(ctx, i.profileName, i.leafClusterName, i.routeToApp)
	})
	return cert.(tls.Certificate), trace.Wrap(err)
}

// isDescendantSubdomain checks if appFQDN belongs in the hierarchy of zone. For example, both
// foo.bar.baz.example.com and bar.baz.example.com belong in the hierarchy of baz.example.com, but
// quux.example.com does not.
func isDescendantSubdomain(appFQDN, zone string) bool {
	return strings.HasSuffix(appFQDN, "."+fullyQualify(zone))
}

// fullyQualify returns a fully-qualified domain name from [domain]. Fully-qualified domain names always end
// with a ".".
func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}

// localProxyMiddleware wraps around [client.CertChecker] and additionally makes it so that its
// OnNewConnection method calls the same method of [AppProvider].
type localProxyMiddleware struct {
	routeToApp      proto.RouteToApp
	profileName     string
	leafClusterName string
	certChecker     *client.CertChecker
	appProvider     AppProvider
}

func (m *localProxyMiddleware) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	err := m.certChecker.OnNewConnection(ctx, lp)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(m.appProvider.OnNewConnection(ctx, m.profileName, m.leafClusterName, m.routeToApp))
}

func (m *localProxyMiddleware) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(m.certChecker.OnStart(ctx, lp))
}
