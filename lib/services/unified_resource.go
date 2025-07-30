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
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/btree"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/pagination"
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
	rw     sync.RWMutex
	logger *slog.Logger
	cfg    UnifiedResourceCacheConfig
	// nameTree is a BTree with items sorted by (hostname)/name/type
	nameTree *btree.BTreeG[*item]
	// typeTree is a BTree with items sorted by type/(hostname)/name
	typeTree *btree.BTreeG[*item]
	// resources is a map of all resources currently tracked in the tree
	// the key is always name/type
	resources       map[string]resourceCollection
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
		resources:       make(map[string]resourceCollection),
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
	if collection, exists := c.resources[key]; exists {
		// If the resource has changed in such a way that the sort keys
		// for the nameTree or typeTree change, remove the old entries
		// from those trees before adding a new one. This can happen
		// when a node's hostname changes
		oldSortKey := makeResourceSortKey(collection.get())
		if oldSortKey.byName.Compare(sortKey.byName) != 0 {
			c.deleteSortKey(oldSortKey)
		}
		collection.put(resource)
	} else {
		c.resources[key] = newResourceCollection(resource)
	}
	c.nameTree.ReplaceOrInsert(&item{Key: sortKey.byName, Value: key})
	c.typeTree.ReplaceOrInsert(&item{Key: sortKey.byType, Value: key})
}

func putResources[T resource](cache *UnifiedResourceCache, resources []T) {
	for _, resource := range resources {
		cache.putLocked(resource)
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
	collection, exists := c.resources[key]
	if !exists {
		return trace.NotFound("cannot delete resource: key %s not found in unified resource cache", key)
	}

	if empty := collection.remove(res); empty {
		sortKey := makeResourceSortKey(collection.get())
		c.deleteSortKey(sortKey)
		delete(c.resources, key)
	}
	return nil
}

func (c *UnifiedResourceCache) getSortTree(sortField string) (*btree.BTreeG[*item], error) {
	switch sortField {
	case "", sortByName:
		return c.nameTree, nil
	case sortByKind:
		return c.typeTree, nil
	default:
		return nil, trace.NotImplemented("sorting by %v is not supported in unified resources", sortField)
	}
}

type iteratedItem struct {
	resource resource
	key      backend.Key
}

// iterateItems is a helper for iterating the correct cache, in the correct order
// for only the specified kinds. All external iteration APIs are built upon this
// method.
func (c *UnifiedResourceCache) iterateItems(ctx context.Context, start string, sortBy types.SortBy, kinds ...string) iter.Seq2[iteratedItem, error] {
	return func(yield func(iteratedItem, error) bool) {
		kindsMap := make(map[string]struct{})
		for _, k := range kinds {
			kindsMap[k] = struct{}{}
		}

		var startKey backend.Key
		if start != "" {
			startKey = backend.KeyFromString(start)
		}

		itemIter := (*btree.BTreeG[*item]).AscendGreaterOrEqual
		if sortBy.IsDesc {
			itemIter = (*btree.BTreeG[*item]).DescendLessOrEqual
		}

		var excludedStart bool
		const defaultPageSize = 100
		items := make([]iteratedItem, 0, defaultPageSize)
		for {
			items = items[:0]

			err := c.read(ctx, func(cache *UnifiedResourceCache) error {
				tree, err := cache.getSortTree(sortBy.Field)
				if err != nil {
					return trace.Wrap(err, "getting sort tree")
				}

				if startKey.IsZero() {
					max, ok := tree.Max()
					if sortBy.IsDesc && ok {
						startKey = max.Key
					} else {
						startKey = backend.NewKey("")
					}
				}

				itemIter(tree, &item{Key: startKey}, func(item *item) bool {
					if excludedStart {
						excludedStart = false
						if item.Key.Compare(startKey) <= 0 {
							return true
						}
					}

					collection, ok := cache.resources[item.Value]
					if !ok {
						return true
					}

					if len(kinds) == 0 || c.itemKindMatches(collection.get(), kindsMap) {
						items = append(items, iteratedItem{key: item.Key, resource: collection.get()})
					}

					if len(items) >= defaultPageSize {
						startKey = item.Key
						excludedStart = true
						return false
					}

					return true
				})

				return nil
			})
			if err != nil {
				yield(iteratedItem{}, err)
				return
			}

			for _, i := range items {
				if !yield(i, nil) {
					return
				}

			}

			if len(items) < defaultPageSize {
				return
			}
		}
	}
}

// Resources iterates over all resources from the start key that match
// one of the provided kinds. If no kinds are provided, resources of all supported
// kinds are returned.
func (c *UnifiedResourceCache) Resources(ctx context.Context, start string, sortBy types.SortBy, kinds ...string) iter.Seq2[types.ResourceWithLabels, error] {
	return func(yield func(types.ResourceWithLabels, error) bool) {
		for item, err := range c.iterateItems(ctx, start, sortBy, kinds...) {
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(item.resource.CloneResource(), nil) {
				return
			}
		}
	}
}

// UnifiedResourcesIterateParams are parameters that are provided to
// UnifiedResourceCache iterators to alter the iteration behavior.
type UnifiedResourcesIterateParams struct {
	Start      string
	Descending bool
}

// Nodes iterates over all cached nodes starting from the provided key.
func (c *UnifiedResourceCache) Nodes(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.Server, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindNode, types.Server.DeepCopy)
}

