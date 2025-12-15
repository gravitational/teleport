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
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v4"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/srv/db/objects"
)

// TODO(Tener): add e2e tests for objectFetcher.
type objectFetcher struct {
	cfg objects.ObjectFetcherConfig
	db  types.Database
}

var _ objects.ObjectFetcher = (*objectFetcher)(nil)

func NewObjectFetcher(ctx context.Context, db types.Database, cfg objects.ObjectFetcherConfig) (objects.ObjectFetcher, error) {
	if db.GetAdminUser().Name == "" {
		return nil, objects.NewErrFetcherDisabled("no admin user configured")
	}
	return &objectFetcher{cfg: cfg, db: db}, nil
}

func (f *objectFetcher) FetchAll(ctx context.Context, dbNameFilter databaseobjectimportrule.DbNameFilter) (map[string]objects.FetchResult, error) {
	names, err := f.getDatabaseNames(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := make(map[string]objects.FetchResult)

	for _, dbName := range names {
		if dbNameFilter(dbName) == false {
			continue
		}
		objs, fetchErr := f.FetchOneDatabase(ctx, dbName)
		result[dbName] = objects.FetchResult{
			Objects: objs,
			Error:   fetchErr,
		}
	}

	return result, nil
}

func (f *objectFetcher) FetchOneDatabase(ctx context.Context, databaseName string) ([]*dbobjectv1.DatabaseObject, error) {
	conn, err := f.connectAsAdmin(ctx, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close(ctx)

	objsFetched, err := fetchDatabaseObjects(ctx, f.db, databaseName, conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	f.cfg.Log.InfoContext(ctx, "fetched objects from database", "count", len(objsFetched))

	return objsFetched, nil
}

func (f *objectFetcher) getDatabaseNames(ctx context.Context) ([]string, error) {
	dbName := f.db.GetAdminUser().DefaultDatabase
	if dbName == "" {
		dbName = "postgres"
		f.cfg.Log.WarnContext(ctx, "No default database configured, using default.", "db_name", dbName)
	}
	conn, err := f.connectAsAdmin(ctx, dbName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(context.Background(), "SELECT pg_database.datname FROM pg_catalog.pg_database WHERE pg_database.datistemplate = false;")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var datname string
		err := rows.Scan(&datname)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		databases = append(databases, datname)
	}

	if rows.Err() != nil {
		return nil, trace.Wrap(rows.Err())
	}

	return databases, nil
}

func (f *objectFetcher) connectAsAdmin(ctx context.Context, databaseName string) (*pgx.Conn, error) {
	conn := &connector{
		auth:       f.cfg.Auth,
		gcpClients: f.cfg.GCPClients,
		log:        f.cfg.Log,

		certExpiry:   time.Now().Add(time.Hour),
		database:     f.db,
		databaseUser: f.db.GetAdminUser().Name,
		databaseName: databaseName,

		startupParams: map[string]string{},
	}
	return conn.connectAsAdmin(ctx)
}
