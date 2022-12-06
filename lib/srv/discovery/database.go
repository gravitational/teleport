/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package discovery

import (
	"context"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/cloud/watchers"
)

func (s *Server) startDatabaseDiscovery() error {
	if s.dbWatcherConfig.IsEmpty() {
		return nil
	}

	watcher, err := watchers.NewWatcher(s.ctx, s.dbWatcherConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	var newDatabases types.Databases
	var mu sync.Mutex

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig{
			Matcher:             func(_ types.ResourceWithLabels) bool { return true },
			GetCurrentResources: s.getCurrentDatabases,
			GetNewResources: func() types.ResourcesWithLabelsMap {
				mu.Lock()
				defer mu.Unlock()
				return newDatabases.AsResources().ToMap()
			},
			Log:      s.Log,
			OnCreate: s.onDatabaseCreate,
			OnUpdate: s.onDatabaseUpdate,
			OnDelete: s.onDatebaseDelete,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	s.Log.Debug("Starting database discovery.")
	go watcher.Start()

	go func() {
		defer s.Log.Debug("Database discovery done.")
		for {
			select {
			case databases := <-watcher.DatabasesC():
				mu.Lock()
				newDatabases = databases
				mu.Unlock()

				if err := reconciler.Reconcile(s.ctx); err != nil {
					s.Log.WithError(err).Warn("Failed to reconcile database resources.")
				} else if s.onDatabaseReconcile != nil {
					s.onDatabaseReconcile()
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Server) getCurrentDatabases() types.ResourcesWithLabelsMap {
	databases, err := s.AccessPoint.GetDatabases(s.ctx)
	if err != nil {
		s.Log.WithError(err).Warn("Failed to get databases from cache.")
		return nil
	}

	return types.Databases(filterResourcesByOrigin(databases, types.OriginCloud)).AsResources().ToMap()
}

func (s *Server) onDatabaseCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.Database, received %T", database)
	}
	s.Log.Debugf("Creating database %s.", database.GetName())
	return trace.Wrap(s.AccessPoint.CreateDatabase(ctx, database))
}

func (s *Server) onDatabaseUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.Database, received %T", database)
	}
	s.Log.Debugf("Updating database %s.", database.GetName())
	return trace.Wrap(s.AccessPoint.UpdateDatabase(ctx, database))
}

func (s *Server) onDatebaseDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("invalid type received; expected types.Database, received %T", database)
	}
	s.Log.Debugf("Deleting database %s.", database.GetName())
	return trace.Wrap(s.AccessPoint.DeleteDatabase(ctx, database.GetName()))
}

func filterResourcesByOrigin[T types.ResourceWithOrigin, S ~[]T](allResources S, wantOrigin string) (cloudResources S) {
	for _, resource := range allResources {
		if resource.Origin() == wantOrigin {
			cloudResources = append(cloudResources, resource)
		}
	}
	return
}
