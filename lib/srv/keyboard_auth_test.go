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

package srv_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	sshpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/ssh/v1"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
)

func TestKeyboardInteractiveAuth_NoPreconds(t *testing.T) {
	t.Parallel()

	h, id := setupKeyboardInteractiveAuthTest(t)

	preconds := []*decisionpb.Precondition{
		// No preconditions.
	}

	inPerms := &ssh.Permissions{
		Extensions: map[string]string{
			"foo": "bar",
		},
	}

	outPerms, err := h.KeyboardInteractiveAuth(t.Context(), preconds, id, inPerms, "test-cluster")
	require.NoError(t, err)
	require.Empty(
		t,
		cmp.Diff(
			inPerms,
			outPerms,
		),
		"KeyboardInteractiveAuth() perms mismatch (-want +got)",
	)
}

func TestKeyboardInteractiveAuth_PreCondInBandMFA_Success(t *testing.T) {
	t.Parallel()

	h, id := setupKeyboardInteractiveAuthTest(t)

	preconds := []*decisionpb.Precondition{
		{
			Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
		},
	}

	inPerms := &ssh.Permissions{
		Extensions: map[string]string{
			"foo": "bar",
		},
	}

	outPerms, err := h.KeyboardInteractiveAuth(t.Context(), preconds, id, inPerms, "test-cluster")
	require.Nil(t, outPerms)

	var sshErr *ssh.PartialSuccessError
	require.ErrorAs(t, err, &sshErr)
	require.NotNil(t, sshErr.Next)
	require.NotNil(t, sshErr.Next.PublicKeyCallback)
	require.NotNil(t, sshErr.Next.KeyboardInteractiveCallback)

	// Verify that the PublicKeyCallback returns the original permissions.
	// TODO(cthach): Remove in v20.0 when PublicKeyCallback is removed.
	outPerms, err = sshErr.Next.PublicKeyCallback(nil, nil)
	require.NoError(t, err)
	require.Empty(
		t,
		cmp.Diff(
			inPerms,
			outPerms,
		),
		"PublicKeyCallback() perms mismatch (-want +got)",
	)

	resp := &sshpb.MFAPromptResponse{
		Response: &sshpb.MFAPromptResponse_Reference{
			Reference: &sshpb.MFAPromptResponseReference{
				ChallengeName: "test-challenge-name",
			},
		},
	}
	respJSON, err := protojson.Marshal(resp)
	require.NoError(t, err)

	metadata := &mockConnMetadata{
		sessionID: []byte("test-session-id"),
		user:      "test-user",
	}

	// Verify that the KeyboardInteractiveCallback processes the MFA response and returns the original permissions.
	outPerms, err = sshErr.Next.KeyboardInteractiveCallback(
		metadata,
		mockKeyboardInteractiveChallengeRaw([]string{string(respJSON)}),
	)
	require.NoError(t, err)
	require.Empty(
		t,
		cmp.Diff(
			inPerms,
			outPerms,
		),
		"KeyboardInteractiveCallback() perms mismatch (-want +got)",
	)
}

// TODO(cthach): Remove in v20.0 when PublicKeyCallback is removed.
func TestKeyboardInteractiveAuth_PreCondInBandMFA_LegacyPublicKeyCallback_RegularCertDenied(t *testing.T) {
	t.Parallel()

	h, id := setupKeyboardInteractiveAuthTest(t)

	id.MFAVerified = "" // Simulate no MFA verification, indicating a regular SSH certificate.

	preconds := []*decisionpb.Precondition{
		{Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA},
	}

	inPerms := &ssh.Permissions{}

	outPerms, err := h.KeyboardInteractiveAuth(t.Context(), preconds, id, inPerms, "test-cluster")
	require.Nil(t, outPerms)

	var sshErr *ssh.PartialSuccessError
	require.ErrorAs(t, err, &sshErr)
	require.NotNil(t, sshErr.Next)
	require.NotNil(t, sshErr.Next.PublicKeyCallback)
	require.NotNil(t, sshErr.Next.KeyboardInteractiveCallback)

	// Verify that the PublicKeyCallback denies authentication since MFA is required but a regular SSH certificate is used.
	outPerms, err = sshErr.Next.PublicKeyCallback(nil, nil)
	require.Nil(t, outPerms)
	require.ErrorIs(
		t,
		err,
		trace.AccessDenied("regular SSH certificates are forbidden when MFA is required and using legacy public key authentication"),
	)
}

