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
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// collection is responsible for managing collection
// of resources updates
type collection interface {
	// fetch fetches resources
	fetch() error
	// process processes event
	processEvent(e services.Event) error
	// watchKind returns a watch
	// requried for this collection
	watchKind() services.WatchKind
	// erase erases all data in the collection
	erase() error
}

// setupCollections returns a mapping of collections
func setupCollections(c *Cache, watches []services.WatchKind) (map[string]collection, error) {
	collections := make(map[string]collection, len(watches))
	for _, watch := range watches {
		switch watch.Kind {
		case services.KindCertAuthority:
			if c.Trust == nil {
				return nil, trace.BadParameter("missing parameter Trust")
			}
			collections[watch.Kind] = &certAuthority{watch: watch, Cache: c}
		case services.KindStaticTokens:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[watch.Kind] = &staticTokens{watch: watch, Cache: c}
		case services.KindToken:
			if c.Provisioner == nil {
				return nil, trace.BadParameter("missing parameter Provisioner")
			}
			collections[watch.Kind] = &provisionToken{watch: watch, Cache: c}
		case services.KindClusterName:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[watch.Kind] = &clusterName{watch: watch, Cache: c}
		case services.KindClusterConfig:
			if c.ClusterConfig == nil {
				return nil, trace.BadParameter("missing parameter ClusterConfig")
			}
			collections[watch.Kind] = &clusterConfig{watch: watch, Cache: c}
		case services.KindUser:
			if c.Users == nil {
				return nil, trace.BadParameter("missing parameter Users")
			}
			collections[watch.Kind] = &user{watch: watch, Cache: c}
		case services.KindRole:
			if c.Access == nil {
				return nil, trace.BadParameter("missing parameter Access")
			}
			collections[watch.Kind] = &role{watch: watch, Cache: c}
		case services.KindNamespace:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &namespace{watch: watch, Cache: c}
		case services.KindNode:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &node{watch: watch, Cache: c}
		case services.KindProxy:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &proxy{watch: watch, Cache: c}
		case services.KindAuthServer:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &authServer{watch: watch, Cache: c}
		case services.KindReverseTunnel:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &reverseTunnel{watch: watch, Cache: c}
		case services.KindTunnelConnection:
			if c.Presence == nil {
				return nil, trace.BadParameter("missing parameter Presence")
			}
			collections[watch.Kind] = &tunnelConnection{watch: watch, Cache: c}
		default:
			return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
		}
	}
	return collections, nil
}

type tunnelConnection struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (c *tunnelConnection) erase() error {
	if err := c.presenceCache.DeleteAllTunnelConnections(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *tunnelConnection) fetch() error {
	resources, err := c.Presence.GetAllTunnelConnections()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		if err := c.presenceCache.UpsertTunnelConnection(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *tunnelConnection) processEvent(event services.Event) error {
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

type reverseTunnel struct {
	*Cache
	watch services.WatchKind
}

// erase erases all data in the collection
func (c *reverseTunnel) erase() error {
	if err := c.presenceCache.DeleteAllReverseTunnels(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *reverseTunnel) fetch() error {
	resources, err := c.Presence.GetReverseTunnels(services.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		c.setTTL(resource)
		if err := c.presenceCache.UpsertReverseTunnel(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *reverseTunnel) processEvent(event services.Event) error {
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
func (c *proxy) erase() error {
	if err := c.presenceCache.DeleteAllProxies(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *proxy) fetch() error {
	resources, err := c.Presence.GetProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}

	for _, resource := range resources {
		c.setTTL(resource)
		if err := c.presenceCache.UpsertProxy(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *proxy) processEvent(event services.Event) error {
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
func (c *authServer) erase() error {
	if err := c.presenceCache.DeleteAllAuthServers(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *authServer) fetch() error {
	resources, err := c.Presence.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}

	for _, resource := range resources {
		c.setTTL(resource)
		if err := c.presenceCache.UpsertAuthServer(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *authServer) processEvent(event services.Event) error {
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
func (c *node) erase() error {
	if err := c.presenceCache.DeleteAllNodes(defaults.Namespace); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *node) fetch() error {
	resources, err := c.Presence.GetNodes(defaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		c.setTTL(resource)
		if _, err := c.presenceCache.UpsertNode(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *node) processEvent(event services.Event) error {
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
func (c *namespace) erase() error {
	if err := c.presenceCache.DeleteAllNamespaces(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *namespace) fetch() error {
	resources, err := c.Presence.GetNamespaces()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		c.setTTL(&resource)
		if err := c.presenceCache.UpsertNamespace(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *namespace) processEvent(event services.Event) error {
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
func (c *certAuthority) erase() error {
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
	return nil
}

func (c *certAuthority) fetch() error {
	if err := c.updateCertAuthorities(services.HostCA); err != nil {
		return trace.Wrap(err)
	}
	if err := c.updateCertAuthorities(services.UserCA); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *certAuthority) updateCertAuthorities(caType services.CertAuthType) error {
	authorities, err := c.Trust.GetCertAuthorities(caType, c.watch.LoadSecrets, services.SkipValidation())
	if err != nil {
		return trace.Wrap(err)
	}
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
}

func (c *certAuthority) processEvent(event services.Event) error {
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
func (c *staticTokens) erase() error {
	err := c.clusterConfigCache.DeleteStaticTokens()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *staticTokens) fetch() error {
	staticTokens, err := c.ClusterConfig.GetStaticTokens()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err := c.erase(); err != nil {
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
}

func (c *staticTokens) processEvent(event services.Event) error {
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
func (c *provisionToken) erase() error {
	if err := c.provisionerCache.DeleteAllTokens(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *provisionToken) fetch() error {
	tokens, err := c.Provisioner.GetTokens()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range tokens {
		c.setTTL(resource)
		if err := c.provisionerCache.UpsertToken(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *provisionToken) processEvent(event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.provisionerCache.DeleteToken(event.Resource.GetName())
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
		if err := c.provisionerCache.UpsertToken(resource); err != nil {
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
func (c *clusterConfig) erase() error {
	err := c.clusterConfigCache.DeleteClusterConfig()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterConfig) fetch() error {
	clusterConfig, err := c.ClusterConfig.GetClusterConfig()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err := c.erase(); err != nil {
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
}

func (c *clusterConfig) processEvent(event services.Event) error {
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
func (c *clusterName) erase() error {
	err := c.clusterConfigCache.DeleteClusterName()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *clusterName) fetch() error {
	clusterName, err := c.ClusterConfig.GetClusterName()
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err := c.erase(); err != nil {
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
}

func (c *clusterName) processEvent(event services.Event) error {
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
func (c *user) erase() error {
	if err := c.usersCache.DeleteAllUsers(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *user) fetch() error {
	resources, err := c.Users.GetUsers(false)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		c.setTTL(resource)
		if err := c.usersCache.UpsertUser(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *user) processEvent(event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.usersCache.DeleteUser(event.Resource.GetName())
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
func (c *role) erase() error {
	if err := c.accessCache.DeleteAllRoles(); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *role) fetch() error {
	resources, err := c.Access.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.erase(); err != nil {
		return trace.Wrap(err)
	}
	for _, resource := range resources {
		c.setTTL(resource)
		if err := c.accessCache.UpsertRole(resource); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *role) processEvent(event services.Event) error {
	switch event.Type {
	case backend.OpDelete:
		err := c.accessCache.DeleteRole(event.Resource.GetName())
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
		if err := c.accessCache.UpsertRole(resource); err != nil {
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