// AppServers iterates over all cached app servers starting from the provided key.
func (c *UnifiedResourceCache) AppServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.AppServer, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindAppServer, types.AppServer.Copy)
}

// DatabaseServers iterates over all cached database servers starting from the provided key.
func (c *UnifiedResourceCache) DatabaseServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.DatabaseServer, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindDatabaseServer, types.DatabaseServer.Copy)
}

// KubernetesServers iterates over all cached Kubernetes servers starting from the provided key.
func (c *UnifiedResourceCache) KubernetesServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.KubeServer, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindKubeServer, types.KubeServer.Copy)
}

// WindowsDesktops iterates over all cached windows desktops starting from the provided key.
func (c *UnifiedResourceCache) WindowsDesktops(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.WindowsDesktop, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindWindowsDesktop, types.WindowsDesktop.Copy)
}

// GitServers iterates over all cached git servers starting from the provided key.
func (c *UnifiedResourceCache) GitServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.Server, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindGitServer, types.Server.DeepCopy)
}

// SAMLIdPServiceProviders iterates over all cached sAML IdP service providers starting from the provided key.
func (c *UnifiedResourceCache) SAMLIdPServiceProviders(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.SAMLIdPServiceProvider, error] {
	return iterateUnifiedResourceCache(ctx, c, params, types.KindSAMLIdPServiceProvider, types.SAMLIdPServiceProvider.Copy)
}

func iterateUnifiedResourceCache[T resource](ctx context.Context, c *UnifiedResourceCache, params UnifiedResourcesIterateParams, kind string, cloneFn func(T) T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		sortBy := types.SortBy{IsDesc: params.Descending, Field: SortByName}
		for i, err := range c.iterateItems(ctx, params.Start, sortBy, kind) {
			if err != nil {
				var t T
				yield(t, err)
				return
			}

			switch r := i.resource.(type) {
			case T:
				if !yield(cloneFn(r), nil) {
					return
				}
			default:
				var t T
				yield(t, trace.BadParameter("expected type %T, got %T", t, r))
				return
			}

		}
	}
}

// IterateUnifiedResources allows building a custom page of resources. All items within the
// range and limit of the request are passed to the matchFn. Only those resource which
// have a true value returned from the matchFn are included in the returned page.
func (c *UnifiedResourceCache) IterateUnifiedResources(ctx context.Context, matchFn func(types.ResourceWithLabels) (bool, error), req *proto.ListUnifiedResourcesRequest) ([]types.ResourceWithLabels, string, error) {
	var resources []types.ResourceWithLabels
	for item, err := range c.iterateItems(ctx, req.StartKey, req.SortBy, req.Kinds...) {
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		match, err := matchFn(item.resource)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		if match {
			if req.Limit != backend.NoLimit && len(resources) == int(req.Limit) {
				return resources, item.key.String(), nil
			}

			resources = append(resources, item.resource.CloneResource())
		}
	}

	return resources, "", nil
}

func (c *UnifiedResourceCache) itemKindMatches(r resource, kinds map[string]struct{}) bool {
	switch r.GetKind() {
	case types.KindNode,
		types.KindWindowsDesktop,
		types.KindGitServer,
		types.KindDatabase,
		types.KindKubernetesCluster:
		_, ok := kinds[r.GetKind()]
		return ok
	case types.KindIdentityCenterAccount:
		if _, ok := kinds[types.KindApp]; ok {
			return ok
		}

		_, ok := kinds[types.KindIdentityCenterAccount]
		return ok
	case types.KindApp:
		if _, ok := kinds[types.KindApp]; ok {
			return ok
		}

		if _, ok := kinds[types.KindAppServer]; ok {
			return ok
		}

		_, ok := kinds[types.KindIdentityCenterAccount]
		return ok
	case types.KindKubeServer:
		if _, ok := kinds[types.KindKubernetesCluster]; ok {
			return ok
		}

		_, ok := kinds[types.KindKubeServer]
		return ok
	case types.KindDatabaseServer:
		if _, ok := kinds[types.KindDatabase]; ok {
			return ok
		}

		_, ok := kinds[types.KindDatabaseServer]
		return ok
	case types.KindSAMLIdPServiceProvider:
		_, ok := kinds[types.KindSAMLIdPServiceProvider]
		return ok
	case types.KindAppServer:
		if r.GetSubKind() == types.KindIdentityCenterAccount {
			if _, ok := kinds[types.KindIdentityCenterAccount]; ok {
				return ok
			}
		}

		if _, ok := kinds[types.KindApp]; ok {
			return ok
		}

		_, ok := kinds[types.KindAppServer]
		return ok
	default:
		return false
	}
}

