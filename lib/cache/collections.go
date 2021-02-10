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
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// collection is responsible for managing collection
// of resources updates
type collection interface {
	// fetch fetches resources and returns a function which
	// will apply said resources to the cache.  fetch *must*
	// not mutate cache state outside of the apply function.
	fetch(ctx context.Context) (apply func(ctx context.Context) error, err error)
	// process processes event
	processEvent(ctx context.Context, e services.Event) error
	// watchKind returns a watch
	// required for this collection
	watchKind() services.WatchKind
	// erase erases all data in the collection
	erase(ctx context.Context) error
}

// setupCollections returns a mapping of collections
func setupCollections(c *Cache, watches []services.WatchKind) (map[resourceKind]collection, error) {
	collections := make(map[resourceKind]collection, len(watches))
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case services.KindCertAuthority:
			if c.Trust == nil {
				return nil, trace.BadParameter("missing parameter Trust")
			}
			collections[resourceKind] = &certAuthority{watch: watch, Cache: c}
		case services.KindStaticTokens:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &staticTokens{watch: watch, Cache: c}
		case services.KindToken:
			if c.Provisioner == nil {
				return nil, trace.BadParameter("missing parameter Provisioner")
			}
			collections[resourceKind] = &provisionToken{watch: watch, Cache: c}
		case services.KindClusterName:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &clusterName{watch: watch, Cache: c}
		case services.KindClusterConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &clusterConfig{watch: watch, Cache: c}
		case types.KindClusterAuthPreference:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[resourceKind] = &authPreference{watch: watch, Cache: c}
		case services.KindUser:
			if c.Users == nil {
				return nil, trace.BadParameter("missing parameter Users")
			}
			collections[resourceKind] = &user{watch: watch, Cache: c}
		case services.KindRole:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[resourceKind] = &role{watch: watch, Cache: c}
		case services.KindNamespace:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &namespace{watch: watch, Cache: c}
		case services.KindNode:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &node{watch: watch, Cache: c}
		case services.KindProxy:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &proxy{watch: watch, Cache: c}
		case services.KindAuthServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &authServer{watch: watch, Cache: c}
		case services.KindReverseTunnel:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &reverseTunnel{watch: watch, Cache: c}
		case services.KindTunnelConnection:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &tunnelConnection{watch: watch, Cache: c}
		case services.KindRemoteCluster:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &remoteCluster{watch: watch, Cache: c}
		case services.KindAccessRequest:
			if c.DynamicAccess == nil {
				return nil, trace.BadParameter("missing parameter DynamicAccess")
			}
			collections[resourceKind] = &accessRequest{watch: watch, Cache: c}
		case services.KindAppServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &appServer{watch: watch, Cache: c}
		case services.KindWebSession:
			switch watch.SubKind {
			case services.KindAppSession:
				if c.AppSession == nil {
					return nil, trace.BadParameter("missing parameter AppSession")
				}
				collections[resourceKind] = &appSession{watch: watch, Cache: c}
			case services.KindWebSession:
				if c.WebSession == nil {
					return nil, trace.BadParameter("missing parameter WebSession")
				}
				collections[resourceKind] = &webSession{watch: watch, Cache: c}
			}
		case services.KindWebToken:
			if c.WebToken == nil {
				return nil, trace.BadParameter("missing parameter WebToken")
			}
			collections[resourceKind] = &webToken{watch: watch, Cache: c}
		case services.KindKubeService:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &kubeService{watch: watch, Cache: c}
		case types.KindDatabaseServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[resourceKind] = &databaseServer{watch: watch, Cache: c}
		default:
			return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
		}
	}
	return collections, nil
}

func resourceKindFromWatchKind(wk services.WatchKind) resourceKind {
	switch wk.Kind {
	case services.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    wk.Kind,
			subkind: wk.SubKind,
		}
	}
	return resourceKind{
		kind: wk.Kind,
	}
}

func resourceKindFromResource(res services.Resource) resourceKind {
	switch res.GetKind() {
	case services.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    res.GetKind(),
			subkind: res.GetSubKind(),
		}
	}
	return resourceKind{
		kind: res.GetKind(),
	}
}

type resourceKind struct {
	kind    string
	subkind string
}

