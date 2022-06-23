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
	"github.com/gravitational/teleport/lib/cloud/watchers"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// startReconciler starts reconciler that registers/unregisters proxied
// databases according to the up-to-date list of database resources and
// databases imported from the cloud.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredDatabases.get,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Log:                 s.log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		for {
			select {
			case <-s.reconcileCh:
				if err := reconciler.Reconcile(ctx); err != nil {
					s.log.WithError(err).Error("Failed to reconcile.")
				} else if s.cfg.OnReconcile != nil {
					s.cfg.OnReconcile(s.getProxiedDatabases())
				}
			case <-ctx.Done():
				s.log.Debug("Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startResourceWatcher starts watching changes to database resources and
// registers/unregisters the proxied databases accordingly.
func (s *Server) startResourceWatcher(ctx context.Context) (*services.DatabaseWatcher, error) {
	if len(s.cfg.ResourceMatchers) == 0 {
		s.log.Debug("Not starting database resource watcher.")
		return nil, nil
	}
	s.log.Debug("Starting database resource watcher.")
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
		defer s.log.Debug("Database resource watcher done.")
		defer watcher.Close()
		for {
			select {
			case databases := <-watcher.DatabasesC:
				s.monitoredDatabases.setResources(databases)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return watcher, nil
}

// startCloudWatcher starts fetching cloud databases according to the
// selectors and register/unregister them appropriately.
func (s *Server) startCloudWatcher(ctx context.Context) error {
	watcher, err := watchers.NewWatcher(ctx, watchers.WatcherConfig{
		AWSMatchers: s.cfg.AWSMatchers,
		Clients:     s.cfg.CloudClients,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			s.log.Debugf("Not starting cloud database watcher: %v.", err)
			return nil
		}
		return trace.Wrap(err)
	}
	go watcher.Start()
	go func() {
		defer s.log.Debug("Cloud database watcher done.")
		for {
			select {
			case databases := <-watcher.DatabasesC():
				s.monitoredDatabases.setCloud(databases)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// getResources returns proxied databases as resources.
func (s *Server) getResources() types.ResourcesWithLabelsMap {
	return s.getProxiedDatabases().AsResources().ToMap()
}

// onCreate is called by reconciler when a new database is created.
func (s *Server) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.registerDatabase(ctx, database)
}

// onUpdate is called by reconciler when an already proxied database is updated.
func (s *Server) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.updateDatabase(ctx, database)
}

// onDelete is called by reconciler when a proxied database is deleted.
func (s *Server) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	database, ok := resource.(types.Database)
	if !ok {
		return trace.BadParameter("expected types.Database, got %T", resource)
	}
	return s.unregisterDatabase(ctx, database)
}

// matcher is used by reconciler to check if database matches selectors.
func (s *Server) matcher(resource types.ResourceWithLabels) bool {
	database, ok := resource.(types.Database)
	if !ok {
		return false
	}

	// In the case of CloudOrigin CloudHosted resources the matchers should be skipped.
	if cloudOrigin(resource) && database.IsCloudHosted() {
		return true // Cloud fetchers return only matching databases.
	}
	return services.MatchResourceLabels(s.cfg.ResourceMatchers, database)
}

func cloudOrigin(r types.ResourceWithLabels) bool {
	return r.Origin() == types.OriginCloud
}
