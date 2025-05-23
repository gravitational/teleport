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
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/pagination"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

// UnifiedResourceCacheConfig is used to configure a UnifiedResourceCache
type UnifiedResourceCacheConfig struct {
	// Clock is a clock for time-related operations
	Clock clockwork.Clock
	ResourceWatcherConfig
	ResourceGetter
}

// UnifiedResourceCache contains a representation of all resources that are displayable in the UI
type UnifiedResourceCache struct {
	rw              sync.RWMutex
	logger          *slog.Logger
	cfg             UnifiedResourceCacheConfig
	itemCache       *sortcache.SortCache[types.ResourceWithLabels, resourceIndex]
	initializationC chan struct{}
	stale           bool
	once            sync.Once
	cache           *utils.FnCache
	resourceGetter  ResourceGetter
}

type unifiedResourcePseudoKey struct {
	name   string
	kind   string
	hostID string
}

// getUnifiedResourcePseudoKey returns a key for the given resource. It's a pseudo
// key because the kinds are remapped.
func getUnifiedResourcePseudoKey(resource types.Resource) unifiedResourcePseudoKey {
	var name, kind, hostID string
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
		hostID = r.GetHostID()
	case types.DatabaseServer:
		db := r.GetDatabase()
		if db != nil {
			name = db.GetName()
			kind = types.KindDatabase
		}
		hostID = r.GetHostID()
	case types.KubeServer:
		cluster := r.GetCluster()
		if cluster != nil {
			name = r.GetCluster().GetName()
			kind = types.KindKubernetesCluster
		}
		hostID = r.GetHostID()
	case types.SAMLIdPServiceProvider:
		name = r.GetName()
		kind = types.KindApp
	default:
		name = resource.GetName()
		kind = resource.GetKind()
	}

	return unifiedResourcePseudoKey{
		name:   name,
		kind:   kind,
		hostID: hostID,
	}
}

// resourceIndex is a [sortcache.SortCache] index.
type resourceIndex int

const (
	// TODO(gavin): nopush godoc
	resourceNameIndex resourceIndex = iota
	// TODO(gavin): nopush godoc
	resourceKindIndex
	// TODO(gavin): nopush godoc
	resourceIdentifierIndex
)

// TODO(gavin): nopush godoc
var unifiedResourceIndices = map[resourceIndex]func(types.ResourceWithLabels) string{
	resourceNameIndex: func(r types.ResourceWithLabels) string {
		key := getUnifiedResourcePseudoKey(r)
		// names should be stored as lowercase to keep items sorted as expected,
		// regardless of case.
		// Note that HA resource servers will overlap on this index and the
		// kind index, as will resources of the same name but different case.
		// That's prior behavior and documented bugs that I'm not going to fix in this PR.
		return strings.ToLower(key.name) + "/" + key.kind
	},
	resourceKindIndex: func(r types.ResourceWithLabels) string {
		key := getUnifiedResourcePseudoKey(r)
		return strings.ToLower(key.kind) + "/" + key.name
	},
	resourceIdentifierIndex: func(r types.ResourceWithLabels) string {
		if h, ok := r.(interface{ GetHostID() string }); ok {
			return r.GetKind() + "/" + r.GetName() + "/" + h.GetHostID()
		}
		return r.GetKind() + "/" + r.GetName()
	},
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
		itemCache: sortcache.New(sortcache.Config[types.ResourceWithLabels, resourceIndex]{
			Indexes: unifiedResourceIndices,
		}),
		initializationC: make(chan struct{}),
		resourceGetter:  cfg.ResourceGetter,
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
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentUnifiedResource
	}
	return nil
}

