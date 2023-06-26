package services

import (
	"bytes"
	"context"
	"sync"

	"github.com/google/btree"
	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
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
	// EventsOff turns off events generation
	EventsOff bool
	// BufferSize sets up event buffer size
	BufferSize int
}

// New creates a new memory cache that holds the unified resources
func NewUnifiedResourceCache(cfg config) (*UnifiedResourceCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(cfg.Context)
	buf := backend.NewCircularBuffer(
		backend.BufferCapacity(cfg.BufferSize),
	)
	buf.SetInit()
	m := &UnifiedResourceCache{
		Mutex: &sync.Mutex{},
		Entry: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentMemory,
		}),
		config: cfg,
		tree: btree.NewG(cfg.BTreeDegree, func(a, b *btreeItem) bool {
			return a.Less(b)
		}),
		cancel: cancel,
		ctx:    ctx,
		buf:    buf,
	}
	return m, nil
}

// CheckAndSetDefaults checks and sets default values
func (cfg *config) CheckAndSetDefaults() error {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = backend.DefaultBufferCapacity
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

type btreeItem struct {
	Item
	index int
}

// Less is used for Btree operations,
// returns true if item is less than the other one
func (i *btreeItem) Less(iother btree.Item) bool {
	switch other := iother.(type) {
	case *btreeItem:
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
	other := iother.(*btreeItem)
	return !bytes.HasPrefix(other.Key, p.prefix)
}

type Item struct {
	// Key is a key of the key value item
	Key []byte
	// Value represents a resource such as types.Server or types.DatabaseServer
	Value types.ResourceWithLabels
}

type UnifiedResourceCache struct {
	*sync.Mutex
	*log.Entry
	config
	// tree is a BTree with items
	tree *btree.BTreeG[*btreeItem]
	// cancel is a function that cancels
	// all operations
	cancel context.CancelFunc
	// ctx is a context signaling close
	ctx context.Context
	buf *backend.CircularBuffer
}

type Event struct {
	// Type is operation type
	Type types.OpType
	// Item is event Item
	Item Item
}

// Close closes memory backend
func (c *UnifiedResourceCache) Close() error {
	c.cancel()
	c.Lock()
	defer c.Unlock()
	c.buf.Close()
	return nil
}

// Clock returns clock used by this backend
func (c *UnifiedResourceCache) Clock() clockwork.Clock {
	return c.config.Clock
}

// Create creates item if it does not exist
func (c *UnifiedResourceCache) Create(ctx context.Context, i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.Lock()
	defer c.Unlock()
	if c.tree.Has(&btreeItem{Item: i}) {
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
	c.Lock()
	defer c.Unlock()
	i, found := c.tree.Get(&btreeItem{Item: Item{Key: key}})
	if !found {
		return nil, trace.NotFound("key %q is not found", string(key))
	}
	return &i.Item, nil
}

// Update updates item if it exists, or returns NotFound error
func (c *UnifiedResourceCache) Update(ctx context.Context, i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.Lock()
	defer c.Unlock()
	if !c.tree.Has(&btreeItem{Item: i}) {
		return trace.NotFound("key %q is not found", string(i.Key))
	}
	event := Event{
		Type: types.OpPut,
		Item: i,
	}
	c.processEvent(event)
	return nil
}

// Put puts value into backend (creates if it does not
// exist, updates it otherwise)
func (c *UnifiedResourceCache) Put(ctx context.Context, i Item) error {
	if len(i.Key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.Lock()
	defer c.Unlock()
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
	c.Lock()
	defer c.Unlock()
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
func (c *UnifiedResourceCache) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return trace.BadParameter("missing parameter key")
	}
	c.Lock()
	defer c.Unlock()
	if !c.tree.Has(&btreeItem{Item: Item{Key: key}}) {
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

// DeleteRange deletes range of items with keys between startKey and endKey
// Note that elements deleted by range do not produce any events
func (c *UnifiedResourceCache) DeleteRange(ctx context.Context, startKey, endKey []byte) error {
	if len(startKey) == 0 {
		return trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return trace.BadParameter("missing parameter endKey")
	}
	c.Lock()
	defer c.Unlock()
	re := c.getRange(ctx, startKey, endKey, backend.NoLimit)
	for _, item := range re.Items {
		event := Event{
			Type: types.OpDelete,
			Item: item,
		}
		c.processEvent(event)
	}
	return nil
}

// GetRange returns query range
func (c *UnifiedResourceCache) GetRange(ctx context.Context, startKey []byte, endKey []byte, limit int) (*GetResult, error) {
	if len(startKey) == 0 {
		return nil, trace.BadParameter("missing parameter startKey")
	}
	if len(endKey) == 0 {
		return nil, trace.BadParameter("missing parameter endKey")
	}
	if limit <= 0 {
		limit = backend.DefaultRangeLimit
	}
	c.Lock()
	defer c.Unlock()
	re := c.getRange(ctx, startKey, endKey, limit)
	if len(re.Items) == backend.DefaultRangeLimit {
		c.Warnf("Range query hit backend limit. (this is a bug!) startKey=%q,limit=%d", startKey, backend.DefaultRangeLimit)
	}
	return &re, nil
}

// CompareAndSwap compares item with existing item and replaces it with replaceWith item
func (c *UnifiedResourceCache) CompareAndSwap(ctx context.Context, expected Item, replaceWith Item) error {
	if len(expected.Key) == 0 {
		return trace.BadParameter("missing parameter Key")
	}
	if len(replaceWith.Key) == 0 {
		return trace.BadParameter("missing parameter Key")
	}
	if !bytes.Equal(expected.Key, replaceWith.Key) {
		return trace.BadParameter("expected and replaceWith keys should match")
	}
	c.Lock()
	defer c.Unlock()
	i, found := c.tree.Get(&btreeItem{Item: expected})
	if !found {
		return trace.CompareFailed("key %q is not found", string(expected.Key))
	}
	existingItem := i.Item
	if existingItem.Value != expected.Value {
		return trace.CompareFailed("current value does not match expected for %v", string(expected.Key))
	}
	event := Event{
		Type: types.OpPut,
		Item: replaceWith,
	}
	c.processEvent(event)
	return nil
}

type GetResult struct {
	Items []Item
}

func (c *UnifiedResourceCache) getRange(ctx context.Context, startKey, endKey []byte, limit int) GetResult {
	var res GetResult
	c.tree.AscendRange(&btreeItem{Item: Item{Key: startKey}}, &btreeItem{Item: Item{Key: endKey}}, func(item *btreeItem) bool {
		res.Items = append(res.Items, item.Item)
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
		item := &btreeItem{Item: event.Item, index: -1}
		c.tree.ReplaceOrInsert(item)
	case types.OpDelete:
		item, found := c.tree.Get(&btreeItem{Item: event.Item})
		if !found {
			return
		}
		c.tree.Delete(item)
	default:
		// skip unsupported record
	}
}

type UnifiedResourceWatcherConfig struct {
	ResourceWatcherConfig
	NodesGetter
	DatabaseServersGetter
	AppServersGetter
	WindowsDesktopGetter
	KubernetesClusterGetter
}

type UnifiedResourceWatcher struct {
	*resourceWatcher
	*unifiedResourceCollector
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree
func (u *UnifiedResourceWatcher) GetUnifiedResources(ctx context.Context, namespace string) ([]types.ResourceWithLabels, error) {
	result, err := u.current.GetRange(ctx, backend.Key(prefix), backend.RangeEnd(backend.Key(prefix)), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resources []types.ResourceWithLabels

	for _, item := range result.Items {
		resources = append(resources, item.Value)
	}

	return resources, nil
}

type unifiedResourceCollector struct {
	UnifiedResourceWatcherConfig
	current         *UnifiedResourceCache
	lock            sync.RWMutex
	initializationC chan struct{}
	once            sync.Once
}

func NewUnifiedResourceWatcher(ctx context.Context, cfg UnifiedResourceWatcherConfig) (*UnifiedResourceWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	mem, err := NewUnifiedResourceCache(config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	collector := &unifiedResourceCollector{
		UnifiedResourceWatcherConfig: cfg,
		current:                      mem,
		initializationC:              make(chan struct{}),
	}

	watcher, err := newResourceWatcher(ctx, collector, cfg.ResourceWatcherConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UnifiedResourceWatcher{
		resourceWatcher:          watcher,
		unifiedResourceCollector: collector,
	}, nil
}

func (u *unifiedResourceCollector) getResourcesAndUpdateCurrent(ctx context.Context) error {
	err := u.getAndUpdateNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateApps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.getAndUpdateDesktops(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	u.defineCollectorAsInitialized()
	return nil
}

// getAndUpdateNodes will get nodes and update the current tree with each Node
func (u *unifiedResourceCollector) getAndUpdateNodes(ctx context.Context) error {
	newNodes, err := u.NodesGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	nodes := make([]Item, 0)
	for _, node := range newNodes {
		nodes = append(nodes, Item{
			Key:   backend.Key(prefix, node.GetNamespace(), node.GetName(), types.KindNode),
			Value: node,
		})
	}
	return u.current.PutRange(ctx, nodes)
}

// getAndUpdateDatabases will get database servers and update the current tree with each DatabaseServer
func (u *unifiedResourceCollector) getAndUpdateDatabases(ctx context.Context) error {
	newDbs, err := u.DatabaseServersGetter.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}
	dbs := make([]Item, 0)
	for _, db := range newDbs {
		dbs = append(dbs, Item{
			Key:   backend.Key(prefix, db.GetNamespace(), db.GetName(), types.KindDatabaseServer),
			Value: db,
		})
	}
	return u.current.PutRange(ctx, dbs)
}

// getAndUpdateApps will get application servers and update the current tree with each AppServer
func (u *unifiedResourceCollector) getAndUpdateApps(ctx context.Context) error {
	newApps, err := u.AppServersGetter.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	apps := make([]Item, 0)
	for _, app := range newApps {
		apps = append(apps, Item{
			Key:   backend.Key(prefix, app.GetNamespace(), app.GetName(), types.KindApp),
			Value: app,
		})
	}
	return u.current.PutRange(ctx, apps)
}

// getAndUpdateDesktops will get windows desktops and update the current tree with each Desktop
func (u *unifiedResourceCollector) getAndUpdateDesktops(ctx context.Context) error {
	newDesktops, err := u.WindowsDesktopGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	desktops := make([]Item, 0)
	for _, desktop := range newDesktops {
		desktops = append(desktops, Item{
			Key:   backend.Key(prefix, desktop.GetMetadata().Namespace, desktop.GetName(), types.KindWindowsDesktop),
			Value: desktop,
		})
	}

	return u.current.PutRange(ctx, desktops)
}

func (u *unifiedResourceCollector) notifyStale() {}

func (u *unifiedResourceCollector) initializationChan() <-chan struct{} {
	return u.initializationC
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
		u.current.Delete(ctx, backend.Key(prefix, event.Resource.GetMetadata().Namespace, event.Resource.GetName(), event.Resource.GetKind()))
	case types.OpPut:
		u.current.Put(ctx, Item{
			Key:   backend.Key(prefix, event.Resource.GetMetadata().Namespace, event.Resource.GetName(), event.Resource.GetKind()),
			Value: event.Resource.(types.ResourceWithLabels),
		})
	default:
		u.Log.Warnf("unsupported event type %s.", event.Type)
		return
	}
}

func (u *unifiedResourceCollector) resourceKind() []types.WatchKind {
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindWindowsDesktop},
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
