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
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	libcloud "github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
)

// ImportRulesReader provides a method to get current set of import rules.
type ImportRulesReader interface {
	GetDatabaseObjectImportRules(ctx context.Context) ([]*dbobjectimportrulev1.DatabaseObjectImportRule, error)
}

// ObjectFetcherConfig provides static object fetcher configuration.
type ObjectFetcherConfig struct {
	ImportRules ImportRulesReader
	Auth        common.Auth
	GCPClients  libcloud.GCPClients
	Log         *slog.Logger
}

// ObjectFetcher defines an interface for retrieving database objects.
type ObjectFetcher interface {
	// FetchAll fetches objects from all databases whose names are accepted by dbNameFilter.
	FetchAll(ctx context.Context, dbNameFilter databaseobjectimportrule.DbNameFilter) (map[string]FetchResult, error)
	// FetchOneDatabase fetches all objects from a single named database.
	FetchOneDatabase(ctx context.Context, dbName string) ([]*dbobjectv1.DatabaseObject, error)
}

// FetchResult contains fetch result for a single database.
type FetchResult struct {
	Objects []*dbobjectv1.DatabaseObject
	Error   error
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

// GetObjectFetcher returns a new object fetcher for given database, respecting global import rules.
func GetObjectFetcher(ctx context.Context, db types.Database, fetcherConfig ObjectFetcherConfig) (ObjectFetcher, error) {
	name := db.GetProtocol()
	objectFetchersMutex.RLock()
	constructor, found := objectFetchers[name]
	objectFetchersMutex.RUnlock()

	if !found {
		return nil, trace.NotImplemented("fetcher not implemented for protocol %q", name)
	}

	fetcher, err := constructor(ctx, db, fetcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// return wrapped fetcher to apply the object import rules.
	return &applyRulesFetcher{
		cfg:          fetcherConfig,
		database:     db,
		innerFetcher: fetcher,
	}, nil
}

// applyRulesFetcher wraps an existing object fetcher and applies the import rules.
type applyRulesFetcher struct {
	cfg          ObjectFetcherConfig
	database     types.Database
	innerFetcher ObjectFetcher
}

func (a *applyRulesFetcher) FetchAll(ctx context.Context, dbNameFilter databaseobjectimportrule.DbNameFilter) (map[string]FetchResult, error) {
	rules, err := a.cfg.ImportRules.GetDatabaseObjectImportRules(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filterFromRules := databaseobjectimportrule.CalculateDatabaseNameFilter(rules, a.database)
	fetched, err := a.innerFetcher.FetchAll(ctx, func(dbName string) bool { return dbNameFilter(dbName) && filterFromRules(dbName) })
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make(map[string]FetchResult)
	for dbName, result := range fetched {
		out[dbName] = FetchResult{
			Objects: a.transform(ctx, dbName, rules, result.Objects),
			Error:   result.Error,
		}
	}

	return out, nil
}

func (a *applyRulesFetcher) FetchOneDatabase(ctx context.Context, dbName string) ([]*dbobjectv1.DatabaseObject, error) {
	rules, err := a.cfg.ImportRules.GetDatabaseObjectImportRules(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fetched, err := a.innerFetcher.FetchOneDatabase(ctx, dbName)
	return a.transform(ctx, dbName, rules, fetched), trace.Wrap(err)
}

func (a *applyRulesFetcher) transform(ctx context.Context, dbName string, rules []*dbobjectimportrulev1.DatabaseObjectImportRule, fetched []*dbobjectv1.DatabaseObject) []*dbobjectv1.DatabaseObject {
	transformed, errCount := databaseobjectimportrule.ApplyDatabaseObjectImportRules(ctx, a.cfg.Log, rules, a.database, fetched)
	if errCount > 0 {
		a.cfg.Log.WarnContext(ctx, "Failed to apply import rules to some objects.",
			"db_name", dbName,
			"error_count", errCount,
			"transformed", len(transformed),
			"fetched", len(fetched),
		)
	}
	return transformed
}
