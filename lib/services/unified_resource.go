// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// UnifiedResourceKinds is a list of all kinds that are stored in the unified resource cache.
var UnifiedResourceKinds []string = []string{types.KindNode, types.KindKubeServer, types.KindDatabaseServer, types.KindAppServer, types.KindSAMLIdPServiceProvider, types.KindWindowsDesktop, types.KindAccessList}

// UnifiedResourceCacheConfig is used to configure a UnifiedResourceCache
type UnifiedResourceCacheConfig struct {
	// BTreeDegree is a degree of B-Tree, 2 for example, will create a
	// 2-3-4 tree (each node contains 1-3 items and 2-4 children).
	BTreeDegree int
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	// Component is a logging component
	Component string
	ResourceWatcherConfig
	ResourceGetter
}

// UnifiedResourceCache contains a representation of all resources that are displayable in the UI
type UnifiedResourceCache struct {
	mu  sync.Mutex
	log *log.Entry
	cfg UnifiedResourceCacheConfig
	// tree is a BTree with items
	tree            *btree.BTreeG[*item]
	initializationC chan struct{}
	stale           bool
	once            sync.Once
	cache           *utils.FnCache
	ResourceGetter
}

// NewUnifiedResourceCache creates a new memory cache that holds the unified resources
func NewUnifiedResourceCache(ctx context.Context, cfg UnifiedResourceCacheConfig) (*UnifiedResourceCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "setting defaults for unified resource cache")
	}

	lazyCache, err := utils.NewFnCache(utils.FnCacheConfig{
		Context: ctx,
		TTL:     15 * time.Second,
		Clock:   cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	m := &UnifiedResourceCache{
		log: log.WithFields(log.Fields{
			trace.Component: cfg.Component,
		}),
		cfg: cfg,
		tree: btree.NewG(cfg.BTreeDegree, func(a, b *item) bool {
			return a.Less(b)
		}),
		initializationC: make(chan struct{}),
		ResourceGetter:  cfg.ResourceGetter,
		cache:           lazyCache,
		stale:           true,
	}

	if err := newWatcher(ctx, m, cfg.ResourceWatcherConfig); err != nil {
		return nil, trace.Wrap(err, "creating unified resource watcher")
	}
	return m, nil
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UnifiedResourceCacheConfig) CheckAndSetDefaults() error {
	if cfg.BTreeDegree <= 0 {
		cfg.BTreeDegree = 8
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentUnifiedResource
	}
	return nil
}

// put stores the value into backend (creates if it does not
// exist, updates it otherwise)
func (c *UnifiedResourceCache) put(ctx context.Context, i item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tree.ReplaceOrInsert(&i)
	return nil
}

func putResources[T resource](tree *btree.BTreeG[*item], resources []T) {
	for _, resource := range resources {
		tree.ReplaceOrInsert(&item{Key: resourceKey(resource), Value: resource})
	}
}

// delete removes the item by key, returns NotFound error
// if item does not exist
func (c *UnifiedResourceCache) delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	return c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		if _, ok := tree.Delete(&item{Key: key}); !ok {
			return trace.NotFound("key %q is not found", string(key))
		}
		return nil
	})
}

