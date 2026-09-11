/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package client

import (
	"cmp"
	"context"
	"crypto/tls"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type mockALPNConn struct {
	net.Conn
	cert     tls.Certificate
	protocol alpncommon.Protocol
}

func newFakeALPNConn(protocol alpncommon.Protocol, cert tls.Certificate) *mockALPNConn {
	return &mockALPNConn{
		Conn:     nil, // currently not using the net.conn
		cert:     cert,
		protocol: protocol,
	}
}

func (c *mockALPNConn) getCertSerialNumber() string {
	leaf, err := utils.TLSCertLeaf(c.cert)
	if err != nil {
		return ""
	}
	return leaf.SerialNumber.String()
}

type mockMCPServerDialerClient struct {
	appServers types.AppServers
	tlsCA      *tlsca.CertAuthority
	clock      *clockwork.FakeClock
	identity   tlsca.Identity

	issueCertCalls    atomic.Int32
	issueCertStarted  chan struct{}
	continueIssueCert <-chan struct{}
	routeToApp        proto.RouteToApp
}

func (m *mockMCPServerDialerClient) DialALPN(_ context.Context, cert tls.Certificate, protocol alpncommon.Protocol) (net.Conn, error) {
	if err := utils.VerifyTLSCertLeafExpiry(cert, m.clock); err != nil {
		return nil, trace.Wrap(err)
	}
	return newFakeALPNConn(protocol, cert), nil
}

func (m *mockMCPServerDialerClient) ListApps(_ context.Context, req *proto.ListResourcesRequest) ([]types.Application, error) {
	filter, err := services.MatchResourceFilterFromListResourceRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServers, err := services.MatchResourcesByFilters(m.appServers, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return slices.Collect(appServers.Applications()), nil
}

func (m *mockMCPServerDialerClient) IssueUserCertsWithMFA(_ context.Context, params ReissueParams) (*KeyRing, error) {
	m.issueCertCalls.Add(1)
	m.routeToApp = params.RouteToApp
	if m.issueCertStarted != nil {
		m.issueCertStarted <- struct{}{}
	}
	if m.continueIssueCert != nil {
		<-m.continueIssueCert
	}
	if params.RouteToApp.Name == "" {
		return nil, trace.BadParameter("missing app name")
	}
	if params.RouteToCluster != m.GetSiteName() {
		return nil, trace.BadParameter("wrong cluster")
	}
	if params.RequesterName != proto.UserCertsRequest_TSH_APP_LOCAL_PROXY {
		return nil, trace.BadParameter("expected in-memory requester name, got %v", params.RequesterName)
	}
	subject, err := m.identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := m.tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     m.clock,
		PublicKey: tlsKey.Public(),
		Subject:   subject,
		NotAfter:  m.clock.Now().Add(cmp.Or(params.TTL, time.Minute)),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KeyRing{
		AppTLSCredentials: map[string]TLSCredential{
			ScopedAppName(scopes.QualifiedName{Name: params.RouteToApp.Name, Scope: params.RouteToApp.Scope}): {
				PrivateKey: tlsKey,
				Cert:       tlsCert,
			},
		},
	}, nil
}

func TestMCPServerDialerSerializesCachedAppAndCertificate(t *testing.T) {
	t.Parallel()

	tlsCA, _, err := newSelfSignedCA(CAPriv, "localhost")
	require.NoError(t, err)

	issueCertStarted := make(chan struct{}, 2)
	continueIssueCert := make(chan struct{})
	mockClient := &mockMCPServerDialerClient{
		appServers:        types.AppServers{mustMakeAppServer(t, "http-mcp", "mcp+http://localhost:1234")},
		clock:             clockwork.NewFakeClock(),
		tlsCA:             tlsCA,
		identity:          tlsca.Identity{Username: "test"},
		issueCertStarted:  issueCertStarted,
		continueIssueCert: continueIssueCert,
	}
	dialer := NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: "http-mcp"})
	dialer.clock = mockClient.clock

	errC := make(chan error, 2)
	go func() {
		_, err := dialer.DialALPN(t.Context())
		errC <- err
	}()
	<-issueCertStarted

	if dialer.mu.TryLock() {
		dialer.mu.Unlock()
		t.Fatal("DialALPN did not lock the app and certificate cache")
	}

	go func() {
		_, err := dialer.DialALPN(t.Context())
		errC <- err
	}()
	close(continueIssueCert)

	require.NoError(t, <-errC)
	require.NoError(t, <-errC)
	require.Equal(t, int32(1), mockClient.issueCertCalls.Load())
}

