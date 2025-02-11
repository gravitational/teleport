// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package local

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func setupWorkloadIdentityX509RevocationServiceTest(
	t *testing.T,
) (context.Context, clockwork.Clock, *WorkloadIdentityX509RevocationService) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewWorkloadIdentityX509RevocationService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return ctx, clock, service
}

func newValidWorkloadIdentityX509Revocation(clock clockwork.Clock, name string) *workloadidentityv1pb.WorkloadIdentityX509Revocation {
	return &workloadidentityv1pb.WorkloadIdentityX509Revocation{
		Kind:    types.KindWorkloadIdentityX509Revocation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: timestamppb.New(clock.Now().Add(time.Hour)),
		},
		Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
			Reason:    "compromised",
			RevokedAt: timestamppb.New(clock.Now()),
		},
	}
}

func TestWorkloadIdentityX509RevocationService_Create(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentityX509Revocation(clock, "aabbcc")
		got, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		require.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("validation occurs", func(t *testing.T) {
		out, err := service.CreateWorkloadIdentityX509Revocation(
			ctx, newValidWorkloadIdentityX509Revocation(clock, ""),
		)
		require.ErrorContains(t, err, "metadata.name: is required")
		require.Nil(t, out)
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newValidWorkloadIdentityX509Revocation(clock, "ccddee")
		_, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		_, err = service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestWorkloadIdentityX509RevocationService_Upsert(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentityX509Revocation(clock, "aa")
		got, err := service.UpsertWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		require.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))

		// Ensure we can upsert over an existing resource
		_, err = service.UpsertWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
	})
	t.Run("validation occurs", func(t *testing.T) {
		out, err := service.UpdateWorkloadIdentityX509Revocation(
			ctx, newValidWorkloadIdentityX509Revocation(clock, ""),
		)
		require.ErrorContains(t, err, "metadata.name: is required")
		require.Nil(t, out)
	})
}

func TestWorkloadIdentityX509RevocationService_List(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)
	// Create entities to list
	createdObjects := []*workloadidentityv1pb.WorkloadIdentityX509Revocation{}
	// Create 49 entities to test an incomplete page at the end.
	for i := 0; i < 49; i++ {
		created, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			newValidWorkloadIdentityX509Revocation(clock, fmt.Sprintf("%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListWorkloadIdentityX509Revocations(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, page, 49)
		require.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			slices.ContainsFunc(page, func(resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) bool {
				return proto.Equal(created, resource)
			})
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*workloadidentityv1pb.WorkloadIdentityX509Revocation{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListWorkloadIdentityX509Revocations(ctx, 10, token)
			require.NoError(t, err)
			fetched = append(fetched, page...)
			if nextToken == "" {
				break
			}
			token = nextToken
		}
		require.Equal(t, 5, iterations)

		require.Len(t, fetched, 49)
		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) bool {
				return proto.Equal(created, resource)
			})
		}
	})
}

func TestWorkloadIdentityX509RevocationService_Get(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentityX509Revocation(clock, "aa")
		_, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		got, err := service.GetWorkloadIdentityX509Revocation(ctx, "aa")
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		require.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("not found", func(t *testing.T) {
		_, err := service.GetWorkloadIdentityX509Revocation(ctx, "ff")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestWorkloadIdentityX509RevocationService_Delete(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			newValidWorkloadIdentityX509Revocation(clock, "aa"),
		)
		require.NoError(t, err)

		_, err = service.GetWorkloadIdentityX509Revocation(ctx, "aa")
		require.NoError(t, err)

		err = service.DeleteWorkloadIdentityX509Revocation(ctx, "aa")
		require.NoError(t, err)

		_, err = service.GetWorkloadIdentityX509Revocation(ctx, "aa")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteWorkloadIdentityX509Revocation(ctx, "bb")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestWorkloadIdentityX509RevocationService_DeleteAll(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)
	_, err := service.CreateWorkloadIdentityX509Revocation(
		ctx,
		newValidWorkloadIdentityX509Revocation(clock, "1"),
	)
	require.NoError(t, err)
	_, err = service.CreateWorkloadIdentityX509Revocation(
		ctx,
		newValidWorkloadIdentityX509Revocation(clock, "2"),
	)
	require.NoError(t, err)

	page, _, err := service.ListWorkloadIdentityX509Revocations(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, page, 2)

	err = service.DeleteAllWorkloadIdentityX509Revocations(ctx)
	require.NoError(t, err)

	page, _, err = service.ListWorkloadIdentityX509Revocations(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, page)
}

func TestWorkloadIdentityX509RevocationService_Update(t *testing.T) {
	ctx, clock, service := setupWorkloadIdentityX509RevocationServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create first to support updating
		toCreate := newValidWorkloadIdentityX509Revocation(clock, "aa")
		got, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		got.Spec.Reason = "changed"
		got2, err := service.UpdateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got2.Metadata.Revision)
		require.Empty(t, cmp.Diff(
			got,
			got2,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
	})
	t.Run("validation occurs", func(t *testing.T) {
		// Create first to support updating
		toCreate := newValidWorkloadIdentityX509Revocation(clock, "bb")
		got, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		got.Spec.Reason = ""
		got2, err := service.UpdateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.ErrorContains(t, err, "spec.reason: is required")
		require.Nil(t, got2)
	})
	t.Run("cond update blocks", func(t *testing.T) {
		toCreate := newValidWorkloadIdentityX509Revocation(clock, "cc")
		got, err := service.CreateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		// We'll now update it twice, but on the second update, we will use the
		// revision from the creation not the second update.
		_, err = service.UpdateWorkloadIdentityX509Revocation(
			ctx,
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.NoError(t, err)
		_, err = service.UpdateWorkloadIdentityX509Revocation(
			ctx,
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.ErrorIs(t, err, backend.ErrIncorrectRevision)
	})
	t.Run("no upsert", func(t *testing.T) {
		toUpdate := newValidWorkloadIdentityX509Revocation(clock, "dd")
		_, err := service.UpdateWorkloadIdentityX509Revocation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toUpdate).(*workloadidentityv1pb.WorkloadIdentityX509Revocation),
		)
		require.Error(t, err)
	})
}
