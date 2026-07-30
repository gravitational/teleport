package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/trace"
)

func TestValidateBeamOwnership(t *testing.T) {
	tests := []struct {
		name        string
		currentUser string
		beamOwner   string
		wantErr     bool
		wantErrType string
	}{
		{
			name:        "owner can access own beam",
			currentUser: "alice",
			beamOwner:   "alice",
			wantErr:     false,
		},
		{
			name:        "other user cannot access beam",
			currentUser: "bob",
			beamOwner:   "alice",
			wantErr:     true,
			wantErrType: "access denied",
		},
		{
			name:        "empty current user",
			currentUser: "",
			beamOwner:   "alice",
			wantErr:     true,
			wantErrType: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beam := createTestBeam(tt.beamOwner, "test-beam-id", "test-alias")
			err := validateBeamOwnership(tt.currentUser, beam)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrType != "" {
					assert.True(t, trace.IsAccessDenied(err), "expected access denied error")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// createTestBeam creates a test beam with the given owner, id, and alias.
func createTestBeam(owner, id, alias string) *beamsv1.Beam {
	return beamsv1.Beam_builder{
		Kind:    "beam",
		Version: "v1",
		Metadata: headerv1.Metadata_builder{
			Name: id,
		}.Build(),
		Status: beamsv1.BeamStatus_builder{
			User:  owner,
			Alias: alias,
		}.Build(),
	}.Build()
}
