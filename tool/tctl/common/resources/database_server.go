/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

// DatabaseServerCollection implements [Collection] for [types.DatabaseServer].
type DatabaseServerCollection struct {
	servers []types.DatabaseServer
}

// NewDatabaseServerCollection creates a [DatabaseServerCollection] over the
// provided database servers.
func NewDatabaseServerCollection(servers []types.DatabaseServer) *DatabaseServerCollection {
	return &DatabaseServerCollection{servers: servers}
}

func (c *DatabaseServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *DatabaseServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		labels := common.FormatLabels(server.GetDatabase().GetAllLabels(), verbose)
		rows = append(rows, []string{
			server.GetHostname(),
			common.FormatResourceName(server.GetDatabase(), verbose),
			server.GetDatabase().GetProtocol(),
			server.GetDatabase().GetURI(),
			labels,
			server.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "Name", "Protocol", "URI", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by hostname then by name.
	t.SortRowsBy([]int{0, 1}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

// WriteJSON writes the database servers as a JSON array to w.
func (c *DatabaseServerCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.servers)
}

// WriteYAML writes the database servers as YAML to w.
func (c *DatabaseServerCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.servers)
}

func databaseServerHandler() Handler {
	return Handler{
		getHandler:    getDatabaseServer,
		deleteHandler: deleteDatabaseServer,
		description:   "Represents a Database Service instance that provides access to a specific database.",
	}
}

func getDatabaseServer(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref.Name == "" {
		return NewDatabaseServerCollection(servers), nil
	}

	servers = FilterByNameOrDiscoveredName(servers, ref.Name)
	if len(servers) == 0 {
		return nil, trace.NotFound("database server %q not found", ref.Name)
	}
	return NewDatabaseServerCollection(servers), nil
}

func deleteDatabaseServer(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	servers, err := client.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "database server"
	servers = FilterByNameOrDiscoveredName(servers, ref.Name)
	name, err := GetOneResourceNameToDelete(servers, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, s := range servers {
		err := client.DeleteDatabaseServer(ctx, apidefaults.Namespace, s.GetHostID(), name)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}
