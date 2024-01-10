/*
 *
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/suite"
)

func TestNewWatcher_CertAuthority(t *testing.T) {
	t.Parallel()

	// setup backend and events service
	clock := clockwork.NewFakeClock()
	bk, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { bk.Close() })
	eventsSvc := NewEventsService(bk)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// setup watchers - one filtered the other not
	filteredWatcher, err := eventsSvc.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{
		Kind: types.KindCertAuthority,
		Filter: types.CertAuthorityFilter{
			types.HostCA: "example.com",
		}.IntoMap(),
		LoadSecrets: false,
	}}})
	require.NoError(t, err)

	unfilteredWatcher, err := eventsSvc.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{
		Kind:        types.KindCertAuthority,
		LoadSecrets: false,
	}}})
	require.NoError(t, err)

	// create some CAs to generate OpPut events.
	userCA := suite.NewTestCA(types.UserCA, "example.com")
	hostCA := suite.NewTestCA(types.HostCA, "example.com")
	hostCARemote := suite.NewTestCA(types.HostCA, "remote.com")
	err = CreateResources(ctx, bk, userCA, hostCA, hostCARemote)
	require.NoError(t, err)

	const fetchTimeout = 3 * time.Second
	t.Run("with filter", func(t *testing.T) {
		event := fetchEvent(t, filteredWatcher, fetchTimeout)
		require.Equal(t, types.OpInit, event.Type)

		event = fetchEvent(t, filteredWatcher, fetchTimeout)
		require.Equal(t, types.OpPut, event.Type)
		ca, ok := event.Resource.(*types.CertAuthorityV2)
		require.True(t, ok)
		require.Equal(t, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: "example.com",
		}, ca.GetID())
	})

	t.Run("without filter", func(t *testing.T) {
		event := fetchEvent(t, unfilteredWatcher, fetchTimeout)
		require.Equal(t, types.OpInit, event.Type)

		var putEvents []types.Event
		putEvents = append(putEvents, fetchEvent(t, unfilteredWatcher, fetchTimeout))
		putEvents = append(putEvents, fetchEvent(t, unfilteredWatcher, fetchTimeout))
		putEvents = append(putEvents, fetchEvent(t, unfilteredWatcher, fetchTimeout))

		gotCertAuthIDSet := map[types.CertAuthID]struct{}{}
		for _, event := range putEvents {
			require.Equal(t, types.OpPut, event.Type)
			ca, ok := event.Resource.(*types.CertAuthorityV2)
			require.True(t, ok)
			gotCertAuthIDSet[ca.GetID()] = struct{}{}
		}
		want := map[types.CertAuthID]struct{}{
			{Type: types.UserCA, DomainName: "example.com"}: {},
			{Type: types.HostCA, DomainName: "example.com"}: {},
			{Type: types.HostCA, DomainName: "remote.com"}:  {},
		}
		require.Empty(t, cmp.Diff(want, gotCertAuthIDSet))
	})
}

func fetchEvent(t *testing.T, w types.Watcher, timeout time.Duration) types.Event {
	t.Helper()
	timeoutC := time.After(timeout)
	var ev types.Event
	select {
	case <-timeoutC:
		require.Fail(t, "Timeout waiting for event", w.Error())
	case <-w.Done():
		require.Fail(t, "Watcher exited with error", w.Error())
	case ev = <-w.Events():
	}
	return ev
}
