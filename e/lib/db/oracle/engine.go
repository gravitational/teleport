package oracle

import (
	"context"
	"net"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/db/oracle/audit"
	"github.com/gravitational/teleport/e/lib/db/oracle/connection"
	"github.com/gravitational/teleport/e/lib/db/oracle/protocol"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine create new Oracle engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements the Oracle database service that accepts client
// connections coming over reverse tunnel from the proxy and proxies
// them between the proxy and the Oracle database instance.
//
// Implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig

	// clientConn is an upstream client connection.
	clientConn net.Conn
	// session is a client session
	session *common.Session

	// startAuditPuller should be called if it is non-nil and serviceName and sessionID are known.
	startAuditPuller func(serviceName string, sessionID string) error

	// onConnectPacketRead is a callback used in tests.
	onConnectPacketRead func(connect *protocol.ConnectPacket)
}

// InitializeConnection initializes the engine with client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sess *common.Session) error {
	e.clientConn = clientConn
	e.session = sess
	return nil
}

// SendError sends an error to connected client in the Oracle understandable format.
func (e *Engine) SendError(err error) {
	// TODO: Investigate way to propagate Oracle error message.
	if err != nil {
		if !utils.IsOKNetworkError(err) {
			e.Log.ErrorContext(e.Context, "Oracle connection error", "error", err)
		}
	}
}

// HandleConnection processes the connection from Oracle proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	defer e.clientConn.Close()

	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	ctx, cancelCause := context.WithCancelCause(ctx)
	defer cancelCause(nil)

	if cfg := sessionCtx.Database.GetOracle(); cfg.IsAuditLogEnabled() {
		auditPuller, err := e.createAuditPuller(ctx, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
		defer auditPuller.Close()

		e.startAuditPuller = func(serviceName string, sessionID string) error {
			if err := auditPuller.Init(serviceName, sessionID); err != nil {
				return trace.NewAggregate(err)
			}
			go func() {
				if err := auditPuller.Run(ctx); err != nil {
					e.Log.ErrorContext(e.Context, "Closing connections due to active audit log fetcher error.", "error", err)
					cancelCause(trace.Wrap(err, "audit log fetcher failed"))
				}
			}()
			return nil
		}
	}

	err = e.handleClientServerConn(ctx, sessionCtx)
	if err != nil && !utils.IsOKNetworkError(err) {
		e.Log.ErrorContext(e.Context, "Error handling connection.", "error", err)
	}
	return trace.Wrap(err)
}

func (e *Engine) createAuditPuller(ctx context.Context, cfg types.OracleOptions) (*audit.Puller, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.session.GetExpiry(), e.session.Database, cfg.AuditUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	af, err := audit.NewPuller(audit.PullerConfig{
		Addr:      e.session.Database.GetURI(),
		TLSConfig: tlsConfig,
		OnQuery: func(entry audit.QueryEntry) {
			e.Audit.OnQuery(e.Context, e.session, common.Query{
				Parameters: []string{entry.Bind},
				Query:      entry.Text,
			})
		},
	})
	return af, trace.Wrap(err)
}

