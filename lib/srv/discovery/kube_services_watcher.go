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
	"time"

	"github.com/gravitational/trace"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/utils"
)

const appEventPrefix = "app/"

func (s *Server) startKubeAppsWatchers() error {
	if len(s.kubeAppsFetchers) == 0 {
		return nil
	}

	var (
		appResources []types.Application
		mu           sync.Mutex
	)

	reconciler, err := services.NewReconciler(
		services.ReconcilerConfig[types.Application]{
			Matcher: func(_ types.Application) bool { return true },
			GetCurrentResources: func() map[string]types.Application {
				apps, err := s.AccessPoint.GetApps(s.ctx)
				if err != nil {
					s.Log.WarnContext(s.ctx, "Unable to get applications from cache", "error", err)
					return nil
				}

				return utils.FromSlice(filterResources(apps, types.OriginDiscoveryKubernetes, s.DiscoveryGroup), types.Application.GetName)
			},
			GetNewResources: func() map[string]types.Application {
				mu.Lock()
				defer mu.Unlock()
				return utils.FromSlice(appResources, types.Application.GetName)
			},
			Logger:   s.Log.With("kind", types.KindApp),
			OnCreate: s.onAppCreate,
			OnUpdate: s.onAppUpdate,
			OnDelete: s.onAppDelete,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := common.NewWatcher(s.ctx, common.WatcherConfig{
		FetchersFn:     common.StaticFetchers(s.kubeAppsFetchers),
		Interval:       5 * time.Minute,
		Logger:         s.Log.With("kind", types.KindApp),
		DiscoveryGroup: s.DiscoveryGroup,
		Origin:         types.OriginDiscoveryKubernetes,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go watcher.Start()

	go func() {
		for {
			select {
			case newResources := <-watcher.ResourcesC():
				apps := make([]types.Application, 0, len(newResources))
				for _, r := range newResources {
					app, ok := r.(types.Application)
					if !ok {
						continue
					}

					apps = append(apps, app)
				}

				mu.Lock()
				appResources = apps
				mu.Unlock()

				if err := reconciler.Reconcile(s.ctx); err != nil {
					s.Log.WarnContext(s.ctx, "Unable to reconcile resources", "error", err)
				}

			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *Server) onAppCreate(ctx context.Context, app types.Application) error {
	s.Log.DebugContext(ctx, "Creating app", "app_name", app.GetName())
	err := s.AccessPoint.CreateApp(ctx, app)
	// If the resource already exists, it means that the resource was created
	// by a previous discovery_service instance that didn't support the discovery
	// group feature or the discovery group was changed.
	// In this case, we need to update the resource with the
	// discovery group label to ensure the user doesn't have to manually delete
	// the resource.
	if err != nil {
		err := s.resolveCreateErr(err, types.OriginDiscoveryKubernetes, func() (types.ResourceWithLabels, error) {
			return s.AccessPoint.GetApp(ctx, app.GetName())
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(s.onAppUpdate(ctx, app, nil))
	}
	err = s.emitUsageEvents(map[string]*usageeventsv1.ResourceCreateEvent{
		appEventPrefix + app.GetName(): {
			ResourceType:   types.DiscoveredResourceApp,
			ResourceOrigin: types.OriginKubernetes,
			// CloudProvider is not set for apps created from Kubernetes services
		},
	})
	if err != nil {
		s.Log.DebugContext(ctx, "Error emitting usage event", "error", err)
	}
	return nil
}

func (s *Server) onAppUpdate(ctx context.Context, app, _ types.Application) error {
	s.Log.DebugContext(ctx, "Updating app", "app_name", app.GetName())
	return trace.Wrap(s.AccessPoint.UpdateApp(ctx, app))
}

func (s *Server) onAppDelete(ctx context.Context, app types.Application) error {
	s.Log.DebugContext(ctx, "Deleting app", "app_name", app.GetName())
	return trace.Wrap(s.AccessPoint.DeleteApp(ctx, app.GetName()))
}
