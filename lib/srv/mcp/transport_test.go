// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package mcp

import (
	"context"
	"crypto/x509"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMakeBasicHTTPTransportClientCert(t *testing.T) {
	newServer := func(t *testing.T, clt workloadidentityv1.WorkloadIdentityIssuanceServiceClient) *Server {
		t.Helper()
		s, err := NewServer(ServerConfig{
			Emitter:       &eventstest.MockRecorderEmitter{},
			ParentContext: t.Context(),
			HostID:        "my-host-id",
			AccessPoint:   fakeAccessPoint{},
			CipherSuites:  utils.DefaultCipherSuites(),
			AuthClient:    &mockAuthClient{workloadIdentityClt: clt},
		})
		require.NoError(t, err)
		return s
	}

	newApp := func(t *testing.T, clientCertMode types.AppClientCertMode) types.Application {
		t.Helper()
		app, err := types.NewAppV3(types.Metadata{Name: "test-mcp"}, types.AppSpecV3{
			URI: "mcp+sse+https://example.com/sse",
			TLS: &types.AppTLS{
				Mode:           types.AppTLSModeVerifyServerName,
				ClientCertMode: clientCertMode,
			},
		})
		require.NoError(t, err)
		return app
	}

	// ctxWithUserCert returns a context carrying a user certificate, as set by
	// the connections handler before dispatching to the MCP server.
	ctxWithUserCert := func() context.Context {
		return authz.ContextWithUserCertificate(context.Background(), &x509.Certificate{Raw: []byte("user-cert")})
	}

	t.Run("non-managed app does not require a workload identity client", func(t *testing.T) {
		s := newServer(t, nil)
		rt, err := s.makeBasicHTTPTransport(context.Background(), newApp(t, types.AppClientCertModeDisabled))
		require.NoError(t, err)

		require.IsType(t, &http.Transport{}, rt)
		tr, _ := rt.(*http.Transport)
		require.NotNil(t, tr.TLSClientConfig)
		require.Nil(t, tr.TLSClientConfig.GetClientCertificate, "no client certificate should be attached")
	})

	t.Run("managed app issues and attaches a client certificate", func(t *testing.T) {
		const ttl = 5 * time.Minute
		s := newServer(t, &fakeIssuanceClient{
			resp: workloadidentityv1.IssueTeleportWorkloadIdentityResponse_builder{
				Credential: workloadidentityv1.Credential_builder{
					Ttl:       durationpb.New(ttl),
					ExpiresAt: timestamppb.New(time.Now().Add(ttl)),
					X509Svid: workloadidentityv1.X509SVIDCredential_builder{
						Cert: []byte("leaf"),
					}.Build(),
				}.Build(),
			}.Build(),
		})

		rt, err := s.makeBasicHTTPTransport(ctxWithUserCert(), newApp(t, types.AppClientCertModeManaged))
		require.NoError(t, err)

		require.IsType(t, &http.Transport{}, rt)
		tr, _ := rt.(*http.Transport)
		require.NotNil(t, tr.TLSClientConfig)
		require.NotNil(t, tr.TLSClientConfig.GetClientCertificate, "managed client certificate should be attached")
	})
}