func (c *UnifiedResourceCache) getRange(ctx context.Context, startKey, endKey []byte, limit int) ([]resource, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}

	var res []resource
	err := c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		tree.AscendRange(&item{Key: startKey}, &item{Key: endKey}, func(item *item) bool {
			res = append(res, item.Value)
			if limit > 0 && len(res) >= limit {
				return false
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(res) == backend.DefaultRangeLimit {
		c.log.Warnf("Range query hit backend limit. (this is a bug!) startKey=%q,limit=%d", startKey, backend.DefaultRangeLimit)
	}

	return res, nil
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree
func (c *UnifiedResourceCache) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	result, err := c.getRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resource range")
	}

	resources := make([]types.ResourceWithLabels, 0, len(result))
	for _, item := range result {
		resources = append(resources, item.CloneResource())
	}

	return resources, nil
}

// ResourceGetter is an interface that provides a way to fetch all the resources
// that can be stored in the UnifiedResourceCache
type ResourceGetter interface {
	NodesGetter
	DatabaseServersGetter
	AppServersGetter
	WindowsDesktopGetter
	KubernetesServerGetter
	AccessListsGetter
	SAMLIdpServiceProviderGetter
}

// newWatcher starts and returns a new resource watcher for unified resources.
func newWatcher(ctx context.Context, resourceCache *UnifiedResourceCache, cfg ResourceWatcherConfig) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "setting defaults for unified resource watcher config")
	}

	if _, err := newResourceWatcher(ctx, resourceCache, cfg); err != nil {
		return trace.Wrap(err, "creating a new unified resource watcher")
	}
	return nil
}

func resourceKey(resource types.Resource) []byte {
	return backend.Key(prefix, resource.GetName(), resource.GetKind())
}

func (c *UnifiedResourceCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newNodes, err := c.getNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newDbs, err := c.getDatabaseServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newKubes, err := c.getKubeServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newApps, err := c.getAppServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newSAMLApps, err := c.getSAMLApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newDesktops, err := c.getDesktops(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newAccessLists, err := c.getAccessLists(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.tree.Clear(false)
	putResources[types.Server](c.tree, newNodes)
	putResources[types.DatabaseServer](c.tree, newDbs)
	putResources[types.AppServer](c.tree, newApps)
	putResources[types.KubeServer](c.tree, newKubes)
	putResources[types.SAMLIdPServiceProvider](c.tree, newSAMLApps)
	putResources[types.WindowsDesktop](c.tree, newDesktops)
	putResources[*accesslist.AccessList](c.tree, newAccessLists)
	c.stale = false
	c.defineCollectorAsInitialized()
	return nil

}

// getNodes will get all nodes
func (c *UnifiedResourceCache) getNodes(ctx context.Context) ([]types.Server, error) {
	newNodes, err := c.ResourceGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting nodes for unified resource watcher")
	}

	return newNodes, err
}

// getDatabaseServers will get all database servers
func (c *UnifiedResourceCache) getDatabaseServers(ctx context.Context) ([]types.DatabaseServer, error) {
	newDbs, err := c.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting database servers for unified resource watcher")
	}
	// because it's possible to have multiple replicas of a database server serving the same database
	// we only want to store one based on its internal database resource
	unique := map[string]struct{}{}
	resources := make([]types.DatabaseServer, 0, len(newDbs))
	for _, dbServer := range newDbs {
		db := dbServer.GetDatabase()
		if _, ok := unique[db.GetName()]; ok {
			continue
		}
		unique[db.GetName()] = struct{}{}
		resources = append(resources, dbServer)
	}

	return resources, nil
}

// getKubeServers will get all kube servers
func (c *UnifiedResourceCache) getKubeServers(ctx context.Context) ([]types.KubeServer, error) {
	newKubes, err := c.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting kube servers for unified resource watcher")
	}
	unique := map[string]struct{}{}
	resources := make([]types.KubeServer, 0, len(newKubes))
	for _, kubeServer := range newKubes {
		cluster := kubeServer.GetCluster()
		if _, ok := unique[cluster.GetName()]; ok {
			continue
		}
		unique[cluster.GetName()] = struct{}{}
		resources = append(resources, kubeServer)
	}

	return resources, nil
}

// getAppServers will get all application servers
func (c *UnifiedResourceCache) getAppServers(ctx context.Context) ([]types.AppServer, error) {
	newApps, err := c.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting app servers for unified resource watcher")
	}
	unique := map[string]struct{}{}
	resources := make([]types.AppServer, 0, len(newApps))
	for _, appServer := range newApps {
		app := appServer.GetApp()
		if _, ok := unique[app.GetName()]; ok {
			continue
		}
		unique[app.GetName()] = struct{}{}
		resources = append(resources, appServer)
	}

	return resources, nil
}

