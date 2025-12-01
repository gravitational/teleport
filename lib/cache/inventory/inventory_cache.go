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

package inventory

import (
	"context"
	"encoding/base32"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charlievieth/strcase"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/defaults"
	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/expression"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/sortcache"
	"github.com/gravitational/teleport/lib/utils/typical"
)

const (
	// instancePrefix is the backend prefix for teleport instances.
	instancePrefix = "instances"

	// botInstancePrefix is the backend prefix for bot instances.
	botInstancePrefix = "bot_instance"
)

var (
	// unifiedExpressionParser is a cached unified expression parser
	unifiedExpressionParser     *typical.Parser[*unifiedFilterEnvironment, bool]
	unifiedExpressionParserOnce sync.Once
)

// instanceTypeToKind converts an InstanceType enum to a resource kind string.
func instanceTypeToKind(instanceType inventoryv1.InstanceType) (string, error) {
	switch instanceType {
	case inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE:
		return types.KindInstance, nil
	case inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE:
		return types.KindBotInstance, nil
	default:
		return "", trace.BadParameter("unknown instance type: %v", instanceType)
	}
}

// bytestring contains binary data (encoded keys) and is not intended to be printable.
type bytestring = string

type inventoryIndex string

const (
	// inventoryAlphabeticalIndex sorts instances by display name (bot name
	// or instance hostname), unique ID (bot instance ID or instance host ID)
	// and type ("bot" or "instance").
	inventoryAlphabeticalIndex inventoryIndex = "alphabetical"

	// inventoryTypeIndex sorts instances by type, display name
	// and unique ID.
	inventoryTypeIndex inventoryIndex = "type"

	// inventoryVersionIndex sorts instances by version, display name
	// and unique ID.
	inventoryVersionIndex inventoryIndex = "version"

	// inventoryIDIndex allows lookup by instance ID.
	// Uses ordered.Encode(bot name, instance ID, kind) where the bot name is "" for regular instances
	inventoryIDIndex inventoryIndex = "id"
)

// inventoryInstance is a wrapper for either a teleport instance or a bot instance.
type inventoryInstance struct {
	instance *types.InstanceV1
	bot      *machineidv1.BotInstance
}

// isInstance returns true if this wrapper contains a teleport instance (not a bot instance).
func (u *inventoryInstance) isInstance() bool {
	return u.instance != nil
}

// getInstanceID returns a unique ID for this instance.
// For instances, this is the instance ID. For bot instances, this is the bot instance ID
func (u *inventoryInstance) getInstanceID() string {
	if u.isInstance() {
		return u.instance.GetName()
	}
	return u.bot.GetSpec().GetInstanceId()
}

// getBotName returns the bot name for bot instances, or an empty string for regular instances.
func (u *inventoryInstance) getBotName() string {
	if u.isInstance() {
		return ""
	}
	return u.bot.GetSpec().GetBotName()
}

// getKind returns the resource kind for this instance.
func (u *inventoryInstance) getKind() string {
	if u.isInstance() {
		return types.KindInstance
	}
	return types.KindBotInstance
}

// getAlphabeticalKey returns the composite key for alphabetical sorting.
func (u *inventoryInstance) getAlphabeticalKey() bytestring {
	var name, id string
	if u.isInstance() {
		name = u.instance.GetHostname()
		id = u.instance.GetName()
	} else {
		name = u.bot.GetSpec().GetBotName()
		id = u.bot.GetSpec().GetInstanceId()
	}

	return bytestring(ordered.Encode(name, id, u.getKind()))
}

// getTypeKey returns the composite key for sorting by type.
func (u *inventoryInstance) getTypeKey() bytestring {
	var name, id string
	if u.isInstance() {
		name = u.instance.GetHostname()
		id = u.instance.GetName()
	} else {
		name = u.bot.GetSpec().GetBotName()
		id = u.bot.GetSpec().GetInstanceId()
	}

	return bytestring(ordered.Encode(u.getKind(), name, id))
}

