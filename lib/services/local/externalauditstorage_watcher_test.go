/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package local

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestClusterExternalAuditWatcher(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	svc := NewExternalAuditStorageService(bk)

	integrationsSvc, err := NewIntegrationsService(bk)
	require.NoError(t, err)

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "test-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "test-role",
		},
	)
	require.NoError(t, err)
	integrationsSvc.CreateIntegration(ctx, oidcIntegration)

	ch := make(chan string)

	for _, tc := range []struct {
		desc         string
		action       func(t *testing.T)
		expectChange bool
	}{
		{
			desc: "create draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalAuditStorage(ctx, "test-integration", "us-west-2")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "promote",
			action: func(t *testing.T) {
				err = svc.PromoteToClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
		{
			desc: "create another draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalAuditStorage(ctx, "test-integration", "us-east-1")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "promote again",
			action: func(t *testing.T) {
				err = svc.PromoteToClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
		{
			desc: "create a third draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalAuditStorage(ctx, "test-integration", "us-east-1")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "delete draft",
			action: func(t *testing.T) {
				err = svc.DeleteDraftExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "delete cluster",
			action: func(t *testing.T) {
				err = svc.DisableClusterExternalAuditStorage(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			watcher, err := NewClusterExternalAuditWatcher(ctx, ClusterExternalAuditStorageWatcherConfig{
				Backend: bk,
				OnChange: func() {
					ch <- tc.desc
				},
			})
			require.NoError(t, err)
			defer watcher.close()

			err = watcher.WaitInit(ctx)
			require.NoError(t, err)
			tc.action(t)

			if tc.expectChange {
				require.Equal(t, tc.desc, <-ch)
			}
		})
	}
}

// TestClusterExternalAuditWatcher_WatcherClosed tests that the
// ExternalAuditWatcher can recover from the underlying backend watcher closing.
func TestClusterExternalAuditWatcher_WatcherClosed(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	svc := NewExternalAuditStorageService(bk)
	integrationsSvc, err := NewIntegrationsService(bk)
	require.NoError(t, err)

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "test-integration"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "test-role",
		},
	)
	require.NoError(t, err)
	integrationsSvc.CreateIntegration(ctx, oidcIntegration)

	interceptor := &watcherInterceptor{
		Backend:  bk,
		watchers: make(chan backend.Watcher, 1),
	}

	changes := make(chan struct{})
	clock := clockwork.NewFakeClock()

	auditWatcher, err := NewClusterExternalAuditWatcher(ctx, ClusterExternalAuditStorageWatcherConfig{
		Backend: interceptor,
		OnChange: func() {
			changes <- struct{}{}
		},
		Clock: clock,
	})
	require.NoError(t, err)

	require.NoError(t, auditWatcher.WaitInit(ctx))

	// Sanity test a change is detected
	_, err = svc.GenerateDraftExternalAuditStorage(ctx, "test-integration", "us-west-2")
	require.NoError(t, err)
	err = svc.PromoteToClusterExternalAuditStorage(ctx)
	require.NoError(t, err)
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher failed to detect change")
	}

	// Close the backend watcher and make sure the audit watcher recovers
	w := <-interceptor.watchers
	w.Close()
	clock.BlockUntil(1)
	clock.Advance(defaults.LowResPollingPeriod)
	require.NoError(t, auditWatcher.WaitInit(ctx))

	// It should still detect changes
	err = svc.DisableClusterExternalAuditStorage(ctx)
	require.NoError(t, err)
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher failed to detect change")
	}
}

// watcherInterceptor wraps a backend.Backend and writes all backend watchers
// returned from NewWatcher to a channel.
type watcherInterceptor struct {
	backend.Backend
	watchers chan backend.Watcher
}

func (i *watcherInterceptor) NewWatcher(ctx context.Context, watch backend.Watch) (backend.Watcher, error) {
	w, err := i.Backend.NewWatcher(ctx, watch)
	if err != nil {
		return nil, err
	}
	i.watchers <- w
	return w, nil
}
