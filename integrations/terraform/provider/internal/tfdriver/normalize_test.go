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

type normalizerTestResource struct {
	kind       string
	metadata   *headerv1.Metadata
	defaulted  bool
	createSeen bool
	updateSeen bool
}

func (r *normalizerTestResource) GetMetadata() *headerv1.Metadata {
	return r.metadata
}

func (r *normalizerTestResource) SetMetadata(m *headerv1.Metadata) {
	r.metadata = m
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

func TestSetDefaultName(t *testing.T) {
	t.Run("sets name when empty", func(t *testing.T) {
		var resource normalizerTestResource
		require.NoError(t, SetDefaultName[normalizerTestResource]("my-default").NormalizeCreate(t.Context(), &resource))
		require.Equal(t, "my-default", resource.metadata.GetName())
	})

	t.Run("sets name when metadata nil", func(t *testing.T) {
		resource := normalizerTestResource{metadata: nil}
		require.NoError(t, SetDefaultName[normalizerTestResource]("my-default").NormalizeCreate(t.Context(), &resource))
		require.NotNil(t, resource.metadata)
		require.Equal(t, "my-default", resource.metadata.GetName())
	})

	t.Run("preserves existing name", func(t *testing.T) {
		meta := &headerv1.Metadata{}
		meta.SetName("user-provided")
		resource := normalizerTestResource{metadata: meta}
		require.NoError(t, SetDefaultName[normalizerTestResource]("my-default").NormalizeCreate(t.Context(), &resource))
		require.Equal(t, "user-provided", resource.metadata.GetName())
	})

	t.Run("works on update", func(t *testing.T) {
		var resource normalizerTestResource
		require.NoError(t, SetDefaultName[normalizerTestResource]("my-default").NormalizeUpdate(t.Context(), &resource))
		require.Equal(t, "my-default", resource.metadata.GetName())
	})
}

func TestSpecificNormalizersRejectUnsupportedResource(t *testing.T) {
	type unsupported struct{}

	err := CheckAndSetDefaults[unsupported]().NormalizeCreate(t.Context(), &unsupported{})
	require.Error(t, err)

	err = ForceKind[unsupported]("kind").NormalizeCreate(t.Context(), &unsupported{})
	require.Error(t, err)
}
