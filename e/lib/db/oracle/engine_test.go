package oracle

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol/testdata"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestOracleEngine(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()

	cert, _ := protocol.MustCreateSelfSignedCert(t)
	server := mockOracleServer{
		listener: listener,
		tlsConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		closeC:   make(chan struct{}),
		receiveC: make(chan protocol.Packet, 100),
		sendC:    make(chan protocol.Packet, 100),
	}
	defer server.close()
	go server.start()

	session := &common.Session{
		DatabaseName: "XE",
		DatabaseUser: "alice",
		Checker: &checkerMock{
			t: t,
			role: types.RoleV6{
				Spec: types.RoleSpecV6{
					Allow: types.RoleConditions{
						DatabaseLabels: types.Labels{"*": []string{"*"}},
						DatabaseNames:  []string{"XE", "DB1"},
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
	engine := Engine{
		EngineConfig: common.EngineConfig{
			Context: context.Background(),
			Log:     logrus.New(),
			Auth:    &authMock{},
			Audit:   &auditMock{},
		},
	}

	t.Run("connection connect package", func(t *testing.T) {
		client, engineConn := net.Pipe()
		defer client.Close()
		defer engineConn.Close()
		err := engine.InitializeConnection(engineConn, session)
		require.NoError(t, err)

		connectBytes := protocol.MustDecodePacketDump(t, testdata.ConnectPacketDump)
		go func() {
			_, wErr := client.Write(connectBytes)
			assert.NoError(t, wErr)
		}()
		go engine.HandleConnection(context.Background(), session)

		select {
		case <-time.After(time.Second):
			t.Fatal("packet receive timout")
		case got := <-server.receiveC:
			require.Equal(t, connectBytes, got.Payload())
			require.True(t, engine.serverNameReceived)
		}
	})

	t.Run("database name connect server name mismatch", func(t *testing.T) {
		client, engineConn := net.Pipe()
		defer client.Close()
		defer engineConn.Close()
		// The database encoded in user identity should match the ConnectPacketDump ServerName content.
		session.Identity.RouteToDatabase.Database = "DB1"
		err := engine.InitializeConnection(engineConn, session)
		require.NoError(t, err)

		connectBytes := protocol.MustDecodePacketDump(t, testdata.ConnectPacketDump)
		go func() {
			_, wErr := client.Write(connectBytes)
			assert.NoError(t, wErr)
		}()
		err = engine.HandleConnection(context.Background(), session)
		require.Error(t, err)
		require.Contains(t, err.Error(), "match between TLS identity database name and Oracle Connect Packet ServerName")
	})

	t.Run("access denied database username", func(t *testing.T) {
		client, engineConn := net.Pipe()
		defer client.Close()
		defer engineConn.Close()
		session.Identity.RouteToDatabase.Database = "XE"
		session.DatabaseName = "XE"
		session.DatabaseUser = "bob"
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

func (a *authMock) GetTLSConfig(ctx context.Context, sessionCtx *common.Session) (*tls.Config, error) {
	return &tls.Config{
		InsecureSkipVerify: true,
	}, nil
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

func (c checkerMock) GetAccessState(authPref types.AuthPreference) services.AccessState {
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
	receiveC  chan protocol.Packet
	sendC     chan protocol.Packet
	closeC    chan struct{}
}

func (m *mockOracleServer) start() error {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return nil
		}
		go func() {
			if err := m.handleConn(conn); err != nil {
				logrus.Warnf("Failed to handle client connection: %v", err)
			}
		}()
	}
}

func (m *mockOracleServer) close() error {
	close(m.closeC)
	return trace.Wrap(m.listener.Close())
}

func (m *mockOracleServer) handleConn(conn net.Conn) error {
	defer conn.Close()
	clientConn := tls.Server(conn, m.tlsConfig)

	oracleClientConn := protocol.NewClientConn(clientConn)
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
			case m.receiveC <- packet:
			case <-m.closeC:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-m.closeC:
				if err := oracleClientConn.Close(); err != nil {
					errC <- err
				}
				return
			case packet := <-m.sendC:
				if err := oracleClientConn.WritePacket(packet); err != nil {
					errC <- err
					return
				}
			}
		}
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case <-m.closeC:
			return nil
		case err := <-errC:
			if err != nil && !utils.IsOKNetworkError(errors.Unwrap(err)) && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
}
