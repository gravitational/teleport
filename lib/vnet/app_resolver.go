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
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
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

// ResolveTCPHandler resolves a fully-qualified domain name to a TCPHandler for a Teleport TCP app that should
// be used to handle all future TCP connections to [fqdn].
func (r *TCPAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler TCPHandler, match bool, err error) {
	profileNames, err := r.appProvider.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	for _, profileName := range profileNames {
		if profileName == appPublicAddr {
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
		return r.resolveTCPHandlerForCluster(ctx, slog, rootClient.CurrentCluster(), profileName, "", appPublicAddr)
	}
	return nil, false, nil
}

func (r *TCPAppResolver) resolveTCPHandlerForCluster(
	ctx context.Context,
	slog *slog.Logger,
	clt apiclient.GetResourcesClient,
	profileName, leafClusterName, appPublicAddr string,
) (handler TCPHandler, match bool, err error) {
	resp, err := apiclient.GetResourcePage[types.AppServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: fmt.Sprintf(`resource.spec.public_addr == "%s" && hasPrefix(resource.spec.uri, "tcp://")`, appPublicAddr),
		Limit:               1,
	})
	if err != nil {
		// Don't return an error so we can try to find the app in different clusters or forward the request upstream.
		slog.InfoContext(ctx, "Failed to list application servers.", "error", err)
		return nil, false, nil
	}
	if len(resp.Resources) == 0 {
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
}

func newTCPAppHandler(
	ctx context.Context,
	appProvider AppProvider,
	profileName string,
	leafClusterName string,
	app types.Application,
) (*tcpAppHandler, error) {
	return &tcpAppHandler{
		profileName:     profileName,
		leafClusterName: leafClusterName,
		app:             app,
	}, nil
}

func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	conn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	// HandleTCPConnector not implemented yet - just echo input back to output.
	return trace.Wrap(utils.ProxyConn(ctx, conn, conn))
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	// Fully-qualify the proxy address
	if !strings.HasSuffix(proxyAddress, ".") {
		proxyAddress = proxyAddress + "."
	}
	return strings.HasSuffix(appFQDN, "."+proxyAddress)
}
