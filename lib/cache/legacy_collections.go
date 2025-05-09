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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/services"
)

// legacyCollection is responsible for managing collection
// of resources updates
type legacyCollection interface {
	// fetch fetches resources and returns a function which will apply said resources to the cache.
	// fetch *must* not mutate cache state outside of the apply function.
	// The provided cacheOK flag indicates whether this collection will be included in the cache generation that is
	// being prepared. If cacheOK is false, fetch shouldn't fetch any resources, but the apply function that it
	// returns must still delete resources from the backend.
	fetch(ctx context.Context, cacheOK bool) (apply func(ctx context.Context) error, err error)
	// processEvent processes event
	processEvent(ctx context.Context, e types.Event) error
	// watchKind returns a watch
	// required for this collection
	watchKind() types.WatchKind
}

// executor[T, R] is a specific way to run the collector operations that we need
// for the genericCollector for a generic resource type T and its reader type R.
type executor[T any, R any] interface {
	// getAll returns all of the target resources from the auth server.
	// For singleton objects, this should be a size-1 slice.
	getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]T, error)

	// upsert will create or update a target resource in the cache.
	upsert(ctx context.Context, cache *Cache, value T) error

	// deleteAll will delete all target resources of the type in the cache.
	deleteAll(ctx context.Context, cache *Cache) error

	// delete will delete a single target resource from the cache. For
	// singletons, this is usually an alias to deleteAll.
	delete(ctx context.Context, cache *Cache, resource types.Resource) error

	// isSingleton will return true if the target resource is a singleton.
	isSingleton() bool

	// getReader returns the appropriate reader type R based on the health status of the cache.
	// Reader type R provides getter methods related to the collection, e.g. GetNodes(), GetRoles().
	// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
	getReader(c *Cache, cacheOK bool) R
}

// noReader is returned by getReader for resources which aren't directly used by the cache, and therefore have no associated reader.
type noReader struct{}

type userTasksGetter interface {
	ListUserTasks(ctx context.Context, pageSize int64, nextToken string, filters *usertasksv1.ListUserTasksFilters) ([]*usertasksv1.UserTask, string, error)
	GetUserTask(ctx context.Context, name string) (*usertasksv1.UserTask, error)
}

// legacyCollections is a registry of resource collections used by Cache.
type legacyCollections struct {
	// byKind is a map of registered collections by resource Kind/SubKind
	byKind map[resourceKind]legacyCollection

	auditQueries                       collectionReader[services.SecurityAuditQueryGetter]
	secReports                         collectionReader[services.SecurityReportGetter]
	secReportsStates                   collectionReader[services.SecurityReportStateGetter]
	discoveryConfigs                   collectionReader[services.DiscoveryConfigsGetter]
	provisioningStates                 collectionReader[provisioningStateGetter]
	identityCenterPrincipalAssignments collectionReader[identityCenterPrincipalAssignmentGetter]
}

// setupLegacyCollections returns a registry of legacyCollections.
func setupLegacyCollections(c *Cache, watches []types.WatchKind) (*legacyCollections, error) {
	collections := &legacyCollections{
		byKind: make(map[resourceKind]legacyCollection, len(watches)),
	}
	for _, watch := range watches {
		resourceKind := resourceKindFromWatchKind(watch)
		switch watch.Kind {
		case types.KindAccessRequest:
			if c.DynamicAccess == nil {
				return nil, trace.BadParameter("missing parameter DynamicAccess")
			}
			collections.byKind[resourceKind] = &genericCollection[types.AccessRequest, noReader, accessRequestExecutor]{cache: c, watch: watch}
		case types.KindDiscoveryConfig:
			if c.DiscoveryConfigs == nil {
				return nil, trace.BadParameter("missing parameter DiscoveryConfigs")
			}
			collections.discoveryConfigs = &genericCollection[*discoveryconfig.DiscoveryConfig, services.DiscoveryConfigsGetter, discoveryConfigExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.discoveryConfigs
		case types.KindHeadlessAuthentication:
			// For headless authentications, we need only process events. We don't need to keep the cache up to date.
			collections.byKind[resourceKind] = &genericCollection[*types.HeadlessAuthentication, noReader, noopExecutor]{cache: c, watch: watch}
		case types.KindAuditQuery:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter SecReports")
			}
			collections.auditQueries = &genericCollection[*secreports.AuditQuery, services.SecurityAuditQueryGetter, auditQueryExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.auditQueries
		case types.KindSecurityReport:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter KindSecurityReport")
			}
			collections.secReports = &genericCollection[*secreports.Report, services.SecurityReportGetter, secReportExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.secReports
		case types.KindSecurityReportState:
			if c.SecReports == nil {
				return nil, trace.BadParameter("missing parameter KindSecurityReport")
			}
			collections.secReportsStates = &genericCollection[*secreports.ReportState, services.SecurityReportStateGetter, secReportStateExecutor]{cache: c, watch: watch}
			collections.byKind[resourceKind] = collections.secReportsStates
		case types.KindProvisioningPrincipalState:
			if c.ProvisioningStates == nil {
				return nil, trace.BadParameter("missing parameter KindProvisioningState")
			}
			collections.provisioningStates = &genericCollection[*provisioningv1.PrincipalState, provisioningStateGetter, provisioningStateExecutor]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.provisioningStates
		case types.KindIdentityCenterPrincipalAssignment:
			if c.IdentityCenter == nil {
				return nil, trace.BadParameter("missing parameter IdentityCenter")
			}
			collections.identityCenterPrincipalAssignments = &genericCollection[
				*identitycenterv1.PrincipalAssignment,
				identityCenterPrincipalAssignmentGetter,
				identityCenterPrincipalAssignmentExecutor,
			]{
				cache: c,
				watch: watch,
			}
			collections.byKind[resourceKind] = collections.identityCenterPrincipalAssignments
		default:
			if _, ok := c.collections.byKind[resourceKind]; !ok {
				return nil, trace.BadParameter("resource %q is not supported", watch.Kind)
			}
		}
	}
	return collections, nil
}

func resourceKindFromWatchKind(wk types.WatchKind) resourceKind {
	switch wk.Kind {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    wk.Kind,
			subkind: wk.SubKind,
		}
	}
	return resourceKind{
		kind: wk.Kind,
	}
}