// getVersionKey returns the composite key for sorting by version using semver ordering.
func (u *inventoryInstance) getVersionKey() bytestring {
	var versionStr, name, id string
	if u.isInstance() {
		versionStr = u.instance.Spec.Version
		name = u.instance.GetHostname()
		id = u.instance.GetName()
	} else {
		if u.bot.Status != nil && len(u.bot.Status.LatestHeartbeats) > 0 {
			versionStr = u.bot.Status.LatestHeartbeats[0].Version
		}
		name = u.bot.GetSpec().GetBotName()
		id = u.bot.GetSpec().GetInstanceId()
	}

	// If version is empty, treat it as 0.0.0
	if versionStr == "" {
		return bytestring(ordered.Encode(uint64(0), uint64(0), uint64(0), ordered.Inf, name, id))
	}

	// Strip "v" prefix if it's there, eg. v1.2.3 -> 1.2.3
	versionStr = strings.TrimPrefix(versionStr, "v")

	// Parse as semver
	v, err := semver.NewVersion(versionStr)
	if err != nil {
		// Invalid semvers sort after all other versions
		return bytestring(ordered.Encode(ordered.Inf, versionStr, name, id))
	}

	// For releases (ie. anything not a pre-release), we use ordered.Inf before the metadata to ensure it sorts after all prereleases
	if v.PreRelease == "" {
		return bytestring(ordered.Encode(v.Major, v.Minor, v.Patch, ordered.Inf, v.Metadata, name, id))
	}

	// For pre-releases

	parts := strings.Split(string(v.PreRelease), ".")

	// Pre-allocate the slice
	// The 3 is for major, minor, patch
	// The len(parts)*2 is for pre-release parts (each one needs 2 for tag+value)
	// The 4 is for the sentinel value, metadata, name, id
	encodeArgs := make([]any, 0, 3+len(parts)*2+4)
	encodeArgs = append(encodeArgs, v.Major, v.Minor, v.Patch)

	// We encode each pre-release part with a tag to ensure correct ordering:
	// We use 0 for numeric parts (so that they always sort before tag 1)
	// We use 1 for alphanumeric parts (so that they always sort after tag 0, but before ordered.Inf which we use for releases)
	for _, part := range parts {
		// Check if the part is numeric or alphanumeric
		if num, err := strconv.ParseUint(part, 10, 64); err == nil {
			encodeArgs = append(encodeArgs, uint64(0), num)
		} else {
			encodeArgs = append(encodeArgs, uint64(1), part)
		}
	}

	// We use "" as sentinel value to mark the "end" of pre-releases
	// This way we can make sure that for example, "alpha" < "alpha.1"
	// "alpha" would be [..., 1, "alpha", ""] and "alpha.1" would be [..., 1, "alpha", 0, 1, ""]
	// Since it'll compare "" < 0, "alpha" will come first
	encodeArgs = append(encodeArgs, "")

	// Metadata eg. "+build123" doesn't affect semver ordering, but we add it to ensure that
	// two versions that differ only in metadata have a consistent order
	encodeArgs = append(encodeArgs, v.Metadata)

	encodeArgs = append(encodeArgs, name, id)

	return bytestring(ordered.Encode(encodeArgs...))
}

// getIDKey returns the key for lookup by instance ID.
// We use ordered encoding with (bot name, instance ID, kind) to ensure uniqueness and safe lexicographic ordering.
// For instances this is ordered.Encode("", instance ID, "instance")
// For bot instances this is ordered.Encode(bot name, instance ID, "bot_instance")
func (u *inventoryInstance) getIDKey() bytestring {
	return bytestring(ordered.Encode(u.getBotName(), u.getInstanceID(), u.getKind()))
}

