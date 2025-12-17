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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// mockAuthServer is a mock implementation of AuthServer for testing MFA challenges.
type mockAuthServer struct {
	*authtest.Server
}

// NewMockAuthServer creates a new instance of mockAuthServer.
func NewMockAuthServer(cfg authtest.ServerConfig, devices []*types.MFADevice) (*mockAuthServer, error) {
	authServer, err := authtest.NewTestServer(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Wrap the Identity service to allow mocking MFA devices.
	authServer.Auth().Identity = &mockAuthServerIdentity{Identity: authServer.Auth().Identity, devices: devices}

	return &mockAuthServer{authServer}, nil
}

// BeginSSOMFAChallenge mocks the SSO MFA challenge initiation.
func (m *mockAuthServer) BeginSSOMFAChallenge(
	_ context.Context,
	_ string,
	sso *types.SSOMFADevice,
	ssoClientRedirectURL,
	_ string,
	_ *mfav1.ChallengeExtensions,
	_ *mfav1.SessionIdentifyingPayload,
) (*proto.SSOChallenge, error) {
	return &proto.SSOChallenge{
		Device:      sso,
		RedirectUrl: ssoClientRedirectURL,
	}, nil
}

// VerifySSOMFASession mocks the verification of an SSO MFA session.
func (m *mockAuthServer) VerifySSOMFASession(
	ctx context.Context,
	username,
	sessionID,
	token string,
	_ *mfav1.ChallengeExtensions,
) (*authz.MFAAuthData, error) {
	// In a real implementation, we would verify the token here.
	// For this mock, we simply check if the token matches a magic token that represents a valid session.
	if token != "valid-token" {
		return nil, trace.AccessDenied("invalid SSO MFA token")
	}

	devices, err := m.Auth().Identity.GetMFADevices(ctx, username, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Find the first SSO MFA device. Good enough for this mock.
	var mfaDevice *types.MFADevice

	for _, dev := range devices {
		if _, ok := dev.Device.(*types.MFADevice_Sso); ok {
			mfaDevice = dev
			break
		}
	}

	if mfaDevice == nil {
		return nil, trace.NotFound("SSO MFA device not found %q", sessionID)
	}

	return &authz.MFAAuthData{Device: mfaDevice}, nil
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
