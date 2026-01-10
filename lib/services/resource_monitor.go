//
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

package services

import (
	"context"
	"iter"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ResourceMonitorConfig holds the dependencies required by a [ResourceMonitor].
type ResourceMonitorConfig[T any] struct {
	// Kind dictates the monitored resource's kind.
	Kind string
	// Key determines the unique key for a given resource.
	Key func(T) string
	// ResourceHeaderKey determines the unique key for a header that may be
	// emitted during a delete event. If delete events emit the full resource
	// instead of the [types.ResourceHeader] this may be omitted and Key will be used.
	ResourceHeaderKey func(*types.ResourceHeader) string
	// CurrentResources an iterator that provides the current set of resources that exist.
	// This iterator may be called multiple times if the state needs to be reloaded.
	CurrentResources func(context.Context) iter.Seq2[T, error]
	// Events is the source watchers are created from to observe events for a particular
	// resource.
	Events types.Events
	// Matches determines if a resource should be processed or not.
	Matches func(T) bool
	// CompareResources determines if two resources are equivalent. This is used to determine
	// whether a change has been made to an existing resource.
	CompareResources func(T, T) int
	// AllowOriginChanges determines whether changes to a resources origin should be allowed.
	AllowOriginChanges bool
	// DeleteResource is called in response to a [types.OpDelete] event being received for an existing resource.
	DeleteResource func(context.Context, T) error
	// CreateResource is called in response to a [types.OpPut] event being received for a new resource.
	CreateResource func(context.Context, T) error
	// UpdateResource is called in response to a [types.OpPut] event being received for an existing resource.
	// Both the existing resource and the updated resource are provided for inspection.
	UpdateResource func(ctx context.Context, new, existing T) error
}

// ResourceMonitor tracks that cluster state for a particular backend resource. It monitors the
// cluster event stream to maintain the current view of a resource set. Any changes to the
// state can be watched by supplying callbacks for creat, update, or delete of a resource.
//
// The [ResourceMonitor] provides the same functionality of a [GenericWatcher] that feeds all changes to
// a [GenericReconciler] in a more performant manner.
type ResourceMonitor[T any] struct {
	cfg    ResourceMonitorConfig[T]
	logger *slog.Logger

	retry retryutils.Retry

	initChan chan struct{}
	initOnce sync.Once

	closed    chan struct{}
	closeOnce sync.Once

	mu        sync.Mutex
	resources map[string]T
}

// NewResourceMonitor creates a [ResourceMonitor] from the provided configuration.
func NewResourceMonitor[T any](cfg ResourceMonitorConfig[T]) (*ResourceMonitor[T], error) {
	switch {
	case cfg.Kind == "":
		return nil, trace.BadParameter("ResourceMonitor kind not provided")
	case cfg.CurrentResources == nil:
		return nil, trace.BadParameter("ResourceMonitor current resources not provided")
	case cfg.Events == nil:
		return nil, trace.BadParameter("ResourceMonitor events not provided")
	case cfg.DeleteResource == nil:
		return nil, trace.BadParameter("ResourceMonitor delete resource not provided")
	case cfg.CreateResource == nil:
		return nil, trace.BadParameter("ResourceMonitor create resource not provided")
	case cfg.UpdateResource == nil:
		return nil, trace.BadParameter("ResourceMonitor update resource not provided")
	case cfg.Matches == nil:
		return nil, trace.BadParameter("ResourceMonitor matches not provided")
	case cfg.CompareResources == nil:
		return nil, trace.BadParameter("ResourceMonitor compare resources not provided")
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(defaults.MaxWatcherBackoff / 10),
		Step:   defaults.MaxWatcherBackoff / 5,
		Max:    defaults.MaxWatcherBackoff,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ResourceMonitor[T]{
		cfg:      cfg,
		initChan: make(chan struct{}),
		closed:   make(chan struct{}),
		retry:    retry,
		logger:   slog.With(teleport.ComponentKey, "resourcemonitor", "kind", cfg.Kind),
	}, nil
}

// Close terminates an ongoing [Run] operation.
func (p *ResourceMonitor[T]) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
	})

	return nil
}

func (p *ResourceMonitor[T]) Initialized() <-chan struct{} {
	return p.initChan
}

// Run monitors resources until the context is canceled or [Close] is called.
func (p *ResourceMonitor[T]) Run(ctx context.Context) {
	for {
		err := p.watch(ctx)

		select {
		case <-p.closed:
			return
		case <-ctx.Done():
			return
		case <-p.retry.After():
			p.retry.Inc()
		}

		if err != nil {
			p.logger.WarnContext(ctx, "Restart watch on error", "error", err)
		}
	}
}

// watch monitors new resource updates, maintains a local view and broadcasts
// notifications to connected agents.
func (p *ResourceMonitor[T]) watch(ctx context.Context) error {
	watch := types.Watch{
		Name:            "resource.monitor",
		MetricComponent: "resource.monitor",
		Kinds:           []types.WatchKind{{Kind: p.cfg.Kind}},
	}

	watcher, err := p.cfg.Events.NewWatcher(ctx, watch)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	// before fetch, make sure watcher is synced by receiving init event,
	// to avoid the scenario:
	// 1. Cache process:   w = NewWatcher()
	// 2. Cache process:   c.fetch()
	// 3. Backend process: addItem()
	// 4. Cache process:   <- w.Events()
	//
	// If there is a way that NewWatcher() on line 1 could
	// return without subscription established first,
	// Code line 3 could execute and line 4 could miss event,
	// wrapping up with out of sync replica.
	// To avoid this, before doing fetch,
	// cache process makes sure the connection is established
	// by receiving init event first.
	select {
	case <-p.closed:
		return nil
	case <-watcher.Done():
		return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
	case <-ctx.Done():
		return trace.ConnectionProblem(ctx.Err(), "context is closing")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	}

	if err := p.getCurrentResources(ctx); err != nil {
		return trace.Wrap(err)
	}

	for {
		select {
		case <-p.closed:
			return nil
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher is closed: %v", watcher.Error())
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context is closing")
		case event := <-watcher.Events():
			p.handleEvent(ctx, event)
		}
	}
}