// getDesktops will get all windows desktops
func (c *UnifiedResourceCache) getDesktops(ctx context.Context) ([]types.WindowsDesktop, error) {
	newDesktops, err := c.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err, "getting desktops for unified resource watcher")
	}

	return newDesktops, nil
}

// getSAMLApps will get all SAML Idp Service Providers
func (c *UnifiedResourceCache) getSAMLApps(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
	var newSAMLApps []types.SAMLIdPServiceProvider
	startKey := ""

	for {
		resp, nextKey, err := c.ListSAMLIdPServiceProviders(ctx, apidefaults.DefaultChunkSize, startKey)

		if err != nil {
			return nil, trace.Wrap(err, "getting SAML apps for unified resource watcher")
		}
		newSAMLApps = append(newSAMLApps, resp...)

		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	return newSAMLApps, nil
}

// getAccessLists will get all access lists
func (c *UnifiedResourceCache) getAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	var accessLists []*accesslist.AccessList
	startKey := ""

	for {
		resp, nextKey, err := c.ListAccessLists(ctx, apidefaults.DefaultChunkSize, startKey)
		if err != nil {
			return nil, trace.Wrap(err, "getting access lists for unified resource watcher")
		}
		accessLists = append(accessLists, resp...)

		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	return accessLists, nil
}

// read applies the supplied closure to either the primary tree or the ttl-based fallback tree depending on
// wether or not the cache is currently healthy.  locking is handled internally and the passed-in tree should
// not be accessed after the closure completes.
func (c *UnifiedResourceCache) read(ctx context.Context, fn func(tree *btree.BTreeG[*item]) error) error {
	c.mu.Lock()

	if !c.stale {
		fn(c.tree)
		c.mu.Unlock()
		return nil
	}

	c.mu.Unlock()
	ttlTree, err := utils.FnCacheGet(ctx, c.cache, "unified_resources", func(ctx context.Context) (*btree.BTreeG[*item], error) {
		fallbackCache := &UnifiedResourceCache{
			cfg: c.cfg,
			tree: btree.NewG(c.cfg.BTreeDegree, func(a, b *item) bool {
				return a.Less(b)
			}),
			ResourceGetter:  c.ResourceGetter,
			initializationC: make(chan struct{}),
		}
		if err := fallbackCache.getResourcesAndUpdateCurrent(ctx); err != nil {
			return nil, trace.Wrap(err)
		}
		return fallbackCache.tree, nil
	})
	c.mu.Lock()

	if !c.stale {
		// primary became healthy while we were waiting
		fn(c.tree)
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if err != nil {
		// ttl-tree setup failed
		return trace.Wrap(err)
	}

	fn(ttlTree)
	return nil
}

func (c *UnifiedResourceCache) notifyStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stale = true
}

func (c *UnifiedResourceCache) initializationChan() <-chan struct{} {
	return c.initializationC
}

// IsInitialized is used to check that the cache has done its initial
// sync
func (c *UnifiedResourceCache) IsInitialized() bool {
	select {
	case <-c.initializationC:
		return true
	default:
		return false
	}
}

func (c *UnifiedResourceCache) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil {
		c.log.Warnf("Unexpected event: %v.", event)
		return
	}

	switch event.Type {
	case types.OpDelete:
		c.delete(ctx, resourceKey(event.Resource))
	case types.OpPut:
		c.put(ctx, item{
			Key:   resourceKey(event.Resource),
			Value: event.Resource.(resource),
		})
	default:
		c.log.Warnf("unsupported event type %s.", event.Type)
		return
	}
}

