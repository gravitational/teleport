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

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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

	username, chal := "alice", newValidatedMFAChallenge()

	_, err = svc.GetValidatedMFAChallenge(t.Context(), username, "does-not-exist")
	require.ErrorIs(t, err, trace.NotFound(`validated_mfa_challenge "does-not-exist" doesn't exist`))

	startTime := time.Now()

	created, err := svc.CreateValidatedMFAChallenge(t.Context(), username, chal)
	require.NoError(t, err)

	want := proto.Clone(chal).(*mfav1.ValidatedMFAChallenge)

	require.Empty(
		t,
		cmp.Diff(
			want,
			created,
			// Ignore expiration time in comparison.
			protocmp.Transform(),
			protocmp.IgnoreFields(&headerv1.Metadata{}, "expires"),
		),
		"CreateValidatedMFAChallenge mismatch (-want +got)",
	)

	// Expiration time should be roughly 5 minutes from creation.
	require.WithinDuration(t, startTime.Add(5*time.Minute), *created.Metadata.Expires, time.Second)

	got, err := svc.GetValidatedMFAChallenge(t.Context(), username, chal.Metadata.Name)
	require.NoError(t, err)

	require.Empty(
		t,
		cmp.Diff(created, got),
		"GetValidatedMFAChallenge mismatch (-want +got)",
	)
}

func TestMFAService_CreateValidatedMFAChallenge_Validation(t *testing.T) {
	t.Parallel()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	defaultUsername := "bob"
	emptyUsername := ""

	for _, testCase := range []struct {
		name     string
		username *string
		chal     *mfav1.ValidatedMFAChallenge
		wantErr  error
	}{
		{
			name:     "valid challenge",
			username: &defaultUsername,
			chal:     newValidatedMFAChallenge(),
			wantErr:  nil,
		},
		{
			name:     "missing username",
			username: &emptyUsername,
			wantErr:  trace.BadParameter("param username must not be empty"),
		},
		{
			name:     "nil challenge",
			username: &defaultUsername,
			chal:     nil,
			wantErr:  trace.BadParameter("param chal must not be nil"),
		},
		{
			name:     "invalid kind",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Kind = "wrong_kind"
				return c
			}(),
			wantErr: trace.BadParameter("invalid kind"),
		},
		{
			name:     "invalid version",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Version = "v2"
				return c
			}(),
			wantErr: trace.BadParameter("invalid version"),
		},
		{
			name:     "missing metadata",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Metadata = nil
				return c
			}(),
			wantErr: trace.BadParameter("metadata must be set"),
		},
		{
			name:     "missing name",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Metadata.Name = ""
				return c
			}(),
			wantErr: trace.BadParameter("name must be set"),
		},
		{
			name:     "missing spec",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec = nil
				return c
			}(),
			wantErr: trace.BadParameter("spec must be set"),
		},
		{
			name:     "missing payload",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.Payload = nil
				return c
			}(),
			wantErr: trace.BadParameter("payload must be set"),
		},
		{
			name:     "empty ssh_session_id",
			username: &defaultUsername,
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
			name:     "missing source_cluster",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.SourceCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("source_cluster must be set"),
		},
		{
			name:     "missing target_cluster",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge()
				c.Spec.TargetCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("target_cluster must be set"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := svc.CreateValidatedMFAChallenge(t.Context(), *testCase.username, testCase.chal)
			if testCase.wantErr != nil {
				assert.ErrorContains(t, err, testCase.wantErr.Error())
			} else {
				assert.NoError(t, err)
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
			SourceCluster: "src",
			TargetCluster: "tgt",
		},
	}
}