func resourceKindFromResource(res types.Resource) resourceKind {
	switch res.GetKind() {
	case types.KindWebSession:
		// Web sessions use subkind to differentiate between
		// the types of sessions
		return resourceKind{
			kind:    res.GetKind(),
			subkind: res.GetSubKind(),
		}
	}
	return resourceKind{
		kind: res.GetKind(),
	}
}

type resourceKind struct {
	kind    string
	subkind string
}

func (r resourceKind) String() string {
	if r.subkind == "" {
		return r.kind
	}
	return fmt.Sprintf("%s/%s", r.kind, r.subkind)
}

type accessRequestExecutor struct{}

func (accessRequestExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.AccessRequest, error) {
	return cache.DynamicAccess.GetAccessRequests(ctx, types.AccessRequestFilter{})
}

func (accessRequestExecutor) upsert(ctx context.Context, cache *Cache, resource types.AccessRequest) error {
	return cache.dynamicAccessCache.UpsertAccessRequest(ctx, resource)
}

func (accessRequestExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.dynamicAccessCache.DeleteAllAccessRequests(ctx)
}

func (accessRequestExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.dynamicAccessCache.DeleteAccessRequest(ctx, resource.GetName())
}

func (accessRequestExecutor) isSingleton() bool { return false }

func (accessRequestExecutor) getReader(_ *Cache, _ bool) noReader {
	return noReader{}
}

var _ executor[types.AccessRequest, noReader] = accessRequestExecutor{}

type userExecutor struct{}

func (userExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]types.User, error) {
	return cache.Users.GetUsers(ctx, loadSecrets)
}

func (userExecutor) upsert(ctx context.Context, cache *Cache, resource types.User) error {
	_, err := cache.usersCache.UpsertUser(ctx, resource)
	return err
}

func (userExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.usersCache.DeleteAllUsers(ctx)
}

func (userExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.usersCache.DeleteUser(ctx, resource.GetName())
}

func (userExecutor) isSingleton() bool { return false }

func (userExecutor) getReader(cache *Cache, cacheOK bool) userGetter {
	if cacheOK {
		return cache.usersCache
	}
	return cache.Config.Users
}