// GetUnifiedResources returns a list of all resources stored in the current unifiedResourceCollector tree in ascending order
func (c *UnifiedResourceCache) GetUnifiedResources(ctx context.Context) ([]types.ResourceWithLabels, error) {
	var resources []types.ResourceWithLabels
	for resource, err := range c.Resources(ctx, "", types.SortBy{IsDesc: false, Field: sortByName}) {
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resource)
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
			collection, ok := cache.resources[res.Value]
			if !ok {
				continue
			}
			resource := collection.get()
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
				sanitizedFriendlyName := strings.ReplaceAll(types.FriendlyName(app), "/", "-")
				// FriendlyName is not unique, and multiple apps may have the same friendly name.
				// To prevent collisions in the resource cache, we append the app name to the
				// friendly name, ensuring uniqueness.
				name = sanitizedFriendlyName + "/" + app.GetName()
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

	putResources(c, newNodes)
	putResources(c, newDbs)
	putResources(c, newApps)
	putResources(c, newKubes)
	putResources(c, newSAMLApps)
	putResources(c, newDesktops)
	putResources(c, newICAccounts)
	putResources(c, newGitServers)
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
	return newDbs, nil
}

// getKubeServers will get all kube servers
func (c *UnifiedResourceCache) getKubeServers(ctx context.Context) ([]types.KubeServer, error) {
	newKubes, err := c.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting kube servers for unified resource watcher")
	}
	return newKubes, nil
}

// getAppServers will get all application servers
func (c *UnifiedResourceCache) getAppServers(ctx context.Context) ([]types.AppServer, error) {
	newApps, err := c.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting app servers for unified resource watcher")
	}
	return newApps, nil
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
		for _, acct := range resultsPage {
			accounts = append(accounts, IdentityCenterAccountToAppServer(acct.Account))
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
			resources:       make(map[string]resourceCollection),
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
			switch event.Resource.GetKind() {
			case types.KindIdentityCenterAccount:
				c.deleteLocked(&types.ResourceHeader{
					Kind: types.KindAppServer,
					Metadata: types.Metadata{
						Name: event.Resource.GetName(),
					},
				})
			default:
				c.deleteLocked(event.Resource)
			}
		case types.OpPut:
			switch r := event.Resource.(type) {
			case resource:
				c.putLocked(r)
			case types.Resource153UnwrapperT[*identitycenterv1.Account]:
				c.putLocked(IdentityCenterAccountToAppServer(r.UnwrapT()))
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
	return []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindKubeServer},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindSAMLIdPServiceProvider},
		{Kind: types.KindIdentityCenterAccount},
		{Kind: types.KindGitServer},
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
		return i.Key.Compare(other.Key) < 0
	default:
		return false
	}
}

type resource interface {
	types.ResourceWithLabels
	CloneResource() types.ResourceWithLabels
}

type resourceCollection interface {
	get() resource
	put(r resource)
	// remove removes a resource from the collection and returns true if the
	// collection itself should be removed.
	remove(r types.Resource) bool
}

func newResourceCollection(r resource) resourceCollection {
	switch r := r.(type) {
	case types.DatabaseServer:
		return newServerResourceCollection(r,
			func(srv types.DatabaseServer, servers map[string]types.DatabaseServer) types.DatabaseServer {
				return &aggregatedDatabase{
					DatabaseServer: srv,
					status:         aggregateHealthStatuses(servers),
				}
			})
	case serverResource:
		return newServerResourceCollection(r, nil)
	default:
		return &singularResourceCollection{latest: r}
	}
}

func aggregateHealthStatuses[T types.TargetHealthStatusGetter](hgs map[string]T) types.TargetHealthStatus {
	return types.AggregateHealthStatus(func(yield func(types.TargetHealthStatus) bool) {
		for _, hg := range hgs {
			if !yield(hg.GetTargetHealthStatus()) {
				return
			}
		}
	})
}

type singularResourceCollection struct {
	latest resource
}

func (c *singularResourceCollection) get() resource { return c.latest }

func (c *singularResourceCollection) put(r resource) { c.latest = r }

func (c *singularResourceCollection) remove(types.Resource) bool { return true }

