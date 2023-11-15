// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package local

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestClusterExternalAuditWatcher(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)

	svc := NewExternalCloudAuditService(bk)
	require.NotNil(t, svc)

	ch := make(chan string)

	for _, tc := range []struct {
		desc         string
		action       func(t *testing.T)
		expectChange bool
	}{
		{
			desc: "create draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "promote",
			action: func(t *testing.T) {
				err = svc.PromoteToClusterExternalCloudAudit(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
		{
			desc: "create another draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-east-1")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "promote again",
			action: func(t *testing.T) {
				err = svc.PromoteToClusterExternalCloudAudit(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
		{
			desc: "create a third draft",
			action: func(t *testing.T) {
				_, err := svc.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-east-1")
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "delete draft",
			action: func(t *testing.T) {
				err = svc.DeleteDraftExternalCloudAudit(ctx)
				require.NoError(t, err)
			},
			expectChange: false,
		},
		{
			desc: "delete cluster",
			action: func(t *testing.T) {
				err = svc.DisableClusterExternalCloudAudit(ctx)
				require.NoError(t, err)
			},
			expectChange: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			watcher, err := NewClusterExternalAuditWatcher(ctx, ClusterExternalCloudAuditWatcherConfig{
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
