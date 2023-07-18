/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol/fixtures"
)

// TestHandleConnectionAuditEvents checks audit events emitted during HandleConnection execution.
func TestHandleConnectionAuditEvents(t *testing.T) {
	type check func(t *testing.T, err error, ee []events.AuditEvent)
	hasNoErr := func() check {
		return func(t *testing.T, err error, ee []events.AuditEvent) {
			require.NoError(t, err)
		}
	}
	hasAuditEventCode := func(want string) check {
		return func(t *testing.T, err error, ee []events.AuditEvent) {
			for _, v := range ee {
				if v.GetCode() == want {
					return
				}
			}
			require.Failf(t, "event not found", "event code: %s", want)
		}
	}
	hasAuditEvent := func(i int, want events.AuditEvent) check {
		return func(t *testing.T, err error, ee []events.AuditEvent) {
			diff := cmp.Diff(want, ee[i])
			require.Empty(t, diff)
		}
	}

	tests := []struct {
		name   string
		packet []byte
		checks []check
	}{
		{
			name:   "rpc request procedure",
			packet: fixtures.RPCClientRequest,
			checks: []check{
				hasNoErr(),
				hasAuditEventCode(libevents.DatabaseSessionStartCode),
				hasAuditEventCode(libevents.DatabaseSessionEndCode),
				hasAuditEvent(1, &events.SQLServerRPCRequest{
					DatabaseMetadata: events.DatabaseMetadata{
						DatabaseUser: "sa",
						DatabaseType: "self-hosted",
					},
					Metadata: events.Metadata{
						Type: libevents.DatabaseSessionSQLServerRPCRequestEvent,
						Code: libevents.SQLServerRPCRequestCode,
					},
					Procname: "foo3",
				}),
			},
		},
		{
			name:   "rpc request param",
			packet: fixtures.RPCClientRequestParam,
			checks: []check{
				hasNoErr(),
				hasAuditEventCode(libevents.DatabaseSessionStartCode),
				hasAuditEventCode(libevents.DatabaseSessionEndCode),
				hasAuditEvent(1, &events.SQLServerRPCRequest{
					DatabaseMetadata: events.DatabaseMetadata{
						DatabaseUser: "sa",
						DatabaseType: "self-hosted",
					},
					Metadata: events.Metadata{
						Type: libevents.DatabaseSessionSQLServerRPCRequestEvent,
						Code: libevents.SQLServerRPCRequestCode,
					},
					Parameters: []string{"select @@version"},
					Procname:   "Sp_ExecuteSql",
				}),
			},
		},
		{
			name:   "sql batch",
			packet: fixtures.SQLBatch,
			checks: []check{
				hasNoErr(),
				hasAuditEventCode(libevents.DatabaseSessionStartCode),
				hasAuditEventCode(libevents.DatabaseSessionEndCode),
				hasAuditEvent(1, &events.DatabaseSessionQuery{
					DatabaseMetadata: events.DatabaseMetadata{
						DatabaseUser: "sa",
						DatabaseType: "self-hosted",
					},
					Metadata: events.Metadata{
						Type: libevents.DatabaseSessionQueryEvent,
						Code: libevents.DatabaseSessionQueryCode,
					},
					DatabaseQuery: "\nselect 'foo' as 'bar'\n        ",
					Status: events.Status{
						Success: true,
					},
				}),
			},
		},
		{
			name:   "malformed packet",
			packet: fixtures.MalformedPacketTest,
			checks: []check{
				hasNoErr(),
				hasAuditEventCode(libevents.DatabaseSessionStartCode),
				hasAuditEventCode(libevents.DatabaseSessionEndCode),
				hasAuditEventCode(libevents.DatabaseSessionMalformedPacketCode),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			_, err := b.Write(fixtures.Login7)
			require.NoError(t, err)

			_, err = b.Write(tc.packet)
			require.NoError(t, err)
			emitterMock := &mockEmitter{}
			audit, err := common.NewAudit(common.AuditConfig{Emitter: emitterMock})
			require.NoError(t, err)

			e := Engine{
				EngineConfig: common.EngineConfig{
					Audit:   audit,
					Log:     logrus.New(),
					Auth:    &mockDBAuth{},
					Context: context.Background(),
				},
				Connector: &mockConnector{
					conn: &mockConn{
						buff: bytes.Buffer{},
					},
				},
				clientConn: &mockConn{
					buff: b,
				},
			}

			err = e.HandleConnection(context.Background(), &common.Session{
				Checker:  &mockChecker{},
				Database: &types.DatabaseV3{},
			})
			for _, ch := range tc.checks {
				ch(t, err, emitterMock.emittedEvents)
			}
		})
	}
}

type mockConn struct {
	net.Conn
	buff bytes.Buffer
}

func (o *mockConn) Read(p []byte) (n int, err error) {
	return o.buff.Read(p)
}
func (o *mockConn) Write(p []byte) (n int, err error) {
	return len(p), nil
}
func (o *mockConn) Close() error {
	return nil
}

type mockEmitter struct {
	Emitter       events.Emitter
	emittedEvents []events.AuditEvent
	mtx           sync.Mutex
}

func (m *mockEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.emittedEvents = append(m.emittedEvents, event)
	return nil
}

type mockDBAuth struct {
	common.Auth
	// GetAzureIdentityResourceID mocks.
	azureIdentityResourceID    string
	azureIdentityResourceIDErr error
}

func (m *mockDBAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		RequireMFAType: types.RequireMFAType_SESSION,
	})
}

func (m *mockDBAuth) GetTLSConfig(_ context.Context, _ *common.Session) (*tls.Config, error) {
	return &tls.Config{}, nil
}

func (m *mockDBAuth) GetAzureIdentityResourceID(_ context.Context, _ string) (string, error) {
	return m.azureIdentityResourceID, m.azureIdentityResourceIDErr
}

type mockChecker struct {
	services.AccessChecker
}

func (m *mockChecker) CheckAccess(r services.AccessCheckable, state services.AccessState, matchers ...services.RoleMatcher) error {
	return nil
}

func (m *mockChecker) GetAccessState(authPref types.AuthPreference) services.AccessState {
	if authPref.GetRequireMFAType().IsSessionMFARequired() {
		return services.AccessState{
			MFARequired: services.MFARequiredAlways,
		}
	}
	return services.AccessState{
		MFARequired: services.MFARequiredNever,
	}
}

type mockConnector struct {
	conn io.ReadWriteCloser
}

func (m *mockConnector) Connect(context.Context, *common.Session, *protocol.Login7Packet) (io.ReadWriteCloser, []mssql.Token, error) {
	return m.conn, []mssql.Token{}, nil
}
