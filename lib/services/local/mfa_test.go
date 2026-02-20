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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	local "github.com/gravitational/teleport/lib/services/local"
)

func TestMFAService_CRUD(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	targetCluster, chal := "leaf-a", newValidatedMFAChallenge()
	chal.Spec.TargetCluster = targetCluster

	_, err = svc.GetValidatedMFAChallenge(t.Context(), targetCluster, "does-not-exist")
	require.ErrorIs(t, err, trace.NotFound(`validated_mfa_challenge "does-not-exist" doesn't exist`))

	startTime := time.Now()

	created, err := svc.CreateValidatedMFAChallenge(t.Context(), targetCluster, chal)
	require.NoError(t, err)

	want := newValidatedMFAChallenge()
	want.Spec.TargetCluster = targetCluster

	require.Empty(
		t,
		cmp.Diff(
			want,
			created,
			// Ignore expiration time in comparison.
			cmpopts.IgnoreFields(types.Metadata{}, "Expires"),
		),
		"CreateValidatedMFAChallenge mismatch (-want +got)",
	)

	// Expiration time should be roughly 5 minutes from creation.
	require.WithinDuration(t, startTime.Add(5*time.Minute), *created.Metadata.Expires, time.Second)

	got, err := svc.GetValidatedMFAChallenge(t.Context(), targetCluster, chal.Metadata.Name)
	require.NoError(t, err)

	require.Empty(
		t,
		cmp.Diff(created, got),
		"GetValidatedMFAChallenge mismatch (-want +got)",
	)

	challenges, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 10, "", "")
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, challenges, 1)
	require.Empty(t, cmp.Diff(created, challenges[0]))
}

func TestMFAService_CreateValidatedMFAChallenge_Validation(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	defaultTargetCluster := "tgt"
	emptyTargetCluster := ""

	for _, testCase := range []struct {
		name          string
		targetCluster *string
		chal          *mfav1.ValidatedMFAChallenge
		wantErr       error
	}{
		{
			name:          "valid challenge",
			targetCluster: &defaultTargetCluster,
			chal:          newValidatedMFAChallenge(),
			wantErr:       nil,
		},
		{
			name:          "missing target cluster",
			targetCluster: &emptyTargetCluster,
			wantErr:       trace.BadParameter("param targetCluster must not be empty"),
		},
		{
			name:          "nil challenge",
			targetCluster: &defaultTargetCluster,
			chal:          nil,
			wantErr:       trace.BadParameter("param chal must not be nil"),
		},
		{
			name:          "invalid kind",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Kind = "wrong_kind"
				return c
			}(),
			wantErr: trace.BadParameter("invalid kind"),
		},
		{
			name:          "invalid version",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Version = "v2"
				return c
			}(),
			wantErr: trace.BadParameter("invalid version"),
		},
		{
			name:          "missing metadata",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Metadata = nil
				return c
			}(),
			wantErr: trace.BadParameter("metadata must be set"),
		},
		{
			name:          "missing name",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Metadata.Name = ""
				return c
			}(),
			wantErr: trace.BadParameter("name must be set"),
		},
		{
			name:          "missing spec",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec = nil
				return c
			}(),
			wantErr: trace.BadParameter("spec must be set"),
		},
		{
			name:          "missing payload",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.Payload = nil
				return c
			}(),
			wantErr: trace.BadParameter("payload must be set"),
		},
		{
			name:          "empty ssh_session_id",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.Payload = &mfav1.SessionIdentifyingPayload{
					Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("")},
				}
				return c
			}(),
			wantErr: trace.BadParameter("ssh_session_id must be set"),
		},
		{
			name:          "missing source_cluster",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.SourceCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("source_cluster must be set"),
		},
		{
			name:          "missing target_cluster",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.TargetCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("target_cluster must be set"),
		},
		{
			name:          "missing username",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.Username = ""
				return c
			}(),
			wantErr: trace.BadParameter("username must be set"),
		},
		{
			name:          "request target_cluster mismatch",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.TargetCluster = "leaf-b"
				return c
			}(),
			wantErr: trace.BadParameter("param targetCluster does not match challenge target cluster"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := svc.CreateValidatedMFAChallenge(t.Context(), *testCase.targetCluster, testCase.chal)
			if testCase.wantErr != nil {
				assert.ErrorContains(t, err, testCase.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMFAService_ListValidatedMFAChallenges_Success(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	challenges := make([]*mfav1.ValidatedMFAChallenge, 0, 3)
	for _, tc := range []struct {
		name     string
		username string
	}{
		{name: "chal-1", username: "alice"},
		{name: "chal-2", username: "alice"},
		{name: "chal-3", username: "bob"},
	} {
		chal := newValidatedMFAChallenge()
		chal.Metadata.Name = tc.name
		chal.Spec.Username = tc.username

		challenges = append(challenges, chal)
	}
	for _, chal := range challenges {
		_, err = svc.CreateValidatedMFAChallenge(t.Context(), chal.GetSpec().GetTargetCluster(), chal)
		require.NoError(t, err)
	}

	got, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 2, "", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.NotEmpty(t, nextPageToken)

	gotNext, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 2, nextPageToken, "")
	require.NoError(t, err)
	require.Len(t, gotNext, 1)
	require.Empty(t, nextPageToken)

	all := append(got, gotNext...)

	want := &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: challenges,
	}

	gotResp := &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: all,
	}

	require.Empty(t, cmp.Diff(
		want,
		gotResp,
	), "ListValidatedMFAChallenges mismatch (-want +got)")

}

func TestMFAService_ListValidatedMFAChallenges_FilterByTargetCluster(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	challenges := make([]*mfav1.ValidatedMFAChallenge, 0, 3)
	for _, tc := range []struct {
		name          string
		username      string
		targetCluster string
	}{
		{name: "chal-target-a-1", username: "alice", targetCluster: "leaf-a"},
		{name: "chal-target-b", username: "alice", targetCluster: "leaf-b"},
		{name: "chal-target-a-2", username: "bob", targetCluster: "leaf-a"},
	} {
		chal := newValidatedMFAChallenge()
		chal.Metadata.Name = tc.name
		chal.Spec.Username = tc.username
		chal.Spec.TargetCluster = tc.targetCluster

		challenges = append(challenges, chal)
	}
	for _, chal := range challenges {
		_, err = svc.CreateValidatedMFAChallenge(t.Context(), chal.GetSpec().GetTargetCluster(), chal)
		require.NoError(t, err)
	}

	t.Run("empty target cluster returns all challenges", func(t *testing.T) {
		got, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 10, "", "")
		require.NoError(t, err)
		require.Empty(t, nextPageToken)
		require.Len(t, got, len(challenges))
	})

	t.Run("target cluster filter returns only matching challenges", func(t *testing.T) {
		got, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 10, "", "leaf-a")
		require.NoError(t, err)
		require.Empty(t, nextPageToken)
		require.Len(t, got, 2)
		for _, chal := range got {
			require.Equal(t, "leaf-a", chal.GetSpec().GetTargetCluster())
		}
	})

	t.Run("target cluster filter with no matches returns empty result", func(t *testing.T) {
		got, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 10, "", "leaf-c")
		require.NoError(t, err)
		require.Empty(t, nextPageToken)
		require.Empty(t, got)
	})
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
			SourceCluster: "src",
			TargetCluster: "tgt",
			Username:      "alice",
		},
	}
}
