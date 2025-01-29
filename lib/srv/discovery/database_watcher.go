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

package discovery

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/slices"
)

const databaseEventPrefix = "db/"

func (s *Server) startDatabaseWatchers() error {
	if len(s.databaseFetchers) == 0 && s.dynamicMatcherWatcher == nil {
		return nil
	}

	var (
		newDatabases []types.Database
		mu           sync.Mutex
	)

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig[types.Database]{
			Matcher:             func(database types.Database) bool { return true },
			GetCurrentResources: s.getCurrentDatabases,
			GetNewResources: func() map[string]types.Database {
				mu.Lock()
				defer mu.Unlock()
				return utils.FromSlice(newDatabases, types.Database.GetName)
			},
			Logger:   s.Log.With("kind", types.KindDatabase),
			OnCreate: s.onDatabaseCreate,
			OnUpdate: s.onDatabaseUpdate,
			OnDelete: s.onDatabaseDelete,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := common.NewWatcher(s.ctx,
		common.WatcherConfig{
			FetchersFn:     s.getAllDatabaseFetchers,
			Logger:         s.Log.With("kind", types.KindDatabase),
			DiscoveryGroup: s.DiscoveryGroup,
			Interval:       s.PollInterval,
			TriggerFetchC:  s.newDiscoveryConfigChangedSub(),
			Origin:         types.OriginCloud,
			Clock:          s.clock,
			PreFetchHookFn: s.databaseWatcherIterationStarted,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			discoveryConfigsChanged := map[string]struct{}{}
			resourcesFoundByGroup := make(map[awsResourceGroup]int)

			select {
			case newResources := <-watcher.ResourcesC():
				dbs := make([]types.Database, 0, len(newResources))
				for _, r := range newResources {
					db, ok := r.(types.Database)
					if !ok {
						continue
					}

					resourceGroup := awsResourceGroupFromLabels(db.GetStaticLabels())
					resourcesFoundByGroup[resourceGroup] += 1
					discoveryConfigsChanged[resourceGroup.discoveryConfigName] = struct{}{}

					dbs = append(dbs, db)

					s.collectRDSIssuesAsUserTasks(db, resourceGroup.integration, resourceGroup.discoveryConfigName)
				}
				mu.Lock()
				newDatabases = dbs
				mu.Unlock()

				for group, count := range resourcesFoundByGroup {
					s.awsRDSResourcesStatus.incrementFound(group, count)
				}

				if err := reconciler.Reconcile(s.ctx); err != nil {
					s.Log.WarnContext(s.ctx, "Unable to reconcile database resources", "error", err)

					// When reconcile fails, it is assumed that everything failed.
					for group, count := range resourcesFoundByGroup {
						s.awsRDSResourcesStatus.incrementFailed(group, count)
					}

					break
				}

				for group, count := range resourcesFoundByGroup {
					s.awsRDSResourcesStatus.incrementEnrolled(group, count)
				}

				if s.onDatabaseReconcile != nil {
					s.onDatabaseReconcile()
				}

			case <-s.ctx.Done():
				return
			}

			for dc := range discoveryConfigsChanged {
				s.updateDiscoveryConfigStatus(dc)
			}
			s.upsertTasksForAWSRDSFailedEnrollments()
		}
	}()
	return nil
}

// collectRDSIssuesAsUserTasks receives a discovered database converted into a Teleport Database resource and creates
// an User Task (discover-rds) if that Database is not properly configured to be accessed from Teleport.
// Eg, an UserTask is created if the IAM DB Authentication is not enabled
func (s *Server) collectRDSIssuesAsUserTasks(db types.Database, integration, discoveryConfigName string) {
	if integration == "" || discoveryConfigName == "" || !db.IsRDS() {
		return
	}

	if db.GetAWS().RDS.IAMAuth {
		return
	}

	isCluster := db.GetAWS().RDS.ClusterID != ""
	databaseIdentifier := db.GetAWS().RDS.InstanceID
	if isCluster {
		databaseIdentifier = db.GetAWS().RDS.ClusterID
	}

	engine := db.GetStaticLabels()[types.DiscoveryLabelEngine]

	s.awsRDSTasks.addFailedEnrollment(
		awsRDSTaskKey{
			integration: integration,
			issueType:   usertasks.AutoDiscoverRDSIssueIAMAuthenticationDisabled,
			accountID:   db.GetAWS().AccountID,
			region:      db.GetAWS().Region,
		},
		&usertasksv1.DiscoverRDSDatabase{
			DiscoveryConfig: discoveryConfigName,
			DiscoveryGroup:  s.DiscoveryGroup,
			SyncTime:        timestamppb.New(s.clock.Now()),
			Name:            databaseIdentifier,
			IsCluster:       isCluster,
			Engine:          engine,
		},
	)
}

func (s *Server) databaseWatcherIterationStarted() {
	allFetchers := s.getAllDatabaseFetchers()
	if len(allFetchers) == 0 {
		return
	}

	s.submitFetchersEvent(allFetchers)

	awsResultGroups := slices.FilterMapUnique(
		allFetchers,
		func(f common.Fetcher) (awsResourceGroup, bool) {
			include := f.GetDiscoveryConfigName() != "" && f.IntegrationName() != ""
			resourceGroup := awsResourceGroup{
				discoveryConfigName: f.GetDiscoveryConfigName(),
				integration:         f.IntegrationName(),
			}
			return resourceGroup, include
		},
	)

	for _, g := range awsResultGroups {
		s.awsRDSResourcesStatus.iterationStarted(g)
	}

	discoveryConfigs := slices.FilterMapUnique(awsResultGroups, func(g awsResourceGroup) (s string, include bool) {
		return g.discoveryConfigName, true
	})
	s.updateDiscoveryConfigStatus(discoveryConfigs...)

	s.awsRDSResourcesStatus.reset()
	s.awsRDSTasks.reset()
}

func (s *Server) getAllDatabaseFetchers() []common.Fetcher {
	allFetchers := make([]common.Fetcher, 0, len(s.databaseFetchers))

	s.muDynamicDatabaseFetchers.RLock()
	for _, fetcherSet := range s.dynamicDatabaseFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicDatabaseFetchers.RUnlock()

	allFetchers = append(allFetchers, s.databaseFetchers...)

	return allFetchers
}

func (s *Server) getCurrentDatabases() map[string]types.Database {
	databases, err := s.AccessPoint.GetDatabases(s.ctx)
	if err != nil {
		s.Log.WarnContext(s.ctx, "Failed to get databases from cache", "error", err)
		return nil
	}

	return utils.FromSlice[types.Database](filterResources(databases, types.OriginCloud, s.DiscoveryGroup), func(database types.Database) string {
		return database.GetName()
	})
}

func (s *Server) onDatabaseCreate(ctx context.Context, database types.Database) error {
	s.Log.DebugContext(ctx, "Creating database", "database", database.GetName())
	err := s.AccessPoint.CreateDatabase(ctx, database)
	// If the database already exists but has cloud origin and an empty
	// discovery group, then update it.
	if err != nil {
		err := s.resolveCreateErr(err, types.OriginCloud, func() (types.ResourceWithLabels, error) {
			return s.AccessPoint.GetDatabase(ctx, database.GetName())
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.onDatabaseUpdate(ctx, database, nil))
	}
	err = s.emitUsageEvents(map[string]*usageeventsv1.ResourceCreateEvent{
		databaseEventPrefix + database.GetName(): {
			ResourceType:   types.DiscoveredResourceDatabase,
			ResourceOrigin: types.OriginCloud,
			CloudProvider:  database.GetCloud(),
			Database: &usageeventsv1.DiscoveredDatabaseMetadata{
				DbType:     database.GetType(),
				DbProtocol: database.GetProtocol(),
			},
		},
	})
	if err != nil {
		s.Log.DebugContext(ctx, "Error emitting usage event", "error", err)
	}
	return nil
}

func (s *Server) onDatabaseUpdate(ctx context.Context, database, _ types.Database) error {
	s.Log.DebugContext(ctx, "Updating database", "database", database.GetName())
	return trace.Wrap(s.AccessPoint.UpdateDatabase(ctx, database))
}

func (s *Server) onDatabaseDelete(ctx context.Context, database types.Database) error {
	s.Log.DebugContext(ctx, "Deleting database", "database", database.GetName())
	return trace.Wrap(s.AccessPoint.DeleteDatabase(ctx, database.GetName()))
}

func filterResources[T types.ResourceWithLabels, S ~[]T](all S, wantOrigin, wantResourceGroup string) (filtered S) {
	for _, resource := range all {
		resourceDiscoveryGroup, _ := resource.GetLabel(types.TeleportInternalDiscoveryGroupName)
		if (wantOrigin != "" && resource.Origin() != wantOrigin) || resourceDiscoveryGroup != wantResourceGroup {
			continue
		}
		filtered = append(filtered, resource)

	}
	return
}
