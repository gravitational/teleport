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

package objects

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	libcloud "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

// ObjectFetcherConfig provides static object fetcher configuration.
type ObjectFetcherConfig struct {
	Auth         common.Auth
	CloudClients libcloud.Clients
	Log          *slog.Logger
}

// ObjectFetcher defines an interface for retrieving database objects.
type ObjectFetcher interface {
	// FetchDatabaseObjects fetches objects from the database.
	// If database supports multiple named databases (e.g. Postgres), the dbNameFilter can be used to determine if a particular database can be skipped.
	// Both the returned slice and error can be non-nil, indicating partial failures.
	// To minimize backend load, ensure fetched objects remain consistent across calls.
	FetchDatabaseObjects(ctx context.Context, dbNameFilter databaseobjectimportrule.DbNameFilter) ([]*dbobjectv1.DatabaseObject, error)
}

// ObjectFetcherFn is a database object fetcher constructor.
type ObjectFetcherFn = func(ctx context.Context, db types.Database, cfg ObjectFetcherConfig) (ObjectFetcher, error)

var (
	objectFetchers      = make(map[string]ObjectFetcherFn)
	objectFetchersMutex sync.RWMutex
)

// RegisterObjectFetcher registers a new object fetcher constructor.
func RegisterObjectFetcher(fn ObjectFetcherFn, names ...string) {
	objectFetchersMutex.Lock()
	defer objectFetchersMutex.Unlock()
	for _, name := range names {
		objectFetchers[name] = fn
	}
}

func unregisterObjectFetcher(names ...string) {
	objectFetchersMutex.Lock()
	defer objectFetchersMutex.Unlock()
	for _, name := range names {
		delete(objectFetchers, name)
	}
}

// getObjectFetcher returns a new object fetcher for given database.
// The returned fetcher may be nil, which indicates that particular database configuration is not supported.
func getObjectFetcher(ctx context.Context, db types.Database, cfg Config) (ObjectFetcher, error) {
	name := db.GetProtocol()
	objectFetchersMutex.RLock()
	constructor, found := objectFetchers[name]
	objectFetchersMutex.RUnlock()

	if !found {
		return nil, nil
	}

	imp, err := constructor(ctx, db, ObjectFetcherConfig{
		Auth:         cfg.Auth,
		CloudClients: cfg.CloudClients,
		Log:          cfg.Log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return imp, nil
}
