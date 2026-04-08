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

package common

import (
	"fmt"
	"slices"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/vnet"
)

var vnetSupportedKinds = []string{
	types.KindApp,
	types.KindDatabase,
	types.KindNode,
}

// vnetLSCommand implements the `tsh vnet ls` command to list resources
// accessible via VNet
type vnetLSCommand struct {
	*kingpin.CmdClause
	kinds               []string
	labels              string
	searchKeywords      string
	predicateExpression string
}

func newVnetLSCommand(parent *kingpin.CmdClause) *vnetLSCommand {
	cmd := &vnetLSCommand{
		CmdClause: parent.Command("ls", "List resources accessible via VNet."),
	}
	cmd.Flag("kind", fmt.Sprintf("Filter by resource kind. Supported: %s.", strings.Join(vnetSupportedKinds, ", "))).
		StringsVar(&cmd.kinds)
	cmd.Flag("search", searchHelp).StringVar(&cmd.searchKeywords)
	cmd.Flag("query", queryHelp).StringVar(&cmd.predicateExpression)
	cmd.Arg("labels", labelHelp).StringVar(&cmd.labels)
	return cmd
}

func (c *vnetLSCommand) run(cf *CLIConf) error {
	kinds := c.kinds
	if len(kinds) == 0 {
		kinds = vnetSupportedKinds
	}
	for _, kind := range kinds {
		if !slices.Contains(vnetSupportedKinds, kind) {
			return trace.BadParameter("unsupported kind %q, supported kinds: %s", kind, strings.Join(vnetSupportedKinds, ", "))
		}
	}

	cf.Labels = c.labels
	cf.SearchKeywords = c.searchKeywords
	cf.PredicateExpression = c.predicateExpression

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var rows [][]string
	var hasDB, hasNode bool

	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		// Reset state between retries to avoid duplicate rows.
		rows = nil
		hasDB = false
		hasNode = false

		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		resources, err := apiclient.GetAllUnifiedResources(cf.Context, clusterClient.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               kinds,
			SortBy:              types.SortBy{Field: types.ResourceKind},
			SearchKeywords:      tc.SearchKeywords,
			PredicateExpression: tc.PredicateExpression,
			Labels:              tc.Labels,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		proxyHost := tc.WebProxyHost()

		for _, r := range resources {
			switch res := r.ResourceWithLabels.(type) {
			case types.AppServer:
				app := res.GetApp()
				if app.IsTCP() {
					rows = append(rows, []string{"app/tcp", app.GetName(), app.GetPublicAddr()})
				} else {
					rows = append(rows, []string{"app/http", app.GetName(), app.GetPublicAddr()})
				}
			case types.DatabaseServer:
				db := res.GetDatabase()
				dbName := db.GetName()
				protocol := db.GetProtocol()
				fqdnLabel := vnet.HashDBName(dbName)
				var addr string
				if vnet.DBProtocolExtractsUsernameFromWire(protocol) {
					addr = fmt.Sprintf("%s.db.%s", fqdnLabel, proxyHost)
				} else {
					addr = fmt.Sprintf("<db-user>.%s.db.%s", fqdnLabel, proxyHost)
				}
				rows = append(rows, []string{"db", dbName, addr})
				hasDB = true
			case types.Server:
				hostname := res.GetHostname()
				clusterName := tc.SiteName
				addr := fmt.Sprintf("%s.%s", hostname, clusterName)
				rows = append(rows, []string{"node", hostname, addr})
				hasNode = true
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if len(rows) == 0 {
		fmt.Fprintln(cf.Stdout(), "No resources found accessible via VNet.")
		return nil
	}

	t := asciitable.MakeTable([]string{"Type", "Name", "Address"}, rows...)
	if _, err := fmt.Fprintln(cf.Stdout(), t.AsBuffer().String()); err != nil {
		return trace.Wrap(err)
	}

	if hasDB {
		fmt.Fprintln(cf.Stdout(), "(*) For databases that show <db-user>, replace it with your database username.")
		fmt.Fprintln(cf.Stdout(), "    For postgres, mysql, and sqlserver databases, specify the user in your client connection string instead.")
	}
	if hasNode {
		fmt.Fprintln(cf.Stdout(), "(**) To connect to SSH nodes via VNet, run: ssh <os-username>@<address>")
		fmt.Fprintln(cf.Stdout(), "     VNet must be configured in your SSH client. Run `tsh vnet-ssh-autoconfig` to set it up.")
	}

	return nil
}
