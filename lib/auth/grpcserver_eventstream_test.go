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

package auth_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestWatchEvents_enterpriseOnlyKinds(t *testing.T) {
	// Don't t.Parallel, uses modulestest.SetTestModules.

	testServer := newTestTLSServer(t)
	adminClient, err := testServer.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	// kindOther is a watch kind we expect to always work.
	const kindOther = types.KindCertAuthority

	const wantErr = "only available in Teleport Enterprise"

	confirmKinds := func(t *testing.T, watcher types.Watcher, wantKinds ...string) {
		t.Helper()

		select {
		case <-t.Context().Done():
			t.Fatal("t.Context() done before first event")
		case <-watcher.Done():
			t.Fatalf("watcher done before first event, err=%v", watcher.Error())
		case e := <-watcher.Events():
			require.Equal(t, types.OpInit, e.Type, "Unexpected event type")

			s, ok := e.Resource.(types.WatchStatus)
			require.True(t, ok, "e.Resource has unexpected type: %T", e.Resource)

			watchKinds := s.GetKinds()
			gotKinds := make([]string, len(watchKinds))
			for i, k := range watchKinds {
				gotKinds[i] = k.Kind
			}
			if diff := cmp.Diff(wantKinds, gotKinds); diff != "" {
				t.Errorf("WatchStatus.GetKinds() mismatch (-want +got)\n%s", diff)
			}
		}
	}

	tests := []struct {
		kind string
	}{
		{kind: types.KindCertAuthorityOverride},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			// Don't t.Parallel, uses modulestest.SetTestModules.

			modulestest.SetTestModules(t, *modulestest.OSSModules())

			t.Run("OSS fails", func(t *testing.T) {
				watcher, err := adminClient.NewWatcher(t.Context(), types.Watch{
					Name: "oss",
					Kinds: []types.WatchKind{
						{Kind: test.kind},
					},
				})
				// The stream is expected to start and fail before the first event.
				require.NoError(t, err)
				t.Cleanup(func() { watcher.Close() })

				// Wait for stream to close and assert failure.
				select {
				case <-watcher.Done():
				case <-time.After(5 * time.Second):
					t.Fatal("Timed out waiting for stream to close")
				}
				assert.ErrorContains(t, watcher.Error(), wantErr, "Watcher error mismatch")
			})

			t.Run("OSS with allow partial succeeds", func(t *testing.T) {
				watcher, err := adminClient.NewWatcher(t.Context(), types.Watch{
					Name: "oss-allow-partial",
					Kinds: []types.WatchKind{
						{Kind: kindOther},
						{Kind: test.kind},
					},
					AllowPartialSuccess: true,
				})
				require.NoError(t, err)
				t.Cleanup(func() { watcher.Close() })

				confirmKinds(t, watcher, kindOther)
				assert.NoError(t, watcher.Error(), "Watcher has unexpected error")
			})

			modulestest.SetTestModules(t, *modulestest.EnterpriseModules())

			t.Run("Ent succeeds", func(t *testing.T) {
				watcher, err := adminClient.NewWatcher(t.Context(), types.Watch{
					Name: "ent-allow-partial",
					Kinds: []types.WatchKind{
						{Kind: kindOther},
						{Kind: test.kind},
					},
				})
				require.NoError(t, err)
				t.Cleanup(func() { watcher.Close() })

				confirmKinds(t, watcher, kindOther, test.kind)
				assert.NoError(t, watcher.Error(), "Watcher has unexpected error")
			})
		})
	}
}