type accessRequest struct {
	*Cache
	watch services.WatchKind
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
	resources, err := r.DynamicAccess.GetAccessRequests(ctx, services.AccessRequestFilter{})
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

func (r *accessRequest) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(*services.AccessRequestV3)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		r.setTTL(resource)
		if err := r.dynamicAccessCache.UpsertAccessRequest(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (r *accessRequest) watchKind() services.WatchKind {
	return r.watch
}

type tunnelConnection struct {
	*Cache
	watch services.WatchKind
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

func (c *tunnelConnection) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.TunnelConnection)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertTunnelConnection(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *tunnelConnection) watchKind() services.WatchKind {
	return c.watch
}

type remoteCluster struct {
	*Cache
	watch services.WatchKind
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

func (c *remoteCluster) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.RemoteCluster)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
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

func (c *remoteCluster) watchKind() services.WatchKind {
	return c.watch
}

type reverseTunnel struct {
	*Cache
	watch services.WatchKind
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
	resources, err := c.Presence.GetReverseTunnels(resource.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			c.setTTL(resource)
			if err := c.presenceCache.UpsertReverseTunnel(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *reverseTunnel) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.ReverseTunnel)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertReverseTunnel(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *reverseTunnel) watchKind() services.WatchKind {
	return c.watch
}

type proxy struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.presenceCache.UpsertProxy(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *proxy) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertProxy(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *proxy) watchKind() services.WatchKind {
	return c.watch
}

type authServer struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.presenceCache.UpsertAuthServer(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *authServer) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertAuthServer(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *authServer) watchKind() services.WatchKind {
	return c.watch
}

type node struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (c *node) erase(ctx context.Context) error {
	if err := c.presenceCache.DeleteAllNodes(defaults.Namespace); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *node) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := c.Presence.GetNodes(defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			c.setTTL(resource)
			if _, err := c.presenceCache.UpsertNode(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *node) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.presenceCache.DeleteNode(event.Resource.GetMetadata().Namespace, event.Resource.GetName())
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case backend.OpPut:
		resource, ok := event.Resource.(services.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if _, err := c.presenceCache.UpsertNode(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *node) watchKind() services.WatchKind {
	return c.watch
}

type namespace struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(&resource)
			if err := c.presenceCache.UpsertNamespace(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *namespace) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(*services.Namespace)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertNamespace(*resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *namespace) watchKind() services.WatchKind {
	return c.watch
}

type certAuthority struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (c *certAuthority) erase(ctx context.Context) error {
	if err := c.trustCache.DeleteAllCertAuthorities(services.UserCA); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	if err := c.trustCache.DeleteAllCertAuthorities(services.HostCA); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	if err := c.trustCache.DeleteAllCertAuthorities(services.JWTSigner); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *certAuthority) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	applyHostCAs, err := c.fetchCertAuthorities(services.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applyUserCAs, err := c.fetchCertAuthorities(services.UserCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applyJWTSigners, err := c.fetchCertAuthorities(services.JWTSigner)
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
		return trace.Wrap(applyJWTSigners(ctx))
	}, nil
}

func (c *certAuthority) fetchCertAuthorities(caType services.CertAuthType) (apply func(ctx context.Context) error, err error) {
	authorities, err := c.Trust.GetCertAuthorities(caType, c.watch.LoadSecrets, resource.SkipValidation())
	if err != nil {
		// DELETE IN: 5.1
		//
		// All clusters will support JWT signers in 5.1.
		if strings.Contains(err.Error(), "authority type is not supported") {
			return func(ctx context.Context) error { return nil }, nil
		}
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := c.trustCache.DeleteAllCertAuthorities(caType); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		for _, resource := range authorities {
			c.setTTL(resource)
			if err := c.trustCache.UpsertCertAuthority(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *certAuthority) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.trustCache.DeleteCertAuthority(services.CertAuthID{
			Type:       services.CertAuthType(event.Resource.GetSubKind()),
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.CertAuthority)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.trustCache.UpsertCertAuthority(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *certAuthority) watchKind() services.WatchKind {
	return c.watch
}

type staticTokens struct {
	*Cache
	watch services.WatchKind
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
		c.setTTL(staticTokens)
		err = c.clusterConfigCache.SetStaticTokens(staticTokens)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *staticTokens) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.StaticTokens)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.clusterConfigCache.SetStaticTokens(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *staticTokens) watchKind() services.WatchKind {
	return c.watch
}

type provisionToken struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.provisionerCache.UpsertToken(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *provisionToken) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.ProvisionToken)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.provisionerCache.UpsertToken(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *provisionToken) watchKind() services.WatchKind {
	return c.watch
}

type clusterConfig struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (c *clusterConfig) erase(ctx context.Context) error {
	err := c.clusterConfigCache.DeleteClusterConfig()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterConfig) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	var noConfig bool
	clusterConfig, err := c.ClusterConfig.GetClusterConfig()
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
		c.setTTL(clusterConfig)
		if err := c.clusterConfigCache.SetClusterConfig(clusterConfig); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *clusterConfig) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.clusterConfigCache.DeleteClusterConfig()
		if err != nil {
			// resource could be missing in the cache
			// expired or not created, if the first consumed
			// event is delete
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete cluster config %v.", err)
				return trace.Wrap(err)
			}
		}
	case backend.OpPut:
		resource, ok := event.Resource.(services.ClusterConfig)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.clusterConfigCache.SetClusterConfig(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *clusterConfig) watchKind() services.WatchKind {
	return c.watch
}

type clusterName struct {
	*Cache
	watch services.WatchKind
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
		c.setTTL(clusterName)
		if err := c.clusterConfigCache.UpsertClusterName(clusterName); err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *clusterName) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.ClusterName)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.clusterConfigCache.UpsertClusterName(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *clusterName) watchKind() services.WatchKind {
	return c.watch
}

type user struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.usersCache.UpsertUser(resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *user) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.User)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.usersCache.UpsertUser(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *user) watchKind() services.WatchKind {
	return c.watch
}

type role struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.accessCache.UpsertRole(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *role) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.Role)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.accessCache.UpsertRole(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *role) watchKind() services.WatchKind {
	return c.watch
}

type databaseServer struct {
	*Cache
	watch services.WatchKind
}

func (s *databaseServer) erase(ctx context.Context) error {
	err := s.presenceCache.DeleteAllDatabaseServers(ctx, defaults.Namespace)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *databaseServer) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := s.Presence.GetDatabaseServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := s.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			s.setTTL(resource)
			if _, err := s.presenceCache.UpsertDatabaseServer(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (s *databaseServer) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(types.DatabaseServer)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		s.setTTL(resource)
		if _, err := s.presenceCache.UpsertDatabaseServer(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		s.Warnf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (s *databaseServer) watchKind() services.WatchKind {
	return s.watch
}

type appServer struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (a *appServer) erase(ctx context.Context) error {
	if err := a.presenceCache.DeleteAllAppServers(ctx, defaults.Namespace); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *appServer) fetch(ctx context.Context) (apply func(ctx context.Context) error, err error) {
	resources, err := a.Presence.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return func(ctx context.Context) error {
		if err := a.erase(ctx); err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range resources {
			a.setTTL(resource)
			if _, err := a.presenceCache.UpsertAppServer(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (a *appServer) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := a.presenceCache.DeleteAppServer(ctx, event.Resource.GetMetadata().Namespace, event.Resource.GetName())
		if err != nil {
			// Resource could be missing in the cache expired or not created, if the
			// first consumed event is delete.
			if !trace.IsNotFound(err) {
				a.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case backend.OpPut:
		resource, ok := event.Resource.(services.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		a.setTTL(resource)
		if _, err := a.presenceCache.UpsertAppServer(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (a *appServer) watchKind() services.WatchKind {
	return a.watch
}

type appSession struct {
	*Cache
	watch services.WatchKind
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
			a.setTTL(resource)
			if err := a.appSessionCache.UpsertAppSession(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (a *appSession) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := a.appSessionCache.DeleteAppSession(ctx, services.DeleteAppSessionRequest{
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.WebSession)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		a.setTTL(resource)
		if err := a.appSessionCache.UpsertAppSession(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (a *appSession) watchKind() services.WatchKind {
	return a.watch
}

type webSession struct {
	*Cache
	watch services.WatchKind
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
			r.setTTL(resource)
			if err := r.webSessionCache.Upsert(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (r *webSession) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(services.WebSession)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		r.setTTL(resource)
		if err := r.webSessionCache.Upsert(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

func (r *webSession) watchKind() services.WatchKind {
	return r.watch
}

type webToken struct {
	*Cache
	watch services.WatchKind
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
			r.setTTL(resource)
			if err := r.webTokenCache.Upsert(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (r *webToken) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
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
	case backend.OpPut:
		resource, ok := event.Resource.(types.WebToken)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		r.setTTL(resource)
		if err := r.webTokenCache.Upsert(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		r.WithField("event", event.Type).Warn("Skipping unsupported event type.")
	}
	return nil
}

func (r *webToken) watchKind() services.WatchKind {
	return r.watch
}

type kubeService struct {
	*Cache
	watch services.WatchKind
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
			c.setTTL(resource)
			if err := c.presenceCache.UpsertKubeService(ctx, resource); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}, nil
}

func (c *kubeService) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.presenceCache.DeleteKubeService(ctx, event.Resource.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case backend.OpPut:
		resource, ok := event.Resource.(services.Server)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.presenceCache.UpsertKubeService(ctx, resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *kubeService) watchKind() services.WatchKind {
	return c.watch
}

type authPreference struct {
	*Cache
	watch services.WatchKind
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
	resource, err := c.ClusterConfig.GetAuthPreference()
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

		c.setTTL(resource)
		if err := c.clusterConfigCache.SetAuthPreference(resource); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}, nil
}

func (c *authPreference) processEvent(ctx context.Context, event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.clusterConfigCache.DeleteAuthPreference(ctx)
		if err != nil {
			if !trace.IsNotFound(err) {
				c.Warningf("Failed to delete resource %v.", err)
				return trace.Wrap(err)
			}
		}
	case backend.OpPut:
		resource, ok := event.Resource.(services.AuthPreference)
		if !ok {
			return trace.BadParameter("unexpected type %T", event.Resource)
		}
		c.setTTL(resource)
		if err := c.clusterConfigCache.SetAuthPreference(resource); err != nil {
			return trace.Wrap(err)
		}
	default:
		c.Warningf("Skipping unsupported event type %v.", event.Type)
	}
	return nil
}

func (c *authPreference) watchKind() services.WatchKind {
	return c.watch
}
