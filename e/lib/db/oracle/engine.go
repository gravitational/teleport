package oracle

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/db/oracle/audit"
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
	// proxyConn is a client connection.
	conn                net.Conn
	clientConn          *protocol.Conn
	serverNameReceived  bool
	serverParamReceived bool
	auditPuller         *audit.Puller
	session             *common.Session
}

// InitializeConnection initializes the engine with client connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sess *common.Session) error {
	e.conn = clientConn
	e.session = sess
	return nil
}

// SendError sends an error to connected client in the Oracle understandable format.
func (e *Engine) SendError(err error) {
	// TODO: Investigate way to propagate Oracle error message.
	if err != nil && !utils.IsOKNetworkError(err) {
		e.Log.ErrorContext(e.Context, "Oracle connection error", "error", err)
	}
}

// HandleConnection processes the connection from Oracle proxy coming
// over reverse tunnel.
//
// It handles all necessary startup actions, authorization and acts as a
// middleman between the proxy and the database intercepting and interpreting
// all messages i.e. doing protocol parsing.
func (e *Engine) HandleConnection(ctx context.Context, sessionCtx *common.Session) error {
	defer e.conn.Close()

	clientConn := protocol.NewClientConn(e.conn)
	e.clientConn = clientConn

	err := e.checkAccess(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverConn, err := e.connectToOracleDB(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serverConn.Close()

	e.Audit.OnSessionStart(e.Context, sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, sessionCtx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if cfg := sessionCtx.Database.GetOracle(); cfg.IsAuditLogEnabled() {
		auditPuller, err := e.createAuditPuller(ctx, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
		defer auditPuller.Close()
		e.auditPuller = auditPuller
	}

	if err := e.handleClientServerConn(ctx, sessionCtx, clientConn, serverConn); err != nil {
		return trace.Wrap(err)
	}

	return nil
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

func (e *Engine) connectToOracleDB(ctx context.Context, sessionCtx *common.Session) (*protocol.Conn, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverConn, err := protocol.NewServerConn(sessionCtx.Database.GetURI(), tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return serverConn, nil
}

func (e *Engine) handleClientConn(sessCtx *common.Session, clientConn, serverConn *protocol.Conn) error {
	defer clientConn.Close()
	defer serverConn.Close()
	for {
		p, err := clientConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		switch t := p.(type) {
		case *protocol.ConnectPacket:
			if t.ServerName != "" {
				e.serverNameReceived = true
			}
			if sessCtx.Identity.RouteToDatabase.Database != t.ServerName {
				return trace.BadParameter("mismatch between TLS identity database name and Oracle Connect Packet ServerName")
			}
		}
		if err := serverConn.WritePacket(p); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) handleServerConn(session *common.Session, clientConn, serverConn *protocol.Conn) error {
	defer serverConn.Close()
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(e.Context)
	defer cancel()

	for {
		packet, err := serverConn.ReadPacket()
		if err != nil {
			return trace.Wrap(err)
		}
		if !serverConn.ConnAccepted() {
			switch t := packet.(type) {
			case *protocol.RefusePacket:
				// Debug connection errors before accepted phase in order to troubleshoot misconfiguration issues.
				e.Log.WarnContext(e.Context, "Received Refuse Packet from server.", "message", t.Message)
			case *protocol.AcceptPacket:
				if !e.serverNameReceived {
					return trace.BadParameter("server name package not received")
				}
			}
		} else {
			switch t := packet.(type) {
			case *protocol.DataPacket:
				if e.needToStartAuditPoller(t) {
					// Audit Puller starts after the ReturnOPIParameterDataID backed is received where the sessionID entryID client
					// identifiers are extracted from Oracle Server Parameters.
					e.serverParamReceived = true
					if err := e.startAuditPuller(ctx, t, clientConn, serverConn); err != nil {
						return trace.Wrap(err)
					}
				}
			}
		}
		if err = clientConn.WritePacket(packet); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) needToStartAuditPoller(t *protocol.DataPacket) bool {
	return t.DataType == protocol.ReturnOPIParameterDataID && !e.serverParamReceived && e.auditPuller != nil
}

func (e *Engine) startAuditPuller(ctx context.Context, data *protocol.DataPacket, clientConn, serverConn *protocol.Conn) error {
	sn := data.Parameters[protocol.AuthSCServiceNameKey]
	sid := data.Parameters[protocol.AuthSessionIDKey]

	if err := e.auditPuller.Init(sn, sid); err != nil {
		return trace.NewAggregate(err)
	}
	go func() {
		if err := e.auditPuller.Run(ctx); err != nil {
			e.Log.ErrorContext(e.Context, "Closing connections due to active audit log fetcher error.", "error", err)
			_ = serverConn.Close()
			_ = clientConn.Close()
		}
	}()
	return nil
}

func (e *Engine) handleClientServerConn(ctx context.Context, sessionCtx *common.Session, clientConn, serverConn *protocol.Conn) error {
	errC := make(chan error, 2)
	go func() {
		err := e.handleClientConn(sessionCtx, clientConn, serverConn)
		errC <- trace.Wrap(err, "client done")
	}()
	go func() {
		var err = e.handleServerConn(sessionCtx, clientConn, serverConn)
		errC <- trace.Wrap(err, "server done")
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case err := <-errC:
			if err != nil && !utils.IsOKNetworkError(errors.Unwrap(err)) && !errors.Is(err, io.EOF) {
				errs = append(errs, err)
			}
		}
	}
	return trace.NewAggregate(errs...)
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
