// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func setupWorkloadIdentityServiceTest(
	t *testing.T,
) (context.Context, *WorkloadIdentityService) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewWorkloadIdentityService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return ctx, service
}

func newValidWorkloadIdentity(name string) *workloadidentityv1pb.WorkloadIdentity {
	return &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/test",
			},
		},
	}
}

func TestWorkloadIdentityService_CreateWorkloadIdentity(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentity("example")
		got, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentity),
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
		out, err := service.CreateWorkloadIdentity(ctx, newValidWorkloadIdentity(""))
		require.ErrorContains(t, err, "metadata.name: is required")
		require.Nil(t, out)
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newValidWorkloadIdentity("duplicate")
		_, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		_, err = service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestWorkloadIdentityService_UpsertWorkloadIdentity(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentity("example")
		got, err := service.UpsertWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentity),
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
		_, err = service.UpsertWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
	})
	t.Run("validation occurs", func(t *testing.T) {
		out, err := service.UpdateWorkloadIdentity(ctx, newValidWorkloadIdentity(""))
		require.ErrorContains(t, err, "metadata.name: is required")
		require.Nil(t, out)
	})
}

func TestWorkloadIdentityService_ListWorkloadIdentities(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)
	// Create entities to list
	createdObjects := []*workloadidentityv1pb.WorkloadIdentity{}
	// Create 49 entities to test an incomplete page at the end.
	for i := 0; i < 49; i++ {
		created, err := service.CreateWorkloadIdentity(
			ctx,
			newValidWorkloadIdentity(fmt.Sprintf("%d", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListWorkloadIdentities(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, page, 49)
		require.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			slices.ContainsFunc(page, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*workloadidentityv1pb.WorkloadIdentity{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListWorkloadIdentities(ctx, 10, token)
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
			slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
		}
	})
}

func TestWorkloadIdentityService_GetWorkloadIdentity(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newValidWorkloadIdentity("example")
		_, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		got, err := service.GetWorkloadIdentity(ctx, "example")
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
		_, err := service.GetWorkloadIdentity(ctx, "not-found")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestWorkloadIdentityService_DeleteWorkloadIdentity(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateWorkloadIdentity(
			ctx,
			newValidWorkloadIdentity("example"),
		)
		require.NoError(t, err)

		_, err = service.GetWorkloadIdentity(ctx, "example")
		require.NoError(t, err)

		err = service.DeleteWorkloadIdentity(ctx, "example")
		require.NoError(t, err)

		_, err = service.GetWorkloadIdentity(ctx, "example")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteWorkloadIdentity(ctx, "foo.example.com")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestWorkloadIdentityService_DeleteAllWorkloadIdentities(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)
	_, err := service.CreateWorkloadIdentity(
		ctx,
		newValidWorkloadIdentity("1"),
	)
	require.NoError(t, err)
	_, err = service.CreateWorkloadIdentity(
		ctx,
		newValidWorkloadIdentity("2"),
	)
	require.NoError(t, err)

	page, _, err := service.ListWorkloadIdentities(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, page, 2)

	err = service.DeleteAllWorkloadIdentities(ctx)
	require.NoError(t, err)

	page, _, err = service.ListWorkloadIdentities(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, page)
}

func TestWorkloadIdentityService_UpdateWorkloadIdentity(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create first to support updating
		toCreate := newValidWorkloadIdentity("example")
		got, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		got.Spec.Spiffe.Id = "/changed"
		got2, err := service.UpdateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentity),
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
		toCreate := newValidWorkloadIdentity("example2")
		got, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got.Metadata.Revision)
		got.Spec.Spiffe.Id = ""
		got2, err := service.UpdateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.ErrorContains(t, err, "spec.spiffe.id: is required")
		require.Nil(t, got2)
	})
	t.Run("cond update blocks", func(t *testing.T) {
		toCreate := newValidWorkloadIdentity("example4")
		got, err := service.CreateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toCreate).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		// We'll now update it twice, but on the second update, we will use the
		// revision from the creation not the second update.
		_, err = service.UpdateWorkloadIdentity(
			ctx,
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		_, err = service.UpdateWorkloadIdentity(
			ctx,
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.ErrorIs(t, err, backend.ErrIncorrectRevision)
	})
	t.Run("no upsert", func(t *testing.T) {
		toUpdate := newValidWorkloadIdentity("example3")
		_, err := service.UpdateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(toUpdate).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.Error(t, err)
	})
}

func TestWorkloadIdentityParser(t *testing.T) {
	t.Parallel()
	parser := newWorkloadIdentityParser()
	t.Run("delete", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpDelete,
			Item: backend.Item{
				Key: backend.NewKey("workload_identity", "example"),
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, "example", resource.GetMetadata().Name)
	})
	t.Run("put", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key:   backend.NewKey("workload_identity", "example"),
				Value: []byte(`{"kind":"workload_identity","version":"v1","metadata":{"name":"example"},"spec":{"spiffe":{"id":"/test"}}}`),
			},
		}
		require.True(t, parser.match(event.Item.Key))
		resource, err := parser.parse(event)
		require.NoError(t, err)
		require.Equal(t, "example", resource.GetMetadata().Name)
	})
	t.Run("does not match workload identity x509 revocation", func(t *testing.T) {
		event := backend.Event{
			Type: types.OpPut,
			Item: backend.Item{
				Key: backend.NewKey("workload_identity_x509_revocation", "example"),
			},
		}
		require.False(t, parser.match(event.Item.Key))
	})
}