func (p *ResourceMonitor[T]) getCurrentResources(ctx context.Context) error {
	// Collect the current set of resources to compare against
	// the previously known set of resources.
	resources := map[string]T{}
	for resource, err := range p.cfg.CurrentResources(ctx) {
		if err != nil {
			return trace.Wrap(err)
		}

		key := p.cfg.Key(resource)
		resources[key] = resource
	}

	// Determine which resources used to exist but no longer do.
	// Note, that deletion does not happen directly to avoid
	// performing operations while holding the lock.
	var toDelete map[string]T
	p.mu.Lock()
	for key, resource := range p.resources {
		if _, ok := resources[key]; !ok {
			if toDelete == nil {
				toDelete = map[string]T{}
			}
			toDelete[key] = resource
		}
	}

	p.initOnce.Do(func() { close(p.initChan) })
	p.resources = resources
	p.mu.Unlock()

	// Delete any resources that have been marked for removal.
	for key, resource := range toDelete {
		p.logger.InfoContext(ctx, "Resource was removed, deleting", "name", key)
		if err := p.cfg.DeleteResource(ctx, resource); err != nil {
			level := slog.LevelWarn
			if trace.IsNotFound(err) {
				level = logutils.TraceLevel
			}
			p.logger.Log(ctx, level, "Failed to delete resource", "name", key, "err", err)
		}
	}

	return nil
}

func (p *ResourceMonitor[T]) handleEvent(ctx context.Context, event types.Event) {
	switch event.Type {
	case types.OpDelete:
		var key string
		switch res := event.Resource.(type) {
		case T:
			key = p.cfg.Key(res)
		case interface{ UnwrapT() T }:
			key = p.cfg.Key(res.UnwrapT())
		case *types.ResourceHeader:
			key = p.cfg.ResourceHeaderKey(res)
		}

		p.mu.Lock()
		deleted, ok := p.resources[key]
		delete(p.resources, key)
		p.mu.Unlock()
		if !p.cfg.Matches(deleted) && !ok {
			return
		}

		p.logger.InfoContext(ctx, "Resource was removed, deleting", "name", key)
		if err := p.cfg.DeleteResource(ctx, deleted); err != nil {
			level := slog.LevelWarn
			if trace.IsNotFound(err) {
				level = logutils.TraceLevel
			}
			p.logger.Log(ctx, level, "Failed to delete resource", "name", key, "err", err)
		}
	case types.OpPut:
		var key string
		var t T
		switch res := event.Resource.(type) {
		case T:
			key = p.cfg.Key(res)
			t = res
		case interface{ UnwrapT() T }:
			key = p.cfg.Key(res.UnwrapT())
			t = res.UnwrapT()
		default:
			p.logger.WarnContext(ctx, "Unexpected resource type", "type", logutils.TypeAttr(event.Resource))
			return
		}

		matches := p.cfg.Matches(t)

		p.mu.Lock()
		existing, ok := p.resources[key]
		if matches {
			p.resources[key] = t
		}
		p.mu.Unlock()

		if ok {
			if err := p.updateResource(ctx, matches, key, existing, t); err != nil {
				p.logger.Log(ctx, logutils.TraceLevel, "Failed to update resource", "name", key, "err", err)
			}
		} else {
			if err := p.createResource(ctx, matches, key, t); err != nil {
				p.logger.Log(ctx, logutils.TraceLevel, "Failed to create resource", "name", key, "err", err)
			}
		}
	default:
		p.logger.WarnContext(ctx, "unknown event type received", "type", event.Type)
	}
}

func (p *ResourceMonitor[T]) createResource(ctx context.Context, matches bool, key string, new T) error {
	if !matches {
		p.logger.DebugContext(ctx, "New resource doesn't match, not creating", "name", key)
		return nil
	}

	p.logger.InfoContext(ctx, "New resource matches, creating", "name", key)

	return trace.Wrap(p.cfg.CreateResource(ctx, new))
}

func (p *ResourceMonitor[T]) updateResource(ctx context.Context, matches bool, key string, current, new T) error {
	if !p.cfg.AllowOriginChanges {
		currentOrigin, err := types.GetOrigin(current)
		if err != nil {
			return trace.Wrap(err)
		}
		newOrigin, err := types.GetOrigin(new)
		if err != nil {
			return trace.Wrap(err)
		}
		if currentOrigin != newOrigin {
			p.logger.WarnContext(ctx, "New resource has different origin, not updating",
				"name", key, "new_origin", newOrigin, "existing_origin", currentOrigin)
			return nil
		}
	}

	if p.cfg.CompareResources(new, current) == 0 {
		p.logger.Log(ctx, logutils.TraceLevel, "Existing resource is already registered", "name", key, "new_resource", new, "current", current)
		return nil
	}

	if matches {
		p.logger.InfoContext(ctx, "Existing resource updated, updating", "name", key)
		return trace.Wrap(p.cfg.UpdateResource(ctx, new, current))
	}

	p.logger.InfoContext(ctx, "Existing resource updated and no longer matches, deleting", "name", key)
	if err := p.cfg.DeleteResource(ctx, current); err != nil {
		p.logger.Log(ctx, logutils.TraceLevel, "Failed to delete resource", "name", key, "err", err)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	return nil
}
