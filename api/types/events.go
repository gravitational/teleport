/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// String returns text description of this event
func (r Event) String() string {
	if r.Type == OpDelete {
		return fmt.Sprintf("%v(%v/%v)", r.Type, r.Resource.GetKind(), r.Resource.GetSubKind())
	}
	return fmt.Sprintf("%v(%v)", r.Type, r.Resource)
}

// Event represents an event that happened in the backend
type Event struct {
	// Type is the event type
	Type OpType
	// Resource is a modified or deleted resource
	// in case of deleted resources, only resource header
	// will be provided
	Resource Resource

	// PreEncodedEventToGRPC, if not empty, is the wire encoding of the event
	// message returned by [client.EventToGRPC].
	PreEncodedEventToGRPC protoreflect.RawFields
}

// OpType specifies operation type
type OpType int

const (
	// OpUnreliable is used to indicate the event stream has become unreliable
	// for maintaining an up-to-date view of the data.
	OpUnreliable OpType = iota - 2
	// OpInvalid is returned for invalid operations
	OpInvalid
	// OpInit is returned by the system whenever the system
	// is initialized, init operation is always sent
	// as a first event over the channel, so the client
	// can verify that watch has been established.
	OpInit
	// OpPut is returned for Put events
	OpPut
	// OpDelete is returned for Delete events
	OpDelete
	// OpGet is used for tracking, not present in the event stream
	OpGet
)

// String returns user-friendly description of the operation
func (o OpType) String() string {
	switch o {
	case OpUnreliable:
		return "Unreliable"
	case OpInvalid:
		return "Invalid"
	case OpInit:
		return "Init"
	case OpPut:
		return "Put"
	case OpDelete:
		return "Delete"
	case OpGet:
		return "Get"
	default:
		return "unknown"
	}
}

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

	// AllowPartialSuccess enables a mode in which a watch will succeed if some of the requested kinds aren't available.
	// When this is set, the client must inspect the WatchStatus resource attached to the first OpInit event emitted
	// by the watcher for a list of kinds confirmed by the event source. Kinds requested but omitted from the confirmation
	// will not be included in the event stream.
	// If AllowPartialSuccess was set, but OpInit doesn't have a resource attached, it means that the event source
	// doesn't support partial success and all requested resource kinds should be considered confirmed.
	AllowPartialSuccess bool
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
	if len(kind.Filter) > 0 && e.Type == OpPut {
		switch res := e.Resource.(type) {
		case AccessRequest:
			var filter AccessRequestFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return false, trace.Wrap(err)
			}
			return filter.Match(res), nil
		case WebSession:
			var filter WebSessionFilter
			if err := filter.FromMap(kind.Filter); err != nil {
				return false, trace.Wrap(err)
			}
			return filter.Match(res), nil
		case Lock:
			var target LockTarget
			if err := target.FromMap(kind.Filter); err != nil {
				return false, trace.Wrap(err)
			}
			return target.Match(res), nil
		case CertAuthority:
			var filter CertAuthorityFilter
			filter.FromMap(kind.Filter)
			return filter.Match(res), nil
		case *HeadlessAuthentication:
			var filter HeadlessAuthenticationFilter
			filter.FromMap(kind.Filter)
			return filter.Match(res), nil
		default:
			// we don't know about this filter, let the event through
		}
	}
	return true, nil
}

// IsTrivial returns true iff the WatchKind only specifies a Kind but no other field.
func (kind WatchKind) IsTrivial() bool {
	return kind.SubKind == "" && kind.Name == "" && kind.Version == "" && !kind.LoadSecrets && len(kind.Filter) == 0
}

// Contains determines whether kind (receiver) targets exactly the same or a wider scope of events as the given subset kind.
// Generally this means that if kind specifies a filter, its subset must have exactly the same or a narrower one.
// Currently, does not take resource versions into account.
func (kind WatchKind) Contains(subset WatchKind) bool {
	// kind and subkind must always be equal
	if kind.Kind != subset.Kind || kind.SubKind != subset.SubKind {
		return false
	}

	if kind.Name != "" && kind.Name != subset.Name {
		return false
	}

	if !kind.LoadSecrets && subset.LoadSecrets {
		return false
	}

	if kind.Kind == KindCertAuthority {
		var a, b CertAuthorityFilter
		a.FromMap(kind.Filter)
		b.FromMap(subset.Filter)
		return a.Contains(b)
	}

	for k, v := range kind.Filter {
		if subset.Filter[k] != v {
			return false
		}
	}

	return true
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

	// Done returns the channel signaling the closure
	Done() <-chan struct{}

	// Close closes the watcher and releases
	// all associated resources
	Close() error

	// Error returns error associated with watcher
	Error() error
}
