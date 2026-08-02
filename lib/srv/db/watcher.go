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
	"iter"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/services/reconcile"
	discovery "github.com/gravitational/teleport/lib/srv/discovery/common"
	dbfetchers "github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
)

// startReconciler starts the reconciler that registers/unregisters proxied
// databases according to the desired database set: static configuration,
// databases imported from the cloud, and — when a database cache is
// available — dynamic database resources read directly from a consistent
// cache snapshot on every cycle. No copy of the dynamic set is retained
// between cycles; the cache's change pulse is the wake-up signal.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := reconcile.New(reconcile.Config[string, types.Database, readonly.Database]{
		Matcher:    s.matcher,
		GetCurrent: s.getProxiedDatabaseUnfiltered,
		RangeCurrent: func() iter.Seq2[string, types.Database] {
			return func(yield func(string, types.Database) bool) {
				s.mu.RLock()
				defer s.mu.RUnlock()
				for name, database := range s.proxiedDatabases {
					if !yield(name, database) {
						return
					}
				}
			}
		},
		RangeDesired: func() iter.Seq2[readonly.Database, error] {
			return s.rangeMonitoredDatabases(ctx)
		},
		KeyOf:       readonly.Database.GetName,
		Materialize: func(v readonly.Database) types.Database { return v.Copy() },
		CompareWithCurrent: func(current types.Database, view readonly.Database) int {
			if view.IsEqual(current) {
				return reconcile.Equal
			}
			return reconcile.Different
		},
		OnCreate: s.onCreate,
		OnUpdate: s.onUpdate,
		OnDelete: s.onDelete,
		Changes:  s.databaseChangesOrNil,
		Logger:   s.log.With("kind", types.KindDatabase),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	s.reconciler = reconciler
	go reconciler.Run(ctx)
	return nil
}

// databaseChangesOrNil returns the database cache's change pulse, or nil —
// which blocks forever in a select — when the dynamic-resource leg is
// disabled.
func (s *Server) databaseChangesOrNil() <-chan struct{} {
	if s.cfg.DatabasesCache == nil || len(s.cfg.ResourceMatchers) == 0 {
		return nil
	}
	return s.cfg.DatabasesCache.DatabaseChanges()
}

// rangeMonitoredDatabases yields read-only views of the desired database set
// in decreasing precedence order — cloud, dynamic (cache snapshot), static —
// matching the last-wins map merge order of the legacy reconciler under the
// view reconciler's first-occurrence-wins contract. The cloud slice is
// captured once at cycle start (it is replaced wholesale, never mutated, and
// every replacement is followed by a Trigger, so a mid-cycle swap converges
// on the next cycle). As views are yielded, s.cloudDatabaseNames and
// s.dynamicDatabaseNames are rebuilt for the source checks performed by the
// matcher and the create/update callbacks within the same cycle.
//
// Must only be invoked from the reconciler goroutine.
func (s *Server) rangeMonitoredDatabases(ctx context.Context) iter.Seq2[readonly.Database, error] {
	return func(yield func(readonly.Database, error) bool) {
		clear(s.dynamicDatabaseNames)
		clear(s.cloudDatabaseNames)

		var cloud types.Databases
		if dbs := s.cloudDatabases.Load(); dbs != nil {
			cloud = *dbs
		}

		for _, database := range cloud {
			s.cloudDatabaseNames[database.GetName()] = struct{}{}
			if !yield(database, nil) {
				return
			}
		}

		if s.cfg.DatabasesCache != nil && len(s.cfg.ResourceMatchers) > 0 {
			for database, err := range s.cfg.DatabasesCache.RangeReadonlyDatabases(ctx, "", "") {
				if err != nil {
					yield(nil, trace.Wrap(err))
					return
				}
				// A name already claimed by a cloud database is not a
				// dynamic resource: the cloud entry wins.
				if _, ok := s.cloudDatabaseNames[database.GetName()]; !ok {
					s.dynamicDatabaseNames[database.GetName()] = struct{}{}
				}
				if !yield(database, nil) {
					return
				}
			}
		}

		for _, database := range s.staticDatabases {
			if !yield(database, nil) {
				return
			}
		}
	}
}

// isDynamicResource returns whether the named database is a dynamic database
// resource (a db object) as of the current reconciliation cycle. Must only be
// used from the reconciler goroutine.
func (s *Server) isDynamicResource(name string) bool {
	_, ok := s.dynamicDatabaseNames[name]
	return ok
}

// isDiscoveryResource returns whether the named database is a dynamic
// resource created by the discovery service. Must only be used from the
// reconciler goroutine.
func (s *Server) isDiscoveryResource(database types.Database) bool {
	return database.Origin() == types.OriginCloud && s.isDynamicResource(database.GetName())
}

// startCloudWatcher starts fetching cloud databases according to the
// selectors and register/unregister them appropriately.
func (s *Server) startCloudWatcher(ctx context.Context) error {
	awsFetchers, err := s.cfg.AWSDatabaseFetcherFactory.MakeFetchers(ctx, s.cfg.AWSMatchers, "" /* discovery config */)
	if err != nil {
		return trace.Wrap(err)
	}
	azureFetchers, err := dbfetchers.MakeAzureFetchers(ctx, func(ctx context.Context, integration string) (azure.Clients, error) {
		if integration != "" {
			return nil, trace.NotImplemented("db_service discovery does not support Azure OIDC authentication; use discovery_service instead.")
		}
		return s.cfg.AzureClients, nil
	}, s.cfg.AzureMatchers, "" /* discovery config */)
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
					cloud := types.Databases(databases)
					s.cloudDatabases.Store(&cloud)
				} else {
					s.log.WarnContext(ctx, "Failed to convert resources to databases.", "error", err)
				}
				s.reconciler.Trigger()
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// onCreate is called by reconciler when a new database is created.
func (s *Server) onCreate(ctx context.Context, database types.Database) error {
	// only apply resource matcher settings to dynamic resources.
	if s.isDynamicResource(database.GetName()) {
		s.applyAWSResourceMatcherSettings(database)
	}

	// Run DiscoveryResourceChecker after resource matchers are applied to make
	// sure the correct AssumeRoleARN is used.
	if s.isDiscoveryResource(database) {
		if err := s.cfg.discoveryResourceChecker.Check(ctx, database); err != nil {
			return trace.Wrap(err)
		}
	}
	return s.registerDatabase(ctx, database)
}

// onUpdate is called by reconciler when an already proxied database is updated.
func (s *Server) onUpdate(ctx context.Context, database, _ types.Database) error {
	// only apply resource matcher settings to dynamic resources.
	if s.isDynamicResource(database.GetName()) {
		s.applyAWSResourceMatcherSettings(database)
	}

	// Run DiscoveryResourceChecker after resource matchers are applied to make
	// sure the correct AssumeRoleARN is used.
	if s.isDiscoveryResource(database) {
		if err := s.cfg.discoveryResourceChecker.Check(ctx, database); err != nil {
			return trace.Wrap(err)
		}
	}
	return s.updateDatabase(ctx, database)
}

// onDelete is called by reconciler when a proxied database is deleted.
func (s *Server) onDelete(ctx context.Context, database types.Database) error {
	return s.unregisterDatabase(ctx, database)
}

// matcher is used by the reconciler to check if a database matches selectors.
// It observes a shared read-only view and must not retain or mutate it. Must
// only be used from the reconciler goroutine.
func (s *Server) matcher(database readonly.Database) bool {
	// In the case of databases discovered by this database server, matchers
	// should be skipped.
	if _, ok := s.cloudDatabaseNames[database.GetName()]; ok {
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
