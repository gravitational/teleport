package common

import (
	"context"
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
//	WaitForEvent(t, w, func(r types.User) bool) { r.GetName() == expected.GetName() })
func WaitForPutEvent[T types.Resource](t *testing.T, watcher types.Watcher, fn func(T) bool) {
	t.Helper()

	ctx, cancel := watcherCtx(t)
	defer cancel()
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatal("watcher closed")
			}
			if event.Type != types.OpPut {
				continue
			}
			resource, ok := event.Resource.(T)
			if ok && fn(resource) {
				return
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for resource kind")
		}
	}
}

// WaitForDeleteEvent waits on the watcher until `fn` returns true.
// Expect minimal resource data such as ResourceHeader on delete events.
func WaitForDeleteEvent(t *testing.T, watcher types.Watcher, fn func(types.Resource) bool) {
	t.Helper()

	ctx, cancel := watcherCtx(t)
	defer cancel()
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatal("watcher closed")
			}
			if event.Type != types.OpDelete {
				continue
			}
			if event.Resource != nil && fn(event.Resource) {
				return
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for resource")
		}
	}
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
