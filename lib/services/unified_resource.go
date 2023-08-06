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
	tree            *btree.BTreeG[*Item]
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
			trace.Component: teleport.ComponentMemory,
		}),
		cfg: cfg,
		tree: btree.NewG(cfg.BTreeDegree, func(a, b *Item) bool {
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

// Put puts value into backend (creates if it does not
// exist, updates it otherwise)
func (c *UnifiedResourceCache) put(i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tree.ReplaceOrInsert(&i)
	return nil
}

func putResources[T resource](c *UnifiedResourceCache, resources []T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, resource := range resources {
		c.tree.ReplaceOrInsert(&Item{Key: keyOf(resource), Value: resource})
	}
	return nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (c *UnifiedResourceCache) delete(key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.tree.Delete(&Item{Key: key}); !ok {
		return trace.NotFound("key %q is not found", string(key))
	}
	return nil
}

func (c *UnifiedResourceCache) getRange(startKey, endKey []byte, limit int) ([]resource, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	var res []resource
	c.tree.AscendRange(&Item{Key: startKey}, &Item{Key: endKey}, func(item *Item) bool {
		res = append(res, item.Value)
		if limit > 0 && len(res) >= limit {
			return false
		}
		return true
	})

	if len(res) == backend.DefaultRangeLimit {
		c.log.Warnf("Range query hit backend limit. (this is a bug!) startKey=%q,limit=%d", startKey, backend.DefaultRangeLimit)
	}
	return res, nil
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree
func (c *UnifiedResourceCache) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	if err := c.refreshStaleResources(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := c.getRange(backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resource range")
	}

	resources := make([]types.ResourceWithLabels, 0, len(result))
	for _, item := range result {
		resources = append(resources, item.CloneResource())
	}

	return resources, nil
}

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

func keyOf(resource types.Resource) []byte {
	return backend.Key(prefix, resource.GetName(), resource.GetKind())
}

func (c *UnifiedResourceCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	err := c.getAndUpdateNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.getAndUpdateDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.getAndUpdateKubes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.getAndUpdateApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.getAndUpdateSAMLApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.getAndUpdateDesktops(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.stale = false
	c.defineCollectorAsInitialized()
	return nil
}

// getAndUpdateNodes will get nodes and update the current tree with each Node
func (c *UnifiedResourceCache) getAndUpdateNodes(ctx context.Context) error {
	newNodes, err := c.ResourceGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting nodes for unified resource watcher")
	}

	return trace.Wrap(putResources[types.Server](c, newNodes))
}

// getAndUpdateDatabases will get database servers and update the current tree with each DatabaseServer
func (c *UnifiedResourceCache) getAndUpdateDatabases(ctx context.Context) error {
	newDbs, err := c.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting databases for unified resource watcher")
	}

	return trace.Wrap(putResources[types.DatabaseServer](c, newDbs))
}

func (c *UnifiedResourceCache) refreshStaleResources(ctx context.Context) error {
	c.mu.Lock()
	if !c.stale && c.isInitialized() {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	_, err := utils.FnCacheGet(ctx, c.cache, "unified_resources", func(ctx context.Context) (any, error) {
		currentResourceCache := &UnifiedResourceCache{
			cfg: c.cfg,
			tree: btree.NewG(c.cfg.BTreeDegree, func(a, b *Item) bool {
				return a.Less(b)
			}),
			ResourceGetter:  c.ResourceGetter,
			initializationC: make(chan struct{}),
		}
		err := currentResourceCache.getResourcesAndUpdateCurrent(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		// There is a chance that the watcher reinitialized while
		// getting resources happened above. Check if we are still stale
		// now that the lock is held to ensure that the refresh is
		// still necessary.
		if !c.stale {
			return nil, nil
		}

		c.tree = currentResourceCache.tree
		return currentResourceCache.tree, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// getAndUpdateKubes will get kube clusters and update the current tree with each KubeCluster
func (c *UnifiedResourceCache) getAndUpdateKubes(ctx context.Context) error {
	newKubes, err := c.GetKubernetesServers(ctx)
	if err != nil {
		return trace.Wrap(err, "getting kubes for unified resource watcher")
	}

	return trace.Wrap(putResources[types.KubeServer](c, newKubes))
}

// getAndUpdateApps will get application servers and update the current tree with each AppServer
func (c *UnifiedResourceCache) getAndUpdateApps(ctx context.Context) error {
	newApps, err := c.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting apps for unified resource watcher")
	}

	return trace.Wrap(putResources[types.AppServer](c, newApps))
}

// getAndUpdateSAMLApps will get SAML Idp Service Providers servers and update the current tree with each SAMLIdpServiceProvider
func (c *UnifiedResourceCache) getAndUpdateSAMLApps(ctx context.Context) error {
	var newSAMLApps []types.SAMLIdPServiceProvider
	startKey := ""

	for {
		resp, nextKey, err := c.ListSAMLIdPServiceProviders(ctx, apidefaults.DefaultChunkSize, startKey)

		if err != nil {
			return trace.Wrap(err, "getting SAML apps for unified resource watcher")
		}
		newSAMLApps = append(newSAMLApps, resp...)

		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	return trace.Wrap(putResources[types.SAMLIdPServiceProvider](c, newSAMLApps))
}

// getAndUpdateDesktops will get windows desktops and update the current tree with each Desktop
func (c *UnifiedResourceCache) getAndUpdateDesktops(ctx context.Context) error {
	newDesktops, err := c.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err, "getting desktops for unified resource watcher")
	}

	return trace.Wrap(putResources[types.WindowsDesktop](c, newDesktops))
}

func (c *UnifiedResourceCache) notifyStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stale = true
}

func (c *UnifiedResourceCache) initializationChan() <-chan struct{} {
	return c.initializationC
}

func (c *UnifiedResourceCache) isInitialized() bool {
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
		c.delete(keyOf(event.Resource))
	case types.OpPut:
		c.put(Item{
			Key:   keyOf(event.Resource),
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
func (i *Item) Less(iother btree.Item) bool {
	switch other := iother.(type) {
	case *Item:
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
	other := iother.(*Item)
	return !bytes.HasPrefix(other.Key, p.prefix)
}

type resource interface {
	types.ResourceWithLabels
	CloneResource() types.ResourceWithLabels
}

type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value represents a resource such as types.Server or types.DatabaseServer
	Value resource
}

// Event represents an event that happened in the backend
type Event struct {
	// Type is operation type
	Type types.OpType
	// Item is event Item
	Item Item
}

const (
	prefix = "unified_resource"
)
