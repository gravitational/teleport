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
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authtest"
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

// CompleteBrowserMFAChallenge mocks the completion of a browser MFA challenge.
func (m *mockAuthServer) CompleteBrowserMFAChallenge(
	ctx context.Context,
	requestID string,
	webauthnResponse *webauthnpb.CredentialAssertionResponse,
) (string, error) {
	_, ok := m.requestIDs.Load(requestID)
	if !ok {
		return "", trace.NotFound("invalid browser MFA challenge request ID %q", requestID)
	}

	// Return a mock redirect URL for testing
	return "http://127.0.0.1:62972/callback?response=mock-encrypted-response", nil
}

type mockAuthServerIdentity struct {
	services.Identity

	devices []*types.MFADevice
}
