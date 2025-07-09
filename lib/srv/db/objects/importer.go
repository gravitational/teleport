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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// singleDatabaseImporter handles importing of objects from a single database.
type singleDatabaseImporter struct {
	cfg Config

	database types.Database
	fetcher  ObjectFetcher

	objects map[string]*objWithExpiry
}

// objWithExpiry holds an object separate to its expiry time, avoiding the need for custom equality method for db objects and making the expiry checks explicit.
type objWithExpiry struct {
	obj    *dbobjectv1.DatabaseObject
	expiry time.Time
}

func startDatabaseImporter(ctx context.Context, cfg Config, database types.Database) (context.CancelFunc, error) {
	cfg.Log = cfg.Log.With("database", database.GetName(), "protocol", database.GetProtocol())

	fetcher, err := GetObjectFetcher(ctx, database, ObjectFetcherConfig{
		ImportRules:  cfg.ImportRules,
		Auth:         cfg.Auth,
		CloudClients: cfg.CloudClients,
		Log:          cfg.Log,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	imp := newSingleDatabaseImporter(cfg, database, fetcher)
	go imp.start(cancelCtx)
	cfg.Log.InfoContext(ctx, "Successfully started database importer.")
	return cancel, nil
}

func newSingleDatabaseImporter(cfg Config, database types.Database, fetcher ObjectFetcher) *singleDatabaseImporter {
	return &singleDatabaseImporter{
		cfg:      cfg,
		database: database,
		fetcher:  fetcher,
		objects:  make(map[string]*objWithExpiry),
	}
}

func (i *singleDatabaseImporter) start(ctx context.Context) {
	i.cfg.Log.DebugContext(ctx, "Starting database importer",
		"scan_interval", i.cfg.ScanInterval.String(),
		"object_ttl", i.cfg.ObjectTTL.String(),
		"refresh_threshold", i.cfg.RefreshThreshold.String(),
	)
	ticker := interval.New(interval.Config{
		Jitter:        retryutils.SeventhJitter,
		Duration:      i.cfg.ScanInterval * 7 / 6,
		FirstDuration: retryutils.FullJitter(i.cfg.ScanInterval),
	})
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Next():
			i.scan(ctx)
		case <-ctx.Done():
			i.cfg.Log.DebugContext(ctx, "Shutting down database importer.")
			return
		}
	}
}

func (i *singleDatabaseImporter) scan(ctx context.Context) {
	start := i.cfg.Clock.Now()
	i.cfg.Log.DebugContext(ctx, "Scanning database objects.")
	objectsNew, err := i.fetchObjects(ctx)
	if err != nil {
		i.cfg.Log.ErrorContext(ctx, "Error fetching objects", "error", err)
		return
	}

	objectsNewMap := utils.FromSlice(objectsNew, func(object *dbobjectv1.DatabaseObject) string {
		return object.GetMetadata().Name
	})

	i.deleteObjects(ctx, calculateDeleted(ctx, i.cfg, i.objects, objectsNewMap))
	i.updateObjects(ctx, calculateUpdates(ctx, i.cfg, i.objects, objectsNewMap))
	elapsed := i.cfg.Clock.Since(start)
	i.cfg.Log.DebugContext(ctx, "Scanning done.", "elapsed", elapsed)
}

func calculateDeleted(ctx context.Context, cfg Config, objects map[string]*objWithExpiry, objsNew map[string]*dbobjectv1.DatabaseObject) []string {
	var deleted []string
	for key := range objects {
		_, found := objsNew[key]
		if !found {
			deleted = append(deleted, key)
		}
	}

	cfg.Log.DebugContext(ctx, "Objects to delete", "count", len(deleted))
	return deleted
}

// deleteObjects function deletes specified keys from backend and internal state.
func (i *singleDatabaseImporter) deleteObjects(ctx context.Context, deleted []string) {
	var errs []error
	for _, key := range deleted {
		delete(i.objects, key)
		err := i.cfg.DatabaseObjectClient.DeleteDatabaseObject(ctx, key)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		i.cfg.Log.ErrorContext(ctx, "Failed to delete some objects.", "error_count", len(errs), "errs", errs)
	}
}

func calculateUpdates(ctx context.Context, cfg Config, objects map[string]*objWithExpiry, objsNew map[string]*dbobjectv1.DatabaseObject) map[string]*objWithExpiry {
	updated := make(map[string]*objWithExpiry)
	now := cfg.Clock.Now()
	expiry := now.Add(cfg.ObjectTTL)

	var countNew, countChanged, countRefresh int

	for key, objNew := range objsNew {
		objOld, found := objects[key]
		// completely new object
		if !found {
			countNew++
			updated[key] = &objWithExpiry{
				obj:    objNew,
				expiry: expiry,
			}
			continue
		}

		// previously seen object, check for changes.
		// we can safely use proto.Equal as these objects are free of revision etc.
		if !proto.Equal(objOld.obj, objNew) {
			countChanged++
			updated[key] = &objWithExpiry{
				obj:    objNew,
				expiry: expiry,
			}
			continue
		}

		// do we need to refresh the object in backend?
		if objOld.expiry.Sub(now) < cfg.RefreshThreshold {
			countRefresh++
			updated[key] = &objWithExpiry{
				obj:    objNew,
				expiry: expiry,
			}
		}
	}

	cfg.Log.DebugContext(ctx, "Objects to update", "new", countNew, "changed", countChanged, "refreshed", countRefresh)

	return updated
}

func (i *singleDatabaseImporter) updateObjects(ctx context.Context, updated map[string]*objWithExpiry) {
	// upsert changed objects
	var errs []error
	for key, objNew := range updated {
		i.objects[key] = objNew
		clone := proto.Clone(objNew.obj).(*dbobjectv1.DatabaseObject)
		clone.Metadata.Expires = timestamppb.New(objNew.expiry)
		_, err := i.cfg.DatabaseObjectClient.UpsertDatabaseObject(ctx, clone)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		i.cfg.Log.ErrorContext(ctx, "Errors occurred when updating objects", "error_count", len(errs), "errors", errs)
	}
}

func (i *singleDatabaseImporter) fetchObjects(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
	var objs []*dbobjectv1.DatabaseObject
	results, err := i.fetcher.FetchAll(ctx, func(_ string) bool { return true })
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, result := range results {
		objs = append(objs, result.Objects...)
	}
	return objs, nil
}
