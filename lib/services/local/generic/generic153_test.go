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

package generic

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// testResource for testing the generic service.
type testResource153 struct {
	Metadata *headerv1.Metadata
}

func (t *testResource153) GetMetadata() *headerv1.Metadata {
	return t.Metadata
}

var _ types.ResourceMetadata = (*testResource153)(nil)
var _ Resource = (*resourceMetadataAdapter[*testResource153])(nil)

func newTestResource153(name string) *testResource153 {
	tr := &testResource153{
		Metadata: &headerv1.Metadata{
			Name: name,
		},
	}
	return tr
}

// marshalResource153 marshals a generic resource.
func marshalResource153(resource *testResource153, opts ...services.MarshalOption) ([]byte, error) {
	return utils.FastMarshal(resource)
}

// unmarshalResource153 unmarshals a generic resource.
func unmarshalResource153(data []byte, opts ...services.MarshalOption) (*testResource153, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}

	var r testResource153
	if err := utils.FastUnmarshal(data, &r); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if r.Metadata == nil {
		r.Metadata = &headerv1.Metadata{}
	}

	if cfg.ID != 0 {
		//nolint:staticcheck // SA1019. Deprecated, but still needed.
		r.Metadata.Id = cfg.ID
	}
	if cfg.Revision != "" {
		r.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		r.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &r, nil
}

// TestGenericCRUD153 tests backend operations with the generic service.
func TestGenericCRUD153(t *testing.T) {
	ctx := context.Background()

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	const backendPrefix = "generic_prefix"

	service, err := NewService153[*testResource153](memBackend,
		"generic resource",
		backendPrefix,
		marshalResource153,
		unmarshalResource153)
	require.NoError(t, err)

	// Create a couple test resources.
	r1 := newTestResource153("r1")
	r2 := newTestResource153("r2")

	// sanity check: marshal/unmarshal round trips
	r1data, err := marshalResource153(r1)
	require.NoError(t, err)
	r1clone, err := unmarshalResource153(r1data)
	require.NoError(t, err)
	require.Equal(t, r1, r1clone)

	// Initially we expect no resources.
	out, nextToken, err := service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both resources.
	r1, err = service.CreateResource(ctx, r1)
	require.NoError(t, err)
	r2, err = service.CreateResource(ctx, r2)
	require.NoError(t, err)

	require.NotEmpty(t, r1.GetMetadata().GetRevision())
	require.NotEmpty(t, r2.GetMetadata().GetRevision())
	require.NotEqual(t, r1.GetMetadata().GetRevision(), r2.GetMetadata().GetRevision())

	// Fetch all resources using paging default.
	out, nextToken, err = service.ListResources(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.NotEmpty(t, out)
	require.Equal(t, []*testResource153{r1, r2}, out)

	// Fetch a paginated list of resources.
	paginatedOut := make([]*testResource153, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.ListResources(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Equal(t, []*testResource153{r1, r2}, paginatedOut)

	// Fetch a list of all resources
	allResources, err := service.GetResources(ctx)
	require.NoError(t, err)
	require.Equal(t, paginatedOut, allResources)

	// Fetch a specific service provider.
	r, err := service.GetResource(ctx, r2.GetMetadata().GetName())
	require.NoError(t, err)
	require.Equal(t, r2, r)

	// Try to fetch a resource that doesn't exist.
	_, err = service.GetResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Try to create the same resource.
	_, err = service.CreateResource(ctx, r1)
	require.ErrorIs(t, err, trace.AlreadyExists(`generic resource "r1" already exists`))

	// Update a resource.
	r1.Metadata.Labels = map[string]string{"newlabel": "newvalue"}
	r1, err = service.UpdateResource(ctx, r1)
	require.NoError(t, err)
	r, err = service.GetResource(ctx, r1.GetMetadata().GetName())
	require.NoError(t, err)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	r.Metadata.Id = r1.Metadata.Id
	require.Equal(t, r1, r)

	// Update a resource that doesn't exist.
	doesNotExist := newTestResource153("doesnotexist")
	_, err = service.UpdateResource(ctx, doesNotExist)
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Delete a resource.
	err = service.DeleteResource(ctx, r1.GetMetadata().GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Equal(t, []*testResource153{r2}, out)

	// Upsert a resource (create).
	r1, err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[0].Metadata.Id = r1.Metadata.Id
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[1].Metadata.Id = r2.Metadata.Id
	require.Equal(t, []*testResource153{r1, r2}, out)

	// Upsert a resource (update).
	r1.Metadata.Labels = map[string]string{"newerlabel": "newervalue"}
	r1, err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[0].Metadata.Id = r1.Metadata.Id
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[1].Metadata.Id = r2.Metadata.Id
	require.Equal(t, []*testResource153{r1, r2}, out)

	// Update and swap a value
	r2, err = service.UpdateAndSwapResource(ctx, r2.GetMetadata().GetName(), func(tr *testResource153) error {
		tr.Metadata.Labels = map[string]string{"updateandswap": "labelvalue"}
		return nil
	})
	require.NoError(t, err)
	r2.Metadata.Labels = map[string]string{"updateandswap": "labelvalue"}
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[0].Metadata.Id = r1.Metadata.Id
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	out[1].Metadata.Id = r2.Metadata.Id
	require.Equal(t, []*testResource153{r1, r2}, out)

	// Try to delete a resource that doesn't exist.
	err = service.DeleteResource(ctx, "doesnotexist")
	require.ErrorIs(t, err, trace.NotFound(`generic resource "doesnotexist" doesn't exist`))

	// Test running while locked.
	err = service.RunWhileLocked(ctx, "test-lock", time.Second*5, func(ctx context.Context, backend backend.Backend) error {
		item, err := backend.Get(ctx, service.(*service153[*testResource153]).svc.makeKey(r1.GetMetadata().GetName()))
		require.NoError(t, err)

		r, err = unmarshalResource153(item.Value, services.WithRevision(item.Revision))
		require.NoError(t, err)
		require.Equal(t, r1, r)

		return nil
	})
	require.NoError(t, err)

	// Delete all resources.
	err = service.DeleteAllResources(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
