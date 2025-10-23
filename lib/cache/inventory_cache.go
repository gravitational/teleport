// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	// instancePrefix is the backend prefix for teleport instances.
	instancePrefix = "instances"

	// botInstancePrefix is the backend prefix for bot instances.
	botInstancePrefix = "bot_instance"
)

type inventoryIndex string

const (
	// inventoryAlphabeticalIndex sorts instances alphabetically.
	// Format: <bot name or instance hostname>/<instance id>/<bot_instance|instance>
	inventoryAlphabeticalIndex inventoryIndex = "alphabetical"

	// inventoryTypeIndex groups instances by type.
	// Format: <bot_instance|instance>/<bot name or instance hostname>/<instance id>
	inventoryTypeIndex inventoryIndex = "type"

	// inventoryIDIndex allows lookup by instance ID.
	// Format: <instance id>
	inventoryIDIndex inventoryIndex = "id"
)

// unifiedInstance is a wrapper for either a teleport instance or a bot instance.
type unifiedInstance struct {
	instance *types.InstanceV1
	bot      *machineidv1.BotInstance
}

// isInstance returns true if this wrapper contains a teleport instance (not a bot instance).
func (u *unifiedInstance) isInstance() bool {
	return u.instance != nil
}

// getInstanceID returns the ID of this instance.
func (u *unifiedInstance) getInstanceID() string {
	if u.isInstance() {
		return u.instance.GetName()
	}
	return u.bot.Metadata.Name
}

// getHostnameOrBotName returns the friendly name of this instance.
// For instances, this is the hostname (or instance ID if there is no hostname).
// For bot instances, this is the bot name.
func (u *unifiedInstance) getHostnameOrBotName() string {
	if u.isInstance() {
		if u.instance.Spec.Hostname != "" {
			return u.instance.Spec.Hostname
		}
		// If no hostname, fall back to the instance ID
		return u.instance.GetName()
	}
	return u.bot.Spec.BotName
}

// getKind returns the resource kind for this instance.
func (u *unifiedInstance) getKind() string {
	if u.isInstance() {
		return types.KindInstance
	}
	return types.KindBotInstance
}

// getAlphabeticalKey returns the composite key for alphabetical sorting.
// Format: <bot name or hostname>/<instance id>/<kind>
func (u *unifiedInstance) getAlphabeticalKey() string {
	return u.getHostnameOrBotName() + "/" + u.getInstanceID() + "/" + u.getKind()
}

// getTypeKey returns the composite key for sorting by type.
// Format: <kind>/<bot name or hostname>/<instance id>
func (u *unifiedInstance) getTypeKey() string {
	return u.getKind() + "/" + u.getHostnameOrBotName() + "/" + u.getInstanceID()
}

// getIDKey returns the key for lookup by instance ID.
// Format: <instance id>
func (u *unifiedInstance) getIDKey() string {
	return u.getInstanceID()
}

// clone returns a deep copy of the unified instance.
func (u *unifiedInstance) clone() *unifiedInstance {
	if u.isInstance() {
		copied := *u.instance
		return &unifiedInstance{instance: &copied}
	}
	return &unifiedInstance{bot: proto.Clone(u.bot).(*machineidv1.BotInstance)}
}

// InventoryCacheConfig holds the configuration parameters for the InventoryCache.
type InventoryCacheConfig struct {
	// PrimaryCache is Teleport's primary cache.
	PrimaryCache *Cache

	Backend backend.Backend

	// Inventory is the inventory service.
	Inventory services.Inventory

	// BotInstanceCache is the service for reading bot instances.
	BotInstanceCache services.BotInstance

	// TargetVersion is the target Teleport version for the cluster.
	TargetVersion string

	Clock  clockwork.Clock
	Logger *slog.Logger
}

