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

package mfav1_test

import (
	"context"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// mockAuthServer is a mock implementation of AuthServer for testing MFA challenges.
type mockAuthServer struct {
	*authtest.Server

	// requestIDs stores valid request IDs.
	requestIDs sync.Map
}

// NewMockAuthServer creates a new instance of mockAuthServer.
func NewMockAuthServer(cfg authtest.ServerConfig, devices []*types.MFADevice) (*mockAuthServer, error) {
	// The authtest.AuthServer implementation currently does not support SSO MFA devices like it does for TOTP and
	// Webauthn. We work around this by wrapping the Identity service with our mock that returns the provided SSO MFA
	// devices after merging with any registered devices during the test. Additionally, this mock AuthServer that wraps
	// authtest.AuthServer overrides the SSO MFA challenge methods to provide mock implementations. Support for SSO MFA
	// devices will be added in https://github.com/gravitational/teleport/issues/62271.
	// TODO(cthach): Remove this workaround once authtest.AuthServer supports SSO MFA devices.
	authServer, err := authtest.NewTestServer(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap the Identity service to allow mocking MFA devices.
	authServer.Auth().Identity = &mockAuthServerIdentity{Identity: authServer.Auth().Identity, devices: devices}

	return &mockAuthServer{Server: authServer}, nil
}

// BeginSSOMFAChallenge mocks the SSO MFA challenge initiation.
func (m *mockAuthServer) BeginSSOMFAChallenge(
	_ context.Context,
	params mfatypes.BeginSSOMFAChallengeParams,
) (*proto.SSOChallenge, error) {
	requestID := strconv.Itoa(int(time.Now().UnixNano()))
	m.requestIDs.Store(requestID, struct{}{})

	return &proto.SSOChallenge{
		RequestId:   requestID,
		Device:      params.SSO,
		RedirectUrl: params.SSOClientRedirectURL,
	}, nil
}

// VerifySSOMFASession mocks the verification of an SSO MFA session.
func (m *mockAuthServer) VerifySSOMFASession(
	ctx context.Context,
	username string,
	requestID string,
	token string,
	_ *mfav1.ChallengeExtensions,
) (*authz.MFAAuthData, error) {
	_, ok := m.requestIDs.Load(requestID)
	if !ok {
		return nil, trace.AccessDenied("invalid SSO MFA challenge request ID %q", requestID)
	}

	devices, err := m.Auth().Identity.GetMFADevices(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Find the first SSO MFA device. Good enough for this mock.
	var ssoDevice *types.MFADevice

	for _, dev := range devices {
		if _, ok := dev.Device.(*types.MFADevice_Sso); ok {
			ssoDevice = dev
			break
		}
	}

	if ssoDevice == nil {
		return nil, trace.NotFound("SSO MFA device not found %q", requestID)
	}

	return &authz.MFAAuthData{
		Device: ssoDevice,
		Payload: &mfatypes.SessionIdentifyingPayload{
			SSHSessionID: []byte("test-session-id"),
		},
		SourceCluster: "test-cluster",
		TargetCluster: "test-cluster",
	}, nil
}

type mockAuthServerIdentity struct {
	services.Identity

	devices []*types.MFADevice
}

// GetMFADevices mocks retrieval of MFA devices for a user.
func (m *mockAuthServerIdentity) GetMFADevices(
	ctx context.Context,
	username string,
	withSecrets bool,
) ([]*types.MFADevice, error) {
	devices, err := m.Identity.GetMFADevices(ctx, username, withSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Combine the devices that were passed in NewMockAuthServer with any registered after the mock was created.
	return slices.Concat([]*types.MFADevice{}, m.devices, devices), nil
}

type mockMFAService struct {
	chal *mfav1.ValidatedMFAChallenge
	mu   sync.Mutex

	createValidatedMFAChallengeError error
	getValidatedMFAChallengeError    error

	listValidatedMFAChallenges          []*mfav1.ValidatedMFAChallenge
	listValidatedMFAChallengesToken     string
	listValidatedMFAChallengesError     error
	listValidatedMFAChallengesPageSize  int32
	listValidatedMFAChallengesPageToken string
	listValidatedMFAChallengesTarget    string
}

func (m *mockMFAService) CreateValidatedMFAChallenge(
	_ context.Context,
	_ string,
	chal *mfav1.ValidatedMFAChallenge,
) (*mfav1.ValidatedMFAChallenge, error) {
	if m.createValidatedMFAChallengeError != nil {
		return nil, m.createValidatedMFAChallengeError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.chal = chal

	return m.chal, nil
}

func (m *mockMFAService) GetValidatedMFAChallenge(
	_ context.Context,
	_ string,
	_ string,
) (*mfav1.ValidatedMFAChallenge, error) {
	if m.getValidatedMFAChallengeError != nil {
		return nil, m.getValidatedMFAChallengeError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return m.chal, nil
}

func (m *mockMFAService) ListValidatedMFAChallenges(
	_ context.Context,
	pageSize int32,
	pageToken string,
	targetCluster string,
) ([]*mfav1.ValidatedMFAChallenge, string, error) {
	if m.listValidatedMFAChallengesError != nil {
		return nil, "", m.listValidatedMFAChallengesError
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.listValidatedMFAChallengesPageSize = pageSize
	m.listValidatedMFAChallengesPageToken = pageToken
	m.listValidatedMFAChallengesTarget = targetCluster

	return m.listValidatedMFAChallenges, m.listValidatedMFAChallengesToken, nil
}
