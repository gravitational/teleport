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

package postgres

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v4"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

// schemaInfo represents information about all schemas in a database.
type schemaInfo map[string]schema

// schema represents single schema.
type schema struct {
	tables []string
}

// schemaInfoQuery is a query to return the schema info.
// TODO(Tener): for very large schemas adding a filtering right into this query could improve performance.
// It doesn't appear necessary right now.
const schemaInfoQuery = "SELECT schemaname, tablename FROM pg_catalog.pg_tables"

func fetchDatabaseObjects(ctx context.Context, session *common.Session, conn *pgx.Conn) ([]*dbobjectv1.DatabaseObject, error) {
	s, err := getSchemaInfo(ctx, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*dbobjectv1.DatabaseObject

	for schemaName, schemaVal := range s {
		for _, table := range schemaVal.tables {
			name := strings.Join([]string{
				session.Database.GetProtocol(),
				session.Database.GetType(),
				session.Database.GetName(),
				databaseobjectimportrule.ObjectKindTable,
				session.DatabaseName,
				schemaName,
				table,
			}, "/")

			obj, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{
				ObjectKind:          databaseobjectimportrule.ObjectKindTable,
				DatabaseServiceName: session.Database.GetName(),
				Protocol:            session.Database.GetProtocol(),
				Database:            session.DatabaseName,
				Schema:              schemaName,
				Name:                table,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out = append(out, obj)
		}
	}

	return out, nil
}

// getSchemaInfo returns SchemaInfo for given database.
func getSchemaInfo(ctx context.Context, conn *pgx.Conn) (schemaInfo, error) {
	type row struct {
		SchemaName string
		TableName  string
	}

	schemaRows, err := conn.Query(ctx, schemaInfoQuery)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer schemaRows.Close()

	var tables []row

	for schemaRows.Next() {
		var r row
		if err := schemaRows.Scan(&r.SchemaName, &r.TableName); err != nil {
			return nil, err
		}
		tables = append(tables, r)
	}

	if err := schemaRows.Err(); err != nil {
		return nil, err
	}

	schemas := map[string]schema{}
	for _, table := range tables {
		sch := schemas[table.SchemaName]
		sch.tables = append(sch.tables, table.TableName)

		schemas[table.SchemaName] = sch
	}

	return schemas, nil
}
