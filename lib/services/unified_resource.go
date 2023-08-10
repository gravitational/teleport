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
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

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
	return c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		tree.ReplaceOrInsert(&i)
		return nil
	})
}

func putResources(ctx context.Context, c *UnifiedResourceCache, resources []resource) error {
	return c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		for _, resource := range resources {
			tree.ReplaceOrInsert(&item{Key: resourceKey(resource), Value: resource})
		}
		return nil
	})
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

func (c *UnifiedResourceCache) clear(ctx context.Context) {
	c.read(ctx, func(tree *btree.BTreeG[*item]) error {
		// passing false means we do NOT add the nodes to a "freelist" as it clears the tree
		// and instead, just dereferences every item and leaves it to the garbage collector to deal with.
		// because we clear on cache create (and re-init), this was chosen to speed up the process as it's O(1)
		tree.Clear(false)
		return nil
	})
}

func (c *UnifiedResourceCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	c.clear(ctx)
	newNodes, err := c.getAndUpdateNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources := make([]resource, len(newNodes))
	newResources = append(newResources, newNodes...)

	newDbs, err := c.getAndUpdateDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources = append(newResources, newDbs...)

	newKubes, err := c.getAndUpdateKubes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources = append(newResources, newKubes...)

	newApps, err := c.getAndUpdateApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources = append(newResources, newApps...)

	newSAMLApps, err := c.getAndUpdateSAMLApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources = append(newResources, newSAMLApps...)

	newDesktops, err := c.getAndUpdateDesktops(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	newResources = append(newResources, newDesktops...)

	err = putResources(ctx, c, newResources)
	if err != nil {
		return trace.Wrap(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stale = false
	c.defineCollectorAsInitialized()
	return nil
}

// getAndUpdateNodes will get nodes and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateNodes(ctx context.Context) ([]resource, error) {
	newNodes, err := c.ResourceGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting nodes for unified resource watcher")
	}
	resources := make([]resource, len(newNodes))
	for _, node := range newNodes {
		resources = append(resources, node)
	}

	return resources, err
}

// getAndUpdateDatabases will get database servers and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateDatabases(ctx context.Context) ([]resource, error) {
	newDbs, err := c.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting databases for unified resource watcher")
	}
	resources := make([]resource, len(newDbs))
	for _, db := range newDbs {
		resources = append(resources, db)
	}

	return resources, nil
}

// getAndUpdateKubes will get kube clusters and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateKubes(ctx context.Context) ([]resource, error) {
	newKubes, err := c.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting kubes for unified resource watcher")
	}
	resources := make([]resource, len(newKubes))
	for _, kube := range newKubes {
		resources = append(resources, kube)
	}

	return resources, nil
}

// getAndUpdateApps will get application servers and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateApps(ctx context.Context) ([]resource, error) {
	newApps, err := c.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting apps for unified resource watcher")
	}
	resources := make([]resource, len(newApps))
	for _, app := range newApps {
		resources = append(resources, app)
	}

	return resources, nil
}

// getAndUpdateDesktops will get windows desktops and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateDesktops(ctx context.Context) ([]resource, error) {
	newDesktops, err := c.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err, "getting desktops for unified resource watcher")
	}
	resources := make([]resource, len(newDesktops))
	for _, desktop := range newDesktops {
		resources = append(resources, desktop)
	}

	return resources, nil
}

// getAndUpdateSAMLApps will get SAML Idp Service Providers servers and return them as a resources
func (c *UnifiedResourceCache) getAndUpdateSAMLApps(ctx context.Context) ([]resource, error) {
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
	resources := make([]resource, len(newSAMLApps))
	for _, app := range newSAMLApps {
		resources = append(resources, app)
	}

	return resources, nil
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
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindKubeServer},
	}
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
