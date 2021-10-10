/*
Copyright 2021 Gravitational, Inc.

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

package db

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// startWatcher starts watching changes to database resources and registers /
// unregisters the proxied databases accordingly.
func (s *Server) startWatcher(ctx context.Context) (*services.DatabaseWatcher, error) {
	if len(s.cfg.Selectors) == 0 {
		s.log.Debug("Not initializing database resource watcher.")
		return nil, nil
	}
	s.log.Debug("Initializing database resource watcher.")
	watcher, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Log:       s.log,
			Client:    s.cfg.AccessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case databases := <-watcher.DatabasesC:
				if err := s.reconciler.Reconcile(ctx, databases.AsResources()); err != nil {
					s.log.WithError(err).Errorf("Failed to reconcile %v.", databases)
				} else if s.cfg.OnReconcile != nil {
					s.cfg.OnReconcile(s.getDatabases())
				}
			case <-ctx.Done():
				s.log.Debug("Database resource watcher done.")
				return
			}
		}
	}()
	return watcher, nil
}

func (s *Server) getReconciler() (*services.Reconciler, error) {
	return services.NewReconciler(services.ReconcilerConfig{
		Selectors:    s.cfg.Selectors,
		GetResources: s.getResources,
		OnCreate:     s.onCreate,
		OnUpdate:     s.onUpdate,
		OnDelete:     s.onDelete,
		Log:          s.log,
	})
}

func (s *Server) getResources() (resources types.ResourcesWithLabels) {
	return s.getDatabases().AsResources()
}

func (s *Server) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.registerDatabase(ctx, database)
}

func (s *Server) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.updateDatabase(ctx, database)
}

func (s *Server) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.unregisterDatabase(ctx, database)
}
