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
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// appProvider is an interface for querying app info from an app fqdn, getting
// certs issued for apps, and reporting connections and errors.
type appProvider interface {
	// ResolveAppInfo returns an *AppInfo for the given app fqdn, or an error if
	// the app is not present in any logged-in cluster.
	ResolveAppInfo(ctx context.Context, fqdn string) (*vnetv1.AppInfo, error)
	// ReissueAppCert issues a new cert for the target app.
	ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) (tls.Certificate, error)
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

// tcpAppResolver implements tcpHandlerResolver for Teleport TCP apps.
type tcpAppResolver struct {
	appProvider appProvider
	log         *slog.Logger
	clock       clockwork.Clock
}

func newTCPAppResolver(appProvider appProvider, clock clockwork.Clock) *tcpAppResolver {
	return &tcpAppResolver{
		appProvider: appProvider,
		log:         log.With(teleport.ComponentKey, "VNet.AppResolver"),
		clock:       clock,
	}
}

// resolveTCPHandler resolves a fully-qualified domain name to a tcpHandlerSpec
// for a Teleport TCP app that should be used to handle all future TCP
// connections to fqdn.
//
// Avoid using [trace.Wrap] on errNoTCPHandler to prevent collecting a full
// stack trace on every unhandled query.
func (r *tcpAppResolver) resolveTCPHandler(ctx context.Context, fqdn string) (*tcpHandlerSpec, error) {
	appInfo, err := r.appProvider.ResolveAppInfo(ctx, fqdn)
	if err != nil {
		// Intentionally don't wrap the error, collecting a trace is expensive
		// and should only be done for unexpected errors
		return nil, err
	}
	appHandler, err := r.newTCPAppHandler(ctx, appInfo)
	if err != nil {
		return nil, err
	}
	return &tcpHandlerSpec{
		ipv4CIDRRange: appInfo.GetIpv4CidrRange(),
		tcpHandler:    appHandler,
	}, nil
}

type tcpAppHandler struct {
	appInfo     *vnetv1.AppInfo
	appProvider appProvider
	log         *slog.Logger
	clock       clockwork.Clock

	// mu guards access to portToLocalProxy.
	mu               sync.Mutex
	portToLocalProxy map[uint16]*alpnproxy.LocalProxy
}

func (r *tcpAppResolver) newTCPAppHandler(ctx context.Context, appInfo *vnetv1.AppInfo) (*tcpAppHandler, error) {
	return &tcpAppHandler{
		appInfo:     appInfo,
		appProvider: r.appProvider,
		log: r.log.With(teleport.ComponentKey, "VNet.tcpAppResolver",
			"profile", appInfo.GetAppKey().GetProfile(), "leaf_cluster", appInfo.GetAppKey().GetLeafCluster(), "fqdn", appInfo.GetApp().GetPublicAddr()),
		clock:            r.clock,
		portToLocalProxy: make(map[uint16]*alpnproxy.LocalProxy),
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
	if len(h.appInfo.GetApp().GetTCPPorts()) == 0 {
		localPort = 0
	}
	lp, ok := h.portToLocalProxy[localPort]
	if ok {
		return lp, nil
	}
	appCertIssuer := &appCertIssuer{
		appProvider: h.appProvider,
		appInfo:     h.appInfo,
		targetPort:  localPort,
	}
	certChecker := client.NewCertChecker(appCertIssuer, h.clock)
	middleware := &localProxyMiddleware{
		certChecker: certChecker,
		appProvider: h.appProvider,
		appKey:      h.appInfo.GetAppKey(),
	}
	dialOptions := h.appInfo.GetDialOptions()
	localProxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOptions.GetWebProxyAddr(),
		Protocols:               []alpncommon.Protocol{alpncommon.ProtocolTCP},
		ParentContext:           ctx,
		SNI:                     dialOptions.GetSni(),
		ALPNConnUpgradeRequired: dialOptions.GetAlpnConnUpgradeRequired(),
		Middleware:              middleware,
		InsecureSkipVerify:      dialOptions.GetInsecureSkipVerify(),
		Clock:                   h.clock,
	}
	if certPoolPEM := dialOptions.GetRootClusterCaCertPool(); len(certPoolPEM) > 0 {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(dialOptions.GetRootClusterCaCertPool()) {
			return nil, trace.Errorf("failed to parse root cluster CA certs")
		}
		localProxyConfig.RootCAs = caPool
	}
	h.log.DebugContext(ctx, "Creating local proxy", "target_port", localPort)
	newLP, err := alpnproxy.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}
	h.portToLocalProxy[localPort] = newLP
	return newLP, nil
}

// handleTCPConnector handles an incoming TCP connection from VNet by passing it to the local alpn proxy,
// which is set up with middleware to automatically handler certificate renewal and re-logins.
func (h *tcpAppHandler) handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error {
	app := h.appInfo.GetApp()
	if len(app.GetTCPPorts()) > 0 {
		if !app.GetTCPPorts().Contains(int(localPort)) {
			h.appProvider.OnInvalidLocalPort(ctx, h.appInfo, localPort)
			return trace.BadParameter("local port %d is not in TCP ports of app %q", localPort, app.GetName())
		}
	}

	lp, err := h.getOrInitializeLocalProxy(ctx, localPort)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

// appCertIssuer implements [client.CertIssuer].
type appCertIssuer struct {
	appProvider appProvider
	appInfo     *vnetv1.AppInfo
	targetPort  uint16
	group       singleflight.Group
}

func (i *appCertIssuer) CheckCert(cert *x509.Certificate) error {
	// appCertIssuer does not perform any additional certificate checks.
	return nil
}

func (i *appCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	cert, err, _ := i.group.Do("", func() (any, error) {
		return i.appProvider.ReissueAppCert(ctx, i.appInfo, i.targetPort)
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
// OnNewConnection method calls the same method of [appProvider].
type localProxyMiddleware struct {
	appKey      *vnetv1.AppKey
	certChecker *client.CertChecker
	appProvider appProvider
}

func (m *localProxyMiddleware) OnNewConnection(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	err := m.certChecker.OnNewConnection(ctx, lp)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(m.appProvider.OnNewConnection(ctx, m.appKey))
}

func (m *localProxyMiddleware) OnStart(ctx context.Context, lp *alpnproxy.LocalProxy) error {
	return trace.Wrap(m.certChecker.OnStart(ctx, lp))
}

// RouteToApp returns a *proto.RouteToApp populated from appInfo and targetPort.
func RouteToApp(appInfo *vnetv1.AppInfo, targetPort uint16) *proto.RouteToApp {
	app := appInfo.GetApp()
	return &proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: appInfo.GetCluster(),
		URI:         app.GetURI(),
		TargetPort:  uint32(targetPort),
	}
}