// serverResource is a type of resource that may have multiple agents
// heartbeating it.
type serverResource interface {
	resource
	GetHostID() string
}

type serverResourceCollection[R serverResource] struct {
	aggregate     R
	aggregationFn func(latest R, servers map[string]R) R
	servers       map[string]R
}

func newServerResourceCollection[R serverResource](r R, aggFn func(latest R, servers map[string]R) R) *serverResourceCollection[R] {
	if aggFn == nil {
		aggFn = func(r R, _ map[string]R) R {
			return r
		}
	}
	collection := &serverResourceCollection[R]{
		servers:       make(map[string]R),
		aggregationFn: aggFn,
	}
	collection.put(r)
	return collection
}

func (c *serverResourceCollection[R]) get() resource {
	return c.aggregate
}

func (c *serverResourceCollection[R]) put(r resource) {
	if r, ok := r.(R); ok {
		c.servers[r.GetHostID()] = r
		c.aggregate = c.aggregationFn(r, c.servers)
	}
}

func (c *serverResourceCollection[R]) remove(r types.Resource) bool {
	// This looks insane, but we only get a [types.ResourceHeader] in
	// [types.OpDelete] events.
	// The types that actually implement [resourceServer] all store the host ID
	// in the description of the resource header metadata on deletion.
	// If a new type is added that implements [resourceServer] and the
	// unified resource watchers starts watching it, then please:
	// - add it to the isResourceServer helper func
	// - ensure host ID is stored in the metadata description in its event parser
	// - add test coverage for it in TestUnifiedResourceWatcher_DeleteEvent
	delete(c.servers, r.GetMetadata().Description)
	for _, s := range c.servers {
		c.aggregate = c.aggregationFn(s, c.servers)
		return false
	}
	return true
}

// aggregatedDatabase wraps a database server with aggregated health status.
// It is assumed that multiple heartbeats with the same resource name but
// different host IDs may be received and they may report different health
// statuses.
// This type provides the following properties:
//   - avoid cloning the resource *before* filtering.
//   - when the resource is cloned *after* filtering, set the clone's health
//     status to the aggregate health status.
//
// Go generics do not support embedding a generic type, otherwise this type
// would be made generic.
type aggregatedDatabase struct {
	types.DatabaseServer
	status types.TargetHealthStatus
}

// This type MUST implement [types.DatabaseServer] to act as a facade type,
// otherwise dynamic assertions elsewhere will fail.
var _ types.DatabaseServer = (*aggregatedDatabase)(nil)

// GetTargetHealthStatus gets the aggregate health status for filtering by
// health status.
func (d *aggregatedDatabase) GetTargetHealthStatus() types.TargetHealthStatus {
	return d.status
}

// Copy returns a copy of the underlying database server with aggregated health
// status.
func (d *aggregatedDatabase) Copy() types.DatabaseServer {
	out := d.DatabaseServer.Copy()
	out.SetTargetHealthStatus(d.status)
	return out
}

// CloneResource returns a copy of the underlying database server with
// aggregated health status.
func (d *aggregatedDatabase) CloneResource() types.ResourceWithLabels {
	return d.Copy()
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
func MakePaginatedResource(requestType string, r types.ResourceWithLabels, requiresRequest bool) (*proto.PaginatedResource, error) {
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
	case types.KindSAMLIdPServiceProvider:
		serviceProvider, ok := resource.(*types.SAMLIdPServiceProviderV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_SAMLIdPServiceProvider{
				SAMLIdPServiceProvider: serviceProvider,
			},
			RequiresRequest: requiresRequest,
		}
	case types.KindIdentityCenterAccount:
		unwrapper, ok := resource.(types.Resource153UnwrapperT[IdentityCenterAccount])
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &proto.PaginatedResource{
			Resource: &proto.PaginatedResource_AppServer{
				AppServer: IdentityCenterAccountToAppServer(unwrapper.UnwrapT().Account),
			},
			RequiresRequest: requiresRequest,
		}
	case types.KindIdentityCenterAccountAssignment:
		unwrapper, ok := resource.(types.Resource153UnwrapperT[IdentityCenterAccountAssignment])
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}
		protoResource = &proto.PaginatedResource{
			Resource:        proto.PackICAccountAssignment(unwrapper.UnwrapT().AccountAssignment),
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

// MakePaginatedResources converts a list of resources into a list of paginated proto representations.
func MakePaginatedResources(requestType string, resources []types.ResourceWithLabels, requestableMap map[string]struct{}) ([]*proto.PaginatedResource, error) {
	paginatedResources := make([]*proto.PaginatedResource, 0, len(resources))
	for _, r := range resources {
		_, requiresRequest := requestableMap[r.GetName()]
		protoResource, err := MakePaginatedResource(requestType, r, requiresRequest)
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
