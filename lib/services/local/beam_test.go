package local

import (
	"context"
	"testing"
	"time"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBeamServiceGetBeamByAlias(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewBeamService(backend)
	require.NoError(t, err)

	beam := &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    "beam-1",
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: &beamsv1.BeamSpec{},
		Status: &beamsv1.BeamStatus{
			User:  "alice",
			Alias: "brisk-otter",
		},
	}
	beam, err = service.CreateBeam(ctx, beam)
	require.NoError(t, err)
	require.NoError(t, service.CreateBeamAliasLease(ctx,
		beam.GetStatus().GetAlias(),
		beam.GetMetadata().GetName(),
		beam.GetMetadata().GetExpires().AsTime()),
	)

	got, err := service.GetBeamByAlias(ctx, "brisk-otter")
	require.NoError(t, err)
	require.Equal(t, "beam-1", got.GetMetadata().GetName())
}

func TestBeamServiceDeleteBeamAliasLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewBeamService(backend)
	require.NoError(t, err)

	require.NoError(t, service.CreateBeamAliasLease(ctx, "brisk-otter", "beam-1", time.Now().Add(time.Hour)))
	require.NoError(t, service.DeleteBeamAliasLease(ctx, "brisk-otter"))

	_, err = service.GetBeamByAlias(ctx, "brisk-otter")
	require.True(t, trace.IsNotFound(err))
}
