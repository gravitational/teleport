// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package tfdriver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

type nameableTestResource struct {
	metadata *headerv1.Metadata
}

func (r *nameableTestResource) GetMetadata() *headerv1.Metadata {
	return r.metadata
}

func (r *nameableTestResource) SetMetadata(metadata *headerv1.Metadata) {
	r.metadata = metadata
}

type normalizerTestResource struct {
	kind       string
	defaulted  bool
	createSeen bool
	updateSeen bool
}

func (r *normalizerTestResource) CheckAndSetDefaults() error {
	r.defaulted = true
	return nil
}

func (r *normalizerTestResource) SetKind(kind string) {
	r.kind = kind
}

func TestResourceNormalizerFuncs(t *testing.T) {
	normalizer := ResourceNormalizerFuncs[normalizerTestResource]{
		Create: func(_ context.Context, resource *normalizerTestResource) error {
			resource.createSeen = true
			return nil
		},
		Update: func(_ context.Context, resource *normalizerTestResource) error {
			resource.updateSeen = true
			return nil
		},
	}

	var resource normalizerTestResource
	require.NoError(t, normalizer.NormalizeUpdate(t.Context(), &resource))
	require.False(t, resource.createSeen)
	require.True(t, resource.updateSeen)
}

func TestResourceNormalizersChainsNormalizers(t *testing.T) {
	normalizers := ResourceNormalizers[normalizerTestResource]{
		ForceKind[normalizerTestResource]("test_kind"),
		CheckAndSetDefaults[normalizerTestResource](),
	}

	var resource normalizerTestResource
	require.NoError(t, normalizers.NormalizeCreate(t.Context(), &resource))
	require.Equal(t, "test_kind", resource.kind)
	require.True(t, resource.defaulted)
}

func TestForceKindFuncUsesCallerSuppliedSetter(t *testing.T) {
	var resource normalizerTestResource
	normalizer := ForceKindFunc[normalizerTestResource](func(resource *normalizerTestResource) {
		resource.kind = "test_kind"
	})
	require.NoError(t, normalizer.NormalizeCreate(t.Context(), &resource))
	require.Equal(t, "test_kind", resource.kind)

	require.NoError(t, normalizer.NormalizeUpdate(t.Context(), &resource))
	require.Equal(t, "test_kind", resource.kind)
}

func TestResourceNormalizersStopsOnError(t *testing.T) {
	sentinel := errors.New("stop")
	normalizers := ResourceNormalizers[normalizerTestResource]{
		ResourceNormalizerFuncs[normalizerTestResource]{
			Create: func(context.Context, *normalizerTestResource) error {
				return sentinel
			},
		},
		ForceKind[normalizerTestResource]("should-not-run"),
	}

	var resource normalizerTestResource
	err := normalizers.NormalizeCreate(t.Context(), &resource)
	require.ErrorIs(t, err, sentinel)
	require.Empty(t, resource.kind)
}

func TestForceName(t *testing.T) {
	// ForceName populates name if not present.
	resource := &nameableTestResource{}
	require.NoError(t, ForceName[nameableTestResource]("foo").NormalizeCreate(t.Context(), resource))
	require.Equal(t, "foo", resource.GetMetadata().GetName())

	// ForceName overrides the name but preserves other existing fields.
	resource = &nameableTestResource{metadata: &headerv1.Metadata{Name: "llama", Description: "keep"}}
	require.NoError(t, ForceName[nameableTestResource]("foo").NormalizeUpdate(t.Context(), resource))
	require.Equal(t, "foo", resource.GetMetadata().GetName())
	require.Equal(t, "keep", resource.GetMetadata().GetDescription())
}

func TestSpecificNormalizersRejectUnsupportedResource(t *testing.T) {
	type unsupported struct{}

	err := CheckAndSetDefaults[unsupported]().NormalizeCreate(t.Context(), &unsupported{})
	require.Error(t, err)

	err = ForceKind[unsupported]("kind").NormalizeCreate(t.Context(), &unsupported{})
	require.Error(t, err)

	err = ForceName[unsupported]("name").NormalizeCreate(t.Context(), &unsupported{})
	require.Error(t, err)
}
