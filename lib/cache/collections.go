/*
Copyright 2018-2019 Gravitational, Inc.

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

package cache

import (
	"context"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
)

// collection is responsible for managing collection
// of resources updates
type collection interface {
	// fetch fetches resources and returns a function which
	// will apply said resources to the cache.  fetch *must*
	// not mutate cache state outside of the apply function.
	fetch(ctx context.Context) (apply func(ctx context.Context) error, err error)
	// processEvent processes event
	processEvent(ctx context.Context, e types.Event) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// setupCollections returns a mapping of collections
func setupCollections(c *Cache, watches []types.WatchKind) (map[resourceKind]collection, error) {
	collections := make(map[resourceKind]collection, len(watches))
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindCertAuthority:
			if c.Trust == nil {
				return nil, trace.BadParameter("missing parameter Trust")
			}
			var filter types.CertAuthorityFilter
			filter.FromMap(watch.Filter)
			collections[resourceKind] = &certAuthority{Cache: c, watch: watch, filter: filter}
		case types.KindStaticTokens:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &staticTokens{watch: watch, Cache: c}
		case types.KindToken:
			if c.Provisioner == nil {
				return nil, trace.BadParameter("missing parameter Provisioner")
			}
			collections[resourceKind] = &provisionToken{watch: watch, Cache: c}
		case types.KindClusterName:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &clusterName{watch: watch, Cache: c}
		case types.KindClusterAuditConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &clusterAuditConfig{watch: watch, Cache: c}
		case types.KindClusterNetworkingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &clusterNetworkingConfig{watch: watch, Cache: c}
		case types.KindClusterAuthPreference:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &authPreference{watch: watch, Cache: c}
		case types.KindSessionRecordingConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &sessionRecordingConfig{watch: watch, Cache: c}
		case types.KindInstaller:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &installerConfig{watch: watch, Cache: c}
		case types.KindUser:
			if c.Users == nil {
				return nil, trace.BadParameter("missing parameter Users")
			}
			collections[resourceKind] = &user{watch: watch, Cache: c}
		case types.KindRole:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[resourceKind] = &role{watch: watch, Cache: c}
		case types.KindNamespace:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &namespace{watch: watch, Cache: c}
		case types.KindNode:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &node{watch: watch, Cache: c}
		case types.KindProxy:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &proxy{watch: watch, Cache: c}
		case types.KindAuthServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &authServer{watch: watch, Cache: c}
		case types.KindReverseTunnel:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &reverseTunnel{watch: watch, Cache: c}
		case types.KindTunnelConnection:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &tunnelConnection{watch: watch, Cache: c}
		case types.KindRemoteCluster:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &remoteCluster{watch: watch, Cache: c}
		case types.KindAccessRequest:
			if c.DynamicAccess == nil {
				return nil, trace.BadParameter("missing parameter DynamicAccess")
			}
			collections[resourceKind] = &accessRequest{watch: watch, Cache: c}
		case types.KindAppServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			switch resourceKind.version {
			case types.V2:
				collections[resourceKind] = &appServerV2{watch: watch, Cache: c}
			default:
				collections[resourceKind] = &appServerV3{watch: watch, Cache: c}
			}
		case types.KindWebSession:
			switch watch.SubKind {
			case types.KindAppSession:
				if c.AppSession == nil {
					return nil, trace.BadParameter("missing parameter AppSession")
				}
				collections[resourceKind] = &appSession{watch: watch, Cache: c}
			case types.KindSnowflakeSession:
				if c.SnowflakeSession == nil {
					return nil, trace.BadParameter("missing parameter SnowflakeSession")
				}
				collections[resourceKind] = &snowflakeSession{watch: watch, Cache: c}
			case types.KindWebSession:
				if c.WebSession == nil {
					return nil, trace.BadParameter("missing parameter WebSession")
				}
				collections[resourceKind] = &webSession{watch: watch, Cache: c}
			}
		case types.KindWebToken:
			if c.WebToken == nil {
				return nil, trace.BadParameter("missing parameter WebToken")
			}
			collections[resourceKind] = &webToken{watch: watch, Cache: c}
		case types.KindKubeService:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &kubeService{watch: watch, Cache: c}
		case types.KindDatabaseServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &databaseServer{watch: watch, Cache: c}
		case types.KindApp:
			if c.Apps == nil {
				return nil, trace.BadParameter("missing parameter Apps")
			}
			collections[resourceKind] = &app{watch: watch, Cache: c}
		case types.KindDatabase:
			if c.Databases == nil {
				return nil, trace.BadParameter("missing parameter Databases")
			}
			collections[resourceKind] = &database{watch: watch, Cache: c}
		case types.KindNetworkRestrictions:
			if c.Restrictions == nil {
				return nil, trace.BadParameter("missing parameter Restrictions")
			}
			collections[resourceKind] = &networkRestrictions{watch: watch, Cache: c}
		case types.KindLock:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[resourceKind] = &lock{watch: watch, Cache: c}
		case types.KindWindowsDesktopService:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &windowsDesktopServices{watch: watch, Cache: c}
		case types.KindWindowsDesktop:
			if c.WindowsDesktops == nil {
				return nil, trace.BadParameter("missing parameter WindowsDesktops")
			}
			collections[resourceKind] = &windowsDesktops{watch: watch, Cache: c}
		default:
			return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
		}
	}
	return collections, nil
}

func resourceKindFromWatchKind(wk types.WatchKind) resourceKind {
	switch wk.Kind {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    wk.Kind,
			subkind: wk.SubKind,
			version: wk.Version,
		}
	}
	return resourceKind{
		kind:    wk.Kind,
		version: wk.Version,
	}
}

func resourceKindFromResource(res types.Resource) resourceKind {
	switch res.GetKind() {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    res.GetKind(),
			subkind: res.GetSubKind(),
		}
	case types.KindAppServer:
		// DELETE IN 9.0.
		switch res.GetVersion() {
		case types.V2:
			return resourceKind{
				kind:    res.GetKind(),
				version: res.GetVersion(),
			}
		}
	}
	return resourceKind{
		kind: res.GetKind(),
	}
}

type resourceKind struct {
	kind    string
	subkind string
	version string
}

type accessRequest struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (r *accessRequest) erase(ctx context.Context) error {
	if err := r.dynamicAccessCache.DeleteAllAccessRequests(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *accessRequest) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := r.DynamicAccess.GetAccessRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := r.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := r.dynamicAccessCache.UpsertAccessRequest(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (r *accessRequest) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := r.dynamicAccessCache.DeleteAccessRequest(ctx, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				r.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(*types.AccessRequestV3)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := r.dynamicAccessCache.UpsertAccessRequest(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (r *accessRequest) watchKind() types.WatchKind {
	return r.watch
}

type tunnelConnection struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *tunnelConnection) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllTunnelConnections(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *tunnelConnection) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetAllTunnelConnections()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.presenceCache.UpsertTunnelConnection(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *tunnelConnection) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteTunnelConnection(event.Resource.GetSubKind(), event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.TunnelConnection)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.presenceCache.UpsertTunnelConnection(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *tunnelConnection) watchKind() types.WatchKind {
	return c.watch
}

type remoteCluster struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *remoteCluster) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllRemoteClusters(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *remoteCluster) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetRemoteClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.presenceCache.CreateRemoteCluster(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *remoteCluster) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteRemoteCluster(event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.WithError(err).Warningf("Failed to delete remote cluster %v.", event.Resource.GetName())
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.RemoteCluster)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		err := c.presenceCache.DeleteRemoteCluster(event.Resource.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				c.WithError(err).Warningf("Failed to delete remote cluster %v.", event.Resource.GetName())
				return trace.Wrap(err)
			}
		}
		if err := c.presenceCache.CreateRemoteCluster(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *remoteCluster) watchKind() types.WatchKind {
	return c.watch
}

type reverseTunnel struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *reverseTunnel) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllReverseTunnels(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *reverseTunnel) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetReverseTunnels(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.presenceCache.UpsertReverseTunnel(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *reverseTunnel) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteReverseTunnel(event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.ReverseTunnel)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.presenceCache.UpsertReverseTunnel(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *reverseTunnel) watchKind() types.WatchKind {
	return c.watch
}

type proxy struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *proxy) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllProxies(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *proxy) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetProxies()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			if err := c.presenceCache.UpsertProxy(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *proxy) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteProxy(event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.presenceCache.UpsertProxy(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *proxy) watchKind() types.WatchKind {
	return c.watch
}

type authServer struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *authServer) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllAuthServers(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *authServer) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			if err := c.presenceCache.UpsertAuthServer(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *authServer) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteAuthServer(event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.presenceCache.UpsertAuthServer(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *authServer) watchKind() types.WatchKind {
	return c.watch
}

type node struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *node) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllNodes(ctx, apidefaults.Namespace); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *node) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if _, err := c.presenceCache.UpsertNode(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *node) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteNode(ctx, event.Resource.GetMetadata().Namespace, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := c.presenceCache.UpsertNode(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *node) watchKind() types.WatchKind {
	return c.watch
}

type namespace struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *namespace) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllNamespaces(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *namespace) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetNamespaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.presenceCache.UpsertNamespace(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *namespace) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteNamespace(event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete namespace %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(*types.Namespace)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.presenceCache.UpsertNamespace(*resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *namespace) watchKind() types.WatchKind {
	return c.watch
}

type certAuthority struct {
	*Cache
	watch types.WatchKind
	// filter extracted from watch.Filter, to avoid rebuilding it on every event
	filter types.CertAuthorityFilter
}

func (c *certAuthority) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	applyHostCAs, err := c.fetchCertAuthorities(ctx, types.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applyUserCAs, err := c.fetchCertAuthorities(ctx, types.UserCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DELETE IN 11.0.
	// missingDatabaseCA is needed only when leaf cluster v9 is connected
	// to root cluster v10. Database CA has been added in v10, so older
	// clusters don't have it and fetchCertAuthorities() returns an error.
	missingDatabaseCA := false
	applyDatabaseCAs, err := c.fetchCertAuthorities(ctx, types.DatabaseCA)
	if trace.IsBadParameter(err) {
		missingDatabaseCA = true
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	applyJWTSigners, err := c.fetchCertAuthorities(ctx, types.JWTSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(ctx context.Context) error {
		if err := applyHostCAs(ctx); err != nil {
			return trace.Wrap(err)
		}
		if err := applyUserCAs(ctx); err != nil {
			return trace.Wrap(err)
		}
		if !missingDatabaseCA {
			if err := applyDatabaseCAs(ctx); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := c.trustCache.DeleteAllCertAuthorities(types.DatabaseCA); err != nil {
				if !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
			}
		}
		return trace.Wrap(applyJWTSigners(ctx))
	}, nil
}

func (c *certAuthority) fetchCertAuthorities(ctx context.Context, caType types.CertAuthType) (apply func(ctx context.Context) error, err error) {
	authorities, err := c.Trust.GetCertAuthorities(ctx, caType, c.watch.LoadSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this can be removed once we get the ability to fetch CAs with a filter,
	// but it should be harmless, and it could be kept as additional safety
	if !c.filter.IsEmpty() {
		filteredCAs := make([]types.CertAuthority, 0, len(authorities))
		for _, ca := range authorities {
			if c.filter.Match(ca) {
				filteredCAs = append(filteredCAs, ca)
			}
		}
		authorities = filteredCAs
	}

	return func(ctx context.Context) error {
		if err := c.trustCache.DeleteAllCertAuthorities(caType); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		for _, resource := range authorities {
			if err := c.trustCache.UpsertCertAuthority(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *certAuthority) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.trustCache.DeleteCertAuthority(types.CertAuthID{
			Type:       types.CertAuthType(event.Resource.GetSubKind()),
			DomainName: event.Resource.GetName(),
		})
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete cert authority %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.CertAuthority)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if !c.filter.Match(resource) {
			return nil
		}
		if err := c.trustCache.UpsertCertAuthority(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *certAuthority) watchKind() types.WatchKind {
	return c.watch
}

type staticTokens struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *staticTokens) erase(ctx context.Context) error {
	err := c.clusterConfigCache.DeleteStaticTokens()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *staticTokens) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noTokens bool
	staticTokens, err := c.ClusterConfig.GetStaticTokens()
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noTokens = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noTokens {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
		err = c.clusterConfigCache.SetStaticTokens(staticTokens)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *staticTokens) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteStaticTokens()
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete static tokens %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.StaticTokens)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetStaticTokens(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *staticTokens) watchKind() types.WatchKind {
	return c.watch
}

type provisionToken struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *provisionToken) erase(ctx context.Context) error {
	if err := c.provisionerCache.DeleteAllTokens(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *provisionToken) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	tokens, err := c.Provisioner.GetTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range tokens {
			if err := c.provisionerCache.UpsertToken(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *provisionToken) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.provisionerCache.DeleteToken(ctx, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete provisioning token %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.ProvisionToken)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.provisionerCache.UpsertToken(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *provisionToken) watchKind() types.WatchKind {
	return c.watch
}

type clusterName struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *clusterName) erase(ctx context.Context) error {
	err := c.clusterConfigCache.DeleteClusterName()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterName) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noName bool
	clusterName, err := c.ClusterConfig.GetClusterName()
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noName = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noName {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
		if err := c.clusterConfigCache.UpsertClusterName(clusterName); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *clusterName) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteClusterName()
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete cluster name %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.ClusterName)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.UpsertClusterName(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *clusterName) watchKind() types.WatchKind {
	return c.watch
}

type user struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *user) erase(ctx context.Context) error {
	if err := c.usersCache.DeleteAllUsers(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *user) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Users.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.usersCache.UpsertUser(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *user) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.usersCache.DeleteUser(ctx, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete user %v.", err)
				return trace.Wrap(err)
			}
			return nil
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.User)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.usersCache.UpsertUser(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *user) watchKind() types.WatchKind {
	return c.watch
}

type role struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (c *role) erase(ctx context.Context) error {
	if err := c.accessCache.DeleteAllRoles(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *role) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Access.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.accessCache.UpsertRole(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *role) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.accessCache.DeleteRole(ctx, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete role %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Role)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.accessCache.UpsertRole(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *role) watchKind() types.WatchKind {
	return c.watch
}

type databaseServer struct {
	*Cache
	watch types.WatchKind
}

func (s *databaseServer) erase(ctx context.Context) error {
	err := s.presenceCache.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *databaseServer) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := s.Presence.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := s.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if _, err := s.presenceCache.UpsertDatabaseServer(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (s *databaseServer) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := s.presenceCache.DeleteDatabaseServer(ctx,
			event.Resource.GetMetadata().Namespace,
			event.Resource.GetMetadata().Description, // Cache passes host ID via description field.
			event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created,
			// if the first consumed event is delete.
			if !trace.IsNotFound(err) {
				s.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.DatabaseServer)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := s.presenceCache.UpsertDatabaseServer(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		s.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (s *databaseServer) watchKind() types.WatchKind {
	return s.watch
}

type database struct {
	*Cache
	watch types.WatchKind
}

func (s *database) erase(ctx context.Context) error {
	err := s.databasesCache.DeleteAllDatabases(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *database) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := s.Databases.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := s.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := s.databasesCache.CreateDatabase(ctx, resource); err != nil {
				if !trace.IsAlreadyExists(err) {
					return trace.Wrap(err)
				}
				if err := s.databasesCache.UpdateDatabase(ctx, resource); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		return nil
	}, nil
}

func (s *database) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := s.databasesCache.DeleteDatabase(ctx, event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created,
			// if the first consumed event is delete.
			if !trace.IsNotFound(err) {
				s.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Database)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := s.databasesCache.CreateDatabase(ctx, resource); err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			if err := s.databasesCache.UpdateDatabase(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
	default:
		s.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (s *database) watchKind() types.WatchKind {
	return s.watch
}

type app struct {
	*Cache
	watch types.WatchKind
}

func (s *app) erase(ctx context.Context) error {
	err := s.appsCache.DeleteAllApps(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *app) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := s.Apps.GetApps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := s.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := s.appsCache.CreateApp(ctx, resource); err != nil {
				if !trace.IsAlreadyExists(err) {
					return trace.Wrap(err)
				}
				if err := s.appsCache.UpdateApp(ctx, resource); err != nil {
					return trace.Wrap(err)
				}
			}
		}
		return nil
	}, nil
}

func (s *app) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := s.appsCache.DeleteApp(ctx, event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created,
			// if the first consumed event is delete.
			if !trace.IsNotFound(err) {
				s.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Application)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := s.appsCache.CreateApp(ctx, resource); err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			if err := s.appsCache.UpdateApp(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
	default:
		s.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (s *app) watchKind() types.WatchKind {
	return s.watch
}

type appServerV3 struct {
	*Cache
	watch types.WatchKind
}

func (s *appServerV3) erase(ctx context.Context) error {
	err := s.presenceCache.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *appServerV3) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := s.Presence.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := s.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if _, err := s.presenceCache.UpsertApplicationServer(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (s *appServerV3) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := s.presenceCache.DeleteApplicationServer(ctx,
			event.Resource.GetMetadata().Namespace,
			event.Resource.GetMetadata().Description, // Cache passes host ID via description field.
			event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created,
			// if the first consumed event is delete.
			if !trace.IsNotFound(err) {
				s.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.AppServer)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := s.presenceCache.UpsertApplicationServer(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		s.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (s *appServerV3) watchKind() types.WatchKind {
	return s.watch
}

// DELETE IN 9.0. Deprecated, use appServerV3.
type appServerV2 struct {
	*Cache
	watch types.WatchKind
}

// erase erases all data in the collection
func (a *appServerV2) erase(ctx context.Context) error {
	if err := a.presenceCache.DeleteAllAppServers(ctx, apidefaults.Namespace); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *appServerV2) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := a.Presence.GetAppServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := a.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if _, err := a.presenceCache.UpsertAppServer(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (a *appServerV2) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := a.presenceCache.DeleteAppServer(ctx, event.Resource.GetMetadata().Namespace, event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is delete.
			if !trace.IsNotFound(err) {
				a.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := a.presenceCache.UpsertAppServer(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (a *appServerV2) watchKind() types.WatchKind {
	return a.watch
}

type appSession struct {
	*Cache
	watch types.WatchKind
}

func (a *appSession) erase(ctx context.Context) error {
	if err := a.appSessionCache.DeleteAllAppSessions(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *appSession) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := a.AppSession.GetAppSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := a.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := a.appSessionCache.UpsertAppSession(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (a *appSession) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := a.appSessionCache.DeleteAppSession(ctx, types.DeleteAppSessionRequest{
			SessionID: event.Resource.GetName(),
		})
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is delete.
			if !trace.IsNotFound(err) {
				a.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WebSession)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := a.appSessionCache.UpsertAppSession(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (a *appSession) watchKind() types.WatchKind {
	return a.watch
}

type snowflakeSession struct {
	*Cache
	watch types.WatchKind
}

func (a *snowflakeSession) erase(ctx context.Context) error {
	if err := a.snowflakeSessionCache.DeleteAllSnowflakeSessions(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *snowflakeSession) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := a.SnowflakeSession.GetSnowflakeSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := a.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := a.snowflakeSessionCache.UpsertSnowflakeSession(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (a *snowflakeSession) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := a.snowflakeSessionCache.DeleteSnowflakeSession(ctx, types.DeleteSnowflakeSessionRequest{
			SessionID: event.Resource.GetName(),
		})
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is deleted.
			if !trace.IsNotFound(err) {
				a.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WebSession)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := a.snowflakeSessionCache.UpsertSnowflakeSession(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (a *snowflakeSession) watchKind() types.WatchKind {
	return a.watch
}

type webSession struct {
	*Cache
	watch types.WatchKind
}

func (r *webSession) erase(ctx context.Context) error {
	err := r.webSessionCache.DeleteAll(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (r *webSession) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := r.WebSession.List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := r.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := r.webSessionCache.Upsert(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (r *webSession) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := r.webSessionCache.Delete(ctx, types.DeleteWebSessionRequest{
			SessionID: event.Resource.GetName(),
		})
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is delete.
			if !trace.IsNotFound(err) {
				r.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WebSession)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := r.webSessionCache.Upsert(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

func (r *webSession) watchKind() types.WatchKind {
	return r.watch
}

type webToken struct {
	*Cache
	watch types.WatchKind
}

func (r *webToken) erase(ctx context.Context) error {
	err := r.webTokenCache.DeleteAll(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (r *webToken) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := r.WebToken.List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := r.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := r.webTokenCache.Upsert(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (r *webToken) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := r.webTokenCache.Delete(ctx, types.DeleteWebTokenRequest{
			Token: event.Resource.GetName(),
		})
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is delete.
			if !trace.IsNotFound(err) {
				r.WithError(err).Warn("Failed to delete resource.")
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WebToken)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := r.webTokenCache.Upsert(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

func (r *webToken) watchKind() types.WatchKind {
	return r.watch
}

type kubeService struct {
	*Cache
	watch types.WatchKind
}

func (c *kubeService) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllKubeServices(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *kubeService) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetKubeServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			if _, err := c.presenceCache.UpsertKubeServiceV2(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *kubeService) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteKubeService(ctx, event.Resource.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := c.presenceCache.UpsertKubeServiceV2(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *kubeService) watchKind() types.WatchKind {
	return c.watch
}

type authPreference struct {
	*Cache
	watch types.WatchKind
}

func (c *authPreference) erase(ctx context.Context) error {
	if err := c.clusterConfigCache.DeleteAuthPreference(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *authPreference) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	resource, err := c.ClusterConfig.GetAuthPreference(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noConfig = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noConfig {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if err := c.clusterConfigCache.SetAuthPreference(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *authPreference) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteAuthPreference(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.AuthPreference)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetAuthPreference(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *authPreference) watchKind() types.WatchKind {
	return c.watch
}

type clusterAuditConfig struct {
	*Cache
	watch types.WatchKind
}

func (c *clusterAuditConfig) erase(ctx context.Context) error {
	if err := c.clusterConfigCache.DeleteClusterAuditConfig(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterAuditConfig) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	resource, err := c.ClusterConfig.GetClusterAuditConfig(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noConfig = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noConfig {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if err := c.clusterConfigCache.SetClusterAuditConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *clusterAuditConfig) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteClusterAuditConfig(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.ClusterAuditConfig)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetClusterAuditConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *clusterAuditConfig) watchKind() types.WatchKind {
	return c.watch
}

type clusterNetworkingConfig struct {
	*Cache
	watch types.WatchKind
}

func (c *clusterNetworkingConfig) erase(ctx context.Context) error {
	if err := c.clusterConfigCache.DeleteClusterNetworkingConfig(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterNetworkingConfig) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	resource, err := c.ClusterConfig.GetClusterNetworkingConfig(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noConfig = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noConfig {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if err := c.clusterConfigCache.SetClusterNetworkingConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *clusterNetworkingConfig) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteClusterNetworkingConfig(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.ClusterNetworkingConfig)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetClusterNetworkingConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *clusterNetworkingConfig) watchKind() types.WatchKind {
	return c.watch
}

type sessionRecordingConfig struct {
	*Cache
	watch types.WatchKind
}

func (c *sessionRecordingConfig) erase(ctx context.Context) error {
	if err := c.clusterConfigCache.DeleteSessionRecordingConfig(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *sessionRecordingConfig) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	resource, err := c.ClusterConfig.GetSessionRecordingConfig(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noConfig = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noConfig {
			if err := c.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}

		if err := c.clusterConfigCache.SetSessionRecordingConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *sessionRecordingConfig) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteSessionRecordingConfig(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.SessionRecordingConfig)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetSessionRecordingConfig(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *sessionRecordingConfig) watchKind() types.WatchKind {
	return c.watch
}

type installerConfig struct {
	*Cache
	watch types.WatchKind
}

func (c *installerConfig) erase(ctx context.Context) error {
	if err := c.clusterConfigCache.DeleteInstaller(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *installerConfig) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	resource, err := c.ClusterConfig.GetInstaller(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		noConfig = true
	}
	return func(ctx context.Context) error {
		// either zero or one instance exists, so we either erase or
		// update, but not both.
		if noConfig {
			return trace.Wrap(c.erase(ctx))
		}

		return trace.Wrap(c.clusterConfigCache.SetInstaller(ctx, resource))
	}, nil
}

func (c *installerConfig) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.clusterConfigCache.DeleteInstaller(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Installer)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.clusterConfigCache.SetInstaller(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *installerConfig) watchKind() types.WatchKind {
	return c.watch
}

type networkRestrictions struct {
	*Cache
	watch types.WatchKind
}

func (r *networkRestrictions) erase(ctx context.Context) error {
	if err := r.restrictionsCache.DeleteNetworkRestrictions(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *networkRestrictions) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	nr, err := r.Restrictions.GetNetworkRestrictions(ctx)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		nr = nil
	}
	return func(ctx context.Context) error {
		if nr == nil {
			if err := r.erase(ctx); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
		return trace.Wrap(r.restrictionsCache.SetNetworkRestrictions(ctx, nr))
	}, nil
}

func (r *networkRestrictions) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		return trace.Wrap(r.restrictionsCache.DeleteNetworkRestrictions(ctx))
	case types.OpPut:
		resource, ok := event.Resource.(types.NetworkRestrictions)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		return trace.Wrap(r.restrictionsCache.SetNetworkRestrictions(ctx, resource))
	default:
		r.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (r *networkRestrictions) watchKind() types.WatchKind {
	return r.watch
}

type lock struct {
	*Cache
	watch types.WatchKind
}

func (c *lock) erase(ctx context.Context) error {
	err := c.accessCache.DeleteAllLocks(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (c *lock) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Access.GetLocks(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			if err := c.accessCache.UpsertLock(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *lock) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.accessCache.DeleteLock(ctx, event.Resource.GetName())
		if err != nil && !trace.IsNotFound(err) {
			c.Warningf("Failed to delete resource %v.", err)
			return trace.Wrap(err)
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.Lock)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.accessCache.UpsertLock(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *lock) watchKind() types.WatchKind {
	return c.watch
}

type windowsDesktopServices struct {
	*Cache
	watch types.WatchKind
}

func (c *windowsDesktopServices) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllWindowsDesktopServices(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *windowsDesktopServices) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			if _, err := c.presenceCache.UpsertWindowsDesktopService(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *windowsDesktopServices) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.presenceCache.DeleteWindowsDesktopService(ctx, event.Resource.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WindowsDesktopService)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if _, err := c.presenceCache.UpsertWindowsDesktopService(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *windowsDesktopServices) watchKind() types.WatchKind {
	return c.watch
}

type windowsDesktops struct {
	*Cache
	watch types.WatchKind
}

func (c *windowsDesktops) erase(ctx context.Context) error {
	if err := c.windowsDesktopsCache.DeleteAllWindowsDesktops(ctx); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *windowsDesktops) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.WindowsDesktops.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			if err := c.windowsDesktopsCache.UpsertWindowsDesktop(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *windowsDesktops) processEvent(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		err := c.windowsDesktopsCache.DeleteWindowsDesktop(ctx,
			event.Resource.GetMetadata().Description, // Cache passes host ID via description field.
			event.Resource.GetName(),
		)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case types.OpPut:
		resource, ok := event.Resource.(types.WindowsDesktop)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		if err := c.windowsDesktopsCache.UpsertWindowsDesktop(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *windowsDesktops) watchKind() types.WatchKind {
	return c.watch
}