type iteratedItem struct {
	resource types.ResourceWithLabels
	key      string
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

		rangeFn := (*sortcache.SortCache[types.ResourceWithLabels, resourceIndex]).Ascend
		if sortBy.IsDesc {
			rangeFn = (*sortcache.SortCache[types.ResourceWithLabels, resourceIndex]).Descend
		}
		var index resourceIndex
		switch sortBy.Field {
		case sortByName:
			index = resourceNameIndex
		case sortByKind:
			index = resourceKindIndex
		default:
			index = resourceNameIndex
		}

		const defaultPageSize = 100
		items := make([]iteratedItem, 0, defaultPageSize)
		for {
			items = items[:0]

			err := c.read(ctx, func(cache *UnifiedResourceCache) error {
				// range over all keys
				for r := range rangeFn(cache.itemCache, index, start, "") {
					switch r.GetKind() {
					case types.KindDatabaseServer:
						key := cache.itemCache.KeyOf(resourceKindIndex, r)
						healthStat := types.AggregateHealthStatus(func(yield func(types.TargetHealthStatus) bool) {
							for r := range cache.itemCache.Ascend(resourceIdentifierIndex, key, sortcache.NextKey(key)) {
								if r, ok := r.(types.DatabaseServer); ok {
									if !yield(types.TargetHealthStatus(r.GetTargetHealth().Status)) {
										return
									}
								}
							}
						})
						r = &aggregatedDatabase{
							DatabaseServer: r.(types.DatabaseServer),
							status:         healthStat,
						}
					}
					if len(kinds) == 0 || c.itemKindMatches(r, kindsMap) {
						key := cache.itemCache.KeyOf(index, r)
						items = append(items, iteratedItem{key: key, resource: r})
						if len(items) >= defaultPageSize {
							if !sortBy.IsDesc {
								start = sortcache.NextKey(key)
							}
							return nil
						}
					}
				}
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

			if !yield(c.cloneResource(item.resource), nil) {
				return
			}
		}
	}
}

func (c *UnifiedResourceCache) cloneResource(res types.Resource) types.ResourceWithLabels {
	switch r := res.(type) {
	case types.Server:
		return r.DeepCopy()
	case types.AppServer:
		return r.Copy()
	case types.DatabaseServer:
		return r.Copy()
	case types.KubeServer:
		return r.Copy()
	case types.WindowsDesktop:
		return r.Copy()

	// TODO(gavin): nopush do we need these? We don't store these?
	case types.Application:
		return r.Copy()
	case types.KubeCluster:
		return r.Copy()
	case types.Database:
		return r.Copy()
	case types.SAMLIdPServiceProvider:
		return r.Copy()
	default:
		// TODO(gavin): nopush do we want to keep this? It could be just a thing while testing.
		panic(fmt.Sprintf("unknown resource! %T", r))
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
	return iterateUnifiedResourceCache[types.Server](ctx, c, params, types.KindNode)
}

// AppServers iterates over all cached app servers starting from the provided key.
func (c *UnifiedResourceCache) AppServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.AppServer, error] {
	return iterateUnifiedResourceCache[types.AppServer](ctx, c, params, types.KindAppServer)
}

// DatabaseServers iterates over all cached database servers starting from the provided key.
func (c *UnifiedResourceCache) DatabaseServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.DatabaseServer, error] {
	return iterateUnifiedResourceCache[types.DatabaseServer](ctx, c, params, types.KindDatabaseServer)
}

// KubernetesServers iterates over all cached Kubernetes servers starting from the provided key.
func (c *UnifiedResourceCache) KubernetesServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.KubeServer, error] {
	return iterateUnifiedResourceCache[types.KubeServer](ctx, c, params, types.KindKubeServer)
}

// WindowsDesktops iterates over all cached windows desktops starting from the provided key.
func (c *UnifiedResourceCache) WindowsDesktops(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.WindowsDesktop, error] {
	return iterateUnifiedResourceCache[types.WindowsDesktop](ctx, c, params, types.KindWindowsDesktop)
}

// GitServers iterates over all cached git servers starting from the provided key.
func (c *UnifiedResourceCache) GitServers(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.Server, error] {
	return iterateUnifiedResourceCache[types.Server](ctx, c, params, types.KindGitServer)
}

// SAMLIdPServiceProviders iterates over all cached sAML IdP service providers starting from the provided key.
func (c *UnifiedResourceCache) SAMLIdPServiceProviders(ctx context.Context, params UnifiedResourcesIterateParams) iter.Seq2[types.SAMLIdPServiceProvider, error] {
	return iterateUnifiedResourceCache[types.SAMLIdPServiceProvider](ctx, c, params, types.KindSAMLIdPServiceProvider)
}

func iterateUnifiedResourceCache[T any](ctx context.Context, c *UnifiedResourceCache, params UnifiedResourcesIterateParams, kind string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		sortBy := types.SortBy{IsDesc: params.Descending, Field: SortByName}
		for i, err := range c.iterateItems(ctx, params.Start, sortBy, kind) {
			if err != nil {
				var t T
				yield(t, err)
				return
			}

			if !yield(c.cloneResource(i.resource).(T), nil) {
				return
			}
		}
	}
}

// IterateUnifiedResources allows building a custom page of resources. All items within the
// range and limit of the request are passed to the matchFn. Only those resource which
// have a true value returned from the matchFn are included in the returned page.
func (c *UnifiedResourceCache) IterateUnifiedResources(ctx context.Context, matchFn func(types.ResourceWithLabels) (bool, error), req *clientproto.ListUnifiedResourcesRequest) ([]types.ResourceWithLabels, string, error) {
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
				return resources, item.key, nil
			}

			resources = append(resources, c.cloneResource(item.resource))
		}
	}

	return resources, "", nil
}

