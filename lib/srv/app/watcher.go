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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// startResourceMonitor starts watching changes to application resources and
// registers/unregisters the proxied applications accordingly.
func (s *Server) startResourceMonitor(ctx context.Context) (*services.ResourceMonitor[types.Application], error) {
	if len(s.c.ResourceMatchers) == 0 {
		s.log.DebugContext(ctx, "Not initializing application resource watcher.")
		return nil, nil
	}

	s.log.DebugContext(ctx, "Initializing application resource watcher.")

	monitor, err := services.NewResourceMonitor(services.ResourceMonitorConfig[types.Application]{
		Kind: types.KindApp,
		Key:  types.Application.GetName,
		ResourceHeaderKey: func(rh *types.ResourceHeader) string {
			return rh.GetMetadata().Name
		},
		CurrentResources: func(ctx context.Context) iter.Seq2[types.Application, error] {
			return stream.Chain(stream.Slice(s.c.Apps), clientutils.Resources(ctx, s.c.AccessPoint.ListApps))
		},
		Events:  s.c.AccessPoint,
		Matches: s.matcher,
		CompareResources: func(a1, a2 types.Application) int {
			if a1.IsEqual(a2) {
				return services.Equal
			}
			return services.Different
		},
		DeleteResource: s.onDelete,
		CreateResource: s.onCreate,
		UpdateResource: s.onUpdate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		monitor.Run(ctx)
		s.log.DebugContext(ctx, "app server resource monitor completed")
	}()

	return monitor, nil
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
func FindPublicAddr(ctx context.Context, client FindPublicAddrClient, appPublicAddr string, appName string) (string, error) {
	// If the application has a public address already set, use it.
	if appPublicAddr != "" {
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
		return utils.DefaultAppPublicAddr(appName, addr.Host()), nil
	}

	// Fall back to cluster name.
	cn, err := client.GetClusterName(context.TODO())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return fmt.Sprintf("%v.%v", appName, cn.GetClusterName()), nil
}

func (s *Server) setAppPublicAddr(ctx context.Context, app types.Application) {
	if app.GetPublicAddr() != "" {
		return
	}

	pubAddr, err := FindPublicAddr(ctx, s.c.AccessPoint, app.GetPublicAddr(), app.GetName())
	if err != nil {
		s.log.ErrorContext(s.closeContext, "Unable to find public address for app, leaving empty",
			"app_name", app.GetName(),
			"error", err,
		)
		return
	}

	app.SetPublicAddr(pubAddr)
}

func (s *Server) onCreate(ctx context.Context, app types.Application) error {
	s.setAppPublicAddr(ctx, app)
	return s.registerApp(ctx, app)
}

func (s *Server) onUpdate(ctx context.Context, app, _ types.Application) error {
	s.setAppPublicAddr(ctx, app)
	return s.updateApp(ctx, app)
}

func (s *Server) onDelete(ctx context.Context, app types.Application) error {
	return s.unregisterAndRemoveApp(ctx, app.GetName())
}

func (s *Server) matcher(app types.Application) bool {
	return services.MatchResourceLabels(s.c.ResourceMatchers, app.GetAllLabels())
}
