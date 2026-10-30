// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package internal

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/set"
)

// ErrNoAccessRequest is returned when no approved access request was found
// for the given resource.
var ErrNoAccessRequest = &trace.NotFoundError{
	Message: "No approved access request found for the given resource.",
}

// Client is used to list and watch access requests.
type Client interface {
	types.Events

	GetAccessRequests(context.Context, types.AccessRequestFilter) ([]types.AccessRequest, error)
}

// AccessRequestTracker tracks resource-based access requests and matches
// them to resources, allowing us to assume a different access request per
// VNet tunnel.
type AccessRequestTracker struct {
	client      Client
	logger      *slog.Logger
	user        string
	clusterName string

	approvedAccessRequestChan chan<- types.AccessRequest

	mu sync.Mutex // guards requests and byKind.
	// requests contains only approved access requests, keyed by ResourceID.
	requests map[string]map[string]types.AccessRequest
	// byKind tracks the approved access requests that include a resource of a given kind.
	byKind map[string]set.Set[string]
}

// AccessRequestTrackerConfig contains the configuration for AccessRequestTracker.
type AccessRequestTrackerConfig struct {
	// Client that will be used to list and watch access requests.
	Client Client

	// User who's access requests will be watched.
	User string

	// Logger to which errors and messages will be written.
	Logger *slog.Logger

	// ClusterName identifies this Teleport cluster. Resources in trusted
	// clusters are not supported.
	ClusterName string

	// ApprovedAccessRequestChan is a channel to which the bot/user's approved
	// access requests will be sent when they have been processed by the tracker.
	// This is useful for synchronization in tests. Sends to this channel are
	// blocking, so it's the callers responsibility to either provide a buffered
	// channel or drain it regularly.
	ApprovedAccessRequestChan chan<- types.AccessRequest
}

// NewAccessRequestTracker creates a new AccessRequestTracker with the given
// configuration.
func NewAccessRequestTracker(cfg AccessRequestTrackerConfig) (*AccessRequestTracker, error) {
	if cfg.Client == nil {
		return nil, trace.BadParameter("Client is required")
	}
	if cfg.User == "" {
		return nil, trace.BadParameter("User is required")
	}
	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("ClusterName is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &AccessRequestTracker{
		client:                    cfg.Client,
		user:                      cfg.User,
		clusterName:               cfg.ClusterName,
		logger:                    cfg.Logger,
		approvedAccessRequestChan: cfg.ApprovedAccessRequestChan,
		requests:                  make(map[string]map[string]types.AccessRequest),
		byKind:                    make(map[string]set.Set[string]),
	}, nil
}

