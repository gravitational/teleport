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

type config struct {
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
}

// NewUnifiedResourceCache creates a new memory cache that holds the unified resources
func NewUnifiedResourceCache(cfg config) (*UnifiedResourceCache, error) {
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
	}
	return m, nil
}

// CheckAndSetDefaults checks and sets default values
func (cfg *config) CheckAndSetDefaults() error {
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

// UnifiedResourceCache contains a representation of all resources that are displayable in the UI
type UnifiedResourceCache struct {
	mu  sync.Mutex
	log *log.Entry
	cfg config
	// tree is a BTree with items
	tree *btree.BTreeG[*Item]
}

// Event represents an event that happened in the backend
type Event struct {
	// Type is operation type
	Type types.OpType
	// Item is event Item
	Item Item
}

// Create creates item if it does not exist
func (c *UnifiedResourceCache) Create(ctx context.Context, i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tree.Has(&i) {
		return trace.AlreadyExists("key %q already exists", string(i.Key))
	}
	event := Event{
		Type: types.OpPut,
		Item: i,
	}
	c.processEvent(event)
	return nil
}

// Get returns a single item or not found error
func (c *UnifiedResourceCache) Get(ctx context.Context, key []byte) (*Item, error) {
	if len(key) == 0 {
		return nil, trace.BadParameter("missing parameter key")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	i, found := c.tree.Get(&Item{Key: key})
	if !found {
		return nil, trace.NotFound("key %q is not found", string(key))
	}
	return i, nil
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

// UnifiedResourceWatcherConfig is a UnifiedResourceWatcher configuration.
type UnifiedResourceWatcherConfig struct {
	ResourceWatcherConfig
	ResourceGetter
}

// UnifiedResourceWatcher is built on top of resourceWatcher to monitor additions
// and deletions to resources displayable in the UI such as Nodes, Dbs, Apps, etc.
type UnifiedResourceWatcher struct {
	*resourceWatcher
	*unifiedResourceCollector
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree
func (u *unifiedResourceCollector) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	u.lock.RLock()
	defer u.lock.RUnlock()
	var resources []types.ResourceWithLabels

	// if the cache is not initialized or stale, instead of returning nothing, return upstream nodes
	if !u.isInitialized() || u.stale {
		nodes, err := u.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return nil, trace.Wrap(err, "getting nodes while unified resource cache is uninitialized or stale")
		}

		for _, node := range nodes {
			resources = append(resources, node)
		}
		return resources, nil
	}

	result, err := u.current.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err, "getting unified resource range")
	}
	for _, item := range result.Items {
		resources = append(resources, item.Value)
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

// NewUnifiedResourceWatcher returns a new instance of UnifiedResourceWatcher.
func NewUnifiedResourceWatcher(ctx context.Context, cfg UnifiedResourceWatcherConfig) (*UnifiedResourceWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "setting defaults for unified resource watcher config")
	}

	mem, err := NewUnifiedResourceCache(config{})
	if err != nil {
		return nil, trace.Wrap(err, "creating a new unified resource cache")
	}

	collector := &unifiedResourceCollector{
		UnifiedResourceWatcherConfig: cfg,
		current:                      mem,
		initializationC:              make(chan struct{}),
	}

	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err, "creating a new unified resource watcher")
	}

	return &UnifiedResourceWatcher{
		resourceWatcher:          watcher,
		unifiedResourceCollector: collector,
	}, nil
}

func keyOf(r types.Resource) []byte {
	return backend.Key(prefix, r.GetName(), r.GetKind())
}

type unifiedResourceCollector struct {
	UnifiedResourceWatcherConfig
	current         *UnifiedResourceCache
	lock            sync.RWMutex
	initializationC chan struct{}
	once            sync.Once
	stale           bool
}

func (u *unifiedResourceCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	defer u.defineCollectorAsInitialized()
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
	return nil
}

// getAndUpdateNodes will get nodes and update the current tree with each Node
func (u *unifiedResourceCollector) getAndUpdateNodes(ctx context.Context) error {
	newNodes, err := u.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting nodes for unified resource watcher")
	}
	nodes := make([]Item, 0, len(newNodes))
	for _, node := range newNodes {
		nodes = append(nodes, Item{
			Key:   keyOf(node),
			Value: node.DeepCopy(),
		})
	}
	return u.current.PutRange(ctx, nodes)
}