func (c *InventoryCacheConfig) CheckAndSetDefaults() error {
	if c.PrimaryCache == nil {
		return trace.BadParameter("missing PrimaryCache")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if c.Inventory == nil {
		return trace.BadParameter("missing Inventory")
	}
	if c.BotInstanceCache == nil {
		return trace.BadParameter("missing BotInstanceCache")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	return nil
}

// InventoryCache is the cache for teleport and bot instances.
type InventoryCache struct {
	mu sync.RWMutex
	// healthy is whether the cache is healthy and ready to serve requests.
	healthy atomic.Bool
	// initDone is a channel used to ensure clean shutdowns.
	initDone chan struct{}

	cfg InventoryCacheConfig

	ctx    context.Context
	cancel context.CancelFunc

	// store is the unified sortcache that holds both teleport and bot instances.
	store *store[*unifiedInstance, inventoryIndex]
}

func NewInventoryCache(cfg InventoryCacheConfig) (*InventoryCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ic := &InventoryCache{
		cfg: cfg,

		// Create the sortcache
		store: newStore(
			"unified_instance",
			func(u *unifiedInstance) *unifiedInstance {
				return u.clone()
			},
			map[inventoryIndex]func(*unifiedInstance) string{
				inventoryAlphabeticalIndex: func(u *unifiedInstance) string {
					return u.getAlphabeticalKey()
				},
				inventoryTypeIndex: func(u *unifiedInstance) string {
					return u.getTypeKey()
				},
				inventoryIDIndex: func(u *unifiedInstance) string {
					return u.getIDKey()
				},
			},
		),

		// Create a channel that will close when the initialization is done.
		initDone: make(chan struct{}),

		ctx:    ctx,
		cancel: cancel,
	}

	go ic.initialize()

	return ic, nil
}

// IsHealthy returns true if the cache is healthy and initialized.
func (ic *InventoryCache) IsHealthy() bool {
	return ic.healthy.Load()
}

func (ic *InventoryCache) Close() error {
	ic.cancel()
	// Wait for initDone channel to finish so we can close gracefully.
	<-ic.initDone
	return nil
}

// calculateReadsPerSecond calculates the rate limit to use for backend reads based on cluster size.
// With this implementation, these are some of the expected rate limits and corresponding total times based on cluster size:
//
// Cluster size | Reads per second | Total time to finish all reads
// -------------|------------------|-------------------------------
// 500          | 283              | 1.77s
// 1,000        | 298              | 3.36s
// 2,000        | 322              | 6.21s
// 4,000        | 363              | 11.02s
// 8,000        | 433              | 18.47s
// 32,000       | 789              | 40.56s
// 64,000       | 1219             | 52.5s
// 128,000      | 2035             | 1m03s
// 256,000      | 3605             | 1m11s
func calculateReadsPerSecond(clusterSize int) int {
	// minimumComponent is the minimum value of reads per second we never want to drop below.
	const minimumComponent = 256

	// linearComponent ensures we stay under a worst-case upper bound init time of 90s.
	linearComponent := clusterSize / 90

	// subLinearComponent ensures that growth is sub-linear across most reasonable cluster sizes.
	subLinearComponent := int(math.Sqrt(float64(clusterSize)))

	return minimumComponent + linearComponent + subLinearComponent
}

// initialize initializes the inventory cache.
func (ic *InventoryCache) initialize() {
	defer close(ic.initDone)

	// Wait for primary cache to be ready.
	if err := ic.waitForPrimaryCacheInit(); err != nil {
		ic.cfg.Logger.ErrorContext(ic.ctx, "Failed to wait for primary cache initialization", "error", err)
		return
	}

	// Setup the backend watcher.
	watcher, err := ic.setupWatcher()
	if err != nil {
		ic.cfg.Logger.ErrorContext(ic.ctx, "Failed to set up backend watcher", "error", err)
		return
	}
	defer watcher.Close()

	// Wait for the watcher to be ready.
	if err := ic.waitForWatcherInit(watcher); err != nil {
		ic.cfg.Logger.ErrorContext(ic.ctx, "Failed to wait for watcher init", "error", err)
		return
	}

	// Calculate the rate limit to use.
	primaryCacheSize := ic.getPrimaryCacheSize()
	readsPerSecond := calculateReadsPerSecond(primaryCacheSize)

	// Populate the cache with teleport instance and bot instances.
	if err := ic.populateCache(readsPerSecond); err != nil {
		ic.cfg.Logger.ErrorContext(ic.ctx, "Failed to populate cache", "error", err)
		return
	}

	// Mark cache as healthy.
	ic.healthy.Store(true)

	ic.processEvents(watcher)
}

// waitForPrimaryCacheInit waits for the primary cache to be initialized.
func (ic *InventoryCache) waitForPrimaryCacheInit() error {
	ticker := ic.cfg.Clock.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ic.ctx.Done():
			return trace.Wrap(ic.ctx.Err())

		case <-ticker.Chan():
			ic.cfg.PrimaryCache.rw.RLock()
			ok := ic.cfg.PrimaryCache.ok
			ic.cfg.PrimaryCache.rw.RUnlock()

			if ok {
				return nil
			}
		}
	}
}