// resourceKinds returns a list of resources to be watched.
func (c *UnifiedResourceCache) resourceKinds() []types.WatchKind {
	watchKinds := make([]types.WatchKind, 0, len(UnifiedResourceKinds))
	for _, kind := range UnifiedResourceKinds {
		watchKinds = append(watchKinds, types.WatchKind{Kind: kind})
	}

	return watchKinds
}

func (c *UnifiedResourceCache) defineCollectorAsInitialized() {
	c.once.Do(func() {
		// mark watcher as initialized.
		close(c.initializationC)
	})
}

// Less is used for Btree operations,
// returns true if item is less than the other one
func (i *item) Less(iother btree.Item) bool {
	switch other := iother.(type) {
	case *item:
		return bytes.Compare(i.Key, other.Key) < 0
	case *prefixItem:
		return !iother.Less(i)
	default:
		return false
	}
}

// prefixItem is used for prefix matches on a B-Tree
type prefixItem struct {
	// prefix is a prefix to match
	prefix []byte
}

// Less is used for Btree operations
func (p *prefixItem) Less(iother btree.Item) bool {
	other := iother.(*item)
	return !bytes.HasPrefix(other.Key, p.prefix)
}

type resource interface {
	types.ResourceWithLabels
	CloneResource() types.ResourceWithLabels
}

type item struct {
	// Key is a key of the key value item
	Key []byte
	// Value represents a resource such as types.Server or types.DatabaseServer
	Value resource
}

const (
	prefix = "unified_resource"
)

// MakePaginatedResources converts a list of resources into a list of paginated proto representations.
func MakePaginatedResources(requestType string, resources []types.ResourceWithLabels) ([]*proto.PaginatedResource, error) {
	paginatedResources := make([]*proto.PaginatedResource, 0, len(resources))
	for _, resource := range resources {
		var protoResource *proto.PaginatedResource
		resourceKind := requestType
		if requestType == types.KindUnifiedResource {
			resourceKind = resource.GetKind()
		}
		switch resourceKind {
		case types.KindDatabaseServer:
			database, ok := resource.(*types.DatabaseServerV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}}
		case types.KindDatabaseService:
			databaseService, ok := resource.(*types.DatabaseServiceV1)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseService{DatabaseService: databaseService}}
		case types.KindAppServer:
			app, ok := resource.(*types.AppServerV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_AppServer{AppServer: app}}
		case types.KindNode:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: srv}}
		case types.KindKubeServer:
			srv, ok := resource.(*types.KubernetesServerV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: srv}}
		case types.KindWindowsDesktop:
			desktop, ok := resource.(*types.WindowsDesktopV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}}
		case types.KindWindowsDesktopService:
			desktopService, ok := resource.(*types.WindowsDesktopServiceV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktopService{WindowsDesktopService: desktopService}}
		case types.KindKubernetesCluster:
			cluster, ok := resource.(*types.KubernetesClusterV3)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubeCluster{KubeCluster: cluster}}
		case types.KindUserGroup:
			userGroup, ok := resource.(*types.UserGroupV1)
			if !ok {
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_UserGroup{UserGroup: userGroup}}
		case types.KindSAMLIdPServiceProvider, types.KindAppOrSAMLIdPServiceProvider:
			switch appOrSP := resource.(type) {
			case *types.AppServerV3:
				protoResource = &proto.PaginatedResource{
					Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
						AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
							Resource: &types.AppServerOrSAMLIdPServiceProviderV1_AppServer{
								AppServer: appOrSP,
							},
						},
					}}
			case *types.SAMLIdPServiceProviderV1:
				protoResource = &proto.PaginatedResource{
					Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
						AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
							Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
								SAMLIdPServiceProvider: appOrSP,
							},
						},
					}}
			default:
				return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
			}
		default:
			return nil, trace.NotImplemented("resource type %s doesn't support pagination", resource.GetKind())
		}

		paginatedResources = append(paginatedResources, protoResource)
	}
	return paginatedResources, nil
}