// TODO(cthach): Remove in v20.0 when PublicKeyCallback is removed.
func TestKeyboardInteractiveAuth_ForceInBandMFAEnv_DisablesLegacyPublicKeyCallback(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_FORCE_IN_BAND_MFA", "yes")

	h, id := setupKeyboardInteractiveAuthTest(t)

	preconds := []*decisionpb.Precondition{
		{
			Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_IN_BAND_MFA,
		},
	}

	inPerms := &ssh.Permissions{}

	outPerms, err := h.KeyboardInteractiveAuth(t.Context(), preconds, id, inPerms, "test-cluster")
	require.Nil(t, outPerms)

	var sshErr *ssh.PartialSuccessError
	require.ErrorAs(t, err, &sshErr)
	require.NotNil(t, sshErr.Next)
	require.NotNil(t, sshErr.Next.PublicKeyCallback)
	require.NotNil(t, sshErr.Next.KeyboardInteractiveCallback)

	outPerms, err = sshErr.Next.PublicKeyCallback(nil, nil)
	require.Nil(t, outPerms)
	require.ErrorIs(
		t,
		err,
		trace.AccessDenied(`legacy public key authentication is forbidden (TELEPORT_UNSTABLE_FORCE_IN_BAND_MFA = "yes")`),
	)
}

func TestKeyboardInteractiveAuth_PreCondUnknownKind(t *testing.T) {
	t.Parallel()

	h, id := setupKeyboardInteractiveAuthTest(t)

	preconds := []*decisionpb.Precondition{
		{
			Kind: decisionpb.PreconditionKind_PRECONDITION_KIND_UNSPECIFIED,
		},
	}

	inPerms := &ssh.Permissions{}

	outPerms, err := h.KeyboardInteractiveAuth(t.Context(), preconds, id, inPerms, "test-cluster")
	require.Nil(t, outPerms)
	require.ErrorIs(t, err, trace.BadParameter(`unexpected precondition type "PRECONDITION_KIND_UNSPECIFIED" found (this is a bug)`))
}

func setupKeyboardInteractiveAuthTest(t *testing.T) (*srv.AuthHandlers, *sshca.Identity) {
	t.Helper()

	authSvr := &mockServer{}

	config := &srv.AuthHandlerConfig{
		Server:                        authSvr,
		Emitter:                       &eventstest.MockRecorderEmitter{},
		AccessPoint:                   authSvr.GetAccessPoint(),
		ValidatedMFAChallengeVerifier: &mockMFAServiceClient{},
	}

	h, err := srv.NewAuthHandlers(config)
	require.NoError(t, err)

	id := &sshca.Identity{
		Username:    "test-user",
		ClusterName: "test-cluster",
		MFAVerified: "non-empty-means-mfa-was-verified",
	}

	return h, id
}

type mockAccessPoint struct {
	srv.AccessPoint
}

type mockServer struct {
	srv.Server
}

func (m *mockServer) GetAccessPoint() srv.AccessPoint {
	return &mockAccessPoint{}
}

type mockMFAServiceClient struct {
	mfav1.MFAServiceClient
}

var _ mfav1.MFAServiceClient = (*mockMFAServiceClient)(nil)

func (m *mockMFAServiceClient) VerifyValidatedMFAChallenge(_ context.Context, _ *mfav1.VerifyValidatedMFAChallengeRequest, _ ...grpc.CallOption) (*mfav1.VerifyValidatedMFAChallengeResponse, error) {
	return &mfav1.VerifyValidatedMFAChallengeResponse{}, nil
}

type mockConnMetadata struct {
	ssh.ConnMetadata

	sessionID []byte
	user      string
}

func (m *mockConnMetadata) SessionID() []byte { return m.sessionID }
func (m *mockConnMetadata) User() string      { return m.user }

func mockKeyboardInteractiveChallengeRaw(answers []string) ssh.KeyboardInteractiveChallenge {
	return func(_ string, _ string, _ []string, _ []bool) ([]string, error) {
		return answers, nil
	}
}
