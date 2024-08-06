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
			// Clone to avoid Marshalling modifying want
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
			// Clone to avoid Marshalling modifying want
			proto.Clone(res).(*machineidv1.SPIFFEFederation),
		)
		require.NoError(t, err)
		_, err = service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshalling modifying want
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
			// Clone to avoid Marshalling modifying want
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
			// Clone to avoid Marshalling modifying want
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

}

func TestSPIFFEFederationService_GetSPIFFEFederation(t *testing.T) {
	ctx, service := setupSPIFFEFederationTest(t)

	t.Run("ok", func(t *testing.T) {
		want := newSPIFFEFederation("example.com")
		_, err := service.CreateSPIFFEFederation(
			ctx,
			// Clone to avoid Marshalling modifying want
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
	require.Len(t, page, 0)
}
