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

	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

type UnifiedResourceCacheConfig struct {
	// Context is a context for opening the
	// database
	Context context.Context
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
	ResourceGetter
}

// NewUnifiedResourceCache creates a new memory cache that holds the unified resources
func NewUnifiedResourceCache(ctx context.Context, cfg UnifiedResourceCacheConfig) (*UnifiedResourceCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "setting defaults for unified resource cache")
	}

	m := &UnifiedResourceCache{
		mu: sync.Mutex{},
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentMemory,
		}),
		cfg: cfg,
		tree: btree.NewG(cfg.BTreeDegree, func(a, b *Item) bool {
			return a.Less(b)
		}),
		initializationC: make(chan struct{}),
		ResourceGetter:  cfg.ResourceGetter,
	}

	err := newWatcher(ctx, m, cfg.ResourceWatcherConfig)

	if err != nil {
		return nil, trace.Wrap(err, "creating unified resource watcher")
	}
	return m, nil
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UnifiedResourceCacheConfig) CheckAndSetDefaults() error {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
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

// PutRange puts range of items into backend (creates if items do not
// exist, updates it otherwise)
func (c *UnifiedResourceCache) PutRange(ctx context.Context, items []Item) error {
	for i := range items {
		if items[i].Key == nil {
			return trace.BadParameter("missing parameter key in item %v", i)
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		event := Event{
			Type: types.OpPut,
			Item: item,
		}
		c.processEvent(event)
	}
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

	// if the cache is not initialized or stale, instead of returning nothing, return upstream nodes
	if !u.isInitialized() || u.stale {
		nodes, err := u.ResourceGetter.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return nil, trace.Wrap(err, "getting nodes while unified resource cache is uninitialized or stale")
		}

		for _, node := range nodes {
			resources = append(resources, node)
		}
		return resources, nil
	}

	result, err := u.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resource range")
	}
	for _, item := range result.Items {
		switch r := item.Value.(type) {
		case types.Server:
			resources = append(resources, r.DeepCopy())
		case types.AppServer:
			resources = append(resources, r.Copy())
		case types.SAMLIdPServiceProvider:
			resources = append(resources, r.Copy())
		case types.DatabaseServer:
			resources = append(resources, r.Copy())
		case types.KubeCluster:
			resources = append(resources, r.Copy())
		case types.WindowsDesktop:
			resources = append(resources, r.Copy())
		default:
			return nil, trace.NotImplemented("unsupported type received from unified resources cache")
		}
	}

	return resources, nil
}

type ResourceGetter interface {
	NodesGetter
	DatabaseServersGetter
	AppServersGetter
	WindowsDesktopGetter
	KubernetesClusterGetter
	SAMLIdpServiceProviderGetter
}

// newWatcher starts and returns a new resource watcher for unified resources.
func newWatcher(ctx context.Context, cache *UnifiedResourceCache, cfg ResourceWatcherConfig) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "setting defaults for unified resource watcher config")
	}
	_, err := newResourceWatcher(ctx, cache, cfg)
	if err != nil {
		return trace.Wrap(err, "creating a new unified resource watcher")
	}
	return nil
}

func keyOf(r types.Resource) []byte {
	return backend.Key(prefix, r.GetName(), r.GetKind())
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
	nodes := make([]Item, 0, len(newNodes))
	for _, node := range newNodes {
		nodes = append(nodes, Item{
			Key:   keyOf(node),
			Value: node,
		})
	}
	return u.PutRange(ctx, nodes)
}

// getAndUpdateDatabases will get database servers and update the current tree with each DatabaseServer
func (u *UnifiedResourceCache) getAndUpdateDatabases(ctx context.Context) error {
	newDbs, err := u.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting databases for unified resource watcher")
	}
	dbs := make([]Item, 0, len(newDbs))
	for _, db := range newDbs {
		dbs = append(dbs, Item{
			Key:   keyOf(db),
			Value: db,
		})
	}
	return u.PutRange(ctx, dbs)
}

// getAndUpdateKubes will get kube clusters and update the current tree with each KubeCluster
func (u *UnifiedResourceCache) getAndUpdateKubes(ctx context.Context) error {
	newKubes, err := u.GetKubernetesClusters(ctx)
	if err != nil {
		return trace.Wrap(err, "getting kubes for unified resource watcher")
	}
	kubes := make([]Item, 0, len(newKubes))
	for _, kube := range newKubes {
		kubes = append(kubes, Item{
			Key:   keyOf(kube),
			Value: kube,
		})
	}
	return u.PutRange(ctx, kubes)
}

// getAndUpdateApps will get application servers and update the current tree with each AppServer
func (u *UnifiedResourceCache) getAndUpdateApps(ctx context.Context) error {
	newApps, err := u.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting apps for unified resource watcher")
	}

	apps := make([]Item, 0, len(newApps))
	for _, app := range newApps {
		apps = append(apps, Item{
			Key:   keyOf(app),
			Value: app,
		})
	}
	return u.PutRange(ctx, apps)
}

// getAndUpdateSAMLApps will get SAML Idp Service Providers servers and update the current tree with each SAMLIdpServiceProvider
func (u *UnifiedResourceCache) getAndUpdateSAMLApps(ctx context.Context) error {
	var newSAMLApps []Item
	startKey := ""

	for {
		resp, nextKey, err := u.ListSAMLIdPServiceProviders(ctx, apidefaults.DefaultChunkSize, startKey)

		if err != nil {
			return trace.Wrap(err, "getting SAML apps for unified resource watcher")
		}
		for _, app := range resp {
			newSAMLApps = append(newSAMLApps, Item{
				Key:   keyOf(app),
				Value: app,
			})
		}
		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	return u.PutRange(ctx, newSAMLApps)
}

// getAndUpdateDesktops will get windows desktops and update the current tree with each Desktop
func (u *UnifiedResourceCache) getAndUpdateDesktops(ctx context.Context) error {
	newDesktops, err := u.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err, "getting desktops for unified resource watcher")
	}

	desktops := make([]Item, 0, len(newDesktops))
	for _, desktop := range newDesktops {
		desktops = append(desktops, Item{
			Key:   keyOf(desktop),
			Value: desktop,
		})
	}

	return u.PutRange(ctx, desktops)
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
			Value: event.Resource.(types.ResourceWithLabels),
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
		{Kind: types.KindKubernetesCluster},
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
	Value types.ResourceWithLabels
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