// setupWatcher sets up a backend watcher for instance and bot_instance events.
func (ic *InventoryCache) setupWatcher() (backend.Watcher, error) {
	watcher, err := ic.cfg.Backend.NewWatcher(ic.ctx, backend.Watch{
		Name: "inventory_cache",
		Prefixes: []backend.Key{
			backend.NewKey(instancePrefix),
			backend.NewKey(botInstancePrefix),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return watcher, nil
}

// waitForWatcherInit waits for the watcher to finish initializing.
func (ic *InventoryCache) waitForWatcherInit(watcher backend.Watcher) error {
	select {
	case <-ic.ctx.Done():
		// Context was cancelled
		return trace.Wrap(ic.ctx.Err())

	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected OpInit event, got %v", event.Type)
		}
		return nil
	}
}

// getPrimaryCacheSize returns the size of the primary cache, based on the number of agents.
func (ic *InventoryCache) getPrimaryCacheSize() int {
	count := 0
	if ic.cfg.PrimaryCache.collections.nodes != nil {
		count += ic.cfg.PrimaryCache.collections.nodes.store.len()
	}
	if ic.cfg.PrimaryCache.collections.apps != nil {
		count += ic.cfg.PrimaryCache.collections.apps.store.len()
	}
	if ic.cfg.PrimaryCache.collections.dbs != nil {
		count += ic.cfg.PrimaryCache.collections.dbs.store.len()
	}
	if ic.cfg.PrimaryCache.collections.kubeClusters != nil {
		count += ic.cfg.PrimaryCache.collections.kubeClusters.store.len()
	}
	if ic.cfg.PrimaryCache.collections.windowsDesktops != nil {
		count += ic.cfg.PrimaryCache.collections.windowsDesktops.store.len()
	}
	return count
}

// populateCache reads teleport and bot instances and populates the cache with rate limiting.
func (ic *InventoryCache) populateCache(readsPerSecond int) error {
	limiter := rate.NewLimiter(rate.Limit(readsPerSecond), readsPerSecond)

	if err := ic.populateInstances(limiter); err != nil {
		return trace.Wrap(err)
	}

	if err := ic.populateBotInstances(limiter); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// populateInstances reads teleport instances from the inventory service with rate limiting.
func (ic *InventoryCache) populateInstances(limiter *rate.Limiter) error {
	instanceStream := ic.cfg.Inventory.GetInstances(ic.ctx, types.InstanceFilter{})

	for instanceStream.Next() {
		if err := limiter.Wait(ic.ctx); err != nil {
			return trace.Wrap(err)
		}

		instance := instanceStream.Item()

		instanceV1, ok := instance.(*types.InstanceV1)
		if !ok {
			ic.cfg.Logger.WarnContext(ic.ctx, "Instance is not InstanceV1", "instance", instance.GetName())
			continue
		}

		// Add it to the cache
		ui := &unifiedInstance{instance: instanceV1}
		if err := ic.store.put(ui); err != nil {
			ic.cfg.Logger.WarnContext(ic.ctx, "Failed to add instance to cache", "instance", instanceV1.GetName(), "error", err)
			continue
		}
	}

	return trace.Wrap(instanceStream.Done())
}

// populateBotInstances reads bot instances from the bot instance service with rate limiting.
func (ic *InventoryCache) populateBotInstances(limiter *rate.Limiter) error {
	var pageToken string

	for {
		if err := limiter.Wait(ic.ctx); err != nil {
			return trace.Wrap(err)
		}

		botInstances, nextToken, err := ic.cfg.BotInstanceCache.ListBotInstances(
			ic.ctx,
			defaults.DefaultChunkSize,
			pageToken,
			nil,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, botInstance := range botInstances {
			// Add it to the cache
			ui := &unifiedInstance{bot: botInstance}
			if err := ic.store.put(ui); err != nil {
				ic.cfg.Logger.WarnContext(ic.ctx, "Failed to add bot instance to cache", "bot_instance", botInstance.Metadata.Name, "error", err)
				continue
			}
		}

		if nextToken == "" {
			break
		}

		pageToken = nextToken
	}

	return nil
}

// processEvents processes events from the backend watcher.
func (ic *InventoryCache) processEvents(watcher backend.Watcher) {
	for {
		select {
		case <-ic.ctx.Done():
			return

		case event := <-watcher.Events():
			if err := ic.processEvent(event); err != nil {
				ic.cfg.Logger.WarnContext(ic.ctx, "Failed to process event", "error", err)
			}
		}
	}
}

// processEvent processes a backend event.
func (ic *InventoryCache) processEvent(event backend.Event) error {
	switch event.Type {
	case types.OpPut:
		return ic.processPutEvent(event)
	case types.OpDelete:
		return ic.processDeleteEvent(event)
	default:
		// Unknown event type
		return nil
	}
}

// processPutEvent processes an OpPut event.
func (ic *InventoryCache) processPutEvent(event backend.Event) error {
	instanceKeyPrefix := backend.NewKey(instancePrefix).String()
	botInstanceKeyPrefix := backend.NewKey(botInstancePrefix).String()

	// If this is a teleport instance
	if strings.HasPrefix(event.Item.Key.String(), instanceKeyPrefix) {
		instance := &types.InstanceV1{}
		if err := instance.Unmarshal(event.Item.Value); err != nil {
			return trace.Wrap(err)
		}

		// Add/update it in the cache
		ui := &unifiedInstance{instance: instance}
		if err := ic.store.put(ui); err != nil {
			return trace.Wrap(err)
		}
	}

	// If this is a bot instance
	if strings.HasPrefix(event.Item.Key.String(), botInstanceKeyPrefix) {
		botInstance := &machineidv1.BotInstance{}
		if err := proto.Unmarshal(event.Item.Value, botInstance); err != nil {
			return trace.Wrap(err)
		}

		// Add/update it in the cache
		ui := &unifiedInstance{bot: botInstance}
		if err := ic.store.put(ui); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// processDeleteEvent handles OpDelete events.
func (ic *InventoryCache) processDeleteEvent(event backend.Event) error {
	instanceKeyPrefix := backend.NewKey(instancePrefix).String()
	botInstanceKeyPrefix := backend.NewKey(botInstancePrefix).String()

	// If this is a teleport instance
	if strings.HasPrefix(event.Item.Key.String(), instanceKeyPrefix) {
		// Extract the instance id from the key
		keyParts := strings.Split(event.Item.Key.String(), "/")
		if len(keyParts) == 0 {
			return trace.BadParameter("invalid instance key: %s", event.Item.Key)
		}
		instanceID := keyParts[len(keyParts)-1]

		// Find and remove the instance from the cache.
		if existing, err := ic.store.get(inventoryIDIndex, instanceID); err == nil {
			ic.store.delete(existing)
		}
	}

	// If this is a bot instance.
	if strings.HasPrefix(event.Item.Key.String(), botInstanceKeyPrefix) {
		// Extract the instance id from the key
		keyParts := strings.Split(event.Item.Key.String(), "/")
		if len(keyParts) == 0 {
			return trace.BadParameter("invalid bot instance key: %s", event.Item.Key)
		}
		botInstanceID := keyParts[len(keyParts)-1]

		// Find and remove the bot instance from the cache
		if existing, err := ic.store.get(inventoryIDIndex, botInstanceID); err == nil {
			ic.store.delete(existing)
		}
	}

	return nil
}

// ListUnifiedInstances returns a page of instances and bot_instances. This API will refuse any requests when the cache is unhealthy or not yet
// fully initialized.
func (ic *InventoryCache) ListUnifiedInstances(ctx context.Context, req *inventoryv1.ListUnifiedInstancesRequest) (*inventoryv1.ListUnifiedInstancesResponse, error) {
	if !ic.IsHealthy() {
		return nil, trace.ConnectionProblem(nil, "inventory cache is not yet healthy")
	}

	if req.PageSize <= 0 {
		req.PageSize = defaults.DefaultChunkSize
	}

	startKey := req.PageToken
	if startKey == "" {
		// If no kinds filter is specified or multiple kinds are, start from the beginning.
		// If we're only filtering for 1 kind, use type index with kind prefix.
		if req.Filter != nil && len(req.Filter.Kinds) == 1 {
			kind := req.Filter.Kinds[0]
			startKey = kind + "/"
		}
	}

	var items []*inventoryv1.UnifiedInstanceItem
	var nextPageToken string

	index := inventoryAlphabeticalIndex
	// Determine if we should use the type index.
	useTypeIndex := req.Filter != nil && len(req.Filter.Kinds) == 1
	if useTypeIndex {
		index = inventoryTypeIndex
	}

	endKey := ""
	// Determine the endKey for type filtering
	if useTypeIndex {
		kind := req.Filter.Kinds[0]
		endKey = kind + "0"
	}

	for sf := range ic.store.cache.Ascend(index, startKey, endKey) {
		if !ic.matchesFilter(sf, req.Filter) {
			continue
		}

		if len(items) == int(req.PageSize) {
			if index == inventoryAlphabeticalIndex {
				nextPageToken = sf.getAlphabeticalKey()
			} else {
				nextPageToken = sf.getTypeKey()
			}
			break
		}

		item := ic.unifiedInstanceToProto(sf)
		items = append(items, item)
	}

	return &inventoryv1.ListUnifiedInstancesResponse{
		Items:         items,
		NextPageToken: nextPageToken,
	}, nil
}

// matchesFilter checks if a unified instance matches the filter criteria.
func (ic *InventoryCache) matchesFilter(_ *unifiedInstance, _ *inventoryv1.ListUnifiedInstancesFilter) bool {
	// TODO(rudream): implement filtering for listing instances.
	return true
}

// unifiedInstanceToProto converts a unified instance to a proto UnifiedInstanceItem.
func (ic *InventoryCache) unifiedInstanceToProto(ui *unifiedInstance) *inventoryv1.UnifiedInstanceItem {
	if ui.isInstance() {
		return &inventoryv1.UnifiedInstanceItem{
			Item: &inventoryv1.UnifiedInstanceItem_Instance{
				Instance: ui.instance,
			},
		}
	}
	return &inventoryv1.UnifiedInstanceItem{
		Item: &inventoryv1.UnifiedInstanceItem_BotInstance{
			BotInstance: ui.bot,
		},
	}
}
