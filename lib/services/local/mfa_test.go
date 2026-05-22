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
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
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
	chal.GetSpec().SetTargetCluster(targetCluster)

	_, err = svc.GetValidatedMFAChallenge(t.Context(), targetCluster, "does-not-exist")
	require.ErrorIs(t, err, trace.NotFound(`validated_mfa_challenge "does-not-exist" doesn't exist`))

	startTime := time.Now()

	created, err := svc.CreateValidatedMFAChallenge(t.Context(), targetCluster, chal)
	require.NoError(t, err)

	want := newValidatedMFAChallenge()
	want.GetSpec().SetTargetCluster(targetCluster)

	require.Empty(
		t,
		cmp.Diff(
			want,
			created,
			// Ignore expiration time and revision in comparison.
			protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
			protocmp.Transform(),
		),
		"CreateValidatedMFAChallenge mismatch (-want +got)",
	)

	// Expiration time should be roughly 5 minutes from creation.
	require.WithinDuration(t, startTime.Add(5*time.Minute), created.GetMetadata().GetExpires().AsTime(), time.Second)

	got, err := svc.GetValidatedMFAChallenge(t.Context(), targetCluster, chal.GetMetadata().GetName())
	require.NoError(t, err)

	require.Empty(
		t,
		cmp.Diff(created, got, protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"), protocmp.Transform()),
		"GetValidatedMFAChallenge mismatch (-want +got)",
	)

	challenges, nextPageToken, err := svc.ListValidatedMFAChallenges(t.Context(), 10, "", "")
	require.NoError(t, err)
	require.Empty(t, nextPageToken)
	require.Len(t, challenges, 1)
	require.Empty(t, cmp.Diff(created, challenges[0], protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"), protocmp.Transform()))
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
		chal          *mfav2.ValidatedMFAChallenge
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
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.SetKind("wrong_kind")
				return c
			}(),
			wantErr: trace.BadParameter("invalid kind"),
		},
		{
			name:          "invalid version",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.SetVersion("v2")
				return c
			}(),
			wantErr: trace.BadParameter("invalid version"),
		},
		{
			name:          "missing metadata",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.SetMetadata(nil)
				return c
			}(),
			wantErr: trace.BadParameter("metadata must be set"),
		},
		{
			name:          "missing name",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetMetadata().Name = ""
				return c
			}(),
			wantErr: trace.BadParameter("name must be set"),
		},
		{
			name:          "missing spec",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.SetSpec(nil)
				return c
			}(),
			wantErr: trace.BadParameter("spec must be set"),
		},
		{
			name:          "missing payload",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetSpec().SetPayload(nil)
				return c
			}(),
			wantErr: trace.BadParameter("payload must be set"),
		},
		{
			name:          "empty ssh_session_id",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				payload := &mfav2.SessionIdentifyingPayload{}
				payload.SetSshSessionId([]byte(""))
				c.GetSpec().SetPayload(payload)
				return c
			}(),
			wantErr: trace.BadParameter("ssh_session_id must be set"),
		},
		{
			name:          "missing source_cluster",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetSpec().SetSourceCluster("")
				return c
			}(),
			wantErr: trace.BadParameter("source_cluster must be set"),
		},
		{
			name:          "missing target_cluster",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetSpec().SetTargetCluster("")
				return c
			}(),
			wantErr: trace.BadParameter("target_cluster must be set"),
		},
		{
			name:          "missing username",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetSpec().SetUsername("")
				return c
			}(),
			wantErr: trace.BadParameter("username must be set"),
		},
		{
			name:          "request target_cluster mismatch",
			targetCluster: &defaultTargetCluster,
			chal: func() *mfav2.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.GetSpec().SetTargetCluster("leaf-b")
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

	challenges := make([]*mfav2.ValidatedMFAChallenge, 0, 3)
	for _, tc := range []struct {
		name     string
		username string
	}{
		{name: "chal-1", username: "alice"},
		{name: "chal-2", username: "alice"},
		{name: "chal-3", username: "bob"},
	} {
		chal := newValidatedMFAChallenge()
		chal.GetMetadata().Name = tc.name
		chal.GetSpec().SetUsername(tc.username)

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

	want := &mfav2.ListValidatedMFAChallengesResponse{}
	want.SetValidatedChallenges(challenges)

	gotResp := &mfav2.ListValidatedMFAChallengesResponse{}
	gotResp.SetValidatedChallenges(all)

	require.Empty(t, cmp.Diff(
		want,
		gotResp,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "expires", "revision"),
		protocmp.Transform(),
	), "ListValidatedMFAChallenges mismatch (-want +got)")

}

func TestMFAService_ListValidatedMFAChallenges_FilterByTargetCluster(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	challenges := make([]*mfav2.ValidatedMFAChallenge, 0, 3)
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
		chal.GetMetadata().Name = tc.name
		chal.GetSpec().SetUsername(tc.username)
		chal.GetSpec().SetTargetCluster(tc.targetCluster)

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

func newValidatedMFAChallenge() *mfav2.ValidatedMFAChallenge {
	return mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test-challenge",
		},
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload: mfav2.SessionIdentifyingPayload_builder{
				SshSessionId: []byte("session-id"),
			}.Build(),
			SourceCluster: "src",
			TargetCluster: "tgt",
			Username:      "alice",
		}.Build(),
	}.Build()
}
