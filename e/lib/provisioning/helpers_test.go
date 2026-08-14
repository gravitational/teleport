package provisioning

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func toWatchKind(kind string) types.WatchKind {
	return types.WatchKind{Kind: kind}
}

// mustCreateWatcher creates an automatically-closing watcher for the supplied resource kinds
func mustCreateWatcher(t *testing.T, watchmaker types.Events, kinds ...string) types.Watcher {
	t.Helper()
	w, err := watchmaker.NewWatcher(t.Context(), types.Watch{
		Name:  t.Name() + "-watcher",
		Kinds: sliceutils.Map(kinds, toWatchKind),
	})
	require.NoError(t, err)
	t.Cleanup(func() { w.Close() })

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(time.Second * 10):
		t.Fatal("context canceled waiting for sync watcher init")
	}
	return w
}

type principalStatePredicate func(*provisioningv1.PrincipalState) bool

// waitForPrincipalState watches events on provisioning state records
// until all supplied predicates return `true` for an OpPut event, returning
// the matching event resource.
//
// It fails the test if the watcher closes or the test context is canceled first.
func waitForPrincipalState(t *testing.T, watcher types.Watcher, predicates ...principalStatePredicate) *provisioningv1.PrincipalState {
	t.Helper()
waitLoop:
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatal("watcher closed")
			}
			if event.Type != types.OpPut {
				continue waitLoop
			}
			unwrapper, ok := event.Resource.(types.Resource153UnwrapperT[*provisioningv1.PrincipalState])
			if !ok {
				continue waitLoop
			}
			resource := unwrapper.UnwrapT()
			if resource == nil {
				continue waitLoop
			}

			for _, predicate := range predicates {
				if !predicate(resource) {
					continue waitLoop
				}
			}
			return resource

		case <-time.After(time.Minute):
			t.Fatal("timed out waiting for resource")
		}
	}
}

func withProvisioningState(s provisioningv1.ProvisioningState) principalStatePredicate {
	return func(state *provisioningv1.PrincipalState) bool {
		return state.GetStatus().GetProvisioningState() == s
	}
}

func withProvisioningStateID(id services.ProvisioningStateID) principalStatePredicate {
	return func(state *provisioningv1.PrincipalState) bool {
		return state.GetMetadata().GetName() == string(id)
	}
}

func withEmptyExternalID(state *provisioningv1.PrincipalState) bool {
	return state.GetStatus().GetExternalId() == ""
}

func withExternalID(externalID string) principalStatePredicate {
	return func(state *provisioningv1.PrincipalState) bool {
		return state.GetStatus().GetExternalId() == externalID
	}
}

func not(p principalStatePredicate) principalStatePredicate {
	return func(state *provisioningv1.PrincipalState) bool {
		return !p(state)
	}
}
