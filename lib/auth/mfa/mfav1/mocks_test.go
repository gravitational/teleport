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

	webauthnpb "github.com/gravitational/teleport/api/types/webauthn"
	"github.com/gravitational/teleport/lib/auth/authtest"
)

// mockAuthServer is a mock implementation of AuthServer for testing MFA challenges.
type mockAuthServer struct {
	*authtest.Server

	// requestIDs stores valid request IDs.
	requestIDs sync.Map
}

// NewMockAuthServer creates a new instance of mockAuthServer.
func NewMockAuthServer(cfg authtest.ServerConfig) (*mockAuthServer, error) {
	authServer, err := authtest.NewTestServer(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &mockAuthServer{Server: authServer}, nil
}

// CompleteBrowserMFAChallenge mocks the completion of a browser MFA challenge.
func (m *mockAuthServer) CompleteBrowserMFAChallenge(
	_ context.Context,
	requestID string,
	_ *webauthnpb.CredentialAssertionResponse,
) (string, error) {
	if !m.validRequestID(requestID) {
		return "", trace.NotFound("invalid browser MFA challenge request ID %q", requestID)
	}

	// Return a mock redirect URL for testing.
	return "http://127.0.0.1:62972/callback?response=mock-encrypted-response", nil
}

// validRequestID checks if a request ID was previously stored.
func (m *mockAuthServer) validRequestID(requestID string) bool {
	_, ok := m.requestIDs.Load(requestID)
	return ok
}
