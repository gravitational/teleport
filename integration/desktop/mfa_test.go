/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package desktop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	"github.com/gravitational/teleport/api/types"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestValidatedMFAChallenge_CRUD(t *testing.T) {
	server, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName("test-cluster"),
	)
	require.NoError(t, err)
	t.Cleanup(
		func() {
			_ = server.Close()
		},
	)

	auth := server.GetAuthServer()

	const challengeName = "test-challenge-crud"

	payload := mfav2.SessionIdentifyingPayload_builder{
		TlsSessionId: []byte("test-tls-session-id"),
	}.Build()

	challenge := mfav2.ValidatedMFAChallenge_builder{
		Kind:    types.KindValidatedMFAChallenge,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    challengeName,
			Expires: timestamppb.New(time.Now().Add(5 * time.Minute)),
		},
		Spec: mfav2.ValidatedMFAChallengeSpec_builder{
			Payload:       payload,
			SourceCluster: "test-cluster",
			TargetCluster: "test-cluster",
			Username:      "test-user",
		}.Build(),
	}.Build()

	created, err := auth.CreateValidatedMFAChallenge(t.Context(), "test-cluster", challenge)
	require.NoError(t, err)
	require.Equal(t, challengeName, created.GetMetadata().GetName())

	got, err := auth.GetValidatedMFAChallenge(t.Context(), "test-cluster", challengeName)
	require.NoError(t, err)
	require.Equal(t, challengeName, got.GetMetadata().GetName())
	require.Equal(t, "test-cluster", got.GetSpec().GetSourceCluster())
	require.Equal(t, "test-cluster", got.GetSpec().GetTargetCluster())
	require.Equal(t, "test-user", got.GetSpec().GetUsername())
	require.Equal(t, []byte("test-tls-session-id"), got.GetSpec().GetPayload().GetTlsSessionId())
}