// InventoryCacheConfig holds the configuration parameters for the InventoryCache.
type InventoryCacheConfig struct {
	// PrimaryCache is Teleport's primary cache.
	PrimaryCache *cache.Cache

	// Events is the events service for watching backend events.
	Events types.Events

	// Inventory is the inventory service.
	Inventory services.Inventory

	// BotInstanceBackend is the backend service for reading bot instances.
	// This must be the backend and not a cache since the watcher is from the backend,
	// so the OpInit event might refer to a "time" after the current "time" of a cache, which could cause
	// us to miss items that are not yet in the cache but were already written in the backend.
	BotInstanceBackend services.BotInstance

	// TargetVersion is the target Teleport version for the cluster.
	TargetVersion string

	Logger *slog.Logger
}

func (c *InventoryCacheConfig) CheckAndSetDefaults() error {
	if c.PrimaryCache == nil {
		return trace.BadParameter("missing PrimaryCache")
	}
	if c.Events == nil {
		return trace.BadParameter("missing Events")
	}
	if c.Inventory == nil {
		return trace.BadParameter("missing Inventory")
	}
	if c.BotInstanceBackend == nil {
		return trace.BadParameter("missing BotInstanceBackend")
	}

	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	return nil
}

// InventoryCache is the cache for teleport and bot instances.
type InventoryCache struct {
	// healthy is whether the cache is healthy and ready to serve requests.
	healthy atomic.Bool
	// done is a channel used to ensure clean shutdowns.
	done chan struct{}

	cfg InventoryCacheConfig

	ctx    context.Context
	cancel context.CancelFunc

	// cache is the unified sortcache that holds both teleport and bot instances.
	cache *sortcache.SortCache[*inventoryInstance, inventoryIndex]
}

func NewInventoryCache(cfg InventoryCacheConfig) (*InventoryCache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ic := &InventoryCache{
		cfg: cfg,

		// Create the sortcache
		cache: sortcache.New(sortcache.Config[*inventoryInstance, inventoryIndex]{
			Indexes: map[inventoryIndex]func(*inventoryInstance) string{
				inventoryAlphabeticalIndex: (*inventoryInstance).getAlphabeticalKey,
				inventoryTypeIndex:         (*inventoryInstance).getTypeKey,
				inventoryVersionIndex:      (*inventoryInstance).getVersionKey,
				inventoryIDIndex:           (*inventoryInstance).getIDKey,
			},
		}),

		// Create a channel that will close when the initialization is done.
		done: make(chan struct{}),

		ctx:    ctx,
		cancel: cancel,
	}

	go func() {
		defer close(ic.done)
		ic.initializeAndWatchWithRetry(ctx)
	}()

	return ic, nil
}

// IsHealthy returns true if the cache is healthy and initialized.
func (ic *InventoryCache) IsHealthy() bool {
	return ic.healthy.Load()
}

func (ic *InventoryCache) Close() error {
	ic.cancel()
	// Wait for done channel to finish so we can close gracefully.
	<-ic.done
	return nil
}

// calculateReadsPerSecond calculates the rate limit to use for backend reads based on cluster size.
// The curve is intentionalled capped to stay below the 90s watcher grace period even in extremely large clusters.
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

// initializeAndWatchWithRetry runs initializeAndWatch with a retry every 10 seconds if it fails.
func (ic *InventoryCache) initializeAndWatchWithRetry(ctx context.Context) {
	const retryInterval = 10 * time.Second

	for {
		ic.cfg.Logger.DebugContext(ctx, "Attempting to initialize inventory cache")

		// Attempt to initialize and watch
		err := ic.initializeAndWatch(ctx)
		if ctx.Err() != nil {
			ic.cfg.Logger.DebugContext(ctx, "Exiting from inventory cache watch loop because context was canceled")
			return
		}

		ic.cfg.Logger.WarnContext(ctx, "Failed to initialize inventory cache, retrying in 10 seconds",
			"error", err)

		// Wait before retrying
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}
	}
}

