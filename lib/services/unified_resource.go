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
func (c *UnifiedResourceCache) put(ctx context.Context, i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	event := Event{
		Type: types.OpPut,
		Item: i,
	}
	c.processEvent(event)
	return nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (c *UnifiedResourceCache) delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.tree.Has(&Item{Key: key}) {
		return trace.NotFound("key %q is not found", string(key))
	}
	event := Event{
		Type: types.OpDelete,
		Item: Item{
			Key: key,
		},
	}
	c.processEvent(event)
	return nil
}

// GetRange returns query range
func (c *UnifiedResourceCache) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*getResult, error) {
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
	re := c.getRange(ctx, startKey, endKey, limit)
	if len(re.Items) == backend.DefaultRangeLimit {
		c.log.Warnf("Range query hit backend limit. (this is a bug!) startKey=%q,limit=%d", startKey, backend.DefaultRangeLimit)
	}
	return &re, nil
}

type getResult struct {
	Items []Item
}

func (c *UnifiedResourceCache) getRange(ctx context.Context, startKey, endKey []byte, limit int) getResult {
	var res getResult
	c.tree.AscendRange(&Item{Key: startKey}, &Item{Key: endKey}, func(item *Item) bool {
		res.Items = append(res.Items, *item)
		if limit > 0 && len(res.Items) >= limit {
			return false
		}
		return true
	})
	return res
}

func (c *UnifiedResourceCache) processEvent(event Event) {
	switch event.Type {
	case types.OpPut:
		item := &event.Item
		c.tree.ReplaceOrInsert(item)
	case types.OpDelete:
		c.tree.Delete(&event.Item)
	default:
		c.log.Warnf("unsupported event type %s.", event.Type)
	}
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree
func (u *UnifiedResourceCache) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	var resources []types.ResourceWithLabels

	u.refreshStaleResources(ctx)

	result, err := u.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resource range")
	}
	for _, item := range result.Items {
		cloned := item.Value.CloneAny()
		clonedResource, ok := cloned.(types.ResourceWithLabels)
		if !ok {
			return nil, trace.BadParameter("clone returned unexpected type %T", clonedResource)
		}

		resources = append(resources, clonedResource)
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
	switch r := resource.(type) {
	case types.Server:
		return backend.Key(prefix, r.GetHostname(), r.GetName(), r.GetKind())
	default:
		return backend.Key(prefix, r.GetName(), r.GetKind())
	}
}

func (u *UnifiedResourceCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	err := u.getAndUpdateNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateKubes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateSAMLApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateDesktops(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	u.stale = false
	u.defineCollectorAsInitialized()
	return nil
}

// getAndUpdateNodes will get nodes and update the current tree with each Node
func (u *UnifiedResourceCache) getAndUpdateNodes(ctx context.Context) error {
	newNodes, err := u.ResourceGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting nodes for unified resource watcher")
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.Server](ctx, u, newNodes))
}

func getCloneAny(resource types.ResourceWithLabels) (types.CloneAny, error) {
	cloner, ok := resource.(types.CloneAny)
	if !ok {
		return nil, trace.NotImplemented("unsupported type %t for unified resources received", resource)
	}
	return cloner, nil
}

func putResources[T types.ResourceWithLabels](ctx context.Context, c *UnifiedResourceCache, resources []T) error {
	for _, resource := range resources {
		r, err := getCloneAny(resource)
		if err != nil {
			return trace.Wrap(err)
		}
		event := Event{
			Type: types.OpPut,
			Item: Item{Key: keyOf(resource), Value: r},
		}
		c.processEvent(event)
	}
	return nil
}

// getAndUpdateDatabases will get database servers and update the current tree with each DatabaseServer
func (u *UnifiedResourceCache) getAndUpdateDatabases(ctx context.Context) error {
	newDbs, err := u.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting databases for unified resource watcher")
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.DatabaseServer](ctx, u, newDbs))
}

