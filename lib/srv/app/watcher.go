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
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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
				appsWithAddr := make(types.Apps, 0, len(apps))
				for _, app := range apps {
					appsWithAddr = append(appsWithAddr, s.guessPublicAddr(app))
				}
				s.monitoredApps.setResources(appsWithAddr)
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

// guessPublicAddr will guess PublicAddr for given application if it is missing, based on proxy information and app name.
func (s *Server) guessPublicAddr(app types.Application) types.Application {
	if app.GetPublicAddr() != "" {
		return app
	}
	appCopy := app.Copy()
	pubAddr, err := FindPublicAddr(s.c.AccessPoint, app.GetPublicAddr(), app.GetName())
	if err == nil {
		appCopy.Spec.PublicAddr = pubAddr
	} else {
		s.log.WithError(err).Errorf("Unable to find public address for app %q, leaving empty.", app.GetName())
	}
	return appCopy
}

// FindPublicAddrClient is a client used for finding public addresses.
type FindPublicAddrClient interface {
	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// FindPublicAddr tries to resolve the public address of the proxy of this cluster.
func FindPublicAddr(client FindPublicAddrClient, appPublicAddr string, appName string) (string, error) {
	// If the application has a public address already set, use it.
	if appPublicAddr != "" {
		return appPublicAddr, nil
	}

	// Fetch list of proxies, if first has public address set, use it.
	servers, err := client.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(servers) == 0 {
		return "", trace.BadParameter("cluster has no proxy registered, at least one proxy must be registered for application access")
	}
	if servers[0].GetPublicAddr() != "" {
		addr, err := utils.ParseAddr(servers[0].GetPublicAddr())
		if err != nil {
			return "", trace.Wrap(err)
		}
		return fmt.Sprintf("%v.%v", appName, addr.Host()), nil
	}

	// Fall back to cluster name.
	cn, err := client.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%v.%v", appName, cn.GetClusterName()), nil
}

func (s *Server) getResources() (resources types.ResourcesWithLabelsMap) {
	return s.getApps().AsResources().ToMap()
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
	return s.unregisterAndRemoveApp(ctx, resource.GetName())
}

func (s *Server) matcher(resource types.ResourceWithLabels) bool {
	app, ok := resource.(types.Application)
	if !ok {
		return false
	}
	return services.MatchResourceLabels(s.c.ResourceMatchers, app)
}