// initializeAndWatch initializes the inventory cache and begins watching for instance and bot_instance backend events.
func (ic *InventoryCache) initializeAndWatch(ctx context.Context) error {
	// Wait for primary cache to be ready.
	if err := ic.waitForPrimaryCacheInit(ctx); err != nil {
		return trace.Wrap(err, "Failed to wait for primary cache init")
	}

	// Setup the backend watcher.
	watcher, err := ic.setupWatcher(ctx)
	if err != nil {
		return trace.Wrap(err, "Failed to set up backend watcher")
	}
	defer watcher.Close()

	// Wait for the watcher to be ready.
	if err := ic.waitForWatcherInit(ctx, watcher); err != nil {
		return trace.Wrap(err, "Failed to wait for watcher init")
	}

	// Calculate the rate limit to use.
	primaryCacheSize := ic.cfg.PrimaryCache.GetUnifiedResourcesAndBotsCount()
	readsPerSecond := calculateReadsPerSecond(primaryCacheSize)

	// Populate the cache with teleport instance and bot instances.
	if err := ic.populateCache(ctx, readsPerSecond); err != nil {
		return trace.Wrap(err, "failed to populate inventory cache")
	}

	// Mark cache as healthy.
	ic.healthy.Store(true)
	ic.cfg.Logger.InfoContext(ctx, "Inventory cache init succeeded")

	// This runs infinitely until the context is canceled.
	ic.processEvents(ctx, watcher)

	return ctx.Err()
}

// waitForPrimaryCacheInit waits for the primary cache to be initialized.
func (ic *InventoryCache) waitForPrimaryCacheInit(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-ic.cfg.PrimaryCache.FirstInit():
		return nil
	}
}

