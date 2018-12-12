/*
Copyright 2018 Gravitational, Inc.

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
)

// Watch sets up watch on the event
type Watch struct {
	// Kinds specifies kinds of objects to watch
	// and whether to load secret data for them
	Kinds []WatchKind
}

// WatchKind specifies resource kind to watch
type WatchKind struct {
	// Kind is a resource kind to watch
	Kind string
	// LoadSecrets specifies whether to load secrets
	LoadSecrets bool
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
