// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package local_test

import (
	"cmp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	local "github.com/gravitational/teleport/lib/services/local"
)

func TestMFAService_CRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(bk)
	require.NoError(t, err)

	username, chal := "alice", newValidatedMFAChallenge()

	// Create
	created, err := svc.CreateValidatedMFAChallenge(t.Context(), username, chal)
	require.NoError(t, err)
	require.Equal(t, chal.Kind, created.Kind)
	require.Equal(t, chal.Version, created.Version)
	require.Equal(t, chal.Metadata.Name, created.Metadata.Name)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), *created.Metadata.Expires, time.Second)

	// Get
	got, err := svc.GetValidatedMFAChallenge(t.Context(), username, chal.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, created.Metadata.Name, got.Metadata.Name)
	require.Equal(t, created.Spec.SourceCluster, got.Spec.SourceCluster)
	require.Equal(t, created.Spec.TargetCluster, got.Spec.TargetCluster)
	require.WithinDuration(t, *created.Metadata.Expires, *got.Metadata.Expires, time.Second)

	// Get non-existent
	_, err = svc.GetValidatedMFAChallenge(t.Context(), username, "does-not-exist")
	require.Error(t, err)
}

func TestMFAService_CreateValidatedMFAChallenge_Validation(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(bk)
	require.NoError(t, err)

	ctx := t.Context()

	defaultUsername := "bob"

	cases := []struct {
		name     string
		username *string
		chal     *mfav1.ValidatedMFAChallenge
		wantErr  bool
	}{
		{
			name:    "valid challenge",
			chal:    newValidatedMFAChallenge(),
			wantErr: false,
		},
		{
			name: "missing username",
			username: func() *string {
				s := ""
				return &s
			}(),
			wantErr: true,
		},
		{
			name:    "nil challenge",
			chal:    nil,
			wantErr: true,
		},
		{
			name: "invalid kind",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     "wrong_kind",
				Version:  "v1",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("id")},
					},
					SourceCluster: "src", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v2",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("id")},
					},
					SourceCluster: "src", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v1",
				Metadata: &types.Metadata{Name: ""},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("id")},
					},
					SourceCluster: "src", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing payload",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v1",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload:       nil,
					SourceCluster: "src", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing ssh_session_id",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v1",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("")},
					},
					SourceCluster: "src", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing source_cluster",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v1",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("id")},
					},
					SourceCluster: "", TargetCluster: "tgt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing target_cluster",
			chal: &mfav1.ValidatedMFAChallenge{
				Kind:     types.KindValidatedMFAChallenge,
				Version:  "v1",
				Metadata: &types.Metadata{Name: "test"},
				Spec: &mfav1.ValidatedMFAChallengeSpec{
					Payload: &mfav1.SessionIdentifyingPayload{
						Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("id")},
					},
					SourceCluster: "src", TargetCluster: "",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateValidatedMFAChallenge(ctx, *cmp.Or(tc.username, &defaultUsername), tc.chal)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newValidatedMFAChallenge() *mfav1.ValidatedMFAChallenge {
	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: "test-challenge",
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("session-id"),
				},
			},
			SourceCluster: "source",
			TargetCluster: "target",
		},
	}
}
