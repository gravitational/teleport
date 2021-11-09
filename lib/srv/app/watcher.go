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

package app

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// startReconciler starts reconciler that registers/unregisters proxied
// apps according to the up-to-date list of application resources.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredApps.get,
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
				} else if s.c.OnReconcile != nil {
					s.c.OnReconcile(s.getApps())
				}
			case <-ctx.Done():
				s.log.Debug("Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startResourceWatcher starts watching changes to application resources and
// registers/unregisters the proxied applications accordingly.
func (s *Server) startResourceWatcher(ctx context.Context) (*services.AppWatcher, error) {
	if len(s.c.ResourceMatchers) == 0 {
		s.log.Debug("Not initializing application resource watcher.")
		return nil, nil
	}
	s.log.Debug("Initializing application resource watcher.")
	watcher, err := services.NewAppWatcher(ctx, services.AppWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentApp,
			Log:       s.log,
			Client:    s.c.AccessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case apps := <-watcher.AppsC:
				s.monitoredApps.setResources(apps)
				select {
				case s.reconcileCh <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				s.log.Debug("Application resource watcher done.")
				return
			}
		}
	}()
	return watcher, nil
}

func (s *Server) getResources() (resources types.ResourcesWithLabels) {
	return s.getApps().AsResources()
}

func (s *Server) onCreate(ctx context.Context, resource types.ResourceWithLabels) error {
	app, ok := resource.(types.Application)
	if !ok {
		return trace.BadParameter("expected types.Application, got %T", resource)
	}
	return s.registerApp(ctx, app)
}

func (s *Server) onUpdate(ctx context.Context, resource types.ResourceWithLabels) error {
	app, ok := resource.(types.Application)
	if !ok {
		return trace.BadParameter("expected types.Application, got %T", resource)
	}
	return s.updateApp(ctx, app)
}

func (s *Server) onDelete(ctx context.Context, resource types.ResourceWithLabels) error {
	return s.unregisterApp(ctx, resource.GetName())
}

func (s *Server) matcher(resource types.ResourceWithLabels) bool {
	app, ok := resource.(types.Application)
	if !ok {
		return false
	}
	return services.MatchResourceLabels(s.c.ResourceMatchers, app)
}