func (c *UnifiedResourceCache) itemKindMatches(r types.ResourceWithLabels, kinds map[string]struct{}) bool {
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
			var res types.ResourceWithLabels
			var dbServer types.DatabaseServer
			healthStatus := types.AggregateHealthStatus(func(yield func(types.TargetHealthStatus) bool) {
				for resource := range c.itemCache.Ascend(resourceNameIndex, id, sortcache.NextKey(id)) {
					if resource == nil {
						continue
					}
					res = resource
					if srv, ok := resource.(types.DatabaseServer); ok {
						if !yield(types.TargetHealthStatus(srv.GetTargetHealth().Status)) {
							return
						}
						dbServer = srv
						continue
					}
					return
				}
			})
			if dbServer != nil {
				res = aggregatedDatabase{
					DatabaseServer: dbServer,
					status:         healthStatus,
				}
			}
			match, err := matchFn(res)
			if err != nil {
				return trace.Wrap(err)
			}
			if match {
				resources = append(resources, c.cloneResource(res))
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resources, nil
}

// aggregatedDatabase wraps a database server with aggregated health status.
// This type exists to avoid cloning the resource unnecessarily, yet still
// prevent data races.
type aggregatedDatabase struct {
	types.DatabaseServer
	status types.TargetHealthStatus
}

func (d *aggregatedDatabase) GetTargetHealth() types.TargetHealth {
	out := d.DatabaseServer.GetTargetHealth()
	out.Status = string(d.status)
	return out
}

// Copy returns a copy of the underlying database server with aggregated health
// status.
func (d *aggregatedDatabase) Copy() types.DatabaseServer {
	out := d.DatabaseServer.Copy()
	out.SetTargetHealth(d.GetTargetHealth())
	return out
}

func (d *aggregatedDatabase) CloneResource() types.ResourceWithLabels {
	return d.Copy()
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

func (c *UnifiedResourceCache) getResourcesAndUpdateCurrent(ctx context.Context) error {
	newNodes, err := c.getNodes(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newDBServers, err := c.getDatabaseServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newKubeServers, err := c.getKubeServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newAppServers, err := c.getAppServers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	newSAMLProviders, err := c.getSAMLApps(ctx)
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
	c.itemCache.Clear()

	for _, n := range newNodes {
		c.itemCache.Put(n)
	}

	for _, d := range newDBServers {
		c.itemCache.Put(d)
	}

	for _, a := range newAppServers {
		c.itemCache.Put(a)
	}

	for _, k := range newKubeServers {
		c.itemCache.Put(k)
	}

	for _, sa := range newSAMLProviders {
		c.itemCache.Put(sa)
	}

	for _, d := range newDesktops {
		c.itemCache.Put(d)
	}

	for _, ica := range newICAccounts {
		c.itemCache.Put(ica)
	}

	for _, g := range newGitServers {
		c.itemCache.Put(g)
	}

	c.stale = false
	c.defineCollectorAsInitialized()
	return nil
}

// getNodes will get all nodes
func (c *UnifiedResourceCache) getNodes(ctx context.Context) ([]types.Server, error) {
	newNodes, err := c.resourceGetter.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting nodes for unified resource watcher")
	}

	return newNodes, err
}

// getDatabaseServers will get all database servers
func (c *UnifiedResourceCache) getDatabaseServers(ctx context.Context) ([]types.DatabaseServer, error) {
	newDbs, err := c.resourceGetter.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting database servers for unified resource watcher")
	}

	return newDbs, nil
}

// getKubeServers will get all kube servers
func (c *UnifiedResourceCache) getKubeServers(ctx context.Context) ([]types.KubeServer, error) {
	newKubes, err := c.resourceGetter.GetKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting kube servers for unified resource watcher")
	}

	return newKubes, nil
}

// getAppServers will get all application servers
func (c *UnifiedResourceCache) getAppServers(ctx context.Context) ([]types.AppServer, error) {
	newApps, err := c.resourceGetter.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err, "getting app servers for unified resource watcher")
	}
	return newApps, nil
}

