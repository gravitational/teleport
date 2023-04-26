/*
Copyright 2023 Gravitational, Inc.

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

package okta

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

func TestAssignmentReconciler(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	svc, _ := newTestService(t, ap)
	onReconcileCh := make(chan struct{}, 1)

	reconciler := newAssignmentReconciler(ctx, svc)
	reconciler.onReconcileCh = onReconcileCh
	require.NoError(t, reconciler.start(ctx))
	t.Cleanup(func() {
		reconciler.stop()
	})

	waitForResult(t, onReconcileCh, struct{}{}, 1)

	// Reconciler should be empty to start
	require.Empty(t, reconciler.getAssignments())
	require.Empty(t, reconciler.getNewAssignments())

	// This Okta assignment should be recognized.
	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: "assignment1",
		},
		types.OktaAssignmentSpecV1{
			User: "test-user@test.user",
			Targets: []*types.OktaAssignmentTargetV1{
				{
					Type: types.OktaAssignmentTargetV1_APPLICATION,
					Id:   "123456",
				},
				{
					Type: types.OktaAssignmentTargetV1_GROUP,
					Id:   "234567",
				},
			},
			Status: types.OktaAssignmentSpecV1_PENDING,
		},
	)
	require.NoError(t, err)

	_, err = ap.CreateOktaAssignment(ctx, assignment)
	require.NoError(t, err)
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Equal(t, types.ResourcesWithLabelsMap{assignment.GetName(): assignment}, reconciler.getAssignments())
	require.Equal(t, types.ResourcesWithLabelsMap{assignment.GetName(): assignment}, reconciler.getNewAssignments())

	foundAssignment, err := ap.GetOktaAssignment(ctx, assignment.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(foundAssignment, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")),
	)

	// Update should be recognized.
	assignment.SetStatus(constants.OktaAssignmentStatusProcessing)
	_, err = ap.UpdateOktaAssignment(ctx, assignment)
	require.NoError(t, err)
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	assignment.SetResourceID(1)

	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{assignment.GetName(): assignment}, reconciler.getAssignments(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID")),
	)
	require.Empty(t, cmp.Diff(types.ResourcesWithLabelsMap{assignment.GetName(): assignment}, reconciler.getNewAssignments(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID")),
	)

	foundAssignment, err = ap.GetOktaAssignment(ctx, assignment.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(foundAssignment, assignment,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")),
	)

	// This delete be recognized.
	require.NoError(t, ap.DeleteOktaAssignment(ctx, assignment.GetName()))
	waitForResult(t, onReconcileCh, struct{}{}, 1)

	require.Empty(t, reconciler.getAssignments())
	require.Empty(t, reconciler.getNewAssignments())

	_, err = ap.GetOktaAssignment(ctx, assignment.GetName())
	require.True(t, trace.IsNotFound(err))
}