// setupWatcher sets up a watcher for instance and bot_instance events.
func (ic *InventoryCache) setupWatcher(ctx context.Context) (types.Watcher, error) {
	watcher, err := ic.cfg.Events.NewWatcher(ctx, types.Watch{
		Name: "inventory_cache",
		Kinds: []types.WatchKind{
			{Kind: types.KindInstance},
			{Kind: types.KindBotInstance},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return watcher, nil
}

// waitForWatcherInit waits for the watcher to finish initializing.
func (ic *InventoryCache) waitForWatcherInit(ctx context.Context, watcher types.Watcher) error {
	select {
	case <-ctx.Done():
		// Context was canceled
		return trace.Wrap(ctx.Err())

	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected OpInit event, got %v", event.Type)
		}
		return nil
	}
}

// populateCache reads teleport and bot instances and populates the cache with rate limiting.
func (ic *InventoryCache) populateCache(ctx context.Context, readsPerSecond int) error {
	limiter := rate.NewLimiter(rate.Limit(readsPerSecond), readsPerSecond)

	if err := ic.populateInstances(ctx, limiter); err != nil {
		return trace.Wrap(err)
	}

	if err := ic.populateBotInstances(ctx, limiter); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// populateInstances reads teleport instances from the inventory service with rate limiting.
func (ic *InventoryCache) populateInstances(ctx context.Context, limiter *rate.Limiter) error {
	instanceStream := ic.cfg.Inventory.GetInstances(ctx, types.InstanceFilter{})

	for instanceStream.Next() {
		if err := limiter.Wait(ctx); err != nil {
			return trace.Wrap(err)
		}

		instance := instanceStream.Item()

		instanceV1, ok := instance.(*types.InstanceV1)
		if !ok {
			ic.cfg.Logger.WarnContext(ctx, "Instance is not InstanceV1", "instance", instance.GetName())
			continue
		}

		// Add it to the cache
		ui := &inventoryInstance{instance: apiutils.CloneProtoMsg(instanceV1)}
		ic.cache.Put(ui)
	}

	return trace.Wrap(instanceStream.Done())
}

// populateBotInstances reads bot instances from the bot instance service with rate limiting.
func (ic *InventoryCache) populateBotInstances(ctx context.Context, limiter *rate.Limiter) error {
	var pageToken string

	for {
		if err := limiter.Wait(ctx); err != nil {
			return trace.Wrap(err)
		}

		botInstances, nextToken, err := ic.cfg.BotInstanceBackend.ListBotInstances(
			ctx,
			defaults.DefaultChunkSize,
			pageToken,
			nil,
		)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, botInstance := range botInstances {
			// Add it to the cache
			ui := &inventoryInstance{bot: proto.CloneOf(botInstance)}
			ic.cache.Put(ui)
		}

		if nextToken == "" || len(botInstances) == 0 {
			break
		}

		pageToken = nextToken
	}

	return nil
}

// processEvents processes events from the watcher.
func (ic *InventoryCache) processEvents(ctx context.Context, watcher types.Watcher) {
	for {
		select {
		case <-ctx.Done():
			return

		case event := <-watcher.Events():
			if err := ic.processEvent(event); err != nil {
				ic.cfg.Logger.WarnContext(ctx, "Failed to process event", "error", err)
			}
		}
	}
}

// processEvent processes an event from the watcher.
func (ic *InventoryCache) processEvent(event types.Event) error {
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
func (ic *InventoryCache) processPutEvent(event types.Event) error {
	switch resource := event.Resource.(type) {
	case *types.InstanceV1:
		// Add/update it in the cache
		ui := &inventoryInstance{instance: apiutils.CloneProtoMsg(resource)}
		ic.cache.Put(ui)
	case types.Resource153UnwrapperT[*machineidv1.BotInstance]:
		// Handle bot instances wrapped in Resource153ToLegacy adapter
		botInstance := resource.UnwrapT()
		ui := &inventoryInstance{bot: proto.CloneOf(botInstance)}
		ic.cache.Put(ui)
	}

	return nil
}

// processDeleteEvent handles OpDelete events.
func (ic *InventoryCache) processDeleteEvent(event types.Event) error {
	// For delete events, the EventsService returns a ResourceHeader
	switch resource := event.Resource.(type) {
	case *types.InstanceV1:
		// Find and remove the instance from the cache.
		instanceID := resource.GetName()
		encodedID := string(ordered.Encode("", instanceID, types.KindInstance))
		ic.cache.Delete(inventoryIDIndex, encodedID)
	case *types.ResourceHeader:
		// For regular instances, use the instance ID directly
		instanceID := resource.GetName()
		encodedID := string(ordered.Encode("", instanceID, types.KindInstance))
		ic.cache.Delete(inventoryIDIndex, encodedID)
	case types.Resource153UnwrapperT[*machineidv1.BotInstance]:
		botInstance := resource.UnwrapT()
		botName := botInstance.GetSpec().GetBotName()
		instanceID := botInstance.GetSpec().GetInstanceId()
		encodedID := string(ordered.Encode(botName, instanceID, types.KindBotInstance))
		ic.cache.Delete(inventoryIDIndex, encodedID)
	}

	return nil
}

// parsedFilter holds the expression filters.
type parsedFilter struct {
	filter                    *inventoryv1.ListUnifiedInstancesFilter
	unifiedInstanceExpression typical.Expression[*unifiedFilterEnvironment, bool]
}

// parseFilter returns a parsedFilter given a ListUnifiedInstancesFilter.
func (ic *InventoryCache) parseFilter(filter *inventoryv1.ListUnifiedInstancesFilter) (*parsedFilter, error) {
	pf := &parsedFilter{
		filter: filter,
	}

	if filter == nil || filter.PredicateExpression == "" {
		return pf, nil
	}

	parser := getUnifiedExpressionParser()

	expr, err := parser.Parse(filter.PredicateExpression)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pf.unifiedInstanceExpression = expr

	return pf, nil
}

// ListUnifiedInstances returns a page of instances and bot_instances. This API will refuse any requests when the cache is unhealthy or not yet
// fully initialized.
func (ic *InventoryCache) ListUnifiedInstances(ctx context.Context, req *inventoryv1.ListUnifiedInstancesRequest) (*inventoryv1.ListUnifiedInstancesResponse, error) {
	if !ic.IsHealthy() {
		// This returns HTTP error 503. Keep in sync with web/packages/teleport/src/Instances/Instances.tsx (isCacheInitializing)
		return nil, trace.ConnectionProblem(nil, "inventory cache is not yet healthy, please try again in a few minutes")
	}

	if req.PageSize <= 0 {
		req.PageSize = defaults.DefaultChunkSize
	}

	// Decode the PageToken from base32hex
	var startKey string
	if req.PageToken != "" {
		decoded, err := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(req.PageToken)
		if err != nil {
			return nil, trace.BadParameter("invalid page token: %v", err)
		}
		startKey = string(decoded)
	}

	parsed, err := ic.parseFilter(req.GetFilter())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []*inventoryv1.UnifiedInstanceItem
	var nextPageToken string

	var index inventoryIndex

	switch req.GetSort() {
	case inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_TYPE:
		index = inventoryTypeIndex
	case inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION:
		index = inventoryVersionIndex
	case inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_NAME:
		index = inventoryAlphabeticalIndex
	default:
		index = inventoryAlphabeticalIndex
	}

	var endKey string

	// Determine sort order (ascending vs descending)
	isDesc := req.GetOrder() == inventoryv1.SortOrder_SORT_ORDER_DESCENDING

	// For type index with a single instance type filter, set bounds
	if index == inventoryTypeIndex && req.GetFilter() != nil && len(req.GetFilter().GetInstanceTypes()) == 1 {
		kind, err := instanceTypeToKind(req.GetFilter().GetInstanceTypes()[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}

		start := string(ordered.Encode(kind))
		end := string(ordered.Encode(kind, ordered.Inf))

		// If we're going in descending order, the start key becomes the end key, and vice versa
		if isDesc {
			start, end = end, start
		}

		if req.PageToken == "" {
			startKey = start
		}
		endKey = end
	}

	// Iterate over items in the cache
	iterator := ic.cache.Ascend
	if isDesc {
		iterator = ic.cache.Descend
	}

	for sf := range iterator(index, startKey, endKey) {
		if !ic.matchesFilter(sf, parsed) {
			continue
		}

		if len(items) == int(req.PageSize) {
			// Get the key for the current item based on the index
			rawKey, err := ic.getKeyForIndex(sf, index)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Encode the next page token to base32hex
			nextPageToken = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(rawKey))
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
func (ic *InventoryCache) matchesFilter(ui *inventoryInstance, parsed *parsedFilter) bool {
	if parsed == nil || parsed.filter == nil {
		return true
	}

	filter := parsed.filter

	// Filter by instance types
	isInstance := ui.isInstance()
	matchesType := false
	for _, instanceType := range filter.InstanceTypes {
		if instanceType == inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE && isInstance {
			matchesType = true
			break
		}
		if instanceType == inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE && !isInstance {
			matchesType = true
			break
		}
	}
	if len(filter.InstanceTypes) > 0 && !matchesType {
		return false
	}

	// Basic search
	if filter.Search != "" {
		var searchableText string
		if ui.isInstance() {
			// For instances, search by hostname or instance ID
			searchableText = ui.instance.Spec.Hostname + " " + ui.instance.GetName()
		} else {
			// For bot instances, search by bot name or instance ID
			searchableText = ui.bot.Spec.BotName + " " + ui.bot.GetMetadata().GetName()
		}

		searchTerms := strings.Fields(filter.Search)
		matchedAll := true
		for _, term := range searchTerms {
			if !strcase.Contains(searchableText, term) {
				matchedAll = false
				break
			}
		}

		if !matchedAll {
			return false
		}
	}

	// Filter by services (only applies to instances)
	if len(filter.Services) > 0 {
		// Bot instances don't have services, so exclude them when the services filter is active
		if !ui.isInstance() {
			return false
		}
		filterServices := utils.NewSet(filter.Services...)
		hasService := false
		for _, svc := range ui.instance.Spec.Services {
			if filterServices.Contains(string(svc)) {
				hasService = true
				break
			}
		}
		if !hasService {
			return false
		}
	}

	// Filter by updater groups
	if len(filter.UpdaterGroups) > 0 {
		var updateGroup string
		if ui.isInstance() {
			if ui.instance.Spec.UpdaterInfo != nil {
				updateGroup = ui.instance.Spec.UpdaterInfo.UpdateGroup
			}
		} else {
			if len(ui.bot.Status.LatestHeartbeats) > 0 && ui.bot.Status.LatestHeartbeats[0].UpdaterInfo != nil {
				updateGroup = ui.bot.Status.LatestHeartbeats[0].UpdaterInfo.UpdateGroup
			}
		}
		if !slices.Contains(filter.UpdaterGroups, updateGroup) {
			return false
		}
	}

	// Filter by upgraders
	if len(filter.Upgraders) > 0 {
		var upgrader string
		if ui.isInstance() {
			upgrader = ui.instance.Spec.ExternalUpgrader
		} else {
			if len(ui.bot.Status.LatestHeartbeats) > 0 {
				upgrader = ui.bot.Status.LatestHeartbeats[0].ExternalUpdater
			}
		}
		if !slices.Contains(filter.Upgraders, upgrader) {
			return false
		}
	}

	// Filter with predicate language query
	if filter.PredicateExpression != "" {
		match, err := ic.matchSearchKeywords(ui, parsed)
		if err != nil {
			ic.cfg.Logger.DebugContext(context.Background(), "Failed to filter instances using predicate expression", "error", err)
			return false
		}
		if !match {
			return false
		}
	}

	return true
}

// matchSearchKeywords evaluates predicate language expressions against a unified instance.
func (ic *InventoryCache) matchSearchKeywords(ui *inventoryInstance, parsed *parsedFilter) (bool, error) {
	if parsed.unifiedInstanceExpression == nil {
		return true, nil
	}

	env := &unifiedFilterEnvironment{
		ui: ui,
	}

	match, err := parsed.unifiedInstanceExpression.Evaluate(env)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return match, nil
}

// unifiedFilterEnvironment is the filter environment for evaluating expressions on both instances and bot instances.
type unifiedFilterEnvironment struct {
	ui *inventoryInstance
}

func (e *unifiedFilterEnvironment) GetVersion() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		return e.ui.instance.Spec.Version
	}
	// For bot instances, get version from latest heartbeat
	if e.ui.bot.Status != nil && len(e.ui.bot.Status.LatestHeartbeats) > 0 {
		return e.ui.bot.Status.LatestHeartbeats[0].Version
	}
	return ""
}

func (e *unifiedFilterEnvironment) GetHostname() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		return e.ui.instance.Spec.Hostname
	}
	// For bot instances, get hostname from latest heartbeat
	if e.ui.bot.Status != nil && len(e.ui.bot.Status.LatestHeartbeats) > 0 {
		return e.ui.bot.Status.LatestHeartbeats[0].Hostname
	}
	return ""
}

func (e *unifiedFilterEnvironment) GetName() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		return e.ui.instance.GetMetadata().Name
	}
	return e.ui.bot.GetMetadata().GetName()
}

func (e *unifiedFilterEnvironment) GetBotName() string {
	if e == nil || e.ui == nil || e.ui.isInstance() {
		return ""
	}
	return e.ui.bot.Spec.BotName
}

func (e *unifiedFilterEnvironment) GetInstanceID() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		return e.ui.instance.GetName()
	}
	return e.ui.bot.Spec.InstanceId
}

