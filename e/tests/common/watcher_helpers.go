package common

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
)

// WaitForPutEvent waits on the watcher until `fn` returns true.
//
// Usage:
//
//	w := NewResourceWatcher(t, types.KindUser)
//	defer pluginWatcher.Close()
//	// Create user resource.
//	expected, err := auth.CreateUser(ctx, &types.UserV2{...})
//	require.NoError(t, err)
//	// Wait for the resource to be created.
//	WaitForPutEvent(t, w, func(r types.User) bool) { r.GetName() == expected.GetName() })
func WaitForPutEvent[T types.Resource](t *testing.T, watcher types.Watcher, fn func(T) bool) T {
	t.Helper()
	return waitForEventOp(t, watcher, types.OpPut, fn)
}

// WaitForDeleteEvent waits on the watcher until `fn` returns true.
// Expect minimal resource data such as ResourceHeader on delete events.
func WaitForDeleteEvent[T types.Resource](t *testing.T, watcher types.Watcher, fn func(T) bool) T {
	t.Helper()
	return waitForEventOp(t, watcher, types.OpDelete, fn)
}

// waitForEventOp waits for an event on the supplied [types.Watcher] that
// satisfies the supplied predicate function.
func waitForEventOp[T types.Resource](t *testing.T, watcher types.Watcher, op types.OpType, fn func(T) bool) T {
	t.Helper()
	results := waitForEvents(t, watcher, op, fn)
	return results[0]
}

// WaitForAllResource153PutEvents waits for `put` events on a [types.Watcher]
// until events satisfying all of the supplied predicates are encountered.
// Returns the resources that satisfied each predicate.
func WaitForAllResource153PutEvents[T types.Resource153](t *testing.T, watcher types.Watcher, predicateFns ...func(T) bool) []T {
	t.Helper()
	return waitForResource153Events(t, watcher, types.OpPut, predicateFns...)
}

// WaitForAllPutEvents waits for `put` events on a [types.Watcher] until events
// satisfying all of the supplied predicates are encountered. Returns the
// resources from the matching events.
func WaitForAllPutEvents[T types.Resource](t *testing.T, watcher types.Watcher, predicateFns ...func(T) bool) []T {
	t.Helper()
	return waitForEvents(t, watcher, types.OpPut, predicateFns...)
}

// WaitForAllPutEvents waits for `delete` events on a [types.Watcher] until events
// satisfying all of the supplied predicates are encountered. Returns the
// resources from the matching events.
func WaitForAllDeleteEvents[T types.Resource](t *testing.T, watcher types.Watcher, predicateFns ...func(T) bool) []T {
	t.Helper()
	return waitForEvents(t, watcher, types.OpDelete, predicateFns...)
}

// waitForResource153Events waits for events on a [types.Watcher] until events
// satisfying all of the supplied predicates are encountered. Returns the resources
// from the matching events. Adds special handling for the new-style RFD153-style
// events.
func waitForResource153Events[T types.Resource153](t *testing.T, watcher types.Watcher, op types.OpType, predicateFns ...func(T) bool) []T {
	// RFD153-style resources don't implement `types.Resource` so we can't use
	// `waitForEvents[T]` directly. They are also passed through the event system
	// wrapped in a private type, so we can't specialize `waitForEvents[T]` on that
	// directly, either. Our only option is to use `waitForEvents[T]` to examine
	// every resource event, and wrap the supplied predicates with the appropriate
	// type checking and unwrapping.
	// We also collect the results independently so that we don't have to re-check
	// and unwrap them before returning them to the caller.
	var results []T
	waitForEvents(t, watcher, op, wrapResource153Predicates(predicateFns, &results)...)
	return results
}

// wrapResource153Predicates wraps the supplied predicateFns with resource
// type assertions and unpacking to allow the to be used with [waitForEvents].
// Resources matching the predicates will be appended to `*dst`.
func wrapResource153Predicates[T types.Resource153](predicates []func(T) bool, dst *[]T) []func(types.Resource) bool {
	wrapped := make([]func(types.Resource) bool, len(predicates))
	for i, predicate := range predicates {
		wrapped[i] = func(r types.Resource) bool {
			unwrapper, ok := r.(types.Resource153UnwrapperT[T])
			if !ok {
				return false
			}
			resource := unwrapper.UnwrapT()
			if !predicate(resource) {
				return false
			}
			*dst = append(*dst, resource)
			return true
		}
	}
	return wrapped
}

// waitForEvents waits for events on a [types.Watcher] until events
// satisfying all of the supplied predicates are encountered. Returns the
// resources from the matching events.
func waitForEvents[T types.Resource](t *testing.T, watcher types.Watcher, op types.OpType, predicates ...func(T) bool) []T {
	t.Helper()

	ctx, cancel := watcherCtx(t)
	defer cancel()

	var results []T

	unsatisfied := slices.Clone(predicates)
	for len(unsatisfied) > 0 {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatal("watcher closed")
			}
			if event.Type != op {
				continue
			}
			resource, ok := event.Resource.(T)
			if !ok {
				continue
			}

			for i, predicate := range unsatisfied {
				if predicate(resource) {
					results = append(results, resource)
					unsatisfied = slices.Delete(unsatisfied, i, i+1)
					break
				}
			}

		case <-ctx.Done():
			t.Fatalf("timed out waiting for resources. %d of %d predicates satisfied.",
				len(predicates)-len(unsatisfied),
				len(predicates))
		}
	}

	return results
}

func watcherCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return context.WithCancel(t.Context())
	}

	// Time out earlier than the test deadline so that the watcher can cleanly exit.
	const timeOutGrace = time.Second
	waitUntil := deadline.Add(-timeOutGrace)
	if time.Until(waitUntil) <= 0 {
		waitUntil = deadline
	}

	return context.WithDeadline(t.Context(), waitUntil)
}
