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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	local "github.com/gravitational/teleport/lib/services/local"
)

func TestMFAService_CRUD(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	svc, err := local.NewMFAService(backend)
	require.NoError(t, err)

	username, chal := "alice", newValidatedMFAChallenge(t)

	t.Run("get non-existent", func(t *testing.T) {
		t.Parallel()

		_, err = svc.GetValidatedMFAChallenge(t.Context(), username, "does-not-exist")

		var notFoundErr *trace.NotFoundError

		assert.ErrorAs(t, err, &notFoundErr, "error type mismatch")
	})

	created, err := svc.CreateValidatedMFAChallenge(t.Context(), username, chal)
	require.NoError(t, err)
	require.Equal(t, chal.Kind, created.Kind)
	require.Equal(t, chal.Version, created.Version)
	require.Equal(t, chal.Metadata.Name, created.Metadata.Name)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), *created.Metadata.Expires, time.Second)

	got, err := svc.GetValidatedMFAChallenge(t.Context(), username, chal.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, created.Metadata.Name, got.Metadata.Name)
	require.Equal(t, created.Spec.SourceCluster, got.Spec.SourceCluster)
	require.Equal(t, created.Spec.TargetCluster, got.Spec.TargetCluster)
	require.WithinDuration(t, *created.Metadata.Expires, *got.Metadata.Expires, time.Second)
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
			chal:     newValidatedMFAChallenge(t),
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
				c := newValidatedMFAChallenge(t)
				c.Kind = "wrong_kind"
				return c
			}(),
			wantErr: trace.BadParameter("invalid kind"),
		},
		{
			name:     "invalid version",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Version = "v2"
				return c
			}(),
			wantErr: trace.BadParameter("invalid version"),
		},
		{
			name:     "missing metadata",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Metadata = nil
				return c
			}(),
			wantErr: trace.BadParameter("metadata must be set"),
		},
		{
			name:     "missing name",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Metadata.Name = ""
				return c
			}(),
			wantErr: trace.BadParameter("name must be set"),
		},
		{
			name:     "missing spec",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Spec = nil
				return c
			}(),
			wantErr: trace.BadParameter("spec must be set"),
		},
		{
			name:     "missing payload",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Spec.Payload = nil
				return c
			}(),
			wantErr: trace.BadParameter("payload must be set"),
		},
		{
			name:     "empty ssh_session_id",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
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
				c := newValidatedMFAChallenge(t)
				c.Spec.SourceCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("source_cluster must be set"),
		},
		{
			name:     "missing target_cluster",
			username: &defaultUsername,
			chal: func() *mfav1.ValidatedMFAChallenge {
				c := newValidatedMFAChallenge(t)
				c.Spec.TargetCluster = ""
				return c
			}(),
			wantErr: trace.BadParameter("target_cluster must be set"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := svc.CreateValidatedMFAChallenge(t.Context(), *testCase.username, testCase.chal)
			if testCase.wantErr != nil {
				require.ErrorContains(t, err, testCase.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newValidatedMFAChallenge(t *testing.T) *mfav1.ValidatedMFAChallenge {
	t.Helper()

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
