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

package app

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// startReconciler starts reconciler that registers/unregisters proxied
// apps according to the up-to-date list of application resources.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := services.NewReconciler(services.ReconcilerConfig[types.Application]{
		Matcher:             s.matcher,
		GetCurrentResources: s.getResources,
		GetNewResources:     s.monitoredApps.get,
		OnCreate:            s.onCreate,
		OnUpdate:            s.onUpdate,
		OnDelete:            s.onDelete,
		Logger:              s.log.With("kind", types.KindApp),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		for {
			select {
			case <-s.reconcileCh:
				if err := reconciler.Reconcile(ctx); err != nil {
					s.log.ErrorContext(ctx, "Failed to reconcile.", "error", err)
				} else if s.c.OnReconcile != nil {
					s.c.OnReconcile(s.getApps())
				}
			case <-ctx.Done():
				s.log.DebugContext(ctx, "Reconciler done.")
				return
			}
		}
	}()
	return nil
}

// startResourceWatcher starts watching changes to application resources and
// registers/unregisters the proxied applications accordingly.
func (s *Server) startResourceWatcher(ctx context.Context) (*services.GenericWatcher[types.Application, readonly.Application], error) {
	if len(s.c.ResourceMatchers) == 0 {
		s.log.DebugContext(ctx, "Not initializing application resource watcher.")
		return nil, nil
	}
	s.log.DebugContext(ctx, "Initializing application resource watcher.")
	watcher, err := services.NewAppWatcher(ctx, services.AppWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentApp,
			Logger:    s.log,
			Client:    s.c.AccessPoint,
		},
		AppGetter: s.c.AccessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case apps := <-watcher.ResourcesC:
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
				s.log.DebugContext(ctx, "Application resource watcher done.")
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
		s.log.ErrorContext(s.closeContext, "Unable to find public address for app, leaving empty",
			"app_name", app.GetName(),
			"error", err,
		)
	}
	return appCopy
}

// FindPublicAddrClient is a client used for finding public addresses.
type FindPublicAddrClient interface {
	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
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
		return utils.DefaultAppPublicAddr(appName, addr.Host()), nil
	}

	// Fall back to cluster name.
	cn, err := client.GetClusterName(context.TODO())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%v.%v", appName, cn.GetClusterName()), nil
}

func (s *Server) getResources() map[string]types.Application {
	return utils.FromSlice(s.getApps(), types.Application.GetName)
}

func (s *Server) onCreate(ctx context.Context, app types.Application) error {
	return s.registerApp(ctx, app)
}

func (s *Server) onUpdate(ctx context.Context, app, _ types.Application) error {
	return s.updateApp(ctx, app)
}

func (s *Server) onDelete(ctx context.Context, app types.Application) error {
	return s.unregisterAndRemoveApp(ctx, app.GetName())
}

func (s *Server) matcher(app types.Application) bool {
	return services.MatchResourceLabels(s.c.ResourceMatchers, app.GetAllLabels())
}
