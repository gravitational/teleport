// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package integrations

import (
	"context"
	"fmt"
	"sort"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// listAWSICAccounts prints the AWS Identity Center accounts synced into the
// cluster along with their permission sets.
func (c *Command) listAWSICAccounts(ctx context.Context, clt *authclient.Client) error {
	resources, err := apiclient.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds: []string{types.KindApp},
		// Returns only the permission sets the user is able to assume.
		IncludeLogins: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	servers := filterICAccounts(resources)
	return trace.Wrap(c.writeAWSICText(servers))
}

func filterICAccounts(resources []*types.EnrichedResource) []types.AppServer {
	var servers []types.AppServer
	for _, r := range resources {
		appServer, ok := r.ResourceWithLabels.(types.AppServer)
		if !ok {
			continue
		}
		if appServer.GetApp().GetIdentityCenter() == nil {
			continue
		}
		servers = append(servers, appServer)
	}

	sort.Slice(servers, func(i, j int) bool {
		return icAccountName(servers[i].GetApp()) < icAccountName(servers[j].GetApp())
	})

	return servers
}

func (c *Command) writeAWSICText(servers []types.AppServer) error {
	headers := []string{"Account Name", "Account ID", "Permission Sets"}
	rows := make([][]string, 0, len(servers))
	for _, server := range servers {
		app := server.GetApp()
		ic := app.GetIdentityCenter()
		name := icAccountName(app)

		if len(ic.PermissionSets) == 0 {
			rows = append(rows, []string{name, ic.AccountID, ""})
			continue
		}

		// Print each permission set in its own row.
		for i, ps := range ic.PermissionSets {
			permSet := fmt.Sprintf("%s (%s)", ps.Name, ps.ARN)
			if i == 0 {
				rows = append(rows, []string{name, ic.AccountID, permSet})
				continue
			}
			rows = append(rows, []string{"", "", permSet})
		}
	}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(c.Stdout)
	return trace.Wrap(err)
}

// icAccountName returns human-readable name for an Identity Center account
// (versus the AWS account ID).
func icAccountName(app types.Application) string {
	if name := types.FriendlyName(app); name != "" {
		return name
	}
	if name, ok := app.GetLabel(types.AWSAccountNameLabel); ok && name != "" {
		return name
	}
	if ic := app.GetIdentityCenter(); ic != nil {
		return ic.AccountID
	}
	return ""
}