type userGetter interface {
	GetUser(ctx context.Context, user string, withSecrets bool) (types.User, error)
	GetUsers(ctx context.Context, withSecrets bool) ([]types.User, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}

var _ executor[types.User, userGetter] = userExecutor{}

// collectionReader extends the collection interface, adding routing capabilities.
type collectionReader[R any] interface {
	legacyCollection

	// getReader returns the appropriate reader type T based on the health status of the cache.
	// Reader type R provides getter methods related to the collection, e.g. GetNodes(), GetRoles().
	// Note that cacheOK set to true means that cache is overall healthy and the collection was confirmed as supported.
	getReader(cacheOK bool) R
}

type resourceGetter interface {
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}

type discoveryConfigExecutor struct{}

func (discoveryConfigExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*discoveryconfig.DiscoveryConfig, error) {
	var discoveryConfigs []*discoveryconfig.DiscoveryConfig
	var nextToken string
	for {
		var page []*discoveryconfig.DiscoveryConfig
		var err error

		page, nextToken, err = cache.DiscoveryConfigs.ListDiscoveryConfigs(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		discoveryConfigs = append(discoveryConfigs, page...)

		if nextToken == "" {
			break
		}
	}
	return discoveryConfigs, nil
}

func (discoveryConfigExecutor) upsert(ctx context.Context, cache *Cache, resource *discoveryconfig.DiscoveryConfig) error {
	_, err := cache.discoveryConfigsCache.UpsertDiscoveryConfig(ctx, resource)
	return trace.Wrap(err)
}

func (discoveryConfigExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return cache.discoveryConfigsCache.DeleteAllDiscoveryConfigs(ctx)
}

func (discoveryConfigExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return cache.discoveryConfigsCache.DeleteDiscoveryConfig(ctx, resource.GetName())
}

func (discoveryConfigExecutor) isSingleton() bool { return false }

func (discoveryConfigExecutor) getReader(cache *Cache, cacheOK bool) services.DiscoveryConfigsGetter {
	if cacheOK {
		return cache.discoveryConfigsCache
	}
	return cache.Config.DiscoveryConfigs
}

var _ executor[*discoveryconfig.DiscoveryConfig, services.DiscoveryConfigsGetter] = discoveryConfigExecutor{}

type auditQueryExecutor struct{}

func (auditQueryExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.AuditQuery, error) {
	var out []*secreports.AuditQuery
	var nextToken string
	for {
		var page []*secreports.AuditQuery
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityAuditQueries(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (auditQueryExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.AuditQuery) error {
	err := cache.secReportsCache.UpsertSecurityAuditQuery(ctx, resource)
	return trace.Wrap(err)
}

func (auditQueryExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReports(ctx))
}

func (auditQueryExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityAuditQuery(ctx, resource.GetName()))
}

func (auditQueryExecutor) isSingleton() bool { return false }

func (auditQueryExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityAuditQueryGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.AuditQuery, services.SecurityAuditQueryGetter] = auditQueryExecutor{}

type secReportExecutor struct{}

func (secReportExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.Report, error) {
	var out []*secreports.Report
	var nextToken string
	for {
		var page []*secreports.Report
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityReports(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (secReportExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.Report) error {
	err := cache.secReportsCache.UpsertSecurityReport(ctx, resource)
	return trace.Wrap(err)
}

func (secReportExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReports(ctx))
}

func (secReportExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityReport(ctx, resource.GetName()))
}

func (secReportExecutor) isSingleton() bool { return false }

func (secReportExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityReportGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.Report, services.SecurityReportGetter] = secReportExecutor{}

type secReportStateExecutor struct{}

func (secReportStateExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*secreports.ReportState, error) {
	var out []*secreports.ReportState
	var nextToken string
	for {
		var page []*secreports.ReportState
		var err error

		page, nextToken, err = cache.secReportsCache.ListSecurityReportsStates(ctx, 0 /* default page size */, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, page...)
		if nextToken == "" {
			break
		}
	}
	return out, nil
}

func (secReportStateExecutor) upsert(ctx context.Context, cache *Cache, resource *secreports.ReportState) error {
	err := cache.secReportsCache.UpsertSecurityReportsState(ctx, resource)
	return trace.Wrap(err)
}

func (secReportStateExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return trace.Wrap(cache.secReportsCache.DeleteAllSecurityReportsStates(ctx))
}

func (secReportStateExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return trace.Wrap(cache.secReportsCache.DeleteSecurityReportsState(ctx, resource.GetName()))
}

func (secReportStateExecutor) isSingleton() bool { return false }

func (secReportStateExecutor) getReader(cache *Cache, cacheOK bool) services.SecurityReportStateGetter {
	if cacheOK {
		return cache.secReportsCache
	}
	return cache.Config.SecReports
}

var _ executor[*secreports.ReportState, services.SecurityReportStateGetter] = secReportStateExecutor{}

// noopExecutor can be used when a resource's events do not need to processed by
// the cache itself, only passed on to other watchers.
type noopExecutor struct{}

func (noopExecutor) getAll(ctx context.Context, cache *Cache, loadSecrets bool) ([]*types.HeadlessAuthentication, error) {
	return nil, nil
}

func (noopExecutor) upsert(ctx context.Context, cache *Cache, resource *types.HeadlessAuthentication) error {
	return nil
}

func (noopExecutor) deleteAll(ctx context.Context, cache *Cache) error {
	return nil
}

func (noopExecutor) delete(ctx context.Context, cache *Cache, resource types.Resource) error {
	return nil
}

func (noopExecutor) isSingleton() bool { return false }

func (noopExecutor) getReader(_ *Cache, _ bool) noReader {
	return noReader{}
}

var _ executor[*types.HeadlessAuthentication, noReader] = noopExecutor{}
