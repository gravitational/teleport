package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

func TestCheckBeamOwnershipForSSH(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		serverLabels map[string]string
		wantErr      bool
		wantErrType  string
	}{
		{
			name:     "non-beam server allows any user",
			username: "alice",
			serverLabels: map[string]string{
				"environment": "prod",
			},
			wantErr: false,
		},
		{
			name:     "owner can access own beam",
			username: "alice",
			serverLabels: map[string]string{
				types.BeamIDLabel:    "beam-123",
				types.BeamOwnerLabel: "alice",
			},
			wantErr: false,
		},
		{
			name:     "other user cannot access beam",
			username: "bob",
			serverLabels: map[string]string{
				types.BeamIDLabel:    "beam-123",
				types.BeamOwnerLabel: "alice",
			},
			wantErr:     true,
			wantErrType: "access denied",
		},
		{
			name:     "beam without owner label is denied",
			username: "alice",
			serverLabels: map[string]string{
				types.BeamIDLabel: "beam-123",
				// Missing BeamOwnerLabel
			},
			wantErr:     true,
			wantErrType: "access denied",
		},
		{
			name:     "empty username cannot access beam",
			username: "",
			serverLabels: map[string]string{
				types.BeamIDLabel:    "beam-123",
				types.BeamOwnerLabel: "alice",
			},
			wantErr:     true,
			wantErrType: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server with the specified labels
			server, err := types.NewServer("test-node", types.KindNode, types.ServerSpecV2{})
			require.NoError(t, err)
			server.SetStaticLabels(tt.serverLabels)

			err = checkBeamOwnershipForSSH(tt.username, server)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrType != "" {
					assert.True(t, trace.IsAccessDenied(err), "expected access denied error, got: %v", err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckBeamOwnershipForSSH_DifferentOwners(t *testing.T) {
	// Test multiple users trying to access each other's beams
	users := []string{"alice", "bob", "charlie"}

	for _, owner := range users {
		for _, accessor := range users {
			t.Run(owner+"->"+accessor, func(t *testing.T) {
				server, err := types.NewServer("beam-"+owner, types.KindNode, types.ServerSpecV2{})
				require.NoError(t, err)
				server.SetStaticLabels(map[string]string{
					types.BeamIDLabel:    "beam-" + owner,
					types.BeamOwnerLabel: owner,
				})

				err = checkBeamOwnershipForSSH(accessor, server)

				if accessor == owner {
					// Owner should be allowed
					require.NoError(t, err)
				} else {
					// Non-owner should be denied
					require.Error(t, err)
					assert.True(t, trace.IsAccessDenied(err))
				}
			})
		}
	}
}