func TestMCPServerDialerRejectsChangedURI(t *testing.T) {
	t.Parallel()

	tlsCA, _, err := newSelfSignedCA(CAPriv, "localhost")
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	mockClient := &mockMCPServerDialerClient{
		appServers: types.AppServers{mustMakeAppServer(t, "http-mcp", "mcp+http://old.example.com")},
		clock:      clock,
		tlsCA:      tlsCA,
		identity:   tlsca.Identity{Username: "test"},
	}
	dialer := NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: "http-mcp"})
	dialer.clock = clock

	app, err := dialer.GetApp(t.Context())
	require.NoError(t, err)
	require.Equal(t, "mcp+http://old.example.com", app.GetURI())

	// The app is replaced while the dialer is in use.
	mockClient.appServers = types.AppServers{mustMakeAppServer(t, "http-mcp", "mcp+http://new.example.com")}

	app, err = dialer.GetApp(t.Context())
	require.NoError(t, err)
	require.Equal(t, "mcp+http://old.example.com", app.GetURI(), "app should come from the cache before it expires")

	for range 2 {
		_, err = dialer.DialALPN(t.Context())
		require.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)
	}
	require.Empty(t, mockClient.routeToApp.URI, "no certificate should be issued for a changed app")

	clock.Advance(mcpAppCacheTTL)
	_, err = dialer.GetApp(t.Context())
	require.True(t, trace.IsCompareFailed(err), "expected CompareFailed, got %v", err)

	dialer = NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: "http-mcp"})
	dialer.clock = clock
	_, err = dialer.DialALPN(t.Context())
	require.NoError(t, err)
	require.Equal(t, "mcp+http://new.example.com", mockClient.routeToApp.URI)
}

func (m *mockMCPServerDialerClient) ProfileStatus() (*ProfileStatus, error) {
	return &ProfileStatus{}, nil
}

func (m *mockMCPServerDialerClient) GetSiteName() string {
	return "teleport.example.com"
}

func TestMCPServerDialer(t *testing.T) {
	tlsCA, _, err := newSelfSignedCA(CAPriv, "localhost")
	require.NoError(t, err)

	mockClient := &mockMCPServerDialerClient{
		appServers: types.AppServers{
			mustMakeAppServer(t, "http-app", "http://localhost:1234"),
			mustMakeAppServer(t, "http-mcp", "mcp+http://localhost:1234"),
			mustMakeAppServer(t, "sse-mcp", "mcp+sse+http://localhost:1234"),
			mustMakeAppServer(t, "scoped-mcp", "mcp+http://prod.example.com", "/prod"),
			mustMakeAppServer(t, "scoped-mcp", "mcp+http://dev.example.com", "/dev"),
		},
		clock: clockwork.NewFakeClock(),
		tlsCA: tlsCA,
		identity: tlsca.Identity{
			Username: "test",
		},
	}

	t.Run("GetApp", func(t *testing.T) {
		tests := []struct {
			name        string
			checkResult require.ErrorAssertionFunc
		}{
			{
				name:        "http-app",
				checkResult: require.Error,
			},
			{
				name:        "http-mcp",
				checkResult: require.NoError,
			},
			{
				name:        "sse-mcp",
				checkResult: require.NoError,
			},
			{
				name:        "not-found",
				checkResult: require.Error,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				dialer := NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: test.name})
				_, err := dialer.GetApp(t.Context())
				test.checkResult(t, err)
			})
		}
	})

	t.Run("GetScopedApp", func(t *testing.T) {
		dialer := NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: "scoped-mcp", Scope: "/prod"})
		app, err := dialer.GetApp(t.Context())
		require.NoError(t, err)
		require.Equal(t, "/prod", app.GetScope())
		require.Equal(t, "mcp+http://prod.example.com", app.GetURI())

		_, err = NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: "scoped-mcp"}).GetApp(t.Context())
		require.Error(t, err)
	})

	t.Run("DialALPN", func(t *testing.T) {
		tests := []struct {
			name     string
			scope    string
			wantALPN alpncommon.Protocol
		}{
			{
				name:     "http-mcp",
				wantALPN: alpncommon.ProtocolHTTP,
			},
			{
				name:     "sse-mcp",
				wantALPN: alpncommon.ProtocolMCP,
			},
			{
				name:     "scoped-mcp",
				scope:    "/prod",
				wantALPN: alpncommon.ProtocolHTTP,
			},
		}
		for _, test := range tests {
			t.Run(scopes.QualifiedName{Name: test.name, Scope: test.scope}.String(), func(t *testing.T) {
				dialer := NewMCPServerDialer(mockClient, scopes.QualifiedName{Name: test.name, Scope: test.scope})
				dialer.clock = mockClient.clock

				// Verify ALPN used.
				firstConn, err := dialer.DialALPN(t.Context())
				require.NoError(t, err)
				require.Equal(t, test.scope, mockClient.routeToApp.Scope)
				firstALPNConn, ok := firstConn.(*mockALPNConn)
				require.True(t, ok)
				require.Equal(t, test.wantALPN, firstALPNConn.protocol)

				// Advance time to trigger issue cert again.
				mockClient.clock.Advance(time.Hour)
				secondConn, err := dialer.DialALPN(t.Context())
				require.NoError(t, err)
				secondALPNConn, ok := secondConn.(*mockALPNConn)
				require.True(t, ok)

				// Double-check a new cert is issued.
				firstSerial := firstALPNConn.getCertSerialNumber()
				secondSerial := secondALPNConn.getCertSerialNumber()
				require.NotEmpty(t, firstSerial)
				require.NotEqual(t, firstSerial, secondSerial)
			})
		}
	})
}

func mustMakeAppServer(t *testing.T, name, uri string, scope ...string) types.AppServer {
	t.Helper()
	app, err := types.NewAppV3(
		types.Metadata{Name: name},
		types.AppSpecV3{URI: uri},
		scope...,
	)
	require.NoError(t, err)
	appServer, err := types.NewAppServerV3FromApp(app, "test", "test")
	require.NoError(t, err)
	return appServer
}
