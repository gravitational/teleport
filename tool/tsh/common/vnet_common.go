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
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/vnet"
)

type tcpAppResolver struct {
	cf          *CLIConf
	clientStore *client.Store
}

func newTCPAppResolver(cf *CLIConf) *tcpAppResolver {
	clientStore := client.NewFSClientStore(cf.HomePath)
	return &tcpAppResolver{
		cf:          cf,
		clientStore: clientStore,
	}
}

// ResolveTCPHandler takes a fully-qualified domain name and, if it should be valid for a currently connected
// app, returns a TCPHandler that should handle all future VNet TCP connections to that FQDN.
func (r *tcpAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler vnet.TCPHandler, match bool, err error) {
	profileNames, err := r.clientStore.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	for _, profileName := range profileNames {
		if !isSubdomain(fqdn, profileName) {
			// TODO(nklaassen): handle custom DNS zones and leaf clusters.
			continue
		}

		tc, err := r.getTeleportClient(ctx, profileName)
		if err != nil {
			return nil, false, trace.Wrap(err, "getting Teleport client")
		}

		appServers, err := tc.ListAppServersWithFilters(ctx, &proto.ListResourcesRequest{
			ResourceType: types.KindAppServer,
			PredicateExpression: builder.Equals(
				builder.Identifier("resource.spec.public_addr"),
				builder.String(appPublicAddr),
			).String(),
		})
		if err != nil {
			return nil, false, trace.Wrap(err, "listing application servers")
		}

		for _, appServer := range appServers {
			app := appServer.GetApp()
			if app.GetPublicAddr() == appPublicAddr && app.IsTCP() {
				return &tcpAppHandler{
					app: app,
				}, true, nil
			}
		}
	}
	return nil, false, nil
}

func (r *tcpAppResolver) getTeleportClient(ctx context.Context, profileName string) (*client.TeleportClient, error) {
	// TODO(nklaassen): handle leaf clusters.
	// TODO(nklaassen/ravicious): cache clients and handle certificate expiry.
	tc, err := makeClientForProxy(r.cf, profileName)
	return tc, trace.Wrap(err)
}

type tcpAppHandler struct {
	app types.Application
}

// HandleTCP handles a TCP connection from VNet and proxies it to the application.
func (h *tcpAppHandler) HandleTCP(ctx context.Context, connector func() (net.Conn, error)) error {
	return trace.NotImplemented("HandleTCP is not implemented for TCP app handler")
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	// Fully-qualify the proxy address
	if !strings.HasSuffix(proxyAddress, ".") {
		proxyAddress = proxyAddress + "."
	}
	return strings.HasSuffix(appFQDN, "."+proxyAddress)
}
