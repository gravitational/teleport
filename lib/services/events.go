/*
Copyright 2018-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
)

// Watch sets up watch on the event
type Watch struct {
	// Name is used for debugging purposes
	Name string

	// Kinds specifies kinds of objects to watch
	// and whether to load secret data for them
	Kinds []WatchKind

	// QueueSize is an optional queue size
	QueueSize int

	// MetricComponent is used for reporting
	MetricComponent string
}

// WatchKind specifies resource kind to watch
type WatchKind struct {
	// Kind is a resource kind to watch
	Kind string
	// Name is an optional specific resource type to watch,
	// if specified only the events with a specific resource
	// name will be sent
	Name string
	// LoadSecrets specifies whether to load secrets
	LoadSecrets bool
	// Filter supplies custom event filter parameters that differ by
	// resource (e.g. "state":"pending" for access requests).
	Filter map[string]string
}

// Matches attempts to determine if the supplied event matches
// this WatchKind.  If the WatchKind is misconfigured, or the
// event appears malformed, an error is returned.
func (kind WatchKind) Matches(e Event) (bool, error) {
	if kind.Kind != e.Resource.GetKind() {
		return false, nil
	}
	if kind.Name != "" && kind.Name != e.Resource.GetName() {
		return false, nil
	}
	// we don't have a good model for filtering non-put events,
	// so only apply filters to OpPut events.
	if len(kind.Filter) > 0 && e.Type == backend.OpPut {
		// Currently only access request make use of filters,
		// so expect the resource to be an access request.
		req, ok := e.Resource.(AccessRequest)
		if !ok {
			return false, trace.BadParameter("unfilterable resource type: %T", e.Resource)
		}
		var filter AccessRequestFilter
		if err := filter.FromMap(kind.Filter); err != nil {
			return false, trace.Wrap(err)
		}
		return filter.Match(req), nil
	}
	return true, nil
}

// Event represents an event that happened in the backend
type Event struct {
	// Type is the event type
	Type backend.OpType
	// Resource is a modified or deleted resource
	// in case of deleted resources, only resource header
	// will be provided
	Resource Resource
}

// Events returns new events interface
type Events interface {
	// NewWatcher returns a new event watcher
	NewWatcher(ctx context.Context, watch Watch) (Watcher, error)
}

// Watcher returns watcher
type Watcher interface {
	// Events returns channel with events
	Events() <-chan Event

	// Done returns the channel signalling the closure
	Done() <-chan struct{}

	// Close closes the watcher and releases
	// all associated resources
	Close() error

	// Error returns error associated with watcher
	Error() error
}