func (e *Engine) readConnect(sessionCtx *common.Session, clientConn *connection.OracleConn) (*protocol.ConnectPacket, error) {
	pkt, err := clientConn.ReadPacket()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connect, ok := pkt.(*protocol.ConnectPacket)
	if !ok {
		return nil, trace.BadParameter("expected connect packet, got %T", pkt)
	}

	// connect is a bit special: its size is limited by the protocol, but the connection string it contains is often above that limit.
	// if that happens, the client will follow it up with a DATA packet containing the connection string in question.
	err = connect.MaybeReadMoreData(clientConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connString, err := connect.GetConnectionString()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceName, err := connect.GetServiceName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.Log.InfoContext(e.Context, "Received connection string", "conn_string", connString, "service_name", serviceName)

	if sessionCtx.Identity.RouteToDatabase.Database != serviceName {
		return nil, trace.BadParameter("mismatch between TLS identity database name %q and Oracle Connect Packet ServerName %q", sessionCtx.Identity.RouteToDatabase.Database, serviceName)
	}

	if e.onConnectPacketRead != nil {
		e.onConnectPacketRead(connect)
	}

	return connect, nil
}

// readPacket reads packet from connection and casts it to a specific packet type.
func readPacket[T protocol.Packet](conn *connection.OracleConn) (T, error) {
	var defaultValue T
	pkt, err := conn.ReadPacket()
	if err != nil {
		return defaultValue, trace.Wrap(err)
	}
	pktTyped, ok := pkt.(T)
	if !ok {
		return defaultValue, trace.BadParameter("unexpected packet type %T", pkt)
	}
	return pktTyped, nil
}

func (e *Engine) handleClientServerConn(ctx context.Context, sessionCtx *common.Session) error {
	serverTcpConn, err := net.Dial("tcp", sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverTcpConn.Close()

	// Note that we don't have to close clientConn or serverConn:
	// - serverTcpConn.Close() already deals with the server connection,
	// - e.clientConn.Close() deals with the client one.
	clientConn, serverConn, err := e.openServerConnection(ctx, sessionCtx, serverTcpConn)
	if err != nil {
		return trace.Wrap(err)
	}

	err = e.negotiateSecureNetworkServices(clientConn, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	err = e.forwardLoop(ctx, clientConn, serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// openServerConnection handles the first phase of the connection.
// The client declares the database it wishes to connect to and server accepts or refuses.
// Server may also request TLS renegotiation.
// Protocol version is negotiated, which impacts the binary message layout.
func (e *Engine) openServerConnection(ctx context.Context, sessionCtx *common.Session, serverTcpConn net.Conn) (*connection.OracleConn, *connection.OracleConn, error) {
	clientConn, err := connection.NewConn(e.clientConn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// TODO: Consider replacing client-made connect packet with a custom one, properly sanitized.
	//       However, that would require better understanding of the various flags involved.
	connectPacket, err := e.readConnect(sessionCtx, clientConn)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	requestedProtocolVersion, err := connectPacket.GetProtocolVersion()
	if err == nil {
		e.Log.DebugContext(e.Context, "Protocol version", "requested_protocol_version", requestedProtocolVersion)
	} else {
		e.Log.DebugContext(e.Context, "Failed to get protocol version", "error", err)
	}

	// CONNECT -> ACCEPT loop; may need to restart TLS and retry.
	// Typical happy flow:
	// - send CONNECT
	// - receive RESEND
	// - send CONNECT
	// - receive ACCEPT
	// We allow for more RESEND packets because handling that isn't hard and the protocol technically allows for that.
	const maxResendAttempts = 3
	for attempt := 1; ; attempt++ {
		if attempt > maxResendAttempts {
			return nil, nil, trace.BadParameter("exceeded max resend attempts")
		}

		e.Log.InfoContext(e.Context, "Sending connect packet", "attempt", attempt)

		serverConn, err := connection.NewConn(serverTcpConn, connection.WithTLS(tlsConfig))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// pass the CONNECT (and optional DATA packet) down to the server.
		err = serverConn.WritePacket(connectPacket)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if connectPacket.DataPacket != nil {
			err = serverConn.WritePacket(connectPacket.DataPacket)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
		}

		// read the response from server. expecting either RESEND or ACCEPT.
		pkt, err := serverConn.ReadPacket()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		switch pktT := pkt.(type) {
		case *protocol.AcceptPacket:
			accept := pktT
			e.Log.InfoContext(e.Context, "Received accept packet", "protocol_version", accept.ProtocolVersion)

			// update negotiated protocol version.
			clientConn.SetProtocolVersion(accept.ProtocolVersion)
			serverConn.SetProtocolVersion(accept.ProtocolVersion)

			// forward the accept packet to the client.
			err = clientConn.WritePacket(accept)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}

			e.Log.InfoContext(e.Context, "Protocol handshake complete.")
			return clientConn, serverConn, nil
		case *protocol.RefusePacket:
			e.Log.WarnContext(e.Context, "Received refuse packet.", "message", pktT.Message)
			return nil, nil, trace.AccessDenied("server refused connection: %s", pktT.Message)
		case *protocol.ResendPacket:
			e.Log.DebugContext(e.Context, "RESEND received, trying again.", "packet", pktT)
			continue
		}

		return nil, nil, trace.BadParameter("received unexpected packet type: %T", pkt)
	}
}

// negotiateSecureNetworkServices performs the SNS negotiation in a way that ensures that the server will use TCPS protocol for authentication,
// which boils down to accepting user certificate we have passed when opening the TLS connection.
// SNS packet can also be used to configure native protocol encryption and checksums, neither of which we support and so we ensure they are not requested from the server.
// Finally, we respond to the client telling it none of the services are needed, as this is the most compatible option.
func (e *Engine) negotiateSecureNetworkServices(clientConn *connection.OracleConn, serverConn *connection.OracleConn) error {
	e.Log.DebugContext(e.Context, "SNS negotiation starting.")

	snsPacket, err := readPacket[*protocol.DataPacket](clientConn)
	if err != nil {
		return trace.Wrap(err)
	}

	if !snsPacket.HasSecureNetworkServices() {
		return trace.BadParameter("expected SNS packet, got different one instead.")
	}

	// TODO: parse the SNS data packet and verify the fields.
	e.Log.DebugContext(e.Context, "SNS packet received.")

	requestSNS := requestSNSTCPS
	if !clientConn.LargeSDU() {
		requestSNS = requestSNSTCPS314
	}
	err = serverConn.WritePacket(requestSNS)
	if err != nil {
		return trace.Wrap(err)
	}

	serverSNSResponse, err := readPacket[*protocol.DataPacket](serverConn)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: actually parse server response in full, instead of merely checking the flag.
	if !serverSNSResponse.HasSecureNetworkServices() {
		e.Log.WarnContext(e.Context, "Unexpected response to SNS packet. Upcoming protocol failure likely.")
	}

	// respond to the original request from the client
	// responseSNSPassword is a response that enables no services.
	responseSNS := responseSNSPassword
	if !clientConn.LargeSDU() {
		responseSNS = responseSNSPassword314
	}
	err = clientConn.WritePacket(responseSNS)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (e *Engine) tryStartAuditPuller(dataPacket *protocol.DataPacket, dataPacketQuota int) error {
	result, err := dataPacket.AuthParameters()
	if err != nil {
		return trace.Wrap(err)
	}

	params := result.ToDictionary()

	serviceName := params[protocol.AuthSCServiceNameKey]
	sessionID := params[protocol.AuthSessionIDKey]

	// advise to increase SDU
	if result.IsPartial {
		if sessionID == "" || serviceName == "" {
			e.Log.WarnContext(e.Context, "Unable to find session ID in partial auth parameters received from server. Consider increasing SDU in client configuration.")
		}
	}

	if sessionID == "" {
		return trace.BadParameter("session ID parameter is missing or empty")
	}

	if serviceName == "" {
		return trace.BadParameter("service name parameter is missing or empty")
	}

	if !strings.EqualFold(serviceName, e.session.Identity.RouteToDatabase.Database) {
		return trace.BadParameter("service name mismatch (expected=%v, got=%v)", e.session.Identity.RouteToDatabase.Database, serviceName)
	}

	e.Log.DebugContext(e.Context, "Starting audit puller", "service_name", serviceName, "session_id", sessionID, "quota", dataPacketQuota)

	err = e.startAuditPuller(serviceName, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// forwardLoop is a general proxying method, where majority of packets are processed.
// Mostly we don't care about their contents and simply pass everything as is,
// except for the initial handful of server packets which we check for session ID, required to start audit puller.
func (e *Engine) forwardLoop(ctx context.Context, clientConn, serverConn *connection.OracleConn) error {
	e.Log.DebugContext(e.Context, "Starting async proxying.")

	// connections are fully initialized. from now on, just forward all traffic between db client and server.
	errC := make(chan error, 2)
	go func() {
		log := e.Log.With("thread", "client -> server")

		for {
			p, err := clientConn.ReadPacket()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					log.DebugContext(e.Context, "error reading packet", "error", err)
				}
				errC <- trace.Wrap(err)
				return
			}

			err = serverConn.WritePacket(p)
			if err != nil {
				errC <- trace.Wrap(err)
				return
			}
		}
	}()

	// read data from server, pass it to the client.
	go func() {
		log := e.Log.With("thread", "server -> client")

		// expect to find session id in within handful of packets, abort the session if that doesn't happen.
		// based on the tests with real clients the quota could be as low as 5, but it is better to leave some breathing room.
		dataPacketQuota := 20

		needAuditPuller := e.startAuditPuller != nil

		for {
			p, err := serverConn.ReadPacket()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					log.DebugContext(e.Context, "error reading packet", "error", err)
				}
				errC <- trace.Wrap(err)
				return
			}

			if needAuditPuller {
				dataPacketQuota--
				if dataPacketQuota < 0 {
					err = trace.BadParameter("Unable to find session id in the initial packet exchange")
					e.Log.ErrorContext(e.Context, "Failed to locate valid session parameters.", "error", err)
					errC <- trace.Wrap(err)
				}

				dataPacket, ok := p.(*protocol.DataPacket)
				if ok && dataPacket.HasAuthParameters() {
					err = e.tryStartAuditPuller(dataPacket, dataPacketQuota)
					if err != nil {
						e.Log.ErrorContext(e.Context, "Failed to locate valid session parameters.", "error", err)
						errC <- trace.Wrap(err)
						return
					}
					needAuditPuller = false
				}
			}

			err = clientConn.WritePacket(p)
			if err != nil {
				errC <- trace.Wrap(err)
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		e.Log.DebugContext(e.Context, "Context done, stopping forwarding.")
		return nil
	case err := <-errC:
		if err == nil {
			e.Log.DebugContext(e.Context, "Exiting without error.")
		} else {
			if utils.IsOKNetworkError(err) {
				e.Log.DebugContext(e.Context, "Closed connection.")
			} else {
				e.Log.WarnContext(e.Context, "Closing connection due to an error.", "error", err)
			}
		}
		return trace.Wrap(err)
	}
}

func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     sessionCtx.Database,
		DatabaseUser: sessionCtx.DatabaseUser,
		DatabaseName: sessionCtx.DatabaseName,
	})
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}
