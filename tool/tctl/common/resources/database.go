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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
	"github.com/gravitational/teleport/tool/common"
)

type databaseCollection struct {
	databases types.Databases
}

// NewDatabaseCollection creates a [Collection] over the provided databases.
func NewDatabaseCollection(databases types.Databases) Collection {
	return &databaseCollection{databases: databases}
}

func (c *databaseCollection) Resources() []types.Resource {
	return sliceutils.Map(c.databases, func(db types.Database) types.Resource {
		return db
	})
}

func (c *databaseCollection) WriteText(w io.Writer, verbose bool) error {
	rows := make([][]string, 0, len(c.databases))
	for _, database := range c.databases {
		labels := common.FormatLabels(database.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(database, verbose),
			database.GetProtocol(),
			database.GetURI(),
			labels,
		})
	}
	headers := []string{"Name", "Protocol", "URI", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func databaseHandler() Handler {
	return Handler{
		getHandler:    getDatabase,
		createHandler: createDatabase,
		updateHandler: updateDatabase,
		deleteHandler: deleteDatabase,
		singleton:     false,
		mfaRequired:   false,
		description:   "A dynamic resource representing a database that can be proxied via a database service.",
	}
}

func getDatabase(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	// TODO(greedy52) implement resource filtering on the backend.
	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	databases, err := clientutils.CollectWithFallback(ctx, client.ListDatabases, client.GetDatabases)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ref.Name == "" {
		return NewDatabaseCollection(databases), nil
	}
	databases = FilterByNameOrDiscoveredName(databases, ref.Name)
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found", ref.Name)
	}
	return NewDatabaseCollection(databases), nil
}

func createDatabase(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	database, err := services.UnmarshalDatabase(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := client.CreateDatabase(ctx, database); err != nil {
		if trace.IsAlreadyExists(err) {
			if !opts.Force {
				return trace.AlreadyExists("database %q already exists", database.GetName())
			}
			if err := client.UpdateDatabase(ctx, database); err != nil {
				return trace.Wrap(err)
			}
			fmt.Printf("database %q has been updated\n", database.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	fmt.Printf("database %q has been created\n", database.GetName())
	return nil
}

func updateDatabase(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	database, err := services.UnmarshalDatabase(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := client.UpdateDatabase(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("database %q has been updated\n", database.GetName())
	return nil
}

func deleteDatabase(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	// TODO(okraport) DELETE IN v21.0.0, replace with regular Collect
	databases, err := clientutils.CollectWithFallback(ctx, client.ListDatabases, client.GetDatabases)
	if err != nil {
		return trace.Wrap(err)
	}
	resDesc := "database"
	databases = FilterByNameOrDiscoveredName(databases, ref.Name)
	name, err := GetOneResourceNameToDelete(databases, ref, resDesc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := client.DeleteDatabase(ctx, name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s %q has been deleted\n", resDesc, name)
	return nil
}
