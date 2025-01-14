/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

// UnifiedResourceKinds is a list of all kinds that are stored in the unified resource cache.
var UnifiedResourceKinds []string = []string{
	types.KindNode,
	types.KindKubeServer,
	types.KindDatabaseServer,
	types.KindAppServer,
	types.KindWindowsDesktop,
	types.KindSAMLIdPServiceProvider,
	types.KindIdentityCenterAccount,
	types.KindIdentityCenterAccountAssignment,
	types.KindGitServer,
}

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
	rw     sync.RWMutex
	logger *slog.Logger
	cfg    UnifiedResourceCacheConfig
	// nameTree is a BTree with items sorted by (hostname)/name/type
	nameTree *btree.BTreeG[*item]
	// typeTree is a BTree with items sorted by type/(hostname)/name
	typeTree *btree.BTreeG[*item]
	// resources is a map of all resources currently tracked in the tree
	// the key is always name/type
	resources       map[string]resource
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
		logger: slog.With(teleport.ComponentKey, cfg.Component),
		cfg:    cfg,
		nameTree: btree.NewG(cfg.BTreeDegree, func(a, b *item) bool {
			return a.Less(b)
		}),
		typeTree: btree.NewG(cfg.BTreeDegree, func(a, b *item) bool {
			return a.Less(b)
		}),
		resources:       make(map[string]resource),
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

func (c *UnifiedResourceCache) putLocked(resource resource) {
	key := resourceKey(resource)
	sortKey := makeResourceSortKey(resource)
	oldResource, exists := c.resources[key]
	if exists {
		// If the resource has changed in such a way that the sort keys
		// for the nameTree or typeTree change, remove the old entries
		// from those trees before adding a new one. This can happen
		// when a node's hostname changes
		oldSortKey := makeResourceSortKey(oldResource)
		if oldSortKey.byName.Compare(sortKey.byName) != 0 {
			c.deleteSortKey(oldSortKey)
		}
	}
	c.resources[key] = resource
	c.nameTree.ReplaceOrInsert(&item{Key: sortKey.byName, Value: key})
	c.typeTree.ReplaceOrInsert(&item{Key: sortKey.byType, Value: key})
}

func putResources[T resource](cache *UnifiedResourceCache, resources []T) {
	for _, resource := range resources {
		// generate the unique resource key and add the resource to the resources map
		key := resourceKey(resource)
		cache.resources[key] = resource

		sortKey := makeResourceSortKey(resource)
		cache.nameTree.ReplaceOrInsert(&item{Key: sortKey.byName, Value: key})
		cache.typeTree.ReplaceOrInsert(&item{Key: sortKey.byType, Value: key})
	}
}

func (c *UnifiedResourceCache) deleteSortKey(sortKey resourceSortKey) error {
	if _, ok := c.nameTree.Delete(&item{Key: sortKey.byName}); !ok {
		return trace.NotFound("key %q is not found in unified cache name sort tree", sortKey.byName.String())
	}
	if _, ok := c.typeTree.Delete(&item{Key: sortKey.byType}); !ok {
		return trace.NotFound("key %q is not found in unified cache type sort tree", sortKey.byType.String())
	}
	return nil
}

func (c *UnifiedResourceCache) deleteLocked(res types.Resource) error {
	key := resourceKey(res)
	resource, exists := c.resources[key]
	if !exists {
		return trace.NotFound("cannot delete resource: key %s not found in unified resource cache", key)
	}

	sortKey := makeResourceSortKey(resource)
	c.deleteSortKey(sortKey)
	delete(c.resources, key)
	return nil
}

func (c *UnifiedResourceCache) getSortTree(sortField string) (*btree.BTreeG[*item], error) {
	switch sortField {
	case sortByName:
		return c.nameTree, nil
	case sortByKind:
		return c.typeTree, nil
	case "":
		return nil, trace.BadParameter("sort field is required")
	default:
		return nil, trace.NotImplemented("sorting by %v is not supported in unified resources", sortField)
	}
}

