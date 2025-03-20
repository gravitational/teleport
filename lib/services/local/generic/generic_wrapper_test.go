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

package generic

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// testResource for testing the generic service. Follows RFD 153.
type testResource153 struct {
	Metadata *headerv1.Metadata
}

func (t *testResource153) GetMetadata() *headerv1.Metadata {
	return t.Metadata
}

func newTestResource153(name string) *testResource153 {
	tr := &testResource153{
		Metadata: &headerv1.Metadata{
			Name: name,
		},
	}
	tr.Metadata.Expires = timestamppb.New(time.Now().AddDate(0, 0, 3))
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

	var r testResource153
	if err := utils.FastUnmarshal(data, &r); err != nil {
		return nil, trace.BadParameter("%s", err)
	}

	if r.Metadata == nil {
		r.Metadata = &headerv1.Metadata{}
	}

	if cfg.Revision != "" {
		r.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		r.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &r, nil
}

// TestGenericWrapperCRUD tests backend operations with the generic service.
func TestGenericWrapperCRUD(t *testing.T) {
	ctx := context.Background()

	ignoreUnexported := cmp.Options{
		cmpopts.IgnoreUnexported(testResource153{}),
		cmpopts.IgnoreUnexported(headerv1.Metadata{}),
		cmpopts.IgnoreUnexported(timestamppb.Timestamp{}),
	}

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	const backendPrefix = "generic_prefix"

	service, err := NewServiceWrapper(
		ServiceConfig[*testResource153]{
			Backend:       memBackend,
			ResourceKind:  "generic resource",
			BackendPrefix: backend.NewKey(backendPrefix),
			MarshalFunc:   marshalResource153,
			UnmarshalFunc: unmarshalResource153,
		})
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

	// Fetch a specific service provider.
	r, err := service.GetResource(ctx, r2.GetMetadata().GetName())
	require.NoError(t, err)
	require.Equal(t, r2, r)

	// Try to fetch a resource that doesn't exist.
	_, err = service.GetResource(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Try to create the same resource.
	_, err = service.CreateResource(ctx, r1)
	require.True(t, trace.IsAlreadyExists(err))

	// Update a resource.
	r1.Metadata.Labels = map[string]string{"newlabel": "newvalue"}
	r1, err = service.UnconditionalUpdateResource(ctx, r1)
	require.NoError(t, err)
	r, err = service.GetResource(ctx, r1.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(r1, r, cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"), ignoreUnexported))

	// Conditionally updating a resource fails if revisions do not match
	r.Metadata.Revision = "fake"
	_, err = service.ConditionalUpdateResource(ctx, r)
	require.True(t, trace.IsCompareFailed(err))

	// Conditionally updating a resource is allowed when revisions match
	r.Metadata.Revision = r1.Metadata.Revision
	r1, err = service.ConditionalUpdateResource(ctx, r1)
	require.NoError(t, err)

	// Update a resource that doesn't exist.
	doesNotExist := newTestResource153("doesnotexist")
	_, err = service.UnconditionalUpdateResource(ctx, doesNotExist)
	require.True(t, trace.IsNotFound(err))

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
	require.Empty(t, cmp.Diff([]*testResource153{r1, r2}, out,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		ignoreUnexported))

	// Upsert a resource (update).
	r1.Metadata.Labels = map[string]string{"newerlabel": "newervalue"}
	r1, err = service.UpsertResource(ctx, r1)
	require.NoError(t, err)
	out, nextToken, err = service.ListResources(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]*testResource153{r1, r2}, out,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		ignoreUnexported))

	// Try to delete a resource that doesn't exist.
	err = service.DeleteResource(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))
}

// TestGenericWrapperWithPrefix tests the withPrefix method of the generic service wrapper.
func TestGenericWrapperWithPrefix(t *testing.T) {
	ctx := context.Background()

	memBackend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	initialBackendPrefix := backend.NewKey("initial_prefix")
	const additionalBackendPrefix = "additional_prefix"

	service, err := NewServiceWrapper(
		ServiceConfig[*testResource153]{
			Backend:       memBackend,
			ResourceKind:  "generic resource",
			BackendPrefix: initialBackendPrefix,
			MarshalFunc:   marshalResource153,
			UnmarshalFunc: unmarshalResource153,
		})
	require.NoError(t, err)

	// Verify that the service's backend prefix matches the initial backend prefix.
	require.Equal(t, initialBackendPrefix, service.service.backendPrefix)

	// Verify that withPrefix appends the additional prefix.
	serviceWithPrefix := service.WithPrefix(additionalBackendPrefix)
	require.Equal(t, backend.NewKey("initial_prefix", "additional_prefix").String(), serviceWithPrefix.service.backendPrefix.String())
}
