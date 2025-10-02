package oracle

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/db/oracle/connection"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/e/lib/db/oracle/testdata"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestOracleEngine(t *testing.T) {
	keyPEM, certPEM, err := utils.GenerateRSASelfSignedSigningCert(pkix.Name{
		Organization: []string{"Teleport Test"},
		CommonName:   "Teleport",
	}, []string{"localhost", "127.0.0.1"}, 10*365*24*time.Hour)
	require.NoError(t, err)

	certificate, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	mkServerAndSession := func() (*mockOracleServer, *common.Session) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		t.Cleanup(func() { listener.Close() })

		server := &mockOracleServer{
			listener: listener,
			tlsConfig: &tls.Config{
				Certificates: []tls.Certificate{certificate},
			},
		}

		session := &common.Session{
			DatabaseName: "oracle",
			DatabaseUser: "alice",
			Checker: &checkerMock{
				t: t,
				role: types.RoleV6{
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							DatabaseLabels: types.Labels{"*": []string{"*"}},
							DatabaseNames:  []string{"oracle", "DB1"},
							DatabaseUsers:  []string{"alice"},
						},
					},
				},
			},
			Database: &types.DatabaseV3{
				Spec: types.DatabaseSpecV3{
					URI:      listener.Addr().String(),
					Protocol: defaults.ProtocolOracle,
				},
			},
			Identity: tlsca.Identity{
				RouteToDatabase: tlsca.RouteToDatabase{
					Username: "alice",
					Database: "XE",
				},
			},
		}
		return server, session
	}

	mkEngine := func() *Engine {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		t.Cleanup(cancelFunc)
		return &Engine{
			EngineConfig: common.EngineConfig{
				Context: ctx,
				Log:     slog.Default(),
				Auth:    &authMock{},
				Audit:   &auditMock{},
			},
		}
	}

	t.Run("connection connect package", func(t *testing.T) {
		t.Parallel()

		client, engineConn := net.Pipe()
		defer client.Close()
		defer engineConn.Close()

		var connectPacket *protocol.ConnectPacket
		engine := mkEngine()
		engine.onConnectPacketRead = func(p *protocol.ConnectPacket) {
			connectPacket = p
		}

		server, session := mkServerAndSession()

		err := engine.InitializeConnection(engineConn, session)
		require.NoError(t, err)

		connectBytes, err := protocol.DecodeHexDump(testdata.ConnectPacketDump)
		require.NoError(t, err)

		connectBytesTransformed, err := protocol.DecodeHexDump(testdata.ConnectPacketDumpTransformed)
		require.NoError(t, err)

		go func() {
			_, wErr := client.Write(connectBytes)
			assert.NoError(t, wErr)
		}()

		go func() {
			engineErr := engine.HandleConnection(context.Background(), session)
			if !utils.IsOKNetworkError(engineErr) {
				assert.NoError(t, engineErr)
			}
		}()

		connChannels, err := server.accept()
		require.NoError(t, err)

		select {
		case <-time.After(time.Second * 10):
			t.Fatal("packet receive timout")
		case got := <-connChannels.receiveC:
			// engine will lower protocol version to 317 and disable OOB.
			require.Equal(t, connectBytesTransformed, got.Payload())
			require.NotNil(t, connectPacket)
			connString, err := connectPacket.GetConnectionString()
			require.NoError(t, err)
			expectedConnString := "(DESCRIPTION=(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))(CONNECT_DATA=(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))(SERVICE_NAME=XE)(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)))"
			require.Equal(t, expectedConnString, connString)
			serviceName, err := connectPacket.GetServiceName()
			require.NoError(t, err)
			require.Equal(t, "XE", serviceName)
			require.Equal(t, "XE", engine.serviceName)
		}
	})

	t.Run("access denied database username", func(t *testing.T) {
		t.Parallel()

		client, engineConn := net.Pipe()
		defer client.Close()
		defer engineConn.Close()
		_, session := mkServerAndSession()
		session.Identity.RouteToDatabase.Database = "XE"
		session.DatabaseName = "oracle"
		session.DatabaseUser = "bob"

		engine := mkEngine()
		err := engine.InitializeConnection(engineConn, session)
		require.NoError(t, err)

		err = engine.HandleConnection(context.Background(), session)
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
	})
}

type authMock struct {
	common.Auth
}

func (a *authMock) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return &types.AuthPreferenceV2{}, nil
}

func (a *authMock) GetTLSConfig(ctx context.Context, certExpiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	return &tls.Config{InsecureSkipVerify: true}, nil
}

type auditMock struct {
	common.Audit
}

func (a *auditMock) OnSessionStart(ctx context.Context, session *common.Session, sessionErr error) {
}

func (a *auditMock) OnSessionEnd(ctx context.Context, session *common.Session) {
}

type checkerMock struct {
	role types.RoleV6
	services.AccessChecker
	t *testing.T
}

func (c checkerMock) GetAccessState(authPref readonly.AuthPreference) services.AccessState {
	c.t.Helper()
	return services.AccessState{}
}

func (c checkerMock) CheckAccess(r services.AccessCheckable, state services.AccessState, matchers ...services.RoleMatcher) error {
	c.t.Helper()
	// only db-user check is enforced for Oracle.
	require.Len(c.t, matchers, 1)
	for _, m := range matchers {
		ok, err := m.Match(&c.role, types.Allow)
		require.NoError(c.t, err)
		if !ok {
			return trace.AccessDenied("access denied")
		}
	}
	return nil
}

type mockOracleServer struct {
	listener  net.Listener
	tlsConfig *tls.Config
}

type connectionChannels struct {
	receiveC chan protocol.Packet
	sendC    chan protocol.Packet
	closeC   chan struct{}

	returnErrC chan error
}

func (m *mockOracleServer) accept() (*connectionChannels, error) {
	conn, err := m.listener.Accept()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ch := &connectionChannels{
		closeC:   make(chan struct{}),
		receiveC: make(chan protocol.Packet, 100),
		sendC:    make(chan protocol.Packet, 100),

		returnErrC: make(chan error),
	}

	go func() {
		if err := m.handleConn(ch, conn); err != nil {
			slog.DebugContext(context.Background(), "Failed to handle client connection", "err", trace.DebugReport(err))
		}
		ch.returnErrC <- err
	}()

	return ch, nil
}

func (m *mockOracleServer) handleConn(ch *connectionChannels, conn net.Conn) error {
	defer conn.Close()
	clientConn := tls.Server(conn, m.tlsConfig)

	oracleClientConn, err := connection.NewConn(clientConn)
	if err != nil {
		return trace.Wrap(err)
	}
	defer oracleClientConn.Close()

	errC := make(chan error, 2)

	go func() {
		for {
			packet, err := oracleClientConn.ReadPacket()
			if err != nil {
				errC <- err
				return
			}
			select {
			case ch.receiveC <- packet:
			case <-ch.closeC:
				errC <- nil
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ch.closeC:
				errC <- oracleClientConn.Close()
				return
			case packet := <-ch.sendC:
				if err := oracleClientConn.WritePacket(packet); err != nil {
					errC <- err
					return
				}
			}
		}
	}()

	var errs []error
	for range 2 {
		errs = append(errs, <-errC)
	}
	return trace.NewAggregate(errs...)
}