func (e *unifiedFilterEnvironment) GetServices() []string {
	if e == nil || e.ui == nil || !e.ui.isInstance() {
		return nil
	}
	services := e.ui.instance.Spec.Services
	result := make([]string, len(services))
	for i, svc := range services {
		result[i] = string(svc)
	}
	return result
}

func (e *unifiedFilterEnvironment) GetUpdaterGroup() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		if e.ui.instance.Spec.UpdaterInfo != nil {
			return e.ui.instance.Spec.UpdaterInfo.UpdateGroup
		}
		return ""
	}
	// For bot instances, get from latest heartbeat
	if e.ui.bot.Status != nil && len(e.ui.bot.Status.LatestHeartbeats) > 0 && e.ui.bot.Status.LatestHeartbeats[0].UpdaterInfo != nil {
		return e.ui.bot.Status.LatestHeartbeats[0].UpdaterInfo.UpdateGroup
	}
	return ""
}

func (e *unifiedFilterEnvironment) GetExternalUpgrader() string {
	if e == nil || e.ui == nil {
		return ""
	}
	if e.ui.isInstance() {
		return e.ui.instance.Spec.ExternalUpgrader
	}
	// For bot instances, get from latest heartbeat
	if e.ui.bot.Status != nil && len(e.ui.bot.Status.LatestHeartbeats) > 0 {
		return e.ui.bot.Status.LatestHeartbeats[0].ExternalUpdater
	}
	return ""
}

