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
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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
	GetCachedClient(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error)

	// ReissueAppCert returns a new app certificate for the given app in the named profile and leaf cluster.
	// Implementations may trigger a re-login to the cluster, but if they do, they MUST clear all cached
	// clients for that cluster so that new working clients will be returned from [GetCachedClient].
	ReissueAppCert(ctx context.Context, profileName, leafClusterName string, app types.Application) (tls.Certificate, error)

	// GetDialOptions returns ALPN dial options for the profile.
	GetDialOptions(ctx context.Context, profileName string) (*DialOptions, error)
}

// DialOptions holds ALPN dial options for dialing apps.
type DialOptions struct {
	// WebProxyAddr is the address to dial.
	WebProxyAddr string
	// ALPNConnUpgradeRequired specifies if ALPN connection upgrade is required.
	ALPNConnUpgradeRequired bool
	// RootClusterCACertPool overrides the x509 certificate pool used to verify the server.
	RootClusterCACertPool *x509.CertPool
	// InsecureSkipTLSVerify turns off verification for x509 upstream ALPN proxy service certificate.
	InsecureSkipVerify bool
}

// TCPAppResolver implements [TCPHandlerResolver] for Teleport TCP apps.
type TCPAppResolver struct {
	appProvider AppProvider
	slog        *slog.Logger
}

// NewTCPAppResolver returns a new *TCPAppResolver which will resolve full-qualified domain names to
// TCPHandlers that will proxy TCP connection to Teleport TCP apps.
//
// It uses [appProvider] to list and retrieve cluster clients which are expected to be cached to avoid
// repeated/unnecessary dials to the cluster. These clients are then used to list TCP apps that should be
// handled.
//
// [appProvider] is also used to get app certificates used to dial the apps.
func NewTCPAppResolver(appProvider AppProvider) *TCPAppResolver {
	return &TCPAppResolver{
		appProvider: appProvider,
		slog:        slog.With(teleport.ComponentKey, "VNet.AppResolver"),
	}
}

// ResolveTCPHandler resolves a fully-qualified domain name (FQDN) to a TCPHandler for handling TCP
// connections to a Teleport TCP app. It returns the TCPHandler if a matching app is found, else it will
// return with match == false. Errors must only be returned for truly unexpected errors that should cause a
// DNS request to fail.
func (r *TCPAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler TCPHandler, match bool, err error) {
	profileNames, err := r.appProvider.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	for _, profileName := range profileNames {
		if fqdn == fullyQualify(profileName) {
			// This is a query for the proxy address, which we'll never want to handle.
			return nil, false, nil
		}
		if !isSubdomain(fqdn, profileName) {
			// TODO(nklaassen): support leaf clusters and custom DNS zones.
			continue
		}

		slog := r.slog.With("profile", profileName, "fqdn", fqdn)
		rootClient, err := r.appProvider.GetCachedClient(ctx, profileName, "")
		if err != nil {
			// The user might be logged out from this one cluster (and retryWithRelogin isn't working). Don't
			// return an error so that DNS resolution will be forwarded upstream instead of failing, to avoid
			// breaking e.g. web app access (we don't know if this is a web or TCP app yet because we can't
			// log in).
			slog.InfoContext(ctx, "Failed to get teleport client.", "error", err)
			continue
		}
		return r.resolveTCPHandlerForCluster(ctx, slog, rootClient.CurrentCluster(), profileName, "", fqdn)
	}

	// fqdn did not match any profile, forward the request upstream.
	return nil, false, nil
}

func (r *TCPAppResolver) resolveTCPHandlerForCluster(
	ctx context.Context,
	slog *slog.Logger,
	clt apiclient.GetResourcesClient,
	profileName, leafClusterName, fqdn string,
) (handler TCPHandler, match bool, err error) {
	// An app public_addr could technically be full-qualified or not, match either way.
	expr := fmt.Sprintf(`(resource.spec.public_addr == "%s" || resource.spec.public_addr == "%s") && hasPrefix(resource.spec.uri, "tcp://")`,
		strings.TrimSuffix(fqdn, "."), fqdn)
	resp, err := apiclient.GetResourcePage[types.AppServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: expr,
		Limit:               1,
	})
	if err != nil {
		// Don't return an error so we can try to find the app in different clusters or forward the request
		// upstream.
		slog.InfoContext(ctx, "Failed to list application servers.", "error", err)
		return nil, false, nil
	}
	if len(resp.Resources) == 0 {
		// Didn't find any matching app, forward the request upstream.
		return nil, false, nil
	}
	app := resp.Resources[0].GetApp()
	appHandler, err := newTCPAppHandler(ctx, r.appProvider, profileName, leafClusterName, app)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	return appHandler, true, nil
}

type tcpAppHandler struct {
	profileName     string
	leafClusterName string
	app             types.Application
	lp              *alpnproxy.LocalProxy
}

func newTCPAppHandler(
	ctx context.Context,
	appProvider AppProvider,
	profileName string,
	leafClusterName string,
	app types.Application,
) (*tcpAppHandler, error) {
	dialOpts, err := appProvider.GetDialOptions(ctx, profileName)
	if err != nil {
		return nil, trace.Wrap(err, "getting dial options for profile %q", profileName)
	}

	appCertIssuer := &appCertIssuer{
		appProvider:     appProvider,
		profileName:     profileName,
		leafClusterName: leafClusterName,
		app:             app,
	}
	middleware := client.NewCertChecker(appCertIssuer, nil /*clock*/)

	localProxyConfig := alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         dialOpts.WebProxyAddr,
		Protocols:               []alpncommon.Protocol{alpncommon.ProtocolTCP},
		ParentContext:           ctx,
		RootCAs:                 dialOpts.RootClusterCACertPool,
		ALPNConnUpgradeRequired: dialOpts.ALPNConnUpgradeRequired,
		Middleware:              middleware,
		InsecureSkipVerify:      dialOpts.InsecureSkipVerify,
	}

	lp, err := alpnproxy.NewLocalProxy(localProxyConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}

	return &tcpAppHandler{
		profileName:     profileName,
		leafClusterName: leafClusterName,
		app:             app,
		lp:              lp,
	}, nil
}

// HandleTCPConnector handles an incoming TCP connection from VNet by passing it to the local alpn proxy,
// which is set up with middleware to automatically handler certificate renewal and re-logins.
func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	return trace.Wrap(h.lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

// appCertIssuer implements [client.CertIssuer].
type appCertIssuer struct {
	appProvider     AppProvider
	profileName     string
	leafClusterName string
	app             types.Application
	group           singleflight.Group
}

func (i *appCertIssuer) CheckCert(cert *x509.Certificate) error {
	// appCertIssuer does not perform any additional certificate checks.
	return nil
}

func (i *appCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	cert, err, _ := i.group.Do("", func() (any, error) {
		return i.appProvider.ReissueAppCert(ctx, i.profileName, i.leafClusterName, i.app)
	})
	return cert.(tls.Certificate), trace.Wrap(err)
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	return strings.HasSuffix(appFQDN, "."+fullyQualify(proxyAddress))
}

// fullyQualify returns a fully-qualified domain name from [domain]. Fully-qualified domain names always end
// with a ".".
func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}
