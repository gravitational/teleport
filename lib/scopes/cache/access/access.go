/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package access

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/cache/assignments"
	"github.com/gravitational/teleport/lib/scopes/cache/roles"
	scopedutils "github.com/gravitational/teleport/lib/scopes/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/set"
)

// CacheConfig configures the scoped access cache.
type CacheConfig struct {
	Events            types.Events
	Reader            services.ScopedAccessReader
	AccessListReader  AccessListReader
	AccessListEvents  types.Events
	MaxRetryPeriod    time.Duration
	TTLCacheRetention time.Duration
}

type AccessListReader interface {
	// GetAccessList returns the specified access list resource.
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
	// ListAccessLists returns a paginated list of access lists.
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	// GetAccessListMember returns the specified access list member resource.
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	// ListAccessListMembers returns a paginated list of all access list members.
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	// ListAllAccessListMembers returns a paginated list of all access list members for all access lists.
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}

// CheckAndSetDefaults verifies required fields and sets default values as appropriate.
func (c *CacheConfig) CheckAndSetDefaults() error {
	if c.Events == nil {
		return trace.BadParameter("missing required parameter Events in scoped access cache config")
	}

	if c.Reader == nil {
		return trace.BadParameter("missing required parameter Reader in scoped access cache config")
	}

	if c.AccessListReader == nil {
		return trace.BadParameter("missing required parameter AccessListReader in scoped access cache config")
	}

	if c.MaxRetryPeriod <= 0 {
		c.MaxRetryPeriod = defaults.MaxLongWatcherBackoff
	}

	if c.TTLCacheRetention <= 0 {
		c.TTLCacheRetention = time.Second * 3
	}

	return nil
}

// state holds the cache state elements.
type state struct {
	roles       *roles.RoleCache
	assignments *assignments.AssignmentCache
}

// Cache is an in-memory cache for scoped access resources. It provides similar features to the primary
// teleport cache, but is specifically tailored to support scope-based queries that are difficult to implement
// with the primary cache.
type Cache struct {
	cfg      CacheConfig
	rw       sync.RWMutex
	state    state
	ok       bool
	closed   bool
	init     chan struct{}
	initOnce sync.Once
	cancel   context.CancelFunc
	ttlCache *utils.FnCache
	done     chan struct{}
}

// NewCache attempts to configure and start a new scoped access cache. The cache is immediately readable if returned,
// but performance may be suboptimal until watcher init has completed.
func NewCache(cfg CacheConfig) (*Cache, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(cfg.MaxRetryPeriod / 16),
		Driver: retryutils.NewExponentialDriver(cfg.MaxRetryPeriod / 16),
		Max:    cfg.MaxRetryPeriod,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, cancel := context.WithCancel(context.Background())

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     cfg.TTLCacheRetention,
		Context: closeContext,
	})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	cache := &Cache{
		cfg:      cfg,
		ttlCache: ttlCache,
		cancel:   cancel,
		init:     make(chan struct{}),
		done:     make(chan struct{}),
	}

	go cache.update(closeContext, retry)

	return cache, nil
}

// Init returns a channel that is closed when the cache has completed its first init. Used in tests that
// want to wait for cache readiness. commonly this avoids the effect of the read state apparently
// "skipping" back in time slightly early in the test if cache init happens after one or more pre-init
// reads.
func (c *Cache) Init() <-chan struct{} {
	return c.init
}

// GetScopedRole retrieves a scoped role by name.
func (c *Cache) GetScopedRole(ctx context.Context, req *scopedaccessv1.GetScopedRoleRequest) (*scopedaccessv1.GetScopedRoleResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.roles.GetScopedRole(ctx, req)
}

// ListScopedRoles returns a paginated list of scoped roles.
func (c *Cache) ListScopedRoles(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest) (*scopedaccessv1.ListScopedRolesResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.roles.ListScopedRoles(ctx, req)
}

// ListScopedRolesWithFilter returns a paginated list of scoped roles filtered by the provided filter function. This
// method is used internally to implement access-controls on the ListScopedRoles grpc method.
func (c *Cache) ListScopedRolesWithFilter(ctx context.Context, req *scopedaccessv1.ListScopedRolesRequest, filter func(*scopedaccessv1.ScopedRole) bool) (*scopedaccessv1.ListScopedRolesResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.roles.ListScopedRolesWithFilter(ctx, req, filter)
}

// GetScopedRoleAssignment retrieves a scoped role assignment by name.
func (c *Cache) GetScopedRoleAssignment(ctx context.Context, req *scopedaccessv1.GetScopedRoleAssignmentRequest) (*scopedaccessv1.GetScopedRoleAssignmentResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.assignments.GetScopedRoleAssignment(ctx, req)
}