// getUnifiedExpressionParser returns a cached parser instance, initializing it once.
// Panics if the parser cannot be created, similar to regexp.MustCompile.
func getUnifiedExpressionParser() *typical.Parser[*unifiedFilterEnvironment, bool] {
	unifiedExpressionParserOnce.Do(func() {
		spec := expression.DefaultParserSpec[*unifiedFilterEnvironment]()

		newVariables := map[string]typical.Variable{
			"version": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetVersion(), nil
			}),
			"status.latest_heartbeat.version": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetVersion(), nil
			}),
			"hostname": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetHostname(), nil
			}),
			"spec.hostname": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetHostname(), nil
			}),
			"status.latest_heartbeat.hostname": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetHostname(), nil
			}),
			"name": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetName(), nil
			}),
			"metadata.name": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetName(), nil
			}),
			"spec.bot_name": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetBotName(), nil
			}),
			"spec.instance_id": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetInstanceID(), nil
			}),
			"spec.services": typical.DynamicVariable(func(env *unifiedFilterEnvironment) ([]string, error) {
				return env.GetServices(), nil
			}),
			"spec.updater_group": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetUpdaterGroup(), nil
			}),
			"status.latest_heartbeat.updater_info.update_group": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetUpdaterGroup(), nil
			}),
			"spec.external_upgrader": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetExternalUpgrader(), nil
			}),
			"status.latest_heartbeat.external_updater": typical.DynamicVariable(func(env *unifiedFilterEnvironment) (string, error) {
				return env.GetExternalUpgrader(), nil
			}),
		}

		if len(spec.Variables) < 1 {
			spec.Variables = newVariables
		} else {
			maps.Copy(spec.Variables, newVariables)
		}

		var err error
		unifiedExpressionParser, err = typical.NewParser[*unifiedFilterEnvironment, bool](spec)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize unified expression parser: %v", err))
		}
	})

	return unifiedExpressionParser
}

// getKeyForIndex returns the key a the given instance based on the index
func (ic *InventoryCache) getKeyForIndex(ui *inventoryInstance, index inventoryIndex) (string, error) {
	switch index {
	case inventoryAlphabeticalIndex:
		return ui.getAlphabeticalKey(), nil
	case inventoryTypeIndex:
		return ui.getTypeKey(), nil
	case inventoryVersionIndex:
		return ui.getVersionKey(), nil
	case inventoryIDIndex:
		return ui.getIDKey(), nil
	default:
		return "", trace.BadParameter("unknown index: %v", index)
	}
}

// unifiedInstanceToProto converts a unified instance to a proto UnifiedInstanceItem.
func (ic *InventoryCache) unifiedInstanceToProto(ui *inventoryInstance) *inventoryv1.UnifiedInstanceItem {
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
