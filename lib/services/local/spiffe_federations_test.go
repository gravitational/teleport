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
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func newSPIFFEFederation(name string) *machineidv1.SPIFFEFederation {
	return &machineidv1.SPIFFEFederation{
		Kind:    types.KindSPIFFEFederation,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &machineidv1.SPIFFEFederationSpec{
			BundleSource: &machineidv1.SPIFFEFederationBundleSource{
				HttpsWeb: &machineidv1.SPIFFEFederationBundleSourceHTTPSWeb{
					BundleEndpointUrl: "https://example.com/bundle.json",
				},
			},
		},
	}
}

func setupSPIFFEFederationTest(
	t *testing.T,
) (context.Context, *SPIFFEFederationService) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewSPIFFEFederationService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return ctx, service
}

func TestSPIFFEFederationService_CreateSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newSPIFFEFederation("example.com")
		got, err := service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*machineidv1.SPIFFEFederation),
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
		out, err := service.CreateSPIFFEFederation(ctx, newSPIFFEFederation("spiffe://i-will-fail"))
		require.ErrorContains(t, err, "metadata.name: must not include the spiffe:// prefix")
		require.Nil(t, out)
	})
	t.Run("no upsert", func(t *testing.T) {
		res := newSPIFFEFederation("twoofme.com")
		_, err := service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*machineidv1.SPIFFEFederation),
		)
		require.NoError(t, err)
		_, err = service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(res).(*machineidv1.SPIFFEFederation),
		)
		require.Error(t, err)
		require.True(t, trace.IsAlreadyExists(err))
	})
}

func TestSPIFFEFederationService_UpsertSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newSPIFFEFederation("example.com")
		got, err := service.UpsertSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*machineidv1.SPIFFEFederation),
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
		_, err = service.UpsertSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*machineidv1.SPIFFEFederation),
		)
		require.NoError(t, err)
	})
	t.Run("validation occurs", func(t *testing.T) {
		out, err := service.UpsertSPIFFEFederation(ctx, newSPIFFEFederation("spiffe://i-will-fail"))
		require.ErrorContains(t, err, "metadata.name: must not include the spiffe:// prefix")
		require.Nil(t, out)
	})
}

func TestSPIFFEFederationService_ListSPIFFEFederations(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)
	// Create entities to list
	createdObjects := []*machineidv1.SPIFFEFederation{}
	// Create 49 entities to test an incomplete page at the end.
	for i := 0; i < 49; i++ {
		created, err := service.CreateSPIFFEFederation(
			ctx,
			newSPIFFEFederation(fmt.Sprintf("%d.example.com", i)),
		)
		require.NoError(t, err)
		createdObjects = append(createdObjects, created)
	}
	t.Run("default page size", func(t *testing.T) {
		page, nextToken, err := service.ListSPIFFEFederations(ctx, 0, "")
		require.NoError(t, err)
		require.Len(t, page, 49)
		require.Empty(t, nextToken)

		// Expect that we get all the things we have created
		for _, created := range createdObjects {
			slices.ContainsFunc(page, func(federation *machineidv1.SPIFFEFederation) bool {
				return proto.Equal(created, federation)
			})
		}
	})
	t.Run("pagination", func(t *testing.T) {
		fetched := []*machineidv1.SPIFFEFederation{}
		token := ""
		iterations := 0
		for {
			iterations++
			page, nextToken, err := service.ListSPIFFEFederations(ctx, 10, token)
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
			slices.ContainsFunc(fetched, func(federation *machineidv1.SPIFFEFederation) bool {
				return proto.Equal(created, federation)
			})
		}
	})
}

func TestSPIFFEFederationService_GetSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newSPIFFEFederation("example.com")
		_, err := service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*machineidv1.SPIFFEFederation),
		)
		require.NoError(t, err)
		got, err := service.GetSPIFFEFederation(ctx, "example.com")
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
		_, err := service.GetSPIFFEFederation(ctx, "foo.example.com")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestSPIFFEFederationService_DeleteSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		_, err := service.CreateSPIFFEFederation(
			ctx,
			newSPIFFEFederation("example.com"),
		)
		require.NoError(t, err)

		_, err = service.GetSPIFFEFederation(ctx, "example.com")
		require.NoError(t, err)

		err = service.DeleteSPIFFEFederation(ctx, "example.com")
		require.NoError(t, err)

		_, err = service.GetSPIFFEFederation(ctx, "example.com")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
	t.Run("not found", func(t *testing.T) {
		err := service.DeleteSPIFFEFederation(ctx, "foo.example.com")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func TestSPIFFEFederationService_DeleteAllSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)
	_, err := service.CreateSPIFFEFederation(
		ctx,
		newSPIFFEFederation("1"),
	)
	require.NoError(t, err)
	_, err = service.CreateSPIFFEFederation(
		ctx,
		newSPIFFEFederation("2"),
	)
	require.NoError(t, err)

	page, _, err := service.ListSPIFFEFederations(ctx, 0, "")
	require.NoError(t, err)
	require.Len(t, page, 2)

	err = service.DeleteAllSPIFFEFederations(ctx)
	require.NoError(t, err)

	page, _, err = service.ListSPIFFEFederations(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, page)
}

func TestSPIFFEFederationService_UpdateSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		// Create resource for us to Update since we can't update a non-existent resource.
		created, err := service.CreateSPIFFEFederation(
			ctx,
			newSPIFFEFederation("example.com"),
		)
		require.NoError(t, err)
		want := proto.Clone(created).(*machineidv1.SPIFFEFederation)
		want.Spec.BundleSource.HttpsWeb.BundleEndpointUrl = "https://example.com/new-bundle.json"

		updated, err := service.UpdateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshaling modifying want
			proto.Clone(want).(*machineidv1.SPIFFEFederation),
		)
		require.NoError(t, err)
		require.NotEqual(t, created.Metadata.Revision, updated.Metadata.Revision)
		require.Empty(t, cmp.Diff(
			want,
			updated,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))

		got, err := service.GetSPIFFEFederation(ctx, "example.com")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(
			want,
			got,
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		))
		require.Equal(t, updated.Metadata.Revision, got.Metadata.Revision)
	})
	t.Run("validation occurs", func(t *testing.T) {
		out, err := service.UpdateSPIFFEFederation(ctx, newSPIFFEFederation("spiffe://i-will-fail"))
		require.ErrorContains(t, err, "metadata.name: must not include the spiffe:// prefix")
		require.Nil(t, out)
	})
	t.Run("no create", func(t *testing.T) {
		_, err := service.UpdateSPIFFEFederation(
			ctx,
			newSPIFFEFederation("non-existing.com"),
		)
		require.Error(t, err)
	})
}
