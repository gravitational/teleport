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

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
)

// AppProvider is an interface providing the necessary methods to log in to apps and get clients able to list
// apps in all clusters in all current profiles. This should be the minimum necessary interface that needs to
// be implemented differently for Connect and `tsh vnet`.
type AppProvider interface {
	// ListProfiles lists the names of all profiles saved for the user.
	ListProfiles() ([]string, error)

	// GetProfile returns the named profile for the user.
	GetProfile(string) (*profile.Profile, error)

	// GetCachedClient returns a [*client.ClusterClient] for the given profile and leaf cluster.
	// [leafClusterName] may be empty when requesting a client for the root cluster. Returned clients are
	// expected to be cached, as this may be called frequently.
	GetCachedClient(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error)
}

// TCPAppResolver implements [TCPHandlerResolver] for Teleport TCP apps.
type TCPAppResolver struct {
	appProvider AppProvider
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
	}
}

// ResolveTCPHandler resolves a fully-qualified domain name to a TCPHandler for a Teleport TCP app that should
// be used to handle all future TCP connections to [fqdn].
func (r *TCPAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler TCPHandler, match bool, err error) {
	// TODO(nklaassen): singleflight
	return r.resolveTCPHandler(ctx, fqdn)
}

func (r *TCPAppResolver) resolveTCPHandler(ctx context.Context, fqdn string) (handler TCPHandler, match bool, err error) {
	profileNames, err := r.appProvider.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	for _, profileName := range profileNames {
		if !isSubdomain(fqdn, profileName) {
			// TODO(nklaassen): handle custom DNS zones and leaf clusters.
			continue
		}
		leafClusterName := ""

		clusterClient, err := r.appProvider.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			// The user might be logged out from the cluster and not able to log in. Don't return an error so
			// that DNS resolution will be forwarded upstream instead of failing, to avoid breaking e.g. web
			// app access (we don't know if this is a web or TCP app because we can't log in).
			slog.WarnContext(ctx, "Failed to get teleport client, DNS request will be forwarded upstream.", "profile_name", profileName, "error", err, "fqdn", fqdn)
			return nil, false, nil
		}

		appServers, err := apiclient.GetAllResources[types.AppServer](ctx, clusterClient.AuthClient, &proto.ListResourcesRequest{
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf(`resource.spec.public_addr == "%s" && hasPrefix(resource.spec.uri, "tcp://")`, appPublicAddr),
		})
		if err != nil {
			return nil, false, trace.Wrap(err, "listing application servers")
		}

		for _, appServer := range appServers {
			app := appServer.GetApp()
			if app.GetPublicAddr() == appPublicAddr && app.IsTCP() {
				appHandler, err := newTCPAppHandler(app)
				if err != nil {
					return nil, false, trace.Wrap(err)
				}
				return appHandler, true, nil
			}
		}
	}
	return nil, false, nil
}

type tcpAppHandler struct {
	app types.Application
}

func newTCPAppHandler(app types.Application) (*tcpAppHandler, error) {
	return &tcpAppHandler{
		app: app,
	}, nil
}

func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	return trace.NotImplemented("HandleTCPConnector is not implemented yet. App: %q", h.app.GetName())
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	// Fully-qualify the proxy address
	if !strings.HasSuffix(proxyAddress, ".") {
		proxyAddress = proxyAddress + "."
	}
	return strings.HasSuffix(appFQDN, "."+proxyAddress)
}
