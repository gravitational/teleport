/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package db

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
	dbfetchers "github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/utils"
)

// startReconciler starts reconciler that registers/unregisters proxied
// databases according to the up-to-date list of database resources and
// databases imported from the cloud.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.Database]{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredDatabases.getLocked,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Logger:              s.log.With("kind", types.KindDatabase),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		for {
			select {
			case <-s.reconcileCh:
				// don't let monitored dbs change during reconciliation
				s.monitoredDatabases.mu.RLock()
				if err := reconciler.Reconcile(ctx); err != nil {
					s.log.ErrorContext(ctx, "Failed to reconcile.", "error", err)
				}
				if s.cfg.OnReconcile != nil {
					s.cfg.OnReconcile(s.getProxiedDatabases())
				}
				s.monitoredDatabases.mu.RUnlock()
			case <-ctx.Done():
				s.log.DebugContext(ctx, "Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startResourceWatcher starts watching changes to database resources and
// registers/unregisters the proxied databases accordingly.
func (s *Server) startResourceWatcher(ctx context.Context) (*services.GenericWatcher[types.Database, readonly.Database], error) {
	if len(s.cfg.ResourceMatchers) == 0 {
		s.log.DebugContext(ctx, "Not starting database resource watcher.")
		return nil, nil
	}
	s.log.DebugContext(ctx, "Starting database resource watcher.")
	watcher, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Logger:    s.log,
			Client:    s.cfg.AccessPoint,
		},
		DatabaseGetter: s.cfg.AccessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer s.log.DebugContext(ctx, "Database resource watcher done.")
		defer watcher.Close()
		for {
			select {
			case databases := <-watcher.ResourcesC:
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
	awsFetchers, err := s.cfg.AWSDatabaseFetcherFactory.MakeFetchers(ctx, s.cfg.AWSMatchers, "" /* discovery config */)
	if err != nil {
		return trace.Wrap(err)
	}
	azureFetchers, err := dbfetchers.MakeAzureFetchers(s.cfg.CloudClients, s.cfg.AzureMatchers, "" /* discovery config */)
	if err != nil {
		return trace.Wrap(err)
	}

	allFetchers := append(awsFetchers, azureFetchers...)
	if len(allFetchers) == 0 {
		s.log.DebugContext(ctx, "Not starting cloud database watcher.", "error", err)
		return nil
	}

	watcher, err := discovery.NewWatcher(ctx, discovery.WatcherConfig{
		FetchersFn: discovery.StaticFetchers(allFetchers),
		Logger:     slog.With(teleport.ComponentKey, "watcher:cloud"),
		Origin:     types.OriginCloud,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()
	go func() {
		defer s.log.DebugContext(ctx, "Cloud database watcher done.")
		for {
			select {
			case resources := <-watcher.ResourcesC():
				databases, err := resources.AsDatabases()
				if err == nil {
					s.monitoredDatabases.setCloud(databases)
				} else {
					s.log.WarnContext(ctx, "Failed to convert resources to databases.", "error", err)
				}
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
func (s *Server) getResources() map[string]types.Database {
	return utils.FromSlice(s.getProxiedDatabases(), types.Database.GetName)
}

// onCreate is called by reconciler when a new database is created.
func (s *Server) onCreate(ctx context.Context, database types.Database) error {
	// OnCreate receives a "new" resource from s.monitoredDatabases. Make a
	// copy here so that any attribute changes to the proxied database will not
	// affect database objects tracked in s.monitoredDatabases.
	databaseCopy := database.Copy()

	// only apply resource matcher settings to dynamic resources.
	if s.monitoredDatabases.isResource_Locked(database) {
		s.applyAWSResourceMatcherSettings(databaseCopy)
	}

	// Run DiscoveryResourceChecker after resource matchers are applied to make
	// sure the correct AssumeRoleARN is used.
	if s.monitoredDatabases.isDiscoveryResource_Locked(database) {
		if err := s.cfg.discoveryResourceChecker.Check(ctx, databaseCopy); err != nil {
			return trace.Wrap(err)
		}
	}
	return s.registerDatabase(ctx, databaseCopy)
}

// onUpdate is called by reconciler when an already proxied database is updated.
func (s *Server) onUpdate(ctx context.Context, database, _ types.Database) error {
	// OnUpdate receives a "new" resource from s.monitoredDatabases. Make a
	// copy here so that any attribute changes to the proxied database will not
	// affect database objects tracked in s.monitoredDatabases.
	databaseCopy := database.Copy()

	// only apply resource matcher settings to dynamic resources.
	if s.monitoredDatabases.isResource_Locked(database) {
		s.applyAWSResourceMatcherSettings(databaseCopy)
	}
	return s.updateDatabase(ctx, databaseCopy)
}

// onDelete is called by reconciler when a proxied database is deleted.
func (s *Server) onDelete(ctx context.Context, database types.Database) error {
	return s.unregisterDatabase(ctx, database)
}

// matcher is used by reconciler to check if database matches selectors.
func (s *Server) matcher(database types.Database) bool {
	// In the case of databases discovered by this database server, matchers
	// should be skipped.
	if s.monitoredDatabases.isCloud_Locked(database) {
		return true // Cloud fetchers return only matching databases.
	}

	// Database resources created via CLI, API, or discovery service are
	// filtered by resource matchers.
	return services.MatchResourceLabels(s.cfg.ResourceMatchers, database.GetAllLabels())
}

func (s *Server) applyAWSResourceMatcherSettings(database types.Database) {
	if !database.IsAWSHosted() {
		// dynamic matchers only apply AWS settings (for now), so skip non-AWS
		// databases.
		return
	}
	dbLabels := database.GetAllLabels()
	for _, matcher := range s.cfg.ResourceMatchers {
		if len(matcher.Labels) == 0 || matcher.AWS.AssumeRoleARN == "" {
			continue
		}
		if match, _, _ := services.MatchLabels(matcher.Labels, dbLabels); !match {
			continue
		}

		// Set status AWS instead of spec. Reconciler ignores status fields
		// when comparing database resources.
		setStatusAWSAssumeRole(database, matcher.AWS.AssumeRoleARN, matcher.AWS.ExternalID)
	}
}

func setStatusAWSAssumeRole(database types.Database, assumeRoleARN, externalID string) {
	meta := database.GetAWS()
	meta.AssumeRoleARN = assumeRoleARN
	meta.ExternalID = externalID
	database.SetStatusAWS(meta)
}
