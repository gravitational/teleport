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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/services/reconcile"
	"github.com/gravitational/teleport/lib/utils"
)

// startReconciler starts the reconciler that registers/unregisters proxied
// apps according to the desired application set: static configuration and —
// when an app cache is available — dynamic application resources read
// directly from a consistent cache snapshot on every cycle. No copy of the
// dynamic set is retained between cycles; the cache's change pulse is the
// wake-up signal.
func (s *Server) startReconciler(ctx context.Context) error {
	reconciler, err := reconcile.New(reconcile.Config[string, types.Application, readonly.Application]{
		Matcher:    s.matcher,
		GetCurrent: s.getAppUnfiltered,
		RangeCurrent: func() iter.Seq2[string, types.Application] {
			return func(yield func(string, types.Application) bool) {
				s.mu.RLock()
				defer s.mu.RUnlock()
				for name, app := range s.apps {
					if !yield(name, app) {
						return
					}
				}
			}
		},
		RangeDesired: func() iter.Seq2[readonly.Application, error] {
			return s.rangeMonitoredApps(ctx)
		},
		KeyOf: readonly.Application.GetName,
		Materialize: func(v readonly.Application) types.Application {
			return s.materializeApp(ctx, v)
		},
		CompareWithCurrent: func(current types.Application, view readonly.Application) int {
			// Registration derives the public address of dynamic resources
			// that do not set one; compare the value registration would
			// produce so a derived address does not read as perpetual drift.
			// (Static apps are registered verbatim and compared verbatim.)
			if view.GetPublicAddr() == "" && s.isDynamicApp(view.GetName()) {
				if s.materializeApp(ctx, view).IsEqual(current) {
					return reconcile.Equal
				}
				return reconcile.Different
			}
			if view.IsEqual(current) {
				return reconcile.Equal
			}
			return reconcile.Different
		},
		OnCreate: s.onCreate,
		OnUpdate: s.onUpdate,
		OnDelete: s.onDelete,
		Changes:  s.appChangesOrNil,
		Logger:   s.log.With("kind", types.KindApp),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	s.reconciler = reconciler
	go reconciler.Run(ctx)
	return nil
}

// dynamicRegistrationActive reports whether dynamic application resources are
// being reconciled.
func (s *Server) dynamicRegistrationActive() bool {
	return s.c.AppsCache != nil && len(s.c.ResourceMatchers) > 0
}

// appChangesOrNil returns the app cache's change pulse, or nil — which blocks
// forever in a select — when the dynamic-resource leg is disabled.
func (s *Server) appChangesOrNil() <-chan struct{} {
	if !s.dynamicRegistrationActive() {
		return nil
	}
	return s.c.AppsCache.AppChanges()
}

// rangeMonitoredApps yields read-only views of the desired application set in
// decreasing precedence order — dynamic (cache snapshot), static — matching
// the last-wins map merge order of the legacy reconciler under the view
// reconciler's first-occurrence-wins contract. As dynamic views are yielded
// their names are recorded in s.dynamicAppNames for the derivation checks
// performed by the compare and materialize callbacks within the same cycle.
//
// Must only be invoked from the reconciler goroutine.
func (s *Server) rangeMonitoredApps(ctx context.Context) iter.Seq2[readonly.Application, error] {
	return func(yield func(readonly.Application, error) bool) {
		clear(s.dynamicAppNames)

		if s.dynamicRegistrationActive() {
			for app, err := range s.c.AppsCache.RangeReadonlyApps(ctx, "", "") {
				if err != nil {
					yield(nil, trace.Wrap(err))
					return
				}
				s.dynamicAppNames[app.GetName()] = struct{}{}
				if !yield(app, nil) {
					return
				}
			}
		}

		for _, app := range s.c.Apps {
			if !yield(app, nil) {
				return
			}
		}
	}
}

// isDynamicApp returns whether the named application is a dynamic resource
// (an app object) as of the current reconciliation cycle. Must only be used
// from the reconciler goroutine.
func (s *Server) isDynamicApp(name string) bool {
	_, ok := s.dynamicAppNames[name]
	return ok
}

// materializeApp converts a desired view into the owned application value the
// agent registers: a copy with the public address derived when the resource
// does not set one (parity with the fill the retired watcher goroutine
// performed on its broadcast copies).
func (s *Server) materializeApp(ctx context.Context, view readonly.Application) types.Application {
	app := view.Copy()
	if app.GetPublicAddr() == "" && s.isDynamicApp(app.GetName()) {
		// TODO (williamo/scopes): Dynamic app registration does not support
		// scoped apps; add scoped app here when we do support it.
		if addr, err := FindPublicAddr(ctx, s.c.AccessPoint, app.GetPublicAddr(), app.GetName(), ""); err == nil {
			app.SetPublicAddr(addr)
		} else {
			s.log.ErrorContext(ctx, "Unable to find public address for app, leaving empty",
				"app_name", app.GetName(),
				"error", err,
			)
		}
	}
	return app
}

// FindPublicAddrClient is a client used for finding public addresses.
type FindPublicAddrClient interface {
	// GetProxies returns a list of proxy servers registered in the cluster
	//
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)

	// ListProxyServers returns a paginated list of registered proxy servers.
	ListProxyServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)

	// GetClusterName gets the name of the cluster from the backend.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// FindPublicAddr tries to resolve the public address of the proxy of this cluster.
//
// For a scoped app, the address is always derived as the
// scope-qualified FQDN "<hash(name,scope)>.<proxy>"
// TODO(williamo/scopes): We added a scopeVar as a variadic parameter to not break the e submodule.
// This will be amended in a future PR.
func FindPublicAddr(ctx context.Context, client FindPublicAddrClient, appPublicAddr, appName string, scopeVar ...string) (string, error) {
	// If the application has a public address already set, use it. Scoped apps
	// always derive their address, so the config value is not honored.
	scope := ""
	switch len(scopeVar) {
	case 1:
		scope = scopeVar[0]
	case 0:
	default:
		return "", trace.BadParameter("multiple scopes not allowed")
	}
	if appPublicAddr != "" && scope == "" {
		return appPublicAddr, nil
	}

	// Fetch list of proxies, if first has public address set, use it.
	servers, err := clientutils.CollectWithFallback(ctx, client.ListProxyServers, func(context.Context) ([]types.Server, error) {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		return client.GetProxies()
	})
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
		if scope != "" {
			return scopedapp.ScopedAppPublicAddr(scope, appName, addr.Host()), nil
		}
		return utils.DefaultAppFQDN(appName, addr.Host(), ""), nil
	}

	// Fall back to cluster name.
	cn, err := client.GetClusterName(context.TODO())
	if err != nil {
		return "", trace.Wrap(err)
	}

	if scope != "" {
		return scopedapp.ScopedAppPublicAddr(scope, appName, cn.GetClusterName()), nil
	}
	return utils.DefaultAppFQDN(appName, "", cn.GetClusterName()), nil
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

// getAppUnfiltered returns the registered application value by name. It is
// the reconciler's view of current state.
func (s *Server) getAppUnfiltered(name string) (types.Application, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, found := s.apps[name]
	return app, found
}

// matcher is used by the reconciler to check if an application matches
// selectors. It observes a shared read-only view and must not retain or
// mutate it.
func (s *Server) matcher(app readonly.Application) bool {
	matchesLabels := services.MatchResourceLabels(s.c.ResourceMatchers, app.GetAllLabels())
	if !matchesLabels {
		return false
	}

	if s.c.IgnoreAppsWithCommandLabels {
		if len(app.GetDynamicLabels()) > 0 {
			s.log.WarnContext(
				context.Background(),
				"refusing to register app with dynamic labels",
				"app_name", app.GetName(),
			)
			return false
		}
	}

	return true
}