func (u *UnifiedResourceCache) refreshStaleResources(ctx context.Context) error {
	u.mu.Lock()
	if !u.stale && u.isInitialized() {
		u.mu.Unlock()
		return nil
	}
	u.mu.Unlock()

	_, err := utils.FnCacheGet(ctx, u.cache, "resources", func(ctx context.Context) (any, error) {

		currentResourceCache := &UnifiedResourceCache{
			cfg: u.cfg,
			tree: btree.NewG(u.cfg.BTreeDegree, func(a, b *Item) bool {
				return a.Less(b)
			}),
			ResourceGetter: u.ResourceGetter,
		}
		err := u.getResourcesAndUpdateCurrent(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		u.mu.Lock()
		defer u.mu.Unlock()

		// There is a chance that the watcher reinitialized while
		// getting resources happened above. Check if we are still stale
		// now that the lock is held to ensure that the refresh is
		// still necessary.
		if !u.stale {
			return nil, nil
		}

		u.tree = currentResourceCache.tree
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// getAndUpdateKubes will get kube clusters and update the current tree with each KubeCluster
func (u *UnifiedResourceCache) getAndUpdateKubes(ctx context.Context) error {
	newKubes, err := u.GetKubernetesServers(ctx)
	if err != nil {
		return trace.Wrap(err, "getting kubes for unified resource watcher")
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.KubeServer](ctx, u, newKubes))
}

// getAndUpdateApps will get application servers and update the current tree with each AppServer
func (u *UnifiedResourceCache) getAndUpdateApps(ctx context.Context) error {
	newApps, err := u.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting apps for unified resource watcher")
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.AppServer](ctx, u, newApps))
}

// getAndUpdateSAMLApps will get SAML Idp Service Providers servers and update the current tree with each SAMLIdpServiceProvider
func (u *UnifiedResourceCache) getAndUpdateSAMLApps(ctx context.Context) error {
	var newSAMLApps []types.SAMLIdPServiceProvider
	startKey := ""

	for {
		resp, nextKey, err := u.ListSAMLIdPServiceProviders(ctx, apidefaults.DefaultChunkSize, startKey)

		if err != nil {
			return trace.Wrap(err, "getting SAML apps for unified resource watcher")
		}
		newSAMLApps = append(newSAMLApps, resp...)

		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.SAMLIdPServiceProvider](ctx, u, newSAMLApps))
}

// getAndUpdateDesktops will get windows desktops and update the current tree with each Desktop
func (u *UnifiedResourceCache) getAndUpdateDesktops(ctx context.Context) error {
	newDesktops, err := u.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err, "getting desktops for unified resource watcher")
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return trace.Wrap(putResources[types.WindowsDesktop](ctx, u, newDesktops))
}

func (u *UnifiedResourceCache) notifyStale() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.stale = true
}

func (u *UnifiedResourceCache) initializationChan() <-chan struct{} {
	return u.initializationC
}

func (u *UnifiedResourceCache) isInitialized() bool {
	select {
	case <-u.initializationC:
		return true
	default:
		return false
	}
}

func (u *UnifiedResourceCache) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil {
		u.log.Warnf("Unexpected event: %v.", event)
		return
	}

	switch event.Type {
	case types.OpDelete:
		u.delete(ctx, keyOf(event.Resource))
	case types.OpPut:
		u.put(ctx, Item{
			Key:   keyOf(event.Resource),
			Value: event.Resource.(types.CloneAny),
		})
	default:
		u.log.Warnf("unsupported event type %s.", event.Type)
		return
	}
}

// resourceKinds returns a list of resources to be watched.
func (u *UnifiedResourceCache) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindKubeServer},
	}
}

func (u *UnifiedResourceCache) defineCollectorAsInitialized() {
	u.once.Do(func() {
		// mark watcher as initialized.
		close(u.initializationC)
	})
}

// less is used for Btree operations,
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

type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value represents a resource such as types.Server or types.DatabaseServer
	Value types.CloneAny
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
