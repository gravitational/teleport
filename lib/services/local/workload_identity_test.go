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
	return workloadidentityv1pb.WorkloadIdentity_builder{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Spec: workloadidentityv1pb.WorkloadIdentitySpec_builder{
			Spiffe: workloadidentityv1pb.WorkloadIdentitySPIFFE_builder{
				Id:   "/test/" + name,
				Hint: "This is hint " + name,
			}.Build(),
		}.Build(),
	}.Build()
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
		require.NotEmpty(t, got.GetMetadata().GetRevision())
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
		require.NotEmpty(t, got.GetMetadata().GetRevision())
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

func TestWorkloadIdentityService_RangeWorkloadIdentities(t *testing.T) {
	ctx, service := setupWorkloadIdentityServiceTest(t)

	// Create entities to range over, in non-sorted insertion order to confirm
	// the range returns them ordered by name.
	names := []string{"c", "a", "e", "b", "d"}
	for _, name := range names {
		_, err := service.CreateWorkloadIdentity(ctx, newValidWorkloadIdentity(name))
		require.NoError(t, err)
	}

	collect := func(start, end string) []string {
		var got []string
		for wi, err := range service.RangeWorkloadIdentities(ctx, start, end, "", false) {
			require.NoError(t, err)
			got = append(got, wi.GetMetadata().GetName())
		}
		return got
	}

	t.Run("full range is ordered by name", func(t *testing.T) {
		require.Equal(t, []string{"a", "b", "c", "d", "e"}, collect("", ""))
	})
	t.Run("bounded range is exclusive of end", func(t *testing.T) {
		require.Equal(t, []string{"b", "c"}, collect("b", "d"))
	})
	t.Run("open start", func(t *testing.T) {
		require.Equal(t, []string{"a", "b"}, collect("", "c"))
	})
	t.Run("open end", func(t *testing.T) {
		require.Equal(t, []string{"d", "e"}, collect("d", ""))
	})
	t.Run("empty range", func(t *testing.T) {
		require.Empty(t, collect("f", ""))
	})
	t.Run("explicit name sort", func(t *testing.T) {
		var got []string
		for wi, err := range service.RangeWorkloadIdentities(ctx, "", "", "name", false) {
			require.NoError(t, err)
			got = append(got, wi.GetMetadata().GetName())
		}
		require.Equal(t, []string{"a", "b", "c", "d", "e"}, got)
	})
	t.Run("unsupported sort field errors", func(t *testing.T) {
		var err error
		for _, iterErr := range service.RangeWorkloadIdentities(ctx, "", "", "spiffe_id", false) {
			err = iterErr
		}
		require.ErrorContains(t, err, `unsupported sort, only name field is supported, but got "spiffe_id"`)
	})
	t.Run("descending sort errors", func(t *testing.T) {
		var err error
		for _, iterErr := range service.RangeWorkloadIdentities(ctx, "", "", "name", true) {
			err = iterErr
		}
		require.ErrorContains(t, err, "unsupported sort, only ascending order is supported")
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
		require.NotEmpty(t, got.GetMetadata().GetRevision())
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

	collect := func() []*workloadidentityv1pb.WorkloadIdentity {
		var out []*workloadidentityv1pb.WorkloadIdentity
		for wi, err := range service.RangeWorkloadIdentities(ctx, "", "", "", false) {
			require.NoError(t, err)
			out = append(out, wi)
		}
		return out
	}

	require.Len(t, collect(), 2)

	err = service.DeleteAllWorkloadIdentities(ctx)
	require.NoError(t, err)

	require.Empty(t, collect())
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
		require.NotEmpty(t, got.GetMetadata().GetRevision())
		got.GetSpec().GetSpiffe().SetId("/changed")
		got2, err := service.UpdateWorkloadIdentity(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(got).(*workloadidentityv1pb.WorkloadIdentity),
		)
		require.NoError(t, err)
		require.NotEmpty(t, got2.GetMetadata().GetRevision())
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
		require.NotEmpty(t, got.GetMetadata().GetRevision())
		got.GetSpec().GetSpiffe().SetId("")
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