// getDesktops will get all windows desktops
func (c *UnifiedResourceCache) getDesktops(ctx context.Context) ([]types.WindowsDesktop, error) {
	newDesktops, err := c.resourceGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
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
		resp, nextKey, err := c.resourceGetter.ListSAMLIdPServiceProviders(ctx, apidefaults.DefaultChunkSize, startKey)
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

func (c *UnifiedResourceCache) getIdentityCenterAccounts(ctx context.Context) ([]types.AppServer, error) {
	var accounts []types.AppServer
	var pageRequest pagination.PageRequestToken
	for {
		resultsPage, nextPage, err := c.resourceGetter.ListIdentityCenterAccounts(ctx, apidefaults.DefaultChunkSize, &pageRequest)
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
		page, nextToken, err = c.resourceGetter.ListGitServers(ctx, apidefaults.DefaultChunkSize, nextToken)
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
			itemCache: sortcache.New(sortcache.Config[types.ResourceWithLabels, resourceIndex]{
				Indexes: unifiedResourceIndices,
			}),
			resourceGetter:  c.resourceGetter,
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
			switch r := event.Resource.(type) {
			case *types.ResourceHeader:
				switch r.Kind {
				case types.KindAppServer,
					types.KindDatabaseServer,
					types.KindKubeServer,
					types.KindWindowsDesktop:
					c.itemCache.Delete(resourceIdentifierIndex, r.GetKind()+"/"+r.GetName()+"/"+r.GetMetadata().Description)
				case types.KindIdentityCenterAccount:
					// we store aws_ic_account resources as app servers
					c.itemCache.Delete(resourceIdentifierIndex, types.KindAppServer+"/"+r.GetMetadata().Name+"/")
				default:
					c.itemCache.Delete(resourceIdentifierIndex, r.GetKind()+"/"+r.GetName())
				}
			default:
				c.logger.WarnContext(ctx, "unsupported Resource type", "resource_type", logutils.TypeAttr(r))
			}

		case types.OpPut:
			switch r := event.Resource.(type) {
			case types.Resource153UnwrapperT[*identitycenterv1.Account]:
				c.itemCache.Put(IdentityCenterAccountToAppServer(r.UnwrapT()))
			case types.ResourceWithLabels:
				c.itemCache.Put(r)
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

// TODO(gavin): nopush I dont think we need this and it doesn't solve the other problem of deduplicated resources.
type resourceWithHealth interface {
	types.ResourceWithLabels
	GetTargetHealth() types.TargetHealth
}

const (
	prefix            = "unified_resource"
	sortByName string = "name"
	sortByKind string = "kind"
)

// MakePaginatedResource converts a resource into a paginated proto representation.
func MakePaginatedResource(requestType string, r types.ResourceWithLabels, requiresRequest bool) (*clientproto.PaginatedResource, error) {
	var protoResource *clientproto.PaginatedResource
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

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_DatabaseServer{DatabaseServer: database}, RequiresRequest: requiresRequest}
	case types.KindDatabaseService:
		databaseService, ok := resource.(*types.DatabaseServiceV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_DatabaseService{DatabaseService: databaseService}, RequiresRequest: requiresRequest}
	case types.KindAppServer:
		app, ok := resource.(*types.AppServerV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_AppServer{AppServer: app}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindNode:
		srv, ok := resource.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_Node{Node: srv}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindKubeServer:
		srv, ok := resource.(*types.KubernetesServerV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_KubernetesServer{KubernetesServer: srv}, RequiresRequest: requiresRequest}
	case types.KindWindowsDesktop:
		desktop, ok := resource.(*types.WindowsDesktopV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}, Logins: logins, RequiresRequest: requiresRequest}
	case types.KindWindowsDesktopService:
		desktopService, ok := resource.(*types.WindowsDesktopServiceV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_WindowsDesktopService{WindowsDesktopService: desktopService}, RequiresRequest: requiresRequest}
	case types.KindKubernetesCluster:
		cluster, ok := resource.(*types.KubernetesClusterV3)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_KubeCluster{KubeCluster: cluster}, RequiresRequest: requiresRequest}
	case types.KindUserGroup:
		userGroup, ok := resource.(*types.UserGroupV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{Resource: &clientproto.PaginatedResource_UserGroup{UserGroup: userGroup}, RequiresRequest: requiresRequest}
	case types.KindSAMLIdPServiceProvider:
		serviceProvider, ok := resource.(*types.SAMLIdPServiceProviderV1)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{
			Resource: &clientproto.PaginatedResource_SAMLIdPServiceProvider{
				SAMLIdPServiceProvider: serviceProvider,
			},
			RequiresRequest: requiresRequest,
		}
	case types.KindGitServer:
		server, ok := resource.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("%s has invalid type %T", resourceKind, resource)
		}

		protoResource = &clientproto.PaginatedResource{
			Resource: &clientproto.PaginatedResource_GitServer{
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
func MakePaginatedResources(requestType string, resources []types.ResourceWithLabels, requestableMap map[string]struct{}) ([]*clientproto.PaginatedResource, error) {
	paginatedResources := make([]*clientproto.PaginatedResource, 0, len(resources))
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
