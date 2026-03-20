package cache

import (
	"context"
	"testing"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestBeamAliasIndex(t *testing.T) {
	t.Parallel()

	collection, err := newBeamCollection(fakeBeamReader{}, types.WatchKind{Kind: types.KindBeam})
	require.NoError(t, err)

	collection.store.put(&beamsv1.Beam{
		Metadata: &headerv1.Metadata{Name: "beam-1"},
		Status:   &beamsv1.BeamStatus{},
	})
	collection.store.put(&beamsv1.Beam{
		Metadata: &headerv1.Metadata{Name: "beam-2"},
		Status:   &beamsv1.BeamStatus{},
	})
	collection.store.put(&beamsv1.Beam{
		Metadata: &headerv1.Metadata{Name: "beam-3"},
		Status:   &beamsv1.BeamStatus{Alias: "brisk-otter"},
	})

	require.Equal(t, 3, collection.store.len())

	got, err := collection.store.get(beamAliasIndex, "brisk-otter")
	require.NoError(t, err)
	require.Equal(t, "beam-3", got.GetMetadata().GetName())
}

type fakeBeamReader struct{}

func (fakeBeamReader) GetBeam(context.Context, string) (*beamsv1.Beam, error) {
	return nil, nil
}

func (fakeBeamReader) GetBeamByAlias(context.Context, string) (*beamsv1.Beam, error) {
	return nil, nil
}

func (fakeBeamReader) ListBeams(context.Context, int, string) ([]*beamsv1.Beam, string, error) {
	return nil, "", nil
}
