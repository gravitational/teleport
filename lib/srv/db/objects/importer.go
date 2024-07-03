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
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/srv/db/common/permissions"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

type databaseImporter interface {
	stop()
}

// singleDatabaseImporter handles importing of objects from a single database.
type singleDatabaseImporter struct {
	cfg    Config
	cancel context.CancelFunc

	database types.Database
	fetcher  ObjectFetcher

	objects map[string]*objWithExpiry
}

// objWithExpiry holds an object separate to its expiry time.
type objWithExpiry struct {
	obj    *dbobjectv1.DatabaseObject
	expiry time.Time
}

type noopImporter struct{}

func (*noopImporter) stop() {}

func startDatabaseImporter(ctx context.Context, cfg Config, database types.Database) (databaseImporter, error) {
	cfg.Log = cfg.Log.With("database", database.GetName(), "protocol", database.GetProtocol())

	fetcher, err := getObjectFetcher(ctx, database, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// nil fetcher indicates unsupported configuration
	if fetcher == nil {
		return &noopImporter{}, nil
	}

	imp := startSingleDatabaseImporter(ctx, cfg, database, fetcher)
	imp.start(ctx)
	cfg.Log.InfoContext(ctx, "Successfully started database importer.", "database", database.GetName(), "protocol", database.GetProtocol())
	return imp, nil
}

func startSingleDatabaseImporter(ctx context.Context, cfg Config, database types.Database, fetcher ObjectFetcher) *singleDatabaseImporter {
	cancelCtx, cancel := context.WithCancel(ctx)
	imp := &singleDatabaseImporter{
		cfg:      cfg,
		cancel:   cancel,
		database: database,
		fetcher:  fetcher,
		objects:  make(map[string]*objWithExpiry),
	}
	imp.start(cancelCtx)
	return imp
}

func (i *singleDatabaseImporter) stop() {
	i.cancel()
}

func (i *singleDatabaseImporter) start(ctx context.Context) {
	go func() {
		i.cfg.Log.DebugContext(ctx, "Starting database importer")
		ticker := interval.New(interval.Config{
			Jitter:        retryutils.NewSeventhJitter(),
			Duration:      i.cfg.ScanInterval * 7 / 6,
			FirstDuration: retryutils.NewHalfJitter()(time.Second * 10),
		})
		defer ticker.Stop()

		for {
			select {
			case <-ticker.Next():
				i.scan(ctx)
			case <-ctx.Done():
				i.cfg.Log.DebugContext(ctx, "Shutting down database importer")
				return
			}
		}
	}()
}

func (i *singleDatabaseImporter) scan(ctx context.Context) {
	objectsNew, err := i.fetchObjects(ctx)
	if err != nil {
		return
	}

	objectsNewMap := utils.FromSlice(objectsNew, func(object *dbobjectv1.DatabaseObject) string {
		return object.GetMetadata().Name
	})

	i.deleteObjects(ctx, calculateDeleted(ctx, i.cfg, i.objects, objectsNewMap))
	i.updateObjects(ctx, calculateUpdates(ctx, i.cfg, i.objects, objectsNewMap))
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
	client := i.cfg.AuthClient.DatabaseObjectsClient()

	var errs []error
	for _, key := range deleted {
		delete(i.objects, key)
		err := client.DeleteDatabaseObject(ctx, key)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		i.cfg.Log.ErrorContext(ctx, "Failed to delete some database objects: ", "error_count", len(errs), "errs", errs)
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
	client := i.cfg.AuthClient.DatabaseObjectsClient()

	// upsert changed objects
	var errs []error
	for key, objNew := range updated {
		i.objects[key] = objNew
		clone := proto.Clone(objNew.obj).(*dbobjectv1.DatabaseObject)
		clone.Metadata.Expires = timestamppb.New(objNew.expiry)
		_, err := client.UpsertDatabaseObject(ctx, clone)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		i.cfg.Log.ErrorContext(ctx, "Errors occurred when updating objects", "error_count", len(errs), "errs", errs)
	}
}

func (i *singleDatabaseImporter) fetchObjects(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
	rules, err := i.cfg.AuthClient.GetDatabaseObjectImportRules(ctx)
	if err != nil {
		i.cfg.Log.ErrorContext(ctx, "Failed to fetch database object import rules", "err", err)
		return nil, trace.Wrap(err)
	}

	dbNameFilter := databaseobjectimportrule.CalculateDatabaseNameFilter(rules, i.database)
	objsFetched, err := i.fetcher.FetchDatabaseObjects(ctx, dbNameFilter)
	if err != nil {
		i.cfg.Log.WarnContext(ctx, "Error while fetching database objects", "err", err, "imported", len(objsFetched))
	}

	// TODO(Tener): switch to cfg.log logger once ApplyDatabaseObjectImportRules is on slog too.
	objsImported, errCount := databaseobjectimportrule.ApplyDatabaseObjectImportRules(logrus.WithContext(ctx).WithField("db", i.database), rules, i.database, objsFetched)
	counts, _ := permissions.CountObjectKinds(objsImported)
	i.cfg.Log.InfoContext(ctx, "Database objects imported", "counts", counts, "err_count", errCount, "total", len(objsImported))

	return objsImported, nil
}
