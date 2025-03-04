package oracle

import (
	"context"
	"net"
	"strings"

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

	err = e.dialServerAndForward(ctx, sessionCtx)
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

func (e *Engine) checkExpectedServiceName(serviceName string) error {
	// We allow empty database name, which is used by Teleport Connect when establishing the tunnel.
	// In Connect you can change the database name dynamically, so we don't want to tie that to the particular certificate, so it ends up as empty.
	if e.session.Identity.RouteToDatabase.Database == "" {
		return nil
	}

	// If the database name is non-empty, we expect it to match in case-insensitive way.
	if strings.EqualFold(serviceName, e.session.Identity.RouteToDatabase.Database) {
		return nil
	}

	return trace.BadParameter("service name mismatch (expected=%v, got=%v)", e.session.Identity.RouteToDatabase.Database, serviceName)
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

	err = e.checkExpectedServiceName(serviceName)
	if err != nil {
		return trace.Wrap(err)
	}

	e.Log.DebugContext(e.Context, "Starting audit puller", "service_name", serviceName, "session_id", sessionID, "quota", dataPacketQuota)

	err = e.startAuditPuller(serviceName, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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