// getAndUpdateDatabases will get database servers and update the current tree with each DatabaseServer
func (u *unifiedResourceCollector) getAndUpdateDatabases(ctx context.Context) error {
	newDbs, err := u.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting databases for unified resource watcher")
	}
	dbs := make([]Item, 0, len(newDbs))
	for _, db := range newDbs {
		dbs = append(dbs, Item{
			Key:   keyOf(db),
			Value: db.Copy(),
		})
	}
	return u.current.PutRange(ctx, dbs)
}

// getAndUpdateKubes will get kube clusters and update the current tree with each KubeCluster
func (u *unifiedResourceCollector) getAndUpdateKubes(ctx context.Context) error {
	newKubes, err := u.GetKubernetesClusters(ctx)
	if err != nil {
		return trace.Wrap(err, "getting kubes for unified resource watcher")
	}
	kubes := make([]Item, 0, len(newKubes))
	for _, kube := range newKubes {
		kubes = append(kubes, Item{
			Key:   keyOf(kube),
			Value: kube.Copy(),
		})
	}
	return u.current.PutRange(ctx, kubes)
}

// getAndUpdateApps will get application servers and update the current tree with each AppServer
func (u *unifiedResourceCollector) getAndUpdateApps(ctx context.Context) error {
	newApps, err := u.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err, "getting apps for unified resource watcher")
	}

	apps := make([]Item, 0, len(newApps))
	for _, app := range newApps {
		apps = append(apps, Item{
			Key:   keyOf(app),
			Value: app.Copy(),
		})
	}
	return u.current.PutRange(ctx, apps)
}

// getAndUpdateSAMLApps will get SAML Idp Service Providers servers and update the current tree with each SAMLIdpServiceProvider
func (u *unifiedResourceCollector) getAndUpdateSAMLApps(ctx context.Context) error {
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
				Value: app.Copy(),
			})
		}
		if nextKey == "" {
			break
		}

		startKey = nextKey
	}

	return u.current.PutRange(ctx, newSAMLApps)
}

// getAndUpdateDesktops will get windows desktops and update the current tree with each Desktop
func (u *unifiedResourceCollector) getAndUpdateDesktops(ctx context.Context) error {
	newDesktops, err := u.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err, "getting desktops for unified resource watcher")
	}

	desktops := make([]Item, 0, len(newDesktops))
	for _, desktop := range newDesktops {
		desktops = append(desktops, Item{
			Key:   keyOf(desktop),
			Value: desktop.Copy(),
		})
	}

	return u.current.PutRange(ctx, desktops)
}

func (u *unifiedResourceCollector) notifyStale() {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.stale = true
}

func (u *unifiedResourceCollector) initializationChan() <-chan struct{} {
	return u.initializationC
}

func (u *unifiedResourceCollector) isInitialized() bool {
	select {
	case <-u.initializationC:
		return true
	default:
		return false
	}
}

func (u *unifiedResourceCollector) processEventAndUpdateCurrent(ctx context.Context, event types.Event) {
	if event.Resource == nil {
		u.Log.Warnf("Unexpected event: %v.", event)
		return
	}

	u.lock.Lock()
	defer u.lock.Unlock()
	switch event.Type {
	case types.OpDelete:
		u.current.delete(ctx, keyOf(event.Resource))
	case types.OpPut:
		u.current.put(ctx, Item{
			Key:   keyOf(event.Resource),
			Value: event.Resource.(types.ResourceWithLabels),
		})
	default:
		u.Log.Warnf("unsupported event type %s.", event.Type)
		return
	}
}

// resourceKinds returns a list of resources to be watched.
func (u *unifiedResourceCollector) resourceKinds() []types.WatchKind {
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindKubernetesCluster},
	}
}

func (u *unifiedResourceCollector) defineCollectorAsInitialized() {
	u.once.Do(func() {
		// mark watcher as initialized.
		close(u.initializationC)
	})
}

const (
	prefix = "unified_resource"
)