func (c *UnifiedResourceCache) getRange(ctx context.Context, startKey backend.Key, matchFn func(types.ResourceWithLabels) (bool, error), req *proto.ListUnifiedResourcesRequest) ([]resource, string, error) {
	if startKey.IsZero() {
		return nil, "", trace.BadParameter("missing parameter startKey")
	}
	if req.Limit <= 0 {
		req.Limit = backend.DefaultRangeLimit
	}

	var res []resource
	var nextKey string
	err := c.read(ctx, func(cache *UnifiedResourceCache) error {
		tree, err := cache.getSortTree(req.SortBy.Field)
		if err != nil {
			return trace.Wrap(err, "getting sort tree")
		}
		var iterateRange func(lessOrEqual, greaterThan *item, iterator btree.ItemIteratorG[*item])
		var endKey backend.Key
		if req.SortBy.IsDesc {
			iterateRange = tree.DescendRange
			endKey = backend.NewKey(prefix)
		} else {
			iterateRange = tree.AscendRange
			endKey = backend.RangeEnd(backend.NewKey(prefix))
		}
		var iteratorErr error
		iterateRange(&item{Key: startKey}, &item{Key: endKey}, func(item *item) bool {
			// get resource from resource map
			resourceFromMap, ok := cache.resources[item.Value]
			if !ok {
				// skip and continue
				return true
			}

			// check if the resource matches our filter
			match, err := matchFn(resourceFromMap)
			if err != nil {
				iteratorErr = err
				// stop the iterator so we can return the error
				return false
			}

			if !match {
				return true
			}

			// do we have all we need? set nextKey and stop iterating
			// we do this after the matchFn to make sure they have access to the "next" node
			if req.Limit > 0 && len(res) >= int(req.Limit) {
				nextKey = item.Key.String()
				return false
			}
			res = append(res, resourceFromMap)
			return true
		})
		return iteratorErr
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	if len(res) == backend.DefaultRangeLimit {
		c.logger.WarnContext(ctx, "Range query hit backend limit. (this is a bug!)",
			"start_key", startKey,
			"range_limit", backend.DefaultRangeLimit,
		)
	}

	return res, nextKey, nil
}

func getStartKey(req *proto.ListUnifiedResourcesRequest) backend.Key {
	// if startkey exists, return it
	if req.StartKey != "" {
		return backend.KeyFromString(req.StartKey)
	}
	// if startkey doesnt exist, we check the sort direction.
	// If sort is descending, startkey is end of the list
	if req.SortBy.IsDesc {
		return backend.RangeEnd(backend.NewKey(prefix))
	}
	// return start of the list
	return backend.NewKey(prefix)
}

func (c *UnifiedResourceCache) IterateUnifiedResources(ctx context.Context, matchFn func(types.ResourceWithLabels) (bool, error), req *proto.ListUnifiedResourcesRequest) ([]types.ResourceWithLabels, string, error) {
	startKey := getStartKey(req)
	result, nextKey, err := c.getRange(ctx, startKey, matchFn, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	resources := make([]types.ResourceWithLabels, 0, len(result))
	for _, item := range result {
		resources = append(resources, item.CloneResource())
	}

	return resources, nextKey, nil
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree in ascending order
func (c *UnifiedResourceCache) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	req := &proto.ListUnifiedResourcesRequest{Limit: backend.NoLimit, SortBy: types.SortBy{IsDesc: false, Field: sortByName}}
	result, _, err := c.getRange(ctx, backend.NewKey(prefix), func(rwl types.ResourceWithLabels) (bool, error) { return true, nil }, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources := make([]types.ResourceWithLabels, 0, len(result))
	for _, item := range result {
		resources = append(resources, item.CloneResource())
	}

	return resources, nil
}

// GetUnifiedResourcesByIDs will take a list of ids and return any items found in the unifiedResourceCache tree by id and that return true from matchFn
func (c *UnifiedResourceCache) GetUnifiedResourcesByIDs(ctx context.Context, ids []string, matchFn func(types.ResourceWithLabels) (bool, error)) ([]types.ResourceWithLabels, error) {
	var resources []types.ResourceWithLabels

	err := c.read(ctx, func(cache *UnifiedResourceCache) error {
		for _, id := range ids {
			key := backend.NewKey(prefix, id)
			res, found := cache.nameTree.Get(&item{Key: key})
			if !found || res == nil {
				continue
			}
			resource := cache.resources[res.Value]
			match, err := matchFn(resource)
			if err != nil {
				return trace.Wrap(err)
			}
			if match {
				resources = append(resources, resource.CloneResource())
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
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
	IdentityCenterAccountGetter
	IdentityCenterAccountAssignmentGetter
	GitServerGetter
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

// resourceName is a unique name to be used as a key in the resources map
func resourceKey(resource types.Resource) string {
	return resource.GetName() + "/" + resource.GetKind()
}

type resourceSortKey struct {
	byName backend.Key
	byType backend.Key
}

// resourceSortKey will generate a key to be used in the sort trees
func makeResourceSortKey(resource types.Resource) resourceSortKey {
	var name, kind string
	// set the kind to the appropriate "contained" type, rather than
	// the container type.
	switch r := resource.(type) {
	case types.Server:
		switch r.GetKind() {
		case types.KindNode, types.KindGitServer:
			name = r.GetHostname() + "/" + r.GetName()
			kind = r.GetKind()
		}
	case types.AppServer:
		app := r.GetApp()
		if app != nil {
			friendlyName := types.FriendlyName(app)
			if friendlyName != "" {
				name = friendlyName
			} else {
				name = app.GetName()
			}
			kind = types.KindApp
		}
	case types.SAMLIdPServiceProvider:
		name = r.GetName()
		kind = types.KindApp
	case types.KubeServer:
		cluster := r.GetCluster()
		if cluster != nil {
			name = r.GetCluster().GetName()
			kind = types.KindKubernetesCluster
		}
	case types.DatabaseServer:
		db := r.GetDatabase()
		if db != nil {
			name = db.GetName()
			kind = types.KindDatabase
		}
	default:
		name = resource.GetName()
		kind = resource.GetKind()
	}

	return resourceSortKey{
		// names should be stored as lowercase to keep items sorted as
		// expected, regardless of case
		byName: backend.NewKey(prefix, strings.ToLower(name), kind),
		byType: backend.NewKey(prefix, kind, strings.ToLower(name)),
	}
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

	newICAccounts, err := c.getIdentityCenterAccounts(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newICAccountAssignments, err := c.getIdentityCenterAccountAssignments(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newGitServers, err := c.getGitServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	c.rw.Lock()
	defer c.rw.Unlock()
	// empty the trees
	c.nameTree.Clear(false)
	c.typeTree.Clear(false)
	// clear the resource map as well
	// c.resources = make(map[string]resource)
	clear(c.resources)

	putResources[types.Server](c, newNodes)
	putResources[types.DatabaseServer](c, newDbs)
	putResources[types.AppServer](c, newApps)
	putResources[types.KubeServer](c, newKubes)
	putResources[types.SAMLIdPServiceProvider](c, newSAMLApps)
	putResources[types.WindowsDesktop](c, newDesktops)
	putResources[resource](c, newICAccounts)
	putResources[resource](c, newICAccountAssignments)
	putResources[types.Server](c, newGitServers)
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

func (c *UnifiedResourceCache) getIdentityCenterAccounts(ctx context.Context) ([]resource, error) {
	var accounts []resource
	var pageRequest pagination.PageRequestToken
	for {
		resultsPage, nextPage, err := c.ListIdentityCenterAccounts(ctx, apidefaults.DefaultChunkSize, &pageRequest)
		if err != nil {
			return nil, trace.Wrap(err, "getting AWS Identity Center accounts for resource watcher")
		}
		for _, a := range resultsPage {
			accounts = append(accounts, types.Resource153ToUnifiedResource(a))
		}

		if nextPage == pagination.EndOfList {
			break
		}
		pageRequest.Update(nextPage)
	}
	return accounts, nil
}

func (c *UnifiedResourceCache) getIdentityCenterAccountAssignments(ctx context.Context) ([]resource, error) {
	var accounts []resource
	var pageRequest pagination.PageRequestToken
	for {
		resultsPage, nextPage, err := c.ListAccountAssignments(ctx, apidefaults.DefaultChunkSize, &pageRequest)
		if err != nil {
			return nil, trace.Wrap(err, "getting AWS Identity Center accounts for resource watcher")
		}
		for _, a := range resultsPage {
			accounts = append(accounts, types.Resource153ToUnifiedResource(a))
		}

		if nextPage == pagination.EndOfList {
			break
		}
		pageRequest.Update(nextPage)
	}
	return accounts, nil
}

func (c *UnifiedResourceCache) getGitServers(ctx context.Context) (all []types.Server, err error) {
	var page []types.Server
	nextToken := ""
	for {
		page, nextToken, err = c.ListGitServers(ctx, apidefaults.DefaultChunkSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err, "getting Git servers for unified resource watcher")
		}

		all = append(all, page...)
		if nextToken == "" {
			break
		}
	}
	return all, nil
}

// read applies the supplied closure to either the primary tree or the ttl-based fallback tree depending on
// wether or not the cache is currently healthy.  locking is handled internally and the passed-in tree should
// not be accessed after the closure completes.
func (c *UnifiedResourceCache) read(ctx context.Context, fn func(cache *UnifiedResourceCache) error) error {
	c.rw.RLock()

	if !c.stale {
		err := fn(c)
		c.rw.RUnlock()
		return err
	}

	c.rw.RUnlock()
	ttlCache, err := utils.FnCacheGet(ctx, c.cache, "unified_resources", func(ctx context.Context) (*UnifiedResourceCache, error) {
		fallbackCache := &UnifiedResourceCache{
			cfg: c.cfg,
			nameTree: btree.NewG(c.cfg.BTreeDegree, func(a, b *item) bool {
				return a.Less(b)
			}),
			typeTree: btree.NewG(c.cfg.BTreeDegree, func(a, b *item) bool {
				return a.Less(b)
			}),
			resources:       make(map[string]resource),
			ResourceGetter:  c.ResourceGetter,
			initializationC: make(chan struct{}),
		}
		if err := fallbackCache.getResourcesAndUpdateCurrent(ctx); err != nil {
			return nil, trace.Wrap(err)
		}
		return fallbackCache, nil
	})
	c.rw.RLock()

	if !c.stale {
		// primary became healthy while we were waiting
		err := fn(c)
		c.rw.RUnlock()
		return err
	}
	c.rw.RUnlock()

	if err != nil {
		// ttl-tree setup failed
		return trace.Wrap(err)
	}

	err = fn(ttlCache)
	return err
}

func (c *UnifiedResourceCache) notifyStale() {
	c.rw.Lock()
	defer c.rw.Unlock()
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

func (c *UnifiedResourceCache) processEventsAndUpdateCurrent(ctx context.Context, events []types.Event) {
	c.rw.Lock()
	defer c.rw.Unlock()

	if c.stale {
		return
	}

	for _, event := range events {
		if event.Resource == nil {
			c.logger.WarnContext(ctx, "Unexpected event",
				"event_type", event.Type,
				"resource_kind", event.Resource.GetKind(),
				"resource_name", event.Resource.GetName(),
			)
			continue
		}

		switch event.Type {
		case types.OpDelete:
			c.deleteLocked(event.Resource)
		case types.OpPut:
			switch r := event.Resource.(type) {
			case resource:
				c.putLocked(r)

			case types.Resource153Unwrapper:
				// Raw RFD-153 style resources generally have very few methods
				// defined on them by design. One way to add complex behavior to
				// these resources is to wrap them inside another type that implements
				// any methods or interfaces they need. Resources arriving here
				// via the cache protocol will have those wrappers stripped away,
				// so we unfortunately need to unwrap and re-wrap these values
				// to restore them to a useful state.
				switch unwrapped := r.Unwrap().(type) {
				case IdentityCenterAccount:
					c.putLocked(types.Resource153ToUnifiedResource(unwrapped))

				case IdentityCenterAccountAssignment:
					c.putLocked(types.Resource153ToUnifiedResource(unwrapped))

				default:
					c.logger.WarnContext(ctx, "unsupported Resource153 type", "resource_type", logutils.TypeAttr(unwrapped))
				}

			default:
				c.logger.WarnContext(ctx, "unsupported Resource type", "resource_type", logutils.TypeAttr(r))
			}

		default:
			c.logger.WarnContext(ctx, "unsupported event type", "event_type", event.Type)
			continue
		}
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
		return i.Key.Compare(other.Key) < 0
	default:
		return false
	}
}

type resource interface {
	types.ResourceWithLabels
	CloneResource() types.ResourceWithLabels
}

type item struct {
	// Key is a key of the key value item. This will be different based on which sorting tree
	// the item is in
	Key backend.Key
	// Value will be the resourceKey used in the resources map to get the resource
	Value string
}

const (
	prefix            = "unified_resource"
	sortByName string = "name"
	sortByKind string = "kind"
)

// MakePaginatedResource converts a resource into a paginated proto representation.
func MakePaginatedResource(ctx context.Context, requestType string, r types.ResourceWithLabels, requiresRequest bool) (*proto.PaginatedResource, error) {
	var protoResource *proto.PaginatedResource
	resourceKind := requestType
	if requestType == types.KindUnifiedResource {
		resourceKind = r.GetKind()
	}

	var logins []string
	resource := r
	if enriched, ok := r.(*types.EnrichedResource); ok {
		resource = enriched.ResourceWithLabels
		logins = enriched.Logins
	}

	switch resourceKind {
	case types.KindDatabaseServer:
		database, ok := resource.(*types.DatabaseServerV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}, RequiresRequest: requiresRequest}
	case types.KindDatabaseService:
		databaseService, ok := resource.(*types.DatabaseServiceV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseService{DatabaseService: databaseService}, RequiresRequest: requiresRequest}
	case types.KindAppServer:
		app, ok := resource.(*types.AppServerV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_AppServer{AppServer: app}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindNode:
		srv, ok := resource.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: srv}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindKubeServer:
		srv, ok := resource.(*types.KubernetesServerV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubernetesServer{KubernetesServer: srv}, RequiresRequest: requiresRequest}
	case types.KindWindowsDesktop:
		desktop, ok := resource.(*types.WindowsDesktopV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindWindowsDesktopService:
		desktopService, ok := resource.(*types.WindowsDesktopServiceV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktopService{WindowsDesktopService: desktopService}, RequiresRequest: requiresRequest}
	case types.KindKubernetesCluster:
		cluster, ok := resource.(*types.KubernetesClusterV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubeCluster{KubeCluster: cluster}, RequiresRequest: requiresRequest}
	case types.KindUserGroup:
		userGroup, ok := resource.(*types.UserGroupV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_UserGroup{UserGroup: userGroup}, RequiresRequest: requiresRequest}
	case types.KindAppOrSAMLIdPServiceProvider:
		//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
		switch appOrSP := resource.(type) {
		case *types.AppServerV3:
			protoResource = &proto.PaginatedResource{
				Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
					AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
						Resource: &types.AppServerOrSAMLIdPServiceProviderV1_AppServer{
							AppServer: appOrSP,
						},
					},
				}, RequiresRequest: requiresRequest,
			}
		case *types.SAMLIdPServiceProviderV1:
			protoResource = &proto.PaginatedResource{
				Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
					AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
						Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
							SAMLIdPServiceProvider: appOrSP,
						},
					},
				}, RequiresRequest: requiresRequest,
			}
		default:
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}
	case types.KindSAMLIdPServiceProvider:
		serviceProvider, ok := resource.(*types.SAMLIdPServiceProviderV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		// TODO(gzdunek): DELETE IN 17.0
		// This is needed to maintain backward compatibility between v16 server and v15 client.
		clientVersion, versionExists := metadata.ClientVersionFromContext(ctx)
		isClientNotSupportingSAMLIdPServiceProviderResource := false
		if versionExists {
			version, err := semver.NewVersion(clientVersion)
			if err == nil && version.Major < 16 {
				isClientNotSupportingSAMLIdPServiceProviderResource = true
			}
		}

		if isClientNotSupportingSAMLIdPServiceProviderResource {
			protoResource = &proto.PaginatedResource{
				Resource: &proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider{
					//nolint:staticcheck // SA1019. TODO(gzdunek): DELETE IN 17.0
					AppServerOrSAMLIdPServiceProvider: &types.AppServerOrSAMLIdPServiceProviderV1{
						Resource: &types.AppServerOrSAMLIdPServiceProviderV1_SAMLIdPServiceProvider{
							SAMLIdPServiceProvider: serviceProvider,
						},
					},
				},
				RequiresRequest: requiresRequest,
			}
		} else {
			protoResource = &proto.PaginatedResource{
				Resource: &proto.PaginatedResource_SAMLIdPServiceProvider{
					SAMLIdPServiceProvider: serviceProvider,
				},
				RequiresRequest: requiresRequest,
			}
		}
	case types.KindIdentityCenterAccount:
		var err error
		protoResource, err = makePaginatedIdentityCenterAccount(resourceKind, resource, requiresRequest)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case types.KindIdentityCenterAccountAssignment:
		unwrapper, ok := resource.(types.Resource153Unwrapper)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}
		assignment, ok := unwrapper.Unwrap().(IdentityCenterAccountAssignment)
		if !ok {
			return nil, trace.BadParameter(
				"Unexpected type for Identity Center Account Assignment: %T",
				unwrapper)
		}

		protoResource = &proto.PaginatedResource{
			Resource:        proto.PackICAccountAssignment(assignment.AccountAssignment),
			RequiresRequest: requiresRequest,
		}

	case types.KindGitServer:
		server, ok := resource.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_GitServer{
				GitServer: server,
			},
			RequiresRequest: requiresRequest,
		}

	default:
		return nil, trace.NotImplemented("resource type %s doesn't support pagination", resource.GetKind())
	}

	return protoResource, nil
}

// makePaginatedIdentityCenterAccount returns a representation of the supplied
// Identity Center account as an App.
func makePaginatedIdentityCenterAccount(resourceKind string, resource types.ResourceWithLabels, requiresRequest bool) (*proto.PaginatedResource, error) {
	unwrapper, ok := resource.(types.Resource153Unwrapper)
	if !ok {
		return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
	}
	acct, ok := unwrapper.Unwrap().(IdentityCenterAccount)
	if !ok {
		return nil, trace.BadParameter("%s has invalid inner type %T", resourceKind, resource)
	}
	srcPSs := acct.GetSpec().GetPermissionSetInfo()
	pss := make([]*types.IdentityCenterPermissionSet, len(srcPSs))
	for i, ps := range acct.GetSpec().GetPermissionSetInfo() {
		pss[i] = &types.IdentityCenterPermissionSet{
			ARN:          ps.Arn,
			Name:         ps.Name,
			AssignmentID: ps.AssignmentId,
		}
	}

	appServer := &types.AppServerV3{
		Kind:     types.KindAppServer,
		Version:  types.V3,
		Metadata: resource.GetMetadata(),
		Spec: types.AppServerSpecV3{
			App: &types.AppV3{
				Kind:     types.KindApp,
				SubKind:  types.KindIdentityCenterAccount,
				Version:  types.V3,
				Metadata: types.Metadata153ToLegacy(acct.Metadata),
				Spec: types.AppSpecV3{
					URI:        acct.Spec.StartUrl,
					PublicAddr: acct.Spec.StartUrl,
					AWS: &types.AppAWS{
						ExternalID: acct.Spec.Id,
					},
					IdentityCenter: &types.AppIdentityCenter{
						AccountID:      acct.Spec.Id,
						PermissionSets: pss,
					},
				},
			},
		},
	}
	appServer.Metadata.Description = acct.Spec.Name

	protoResource := &proto.PaginatedResource{
		Resource: &proto.PaginatedResource_AppServer{
			AppServer: appServer,
		},
		RequiresRequest: requiresRequest,
	}

	return protoResource, nil
}

// MakePaginatedResources converts a list of resources into a list of paginated proto representations.
func MakePaginatedResources(ctx context.Context, requestType string, resources []types.ResourceWithLabels, requestableMap map[string]struct{}) ([]*proto.PaginatedResource, error) {
	paginatedResources := make([]*proto.PaginatedResource, 0, len(resources))
	for _, r := range resources {
		_, requiresRequest := requestableMap[r.GetName()]
		protoResource, err := MakePaginatedResource(ctx, requestType, r, requiresRequest)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		paginatedResources = append(paginatedResources, protoResource)
	}
	return paginatedResources, nil
}

const (
	SortByName string = "name"
	SortByKind string = "kind"
)