// RunWatcher watches the user's access requests, until the given context is done.
func (t *AccessRequestTracker) RunWatcher(ctx context.Context) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:     0,
		Driver:    retryutils.NewExponentialDriver(150 * time.Millisecond),
		Max:       3 * time.Minute,
		Jitter:    retryutils.HalfJitter,
		AutoReset: 2,
	})
	if err != nil {
		return trace.Wrap(err, "creating retry")
	}

	for {
		err := t.runWatcher(ctx)
		if ctx.Err() != nil {
			return err
		}

		retry.Inc()
		wait := retry.Duration()
		t.logger.ErrorContext(ctx, "Access request watcher failed; waiting to retry", "error", err, "wait", wait)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (t *AccessRequestTracker) runWatcher(ctx context.Context) error {
	filter := &types.AccessRequestFilter{User: t.user}
	watcher, err := t.client.NewWatcher(ctx, types.Watch{
		Name: "access-requests-watcher",
		Kinds: []types.WatchKind{
			{
				Kind:   types.KindAccessRequest,
				Filter: filter.IntoMap(),
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "creating access request watcher")
	}
	defer watcher.Close()

	// Wait for OpInit event so that returned watcher is ready.
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.Errorf("expected initial event to be OpInit, got: %s", event.Type)
		}
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	}

	requests, err := t.client.GetAccessRequests(ctx, types.AccessRequestFilter{
		User:  t.user,
		State: types.RequestState_APPROVED,
	})
	if err != nil {
		return trace.Wrap(err, "listing access requests")
	}
	t.replaceRequests(ctx, requests)

	// Consume the event stream.
	for {
		select {
		case event := <-watcher.Events():
			switch event.Type {
			case types.OpPut:
				req, ok := event.Resource.(types.AccessRequest)
				if !ok {
					t.logger.WarnContext(ctx, "Unexpected access request type", "access_request_type", fmt.Sprintf("%T", event.Resource))
					continue
				}

				if req.GetState() == types.RequestState_APPROVED {
					t.putRequest(ctx, req)
				} else {
					t.deleteRequest(req.GetName())
				}
			case types.OpDelete:
				if event.Resource == nil {
					t.logger.WarnContext(ctx, "Access request delete event is missing a resource")
					continue
				}
				t.deleteRequest(event.Resource.GetName())
			default:
				t.logger.WarnContext(ctx, "Ignoring unknown event type", "event_type", event.Type)
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		}
	}
}

func (t *AccessRequestTracker) replaceRequests(ctx context.Context, requests []types.AccessRequest) {
	requestsByResource := make(map[string]map[string]types.AccessRequest)
	byKind := make(map[string]set.Set[string])

	for _, req := range requests {
		if req.GetState() != types.RequestState_APPROVED {
			continue
		}
		addRequest(requestsByResource, byKind, req)
	}

	t.mu.Lock()
	t.requests = requestsByResource
	t.byKind = byKind
	t.mu.Unlock()

	for _, req := range requests {
		if req.GetState() == types.RequestState_APPROVED {
			t.notifyApproved(ctx, req)
		}
	}
}

// ApprovedRequest returns the approved access request for the given resource
// or ErrNoAccessRequest if there is none.
func (t *AccessRequestTracker) ApprovedRequest(kind, name string) (types.AccessRequest, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	const clockSkewTolerance = 30 * time.Second

	matches := slices.Collect(maps.Values(t.requests[t.mapKey(kind, name)]))

	// Filter out any access requests that are expired or cannot be assumed yet.
	matches = slices.DeleteFunc(matches, func(req types.AccessRequest) bool {
		expiryTime := req.GetAccessExpiry()
		expired := time.Now().Add(-clockSkewTolerance).After(expiryTime)

		startTime := req.GetAssumeStartTime()
		notReady := startTime != nil && time.Now().
			Add(clockSkewTolerance).
			Before(*startTime)

		return expired || notReady
	})

	if len(matches) == 0 {
		return nil, ErrNoAccessRequest
	}

	// Take the request that expires last.
	sort.Slice(matches, func(a, b int) bool {
		return matches[a].GetAccessExpiry().
			After(matches[b].GetAccessExpiry())
	})
	return matches[0], nil
}

// HasApprovedRequestWithKind is a pre-flight check of whether we have any
// approved access requests for resources of the given kind. This allows us to
// skip falling back to "search as roles" if the resulting resources could never
// be used anyway.
func (t *AccessRequestTracker) HasApprovedRequestWithKind(kind string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.byKind[normalizeKind(kind)].Len() != 0
}

func normalizeKind(kind string) string {
	switch kind {
	case types.KindAppServer:
		return types.KindApp
	case types.KindDatabaseServer:
		return types.KindDatabase
	default:
		return kind
	}
}

func (t *AccessRequestTracker) mapKey(kind, name string) string {
	// TODO(boxofrad): Support sub-resources, for Kubernetes access when that is
	// supported by VNet.
	return types.ResourceIDToString(types.ResourceID{
		ClusterName: t.clusterName,
		Kind:        normalizeKind(kind),
		Name:        name,
	})
}

func (t *AccessRequestTracker) putRequest(ctx context.Context, req types.AccessRequest) {
	t.mu.Lock()
	addRequest(t.requests, t.byKind, req)
	t.mu.Unlock()

	t.notifyApproved(ctx, req)
}

func addRequest(requests map[string]map[string]types.AccessRequest, byKind map[string]set.Set[string], req types.AccessRequest) {
	for _, id := range req.GetAllRequestedResourceIDs() {
		resourceID := id.GetResourceID()

		key := types.ResourceIDToString(resourceID)
		matches, ok := requests[key]
		if !ok {
			matches = make(map[string]types.AccessRequest)
			requests[key] = matches
		}
		matches[req.GetName()] = req

		requestsForKind, ok := byKind[resourceID.Kind]
		if !ok {
			requestsForKind = set.New[string]()
			byKind[resourceID.Kind] = requestsForKind
		}
		requestsForKind.Add(req.GetName())
	}
}

func (t *AccessRequestTracker) notifyApproved(ctx context.Context, req types.AccessRequest) {
	if t.approvedAccessRequestChan != nil {
		select {
		case t.approvedAccessRequestChan <- req:
		case <-ctx.Done():
		}
	}
}

func (t *AccessRequestTracker) deleteRequest(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for key, matches := range t.requests {
		delete(matches, name)
		if len(matches) == 0 {
			delete(t.requests, key)
		}
	}
	for kind, requests := range t.byKind {
		requests.Remove(name)
		if requests.Len() == 0 {
			delete(t.byKind, kind)
		}
	}
}
