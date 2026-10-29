/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type appServerCollection struct {
	servers []types.AppServer
}

// NewAppServerCollection creates a [Collection] over the provided applications.
func NewAppServerCollection(servers []types.AppServer) Collection {
	return &appServerCollection{servers: servers}
}

func (a *appServerCollection) Resources() (r []types.Resource) {
	for _, resource := range a.servers {
		r = append(r, resource)
	}
	return r
}

func (a *appServerCollection) WriteText(w io.Writer, verbose bool) error {
	rows := make([][]string, 0, len(a.servers))
	for _, server := range a.servers {
		app := server.GetApp()
		labels := common.FormatLabels(app.GetAllLabels(), verbose)
		rows = append(rows, []string{
			server.GetHostname(), appServerSQN(server), app.GetProtocol(), app.GetPublicAddr(), app.GetURI(), labels, server.GetTeleportVersion(),
		})
	}
	var t asciitable.Table
	headers := []string{"Host", "Name", "Type", "Public Address", "URI", "Labels", "Version"}
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func appServerHandler() Handler {
	return Handler{
		getHandler:    getAppServer,
		createHandler: createAppServer,
		updateHandler: updateAppServer,
		deleteHandler: deleteAppServer,
		singleton:     false,
		mfaRequired:   false,
		description:   "Represents a proxied application in the cluster.",
	}
}

func listAppServersWithFilter(ctx context.Context, client *authclient.Client, predicateExpression string) ([]types.AppServer, error) {
	appServers, err := apiclient.GetAllResources[types.AppServer](ctx, client, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: predicateExpression,
	})
	return appServers, trace.Wrap(err)
}

func getAppServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		appServers, err := listAppServersWithFilter(ctx, client, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &appServerCollection{servers: appServers}, nil
	}
	appServers, err := listAppServersBySQN(ctx, client, scopes.QualifiedName{Name: ref.Name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &appServerCollection{servers: appServers}, nil
}

func createAppServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	appServer, err := services.UnmarshalAppServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if appServer.GetApp().GetIntegration() == "" {
		return trace.BadParameter("only applications that use an integration can be created")
	}
	if _, err := client.UpsertApplicationServer(ctx, appServer); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application server %q has been upserted\n", appServerSQN(appServer))
	return nil
}

func appServerSQN(server types.AppServer) string {
	return scopes.QualifiedName{
		Scope: server.GetScope(),
		Name:  server.GetName(),
	}.String()
}

func updateAppServer(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	appServer, err := services.UnmarshalAppServer(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if appServer.GetApp().GetIntegration() == "" {
		return trace.BadParameter("only applications that use an integration can be updated")
	}

	// Check if an app server with the same scope, name and host ID exists.
	appServersWithSameName, err := listAppServersWithFilter(ctx, client, makeNamePredicate(appServer.GetName()))
	if err != nil {
		return trace.Wrap(err)
	}
	if !slices.ContainsFunc(appServersWithSameName, func(e types.AppServer) bool {
		return e.GetHostID() == appServer.GetHostID() && e.GetScope() == appServer.GetScope()
	}) {
		return trace.NotFound("application server %q with host ID %q not found", appServerSQN(appServer), appServer.GetHostID())
	}

	if _, err := client.UpsertApplicationServer(ctx, appServer); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("application server %q has been updated\n", appServerSQN(appServer))
	return nil
}

func deleteAppServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	return trace.Wrap(deleteScopedAppServer(ctx, client, ref.SubKind, scopes.QualifiedName{Name: ref.Name}))
}

func scopedAppServerHandler() ScopedHandler {
	return ScopedHandler{
		getHandler:    getScopedAppServer,
		createHandler: createAppServer,
		updateHandler: updateAppServer,
		deleteHandler: deleteScopedAppServer,
		description:   "Represents a proxied application in the cluster.",
	}
}

func getScopedAppServer(ctx context.Context, client *authclient.Client, subKind string, sqn *scopes.QualifiedName, _ GetOpts) (Collection, error) {
	if subKind != "" {
		return nil, rejectSubKind(types.KindAppServer, subKind)
	}
	if sqn == nil {
		servers, err := listAppServersWithFilter(ctx, client, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewAppServerCollection(servers), nil
	}
	servers, err := listAppServersBySQN(ctx, client, *sqn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewAppServerCollection(servers), nil
}

func deleteScopedAppServer(ctx context.Context, client *authclient.Client, subKind string, sqn scopes.QualifiedName) error {
	if subKind != "" {
		return rejectSubKind(types.KindAppServer, subKind)
	}
	servers, err := listAppServersBySQN(ctx, client, sqn)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, server := range servers {
		if err := client.DeleteAppServer(ctx, presencev1.DeleteAppServerRequest_builder{
			HostId: server.GetHostID(),
			Name:   server.GetName(),
			Scope:  server.GetScope(),
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("application server %q has been deleted\n", sqn.String())
	return nil
}

// listAppServersBySQN returns every app server (one per host) matching the scope-qualified name, or
// NotFound when there is none.
func listAppServersBySQN(ctx context.Context, client *authclient.Client, sqn scopes.QualifiedName) ([]types.AppServer, error) {
	servers, err := listAppServersWithFilter(ctx, client, makeNamePredicate(sqn.Name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers = FilterBySQNOrDiscoveredName(servers, sqn)
	if len(servers) == 0 {
		return nil, trace.NotFound("application server %q not found", sqn.String())
	}
	return servers, nil
}