// ListScopedRoleAssignments returns a paginated list of scoped role assignments.
func (c *Cache) ListScopedRoleAssignments(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.assignments.ListScopedRoleAssignments(ctx, req)
}

// ListScopedRoleAssignmentsWithFilter returns a paginated list of scoped role assignments filtered by the provided
// filter function. This method is used internally to implement access-controls on the ListScopedRoleAssignments grpc
// method.
func (c *Cache) ListScopedRoleAssignmentsWithFilter(ctx context.Context, req *scopedaccessv1.ListScopedRoleAssignmentsRequest, filter func(*scopedaccessv1.ScopedRoleAssignment) bool) (*scopedaccessv1.ListScopedRoleAssignmentsResponse, error) {
	state, err := c.read(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.assignments.ListScopedRoleAssignmentsWithFilter(ctx, req, filter)
}

// PopulatePinnedAssignmentsForUser populates the provided scope pin with all relevant assignments related to the
// given user. The provided pin must already have its Scope field set.
func (c *Cache) PopulatePinnedAssignmentsForUser(ctx context.Context, user string, pin *scopesv1.Pin) error {
	state, err := c.read(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return state.assignments.PopulatePinnedAssignmentsForUser(ctx, user, pin)
}

// Close stops cache background operations and causes future reads to fail. It is safe to call multiple times.
func (c *Cache) Close() error {
	c.cancel()

	// wait for done signal so that all reads with a "happens after" relation to close
	// fail consistently.
	<-c.done
	return nil
}

// update is the main background loop that handles cache setup, update, and retry.
func (c *Cache) update(ctx context.Context, retry retryutils.Retry) {
	defer func() {
		slog.InfoContext(ctx, "scoped access cache closing")
		c.rw.Lock()
		c.closed = true
		c.rw.Unlock()
		close(c.done)
	}()

	for {
		err := c.fetchAndWatch(ctx, retry)
		if ctx.Err() != nil {
			return
		}

		slog.WarnContext(ctx, "scoped access cache failed", "error", err)

		waitStart := time.Now()
		select {
		case <-retry.After():
			retry.Inc()
			slog.InfoContext(ctx, "attempting re-init of scoped access cache after delay", "delay", time.Since(waitStart))
		case <-ctx.Done():
			return
		}
	}
}

// fetchAndWatch attempts to establish a watcher with the upstream events service, populate the cache
// state, and process changes as they come in.
func (c *Cache) fetchAndWatch(ctx context.Context, retry retryutils.Retry) error {
	watcher, err := c.cfg.Events.NewWatcher(ctx, types.Watch{
		Name: "scoped-access-cache",
		Kinds: []types.WatchKind{
			{
				Kind: scopedaccess.KindScopedRole,
			},
			{
				Kind: scopedaccess.KindScopedRoleAssignment,
			},
		},
	})
	if err != nil {
		return trace.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	accessListWatcher, err := c.cfg.AccessListEvents.NewWatcher(ctx, types.Watch{
		Name: "scoped-access-list-cache",
		Kinds: []types.WatchKind{
			{Kind: types.KindAccessList},
			{Kind: types.KindAccessListMember},
		},
	})
	if err != nil {
		return trace.Wrap(err, "creating access list watcher")
	}
	defer accessListWatcher.Close()

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-watcher.Done():
		if err := watcher.Error(); err != nil {
			// watcher errors are expected if the watcher is closed before init completes.
			return trace.Errorf("watcher failed while waiting for init event: %w", err)
		}
		return trace.Errorf("watcher failed while waiting for init event")
	case <-time.After(retryutils.SeventhJitter(time.Minute)):
		return trace.Errorf("timed out waiting for init event from watcher")
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}

	select {
	case event := <-accessListWatcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-accessListWatcher.Done():
		if err := accessListWatcher.Error(); err != nil {
			// watcher errors are expected if the watcher is closed before init completes.
			return trace.Errorf("watcher failed while waiting for init event: %w", err)
		}
		return trace.Errorf("watcher failed while waiting for init event")
	case <-time.After(retryutils.SeventhJitter(time.Minute)):
		return trace.Errorf("timed out waiting for init event from watcher")
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}

	fetchStart := time.Now()
	state, accessListMaterializer, err := c.fetch(ctx)
	if err != nil {
		return trace.Errorf("failed to fetch initial state: %w", err)
	}

	fetchEnd := time.Now()
	slog.InfoContext(ctx, "scoped access cache fetched initial state", "elapsed", fetchEnd.Sub(fetchStart))

	c.rw.Lock()
	c.state = state
	c.ok = true
	c.rw.Unlock()

	slog.InfoContext(ctx, "scoped access cache successfully initialized")
	retry.Reset()

	// signal that init has completed
	c.initOnce.Do(func() {
		close(c.init)
	})

	// start processing and applying changes
	for {
		select {
		case event := <-watcher.Events():
			if err := processEvent(ctx, state, accessListMaterializer, event); err != nil {
				return trace.Errorf("failed to process event: %w", err)
			}
		case event := <-accessListWatcher.Events():
			if err := processEvent(ctx, state, accessListMaterializer, event); err != nil {
				return trace.Errorf("failed to process access list event: %w", err)
			}
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				// watcher errors are expected if the watcher is closed before init completes.
				return trace.Errorf("watcher failed during event processing: %w", err)
			}
			return trace.Errorf("watcher failed during event processing")
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// processEvent attempts to update the provided cache state with the given event.
func processEvent(ctx context.Context, state state, accessListMaterializer *accessListMaterializer, event types.Event) error {
	switch event.Type {
	case types.OpPut:
		switch item := event.Resource.(type) {
		case types.Resource153UnwrapperT[*scopedaccessv1.ScopedRole]:
			if err := state.roles.Put(item.UnwrapT()); err != nil {
				return trace.Errorf("failed to put scoped role %q: %w", item.UnwrapT().GetMetadata().GetName(), err)
			}
		case types.Resource153UnwrapperT[*scopedaccessv1.ScopedRoleAssignment]:
			if err := state.assignments.Put(item.UnwrapT()); err != nil {
				return trace.Errorf("failed to put scoped role assignment %q: %w", item.UnwrapT().GetMetadata().GetName(), err)
			}
		case *accesslist.AccessList:
			if err := accessListMaterializer.handleAccessListPut(state, item); err != nil {
				return trace.Wrap(err)
			}
		case *accesslist.AccessListMember:
			if err := accessListMaterializer.handleAccessListMemberPut(ctx, state, item); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("unexpected resource type %T in put event", event.Resource)
		}
	case types.OpDelete:
		switch event.Resource.GetKind() {
		case scopedaccess.KindScopedRole:
			state.roles.Delete(event.Resource.GetName())
		case scopedaccess.KindScopedRoleAssignment:
			state.assignments.Delete(event.Resource.GetName())
		case types.KindAccessList:
			if err := accessListMaterializer.handleAccessListDelete(event.Resource.GetName()); err != nil {
				return trace.Wrap(err)
			}
		case types.KindAccessListMember:
			listName := event.Resource.GetMetadata().Description
			if listName == "" {
				return trace.Errorf("missing scoped access list name in scoped access list member delete event description")
			}
			if err := accessListMaterializer.handleAccessListMemberDelete(ctx, state, listName, event.Resource.GetName()); err != nil {
				return trace.Wrap(err)
			}
		default:
			return trace.BadParameter("unexpected resource kind %q in event delete event", event.Resource.GetKind())
		}
	default:
		slog.WarnContext(ctx, "scoped access cache skipping unexpected event type", "event_type", event.Type)
		return nil
	}
	return nil
}

// read gets a read-ready cache state suitable for use in serving reads. the underlying state may
// be the actual primary cache state, or a ttl-cached image if the primary is unavailable.
func (c *Cache) read(ctx context.Context) (state, error) {
	c.rw.RLock()
	primary, ok, closed := c.state, c.ok, c.closed
	c.rw.RUnlock()

	if closed {
		// theoretically there's nothing wrong with reading *immediately* after close since cache reads are async/trailing
		// anyhow, but allowing reads of a closed cache might mask more serious bugs so its better to fail fast.
		return state{}, trace.Errorf("scoped access cache is closed")
	}

	if ok {
		// the primary cache is available, return it immediately.
		return primary, nil
	}

	// the cache is not ready, load a frozen readonly copy via ttl cache
	temp, err := utils.FnCacheGet(ctx, c.ttlCache, "access-cache", func(ctx context.Context) (state, error) {
		state, _, err := c.fetch(ctx)
		return state, trace.Wrap(err)
	})

	// primary may have been concurrently loaded. prefer using it if so.
	c.rw.RLock()
	primary, ok = c.state, c.ok
	c.rw.RUnlock()

	if ok {
		return primary, nil
	}

	return temp, trace.Wrap(err)
}

// fetch loads all currently available roles and assignments from the upstream and builds a cache state.
func (c *Cache) fetch(ctx context.Context) (state, *accessListMaterializer, error) {
	roleCache := roles.NewRoleCache()

	for role, err := range scopedutils.RangeScopedRoles(ctx, c.cfg.Reader, &scopedaccessv1.ListScopedRolesRequest{}) {
		if err != nil {
			return state{}, nil, trace.Wrap(err)
		}

		if err := roleCache.Put(role); err != nil {
			return state{}, nil, trace.Wrap(err)
		}
	}

	assignmentCache := assignments.NewAssignmentCache()

	for assignment, err := range scopedutils.RangeScopedRoleAssignments(ctx, c.cfg.Reader, &scopedaccessv1.ListScopedRoleAssignmentsRequest{}) {
		if err != nil {
			return state{}, nil, trace.Wrap(err)
		}

		if err := assignmentCache.Put(assignment); err != nil {
			return state{}, nil, trace.Wrap(err)
		}
	}

	s := state{
		roles:       roleCache,
		assignments: assignmentCache,
	}

	accessListMaterializer := newAccessListMaterializer(c.cfg.AccessListReader)
	materializeStart := time.Now()
	if err := accessListMaterializer.init(ctx, s); err != nil {
		return state{}, nil, trace.Wrap(err, "initializing access list materializer")

	}
	slog.InfoContext(ctx, "access list materializer initialized", "elapsed", time.Since(materializeStart))

	return s, accessListMaterializer, nil
}

func newAccessListMaterializer(upstream AccessListReader) *accessListMaterializer {
	return &accessListMaterializer{
		upstream:                upstream,
		allLists:                set.New[string](),
		directUserMembers:       make(map[string]set.Set[string]),
		directListMembers:       make(map[string]set.Set[string]),
		directOwnerUsers:        make(map[string]set.Set[string]),
		directOwnerLists:        make(map[string]set.Set[string]),
		ownerParentLists:        make(map[string]set.Set[string]),
		materializedAssignments: make(map[materializedAssignmentKey]string),
	}
}

type accessListMaterializer struct {
	upstream AccessListReader

	// all scoped access lists.
	allLists set.Set[string]
	// list -> all users that are direct members of list
	directUserMembers map[string]set.Set[string]
	// list -> all lists that are direct members of list
	directListMembers map[string]set.Set[string]
	// list -> all users that are direct owners of list
	directOwnerUsers map[string]set.Set[string]
	// list -> all lists that are direct owners of list
	directOwnerLists map[string]set.Set[string]
	// owner list -> lists that this list owns
	ownerParentLists map[string]set.Set[string]
	// list -> all nested member lists of that list
	nestedListMembers map[string]set.Set[string]
	// list -> (user -> count of how many times user is a member of (list and nestedListMembers[list]))
	nestedUserMembers map[string]map[string]int
	// list -> (user -> count of how many times user is an owner of list)
	nestedOwnerUsers map[string]map[string]int
	// materializedAssignmentKey -> id of materialized assignment
	materializedAssignments map[materializedAssignmentKey]string
}

type materializedAssignmentKey struct {
	list string
	user string
}

func (m *accessListMaterializer) init(ctx context.Context, state state) error {
	slog := slog.With(teleport.ComponentKey, teleport.Component("aclmaterializer"))
	slog.DebugContext(ctx, "Initializing access list materializer")

	slog.DebugContext(ctx, "Initializing lists")
	for list, err := range clientutils.Resources(ctx, m.upstream.ListAccessLists) {
		if err != nil {
			return trace.Wrap(err)
		}
		listName := list.GetName()
		m.allLists.Add(listName)
		m.directOwnerUsers[listName] = ownerUsersForList(list)
		m.directOwnerLists[listName] = ownerListsForList(list)
		for ownerList := range m.directOwnerLists[listName] {
			if parents, ok := m.ownerParentLists[ownerList]; ok {
				parents.Add(listName)
			} else {
				m.ownerParentLists[ownerList] = set.New(listName)
			}
		}
	}

	slog.DebugContext(ctx, "Initializing members")
	for member, err := range clientutils.Resources(ctx, m.upstream.ListAllAccessListMembers) {
		if err != nil {
			return trace.Wrap(err)
		}
		if member.IsUser() {
			list, member := member.Spec.AccessList, member.Spec.Name
			if directUserMembers, ok := m.directUserMembers[list]; ok {
				directUserMembers.Add(member)
			} else {
				m.directUserMembers[list] = set.New(member)
			}
		} else {
			parentList, memberList := member.Spec.AccessList, member.Spec.Name
			if directListMembers, ok := m.directListMembers[parentList]; ok {
				directListMembers.Add(memberList)
			} else {
				m.directListMembers[parentList] = set.New(memberList)
			}
		}
	}

	// Initialize empty sets for any lists with no members.
	for list := range m.allLists {
		if _, ok := m.directUserMembers[list]; !ok {
			m.directUserMembers[list] = set.New[string]()
		}
		if _, ok := m.directListMembers[list]; !ok {
			m.directListMembers[list] = set.New[string]()
		}
		if _, ok := m.directOwnerUsers[list]; !ok {
			m.directOwnerUsers[list] = set.New[string]()
		}
		if _, ok := m.directOwnerLists[list]; !ok {
			m.directOwnerLists[list] = set.New[string]()
		}
	}

	slog.DebugContext(ctx, "reinitNestedListMembers")
	m.reinitNestedListMembers()
	slog.DebugContext(ctx, "reinitNestedUserMembers")
	m.reinitNestedUserMembers()
	slog.DebugContext(ctx, "reinitNestedOwnerUsers")
	m.reinitNestedOwnerUsers()

	if err := m.rematerialize(ctx, state); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (m *accessListMaterializer) reinitNestedListMembers() {
	if m.nestedListMembers == nil {
		m.nestedListMembers = make(map[string]set.Set[string], len(m.directListMembers))
	} else {
		clear(m.nestedListMembers)
	}
	for list, directMembers := range m.directListMembers {
		m.nestedListMembers[list] = directMembers.Clone()
	}
	m.propagateNestedListMembers()
}

func (m *accessListMaterializer) propagateNestedListMembers() {
	changed := true
	for changed {
		changed = false
		for list, nestedMembers := range m.nestedListMembers {
			lenBefore := nestedMembers.Len()
			for nestedMember := range nestedMembers {
				nestedMembers.Union(m.nestedListMembers[nestedMember])
			}
			nestedMembers.Remove(list)
			changed = changed || nestedMembers.Len() != lenBefore
		}
	}
}

func (m *accessListMaterializer) reinitNestedUserMembers() {
	if m.nestedUserMembers == nil {
		m.nestedUserMembers = make(map[string]map[string]int, m.allLists.Len())
	} else {
		clear(m.nestedUserMembers)
	}
	for list := range m.allLists {
		counts := make(map[string]int)
		for user := range m.directUserMembers[list] {
			counts[user]++
		}
		for listMember := range m.nestedListMembers[list] {
			for user := range m.directUserMembers[listMember] {
				counts[user]++
			}
		}
		m.nestedUserMembers[list] = counts
	}
}

func (m *accessListMaterializer) reinitNestedOwnerUsers() {
	if m.nestedOwnerUsers == nil {
		m.nestedOwnerUsers = make(map[string]map[string]int, m.allLists.Len())
	} else {
		clear(m.nestedOwnerUsers)
	}
	for list := range m.allLists {
		m.reinitNestedOwnerUsersForList(list)
	}
}

func (m *accessListMaterializer) reinitNestedOwnerUsersForList(list string) {
	counts := make(map[string]int)
	for user := range m.directOwnerUsers[list] {
		counts[user]++
	}
	for ownerList := range m.directOwnerLists[list] {
		for user, count := range m.nestedUserMembers[ownerList] {
			counts[user] += count
		}
	}
	m.nestedOwnerUsers[list] = counts
}

func (m *accessListMaterializer) rematerialize(ctx context.Context, state state) error {
	slog.DebugContext(ctx, "rematerialize")
	unseenAssignments := maps.Clone(m.materializedAssignments)
	materializedCount := 0
	for listName := range m.allLists {
		list, err := m.upstream.GetAccessList(ctx, listName)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.Errorf("invariant violated, list %s not found", listName)
			}
			return trace.Wrap(err)
		}
		userSet := make(map[string]struct{})
		for user := range m.nestedUserMembers[listName] {
			userSet[user] = struct{}{}
		}
		for user := range m.nestedOwnerUsers[listName] {
			userSet[user] = struct{}{}
		}
		for user := range userSet {
			isMember := m.nestedUserMembers[listName][user] > 0
			isOwner := m.nestedOwnerUsers[listName][user] > 0
			if !isMember && !isOwner {
				continue
			}
			key := materializedAssignmentKey{
				list: listName,
				user: user,
			}
			delete(unseenAssignments, key)
			assignmentID, alreadyMaterialized := m.materializedAssignments[key]
			if !alreadyMaterialized {
				assignmentID = uuid.NewString()
				m.materializedAssignments[key] = assignmentID
			}
			materializedCount++
			assignment := materializeScopedRoleAssignment(user, list, assignmentID, isMember, isOwner)
			if err := state.assignments.Put(assignment); err != nil {
				return trace.Wrap(err, "putting materialized assignment for user %q in list %q into the cache", user, listName)
			}
		}
	}
	slog.DebugContext(ctx, "Materialized new scoped role assignments", "assignment_count", materializedCount)
	for assignmentKey, assignmentID := range unseenAssignments {
		state.assignments.Delete(assignmentID)
		delete(m.materializedAssignments, assignmentKey)
	}
	if len(unseenAssignments) > 0 {
		slog.DebugContext(ctx, "Deleted stale scoped role assignments", "assignment_count", len(unseenAssignments))
	}
	return nil
}

func (m *accessListMaterializer) handleAccessListPut(state state, list *accesslist.AccessList) error {
	listName := list.GetName()
	m.allLists.Add(listName)
	m.ensureListEntries(listName)
	m.updateOwnersForList(list)
	return trace.Wrap(m.rematerializeList(context.Background(), state, listName))
}

func (m *accessListMaterializer) handleAccessListDelete(listName string) error {
	if directUserMembers, ok := m.directUserMembers[listName]; ok {
		if directUserMembers.Len() > 0 {
			return trace.Errorf("invariant violated, access list %q still has direct user members while being deleted", listName)
		}
		delete(m.directUserMembers, listName)
	}
	if directListMembers, ok := m.directListMembers[listName]; ok {
		if directListMembers.Len() > 0 {
			return trace.Errorf("invariant violated, access list %q still has direct list members while being deleted", listName)
		}
		delete(m.directListMembers, listName)
	}
	if nestedUserMembers, ok := m.nestedUserMembers[listName]; ok {
		if len(nestedUserMembers) > 0 {
			return trace.Errorf("invariant violated, access list %q still has nested user members while being deleted", listName)
		}
		delete(m.nestedUserMembers, listName)
	}
	if nestedListMembers, ok := m.nestedListMembers[listName]; ok {
		if nestedListMembers.Len() > 0 {
			return trace.Errorf("invariant violated, access list %q still has nested list members while being deleted", listName)
		}
		delete(m.nestedListMembers, listName)
	}
	if directOwnerUsers, ok := m.directOwnerUsers[listName]; ok {
		if directOwnerUsers.Len() > 0 {
			return trace.Errorf("invariant violated, access list %q still has direct owners while being deleted", listName)
		}
		delete(m.directOwnerUsers, listName)
	}
	if directOwnerLists, ok := m.directOwnerLists[listName]; ok {
		for ownerList := range directOwnerLists {
			if parents, ok := m.ownerParentLists[ownerList]; ok {
				parents.Remove(listName)
				if parents.Len() == 0 {
					delete(m.ownerParentLists, ownerList)
				}
			}
		}
		delete(m.directOwnerLists, listName)
	}
	if nestedOwnerUsers, ok := m.nestedOwnerUsers[listName]; ok {
		if len(nestedOwnerUsers) > 0 {
			return trace.Errorf("invariant violated, access list %q still has nested owner members while being deleted", listName)
		}
		delete(m.nestedOwnerUsers, listName)
	}
	if parentLists, ok := m.ownerParentLists[listName]; ok && parentLists.Len() > 0 {
		return trace.Errorf("invariant violated, access list %q still owns parent lists while being deleted", listName)
	}
	for key := range m.materializedAssignments {
		if key.list == listName {
			return trace.Errorf("invariant violated, access list %q still has materialized scoped role assignment while being deleted", listName)
		}
	}
	m.allLists.Remove(listName)
	return nil
}

func (m *accessListMaterializer) handleAccessListMemberPut(ctx context.Context, state state, member *accesslist.AccessListMember) error {
	if member.IsUser() {
		return m.handleAccessListUserMemberPut(ctx, state, member)
	}
	return m.handleAccessListListMemberPut(ctx, state, member)
}

func (m *accessListMaterializer) handleAccessListMemberDelete(ctx context.Context, state state, listName, memberName string) error {
	maybeUser := m.directUserMembers[listName].Contains(memberName)
	maybeList := m.directListMembers[listName].Contains(memberName)
	switch {
	case !maybeUser && !maybeList:
		// This is already not a member of the list.
		return nil
	case maybeUser && maybeList:
		// The delete event didn't give us enough information...
		// TODO(nklaassen): figure out how to handle this.
		return trace.Errorf("deleted access list member could be a list or a user, both exist with the same name")
	case maybeUser:
		return m.handleAccessListUserMemberDelete(ctx, state, listName, memberName)
	case maybeList:
		return m.handleAccessListListMemberDelete(ctx, state, listName, memberName)
	}
	panic("unreachable")
}

func (m *accessListMaterializer) handleAccessListUserMemberPut(ctx context.Context, state state, member *accesslist.AccessListMember) error {
	listName, user := member.Spec.AccessList, member.Spec.Name
	if m.directUserMembers[listName].Contains(user) {
		// User is already a direct member of this list, nothing to do.
		return nil
	}

	// First update direct membership.
	m.directUserMembers[listName].Add(user)

	// Then update nested memberships.
	m.nestedUserMembers[listName][user]++
	membershipDeltaLists := []string{listName}
	for otherList, otherListMembers := range m.nestedListMembers {
		if otherList == listName || !otherListMembers.Contains(listName) {
			// The list this user was just added to is not a nested member of otherList
			continue
		}
		// User is now a nested member of this list for one more reason (they are newly a direct member of listName)
		m.nestedUserMembers[otherList][user]++
		membershipDeltaLists = append(membershipDeltaLists, otherList)
	}

	affectedLists := set.New[string]()
	for _, listName := range membershipDeltaLists {
		affectedLists.Add(listName)
		for parentList := range m.ownerParentLists[listName] {
			m.ensureListEntries(parentList)
			m.nestedOwnerUsers[parentList][user]++
			affectedLists.Add(parentList)
		}
	}

	for listName := range affectedLists {
		if err := m.rematerializeList(ctx, state, listName); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (m *accessListMaterializer) handleAccessListUserMemberDelete(ctx context.Context, state state, listName, user string) error {
	if !m.directUserMembers[listName].Contains(user) {
		// User is somehow already not a direct member of this list, nothing to do.
		return nil
	}

	// First update direct membership.
	m.directUserMembers[listName].Remove(user)

	// Then update nested memberships.
	var removedMemberships []string
	membershipDeltaLists := []string{listName}
	m.nestedUserMembers[listName][user]--
	if m.nestedUserMembers[listName][user] == 0 {
		delete(m.nestedUserMembers[listName], user)
		removedMemberships = append(removedMemberships, listName)
	}
	for otherList, otherListMembers := range m.nestedListMembers {
		if otherList == listName || !otherListMembers.Contains(listName) {
			// The list this user was just removed from is not a nested member of otherList
			continue
		}
		// User is now a nested member of this list for one fewer reasons (they are no longer a direct member of listName)
		m.nestedUserMembers[otherList][user]--
		membershipDeltaLists = append(membershipDeltaLists, otherList)
		if m.nestedUserMembers[otherList][user] == 0 {
			delete(m.nestedUserMembers[otherList], user)
			removedMemberships = append(removedMemberships, otherList)
		}
	}

	affectedLists := set.New[string]()
	for _, listName := range membershipDeltaLists {
		affectedLists.Add(listName)
		for parentList := range m.ownerParentLists[listName] {
			if m.nestedOwnerUsers[parentList][user] > 0 {
				m.nestedOwnerUsers[parentList][user]--
				if m.nestedOwnerUsers[parentList][user] == 0 {
					delete(m.nestedOwnerUsers[parentList], user)
				}
			}
			affectedLists.Add(parentList)
		}
	}

	// Then make sure to remove materialized assignments for all lists user is no longer a nested member/owner of.
	for _, listName := range removedMemberships {
		assignmentKey := materializedAssignmentKey{list: listName, user: user}
		currentAssignmentID, assignmentCurrentlyMaterialized := m.materializedAssignments[assignmentKey]
		if !assignmentCurrentlyMaterialized {
			continue
		}
		if m.nestedOwnerUsers[listName][user] > 0 {
			continue
		}
		state.assignments.Delete(currentAssignmentID)
		delete(m.materializedAssignments, assignmentKey)
	}

	for listName := range affectedLists {
		if err := m.rematerializeList(ctx, state, listName); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (m *accessListMaterializer) handleAccessListListMemberPut(ctx context.Context, state state, member *accesslist.AccessListMember) error {
	parentListName, memberListName := member.Spec.AccessList, member.Spec.Name
	if m.directListMembers[parentListName].Contains(memberListName) {
		// list is already a direct member of parent list, nothing to do.
		return nil
	}

	// First update direct membership.
	m.directListMembers[parentListName].Add(memberListName)

	// Then update nested list memberships.
	m.nestedListMembers[parentListName].Add(memberListName)
	m.propagateNestedListMembers()

	// Then update nested user memberships.
	m.reinitNestedUserMembers()
	m.reinitNestedOwnerUsers()

	// Then ensure there is a materialized assignment for all (list, user) pairs.
	return trace.Wrap(m.rematerialize(ctx, state))
}

func (m *accessListMaterializer) handleAccessListListMemberDelete(ctx context.Context, state state, parentListName, memberListName string) error {
	if !m.directListMembers[parentListName].Contains(memberListName) {
		// list is somehow already not a direct member of parent list, nothing to do.
		return nil
	}

	// First update direct membership.
	m.directListMembers[parentListName].Remove(memberListName)

	// Then update nested list memberships.
	m.reinitNestedListMembers()

	// Then update nested user memberships.
	m.reinitNestedUserMembers()
	m.reinitNestedOwnerUsers()

	// Then ensure there is a materialized assignment for all (list, user)
	// pairs and dangling assignements are cleaned up.
	return trace.Wrap(m.rematerialize(ctx, state))
}

func materializeScopedRoleAssignment(user string, list *accesslist.AccessList, uuid string, isMember bool, isOwner bool) *scopedaccessv1.ScopedRoleAssignment {
	roleGrants := make([]accesslist.ScopedRoleGrant, 0, len(list.Spec.Grants.ScopedRoles)+len(list.Spec.OwnerGrants.ScopedRoles))
	if isMember {
		roleGrants = append(roleGrants, list.Spec.Grants.ScopedRoles...)
	}
	if isOwner {
		roleGrants = append(roleGrants, list.Spec.OwnerGrants.ScopedRoles...)
	}
	assignment := &scopedaccessv1.ScopedRoleAssignment{
		Kind:    scopedaccess.KindScopedRoleAssignment,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: uuid,
		},
		Scope: "/",
		Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
			User:        user,
			Assignments: make([]*scopedaccessv1.Assignment, 0, len(roleGrants)),
		},
	}
	for _, grant := range roleGrants {
		assignment.Spec.Assignments = append(assignment.Spec.Assignments, &scopedaccessv1.Assignment{
			Role:  grant.Role,
			Scope: grant.Scope,
		})
	}
	return assignment
}

func ownerUsersForList(list *accesslist.AccessList) set.Set[string] {
	owners := set.New[string]()
	for _, owner := range list.Spec.Owners {
		if owner.IsMembershipKindUser() {
			owners.Add(owner.Name)
		}
	}
	return owners
}

func ownerListsForList(list *accesslist.AccessList) set.Set[string] {
	owners := set.New[string]()
	for _, owner := range list.Spec.Owners {
		if !owner.IsMembershipKindUser() {
			owners.Add(owner.Name)
		}
	}
	return owners
}

func (m *accessListMaterializer) ensureListEntries(listName string) {
	if _, ok := m.directListMembers[listName]; !ok {
		m.directListMembers[listName] = set.New[string]()
	}
	if _, ok := m.directUserMembers[listName]; !ok {
		m.directUserMembers[listName] = set.New[string]()
	}
	if _, ok := m.nestedListMembers[listName]; !ok {
		m.nestedListMembers[listName] = set.New[string]()
	}
	if _, ok := m.nestedUserMembers[listName]; !ok {
		m.nestedUserMembers[listName] = make(map[string]int)
	}
	if _, ok := m.directOwnerUsers[listName]; !ok {
		m.directOwnerUsers[listName] = set.New[string]()
	}
	if _, ok := m.directOwnerLists[listName]; !ok {
		m.directOwnerLists[listName] = set.New[string]()
	}
	if _, ok := m.nestedOwnerUsers[listName]; !ok {
		m.nestedOwnerUsers[listName] = make(map[string]int)
	}
}

func (m *accessListMaterializer) updateOwnersForList(list *accesslist.AccessList) {
	listName := list.GetName()
	oldOwnerLists := m.directOwnerLists[listName]

	newOwnerUsers := ownerUsersForList(list)
	newOwnerLists := ownerListsForList(list)

	for ownerList := range oldOwnerLists {
		if newOwnerLists.Contains(ownerList) {
			continue
		}
		if parents, ok := m.ownerParentLists[ownerList]; ok {
			parents.Remove(listName)
			if parents.Len() == 0 {
				delete(m.ownerParentLists, ownerList)
			}
		}
	}
	for ownerList := range newOwnerLists {
		if oldOwnerLists.Contains(ownerList) {
			continue
		}
		if parents, ok := m.ownerParentLists[ownerList]; ok {
			parents.Add(listName)
		} else {
			m.ownerParentLists[ownerList] = set.New(listName)
		}
	}

	m.directOwnerUsers[listName] = newOwnerUsers
	m.directOwnerLists[listName] = newOwnerLists
	m.reinitNestedOwnerUsersForList(listName)
}

func (m *accessListMaterializer) rematerializeList(ctx context.Context, state state, listName string) error {
	list, err := m.upstream.GetAccessList(ctx, listName)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Errorf("invariant violated, list %s not found", listName)
		}
		return trace.Wrap(err)
	}

	users := make(map[string]struct{})
	for user := range m.nestedUserMembers[listName] {
		users[user] = struct{}{}
	}
	for user := range m.nestedOwnerUsers[listName] {
		users[user] = struct{}{}
	}

	existing := make(map[materializedAssignmentKey]struct{})
	for key := range m.materializedAssignments {
		if key.list == listName {
			existing[key] = struct{}{}
		}
	}

	for user := range users {
		isMember := m.nestedUserMembers[listName][user] > 0
		isOwner := m.nestedOwnerUsers[listName][user] > 0
		if !isMember && !isOwner {
			continue
		}
		key := materializedAssignmentKey{list: listName, user: user}
		delete(existing, key)
		assignmentID, alreadyMaterialized := m.materializedAssignments[key]
		if !alreadyMaterialized {
			assignmentID = uuid.NewString()
			m.materializedAssignments[key] = assignmentID
		}
		assignment := materializeScopedRoleAssignment(user, list, assignmentID, isMember, isOwner)
		if err := state.assignments.Put(assignment); err != nil {
			return trace.Wrap(err, "putting updated materialized assignment for user %q in list %q into assignment cache", user, listName)
		}
	}

	for key := range existing {
		if m.nestedUserMembers[listName][key.user] > 0 || m.nestedOwnerUsers[listName][key.user] > 0 {
			continue
		}
		state.assignments.Delete(m.materializedAssignments[key])
		delete(m.materializedAssignments, key)
	}

	return nil
}
