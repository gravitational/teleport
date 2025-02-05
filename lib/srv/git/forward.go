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

package git

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// ForwardServerConfig is the configuration for the ForwardServer.
type ForwardServerConfig struct {
	// ParentContext is a parent context, used to signal global
	// closure
	ParentContext context.Context
	// TargetServer is the target server that represents the git-hosting
	// service.
	TargetServer types.Server
	// TargetConn is the TCP connection to the remote host.
	TargetConn net.Conn
	// AuthClient is a client connected to the Auth server of this local cluster.
	AuthClient authclient.ClientI
	// AccessPoint is a caching client that provides access to this local cluster.
	AccessPoint srv.AccessPoint
	// Emitter is audit events emitter
	Emitter events.StreamEmitter
	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
	// KeyManager manages keys for git proxies.
	KeyManager *KeyManager
	// HostCertificate is the SSH host certificate this in-memory server presents
	// to the client.
	HostCertificate ssh.Signer
	// SrcAddr is the source address
	SrcAddr net.Addr
	// DstAddr is the destination address
	DstAddr net.Addr
	// HostUUID is the UUID of the underlying proxy that the forwarding server
	// is running in.
	HostUUID string

	// Ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string
	// KEXAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string
	// MACAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string
	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// Clock is an optoinal clock to override default real time clock
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (c *ForwardServerConfig) CheckAndSetDefaults() error {
	if c.TargetServer == nil {
		return trace.BadParameter("missing parameter TargetServer")
	}
	if c.TargetConn == nil {
		return trace.BadParameter("missing parameter TargetConn")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("missing parameter AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if c.KeyManager == nil {
		return trace.BadParameter("missing parameter KeyManager")
	}
	if c.HostCertificate == nil {
		return trace.BadParameter("missing parameter HostCertificate")
	}
	if c.ParentContext == nil {
		return trace.BadParameter("missing parameter ParentContext")
	}
	if c.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	if c.SrcAddr == nil {
		return trace.BadParameter("source address required to identify client")
	}
	if c.DstAddr == nil {
		return trace.BadParameter("destination address required to identify client")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// ForwardServer is an in-memory SSH server that forwards git commands to remote
// git-hosting services like "github.com".
type ForwardServer struct {
	events.StreamEmitter
	cfg    *ForwardServerConfig
	logger *slog.Logger
	auth   *srv.AuthHandlers
	reply  *sshutils.Reply
	id     string

	// serverConn is the server side of the pipe to the client connection.
	serverConn net.Conn
	// clientConn is the client side of the pipe to the client connection.
	clientConn net.Conn
	// remoteClient is the client connected to the git-hosting service.
	remoteClient *tracessh.Client

	// verifyRemoteHost is a callback to verify remote host like "github.com".
	// Can be overridden for tests. Defaults to cfg.KeyManager.HostKeyCallback.
	verifyRemoteHost ssh.HostKeyCallback
	// makeRemoteSigner generates the client certificate for connecting to the
	// remote server. Can be overridden for tests. Defaults to makeRemoteSigner.
	makeRemoteSigner func(context.Context, *ForwardServerConfig, srv.IdentityContext) (ssh.Signer, error)
}

// Dial returns the client connection of the pipe
func (s *ForwardServer) Dial() (net.Conn, error) {
	return s.clientConn, nil
}

// NewForwardServer creates a new in-memory SSH server that forwards git
// commands to remote git-hosting services like "github.com".
func NewForwardServer(cfg *ForwardServerConfig) (*ForwardServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	serverConn, clientConn, err := utils.DualPipeNetConn(cfg.SrcAddr, cfg.DstAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifyRemoteHost, err := cfg.KeyManager.HostKeyCallback(cfg.TargetServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger := slog.With(teleport.ComponentKey, teleport.ComponentForwardingGit,
		"src_addr", cfg.SrcAddr.String(),
		"dst_addr", cfg.DstAddr.String(),
	)

	s := &ForwardServer{
		StreamEmitter:    cfg.Emitter,
		cfg:              cfg,
		serverConn:       serverConn,
		clientConn:       clientConn,
		logger:           logger,
		reply:            sshutils.NewReply(logger),
		id:               uuid.NewString(),
		verifyRemoteHost: verifyRemoteHost,
		makeRemoteSigner: makeRemoteSigner,
	}
	// TODO(greedy52) extract common parts from srv.NewAuthHandlers like
	// CreateIdentityContext and UserKeyAuth to a common package.
	s.auth, err = srv.NewAuthHandlers(&srv.AuthHandlerConfig{
		Server:       s,
		Component:    teleport.ComponentForwardingGit,
		Emitter:      s.cfg.Emitter,
		AccessPoint:  cfg.AccessPoint,
		TargetServer: cfg.TargetServer,
		FIPS:         cfg.FIPS,
		Clock:        cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil

}

// Serve starts an SSH server that forwards git commands.
func (s *ForwardServer) Serve() {
	defer s.close()
	s.logger.DebugContext(s.cfg.ParentContext, "Starting forwarding git")
	defer s.logger.DebugContext(s.cfg.ParentContext, "Finished forwarding git")
	server, err := sshutils.NewServer(
		teleport.ComponentForwardingGit,
		utils.NetAddr{}, /* empty addr, this is one time use so no use for listener*/
		sshutils.NewChanHandlerFunc(s.onChannel),
		sshutils.StaticHostSigners(s.cfg.HostCertificate),
		sshutils.AuthMethods{
			PublicKey: s.userKeyAuth,
		},
		sshutils.SetFIPS(s.cfg.FIPS),
		sshutils.SetCiphers(s.cfg.Ciphers),
		sshutils.SetKEXAlgorithms(s.cfg.KEXAlgorithms),
		sshutils.SetMACAlgorithms(s.cfg.MACAlgorithms),
		sshutils.SetClock(s.cfg.Clock),
		sshutils.SetNewConnHandler(sshutils.NewConnHandlerFunc(s.onConnection)),
	)
	if err != nil {
		s.logger.ErrorContext(s.cfg.ParentContext, "Failed to create git forward server", "error", err)
		return
	}
	server.HandleConnection(s.serverConn)
}

func (s *ForwardServer) close() {
	if err := s.serverConn.Close(); err != nil && !utils.IsOKNetworkError(err) {
		s.logger.WarnContext(s.cfg.ParentContext, "Failed to close server conn", "error", err)
	}
	if err := s.clientConn.Close(); err != nil && !utils.IsOKNetworkError(err) {
		s.logger.WarnContext(s.cfg.ParentContext, "Failed to close client conn", "error", err)
	}
	if err := s.cfg.TargetConn.Close(); err != nil && !utils.IsOKNetworkError(err) {
		s.logger.WarnContext(s.cfg.ParentContext, "Failed to close target conn", "error", err)
	}
}

func (s *ForwardServer) userKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("unsupported key type")
	}

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ident.GitHubUserID == "" {
		return nil, trace.BadParameter("missing GitHub user ID")
	}

	// Verify incoming user is "git" and override it with any valid principle
	// to bypass principle validation.
	if conn.User() != gitUser {
		return nil, trace.BadParameter("only git is expected as user for git connections")
	}
	if len(ident.Principals) > 0 {
		conn = sshutils.NewSSHConnMetadataWithUser(conn, ident.Principals[0])
	}

	// Use auth.UserKeyAuth to verify user cert is signed by UserCA.
	permissions, err := s.auth.UserKeyAuth(conn, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check RBAC on the git server resource (aka s.cfg.TargetServer).
	if err := s.checkUserAccess(ident); err != nil {
		s.logger.ErrorContext(s.Context(), "Permission denied",
			"error", err,
			"local_addr", logutils.StringerAttr(conn.LocalAddr()),
			"remote_addr", logutils.StringerAttr(conn.RemoteAddr()),
			"key", key.Type(),
			"fingerprint", sshutils.Fingerprint(key),
			"user", cert.KeyId,
		)
		return nil, trace.Wrap(err)
	}
	return permissions, nil
}

func (s *ForwardServer) checkUserAccess(ident *sshca.Identity) error {
	clusterName, err := s.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	accessInfo := services.AccessInfoFromLocalSSHIdentity(ident)
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), s.cfg.AccessPoint)
	if err != nil {
		return trace.Wrap(err)
	}
	state, err := services.AccessStateFromSSHIdentity(s.Context(), ident, accessChecker, s.cfg.AccessPoint)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(accessChecker.CheckAccess(s.cfg.TargetServer, state))
}

func (s *ForwardServer) onConnection(ctx context.Context, ccx *sshutils.ConnectionContext) (context.Context, error) {
	s.logger.Log(ctx, logutils.TraceLevel, "Handling new connection")

	identityCtx, err := s.auth.CreateIdentityContext(ccx.ServerConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initRemoteConn(ctx, ccx, identityCtx); err != nil {
		s.logger.DebugContext(ctx, "onConnection failed", "error", err)
		return ctx, trace.Wrap(err)
	}

	// TODO(greedy52) decouple from srv.NewServerContext. We only need
	// connection monitoring.
	_, serverCtx, err := srv.NewServerContext(ctx, ccx, s, identityCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.logger.Log(ctx, logutils.TraceLevel, "New connection accepted")
	ccx.AddCloser(serverCtx)
	return context.WithValue(ctx, serverContextKey{}, serverCtx), nil
}

func (s *ForwardServer) onChannel(ctx context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	s.logger.DebugContext(ctx, "Handling channel request", "channel", nch.ChannelType())

	serverCtx, ok := ctx.Value(serverContextKey{}).(*srv.ServerContext)
	if !ok {
		// This should not happen. Double check just in case.
		s.reply.RejectChannel(ctx, nch, ssh.ResourceShortage, "server context not found")
		return
	}

	// Only expecting a session to execute a command.
	if nch.ChannelType() != teleport.ChanSession {
		s.reply.RejectUnknownChannel(ctx, nch)
		return
	}

	if s.remoteClient == nil {
		s.reply.RejectWithNewRemoteSessionError(ctx, nch, trace.NotFound("missing remote client"))
		return
	}
	remoteSession, err := s.remoteClient.NewSession(ctx)
	if err != nil {
		s.reply.RejectWithNewRemoteSessionError(ctx, nch, err)
		return
	}
	defer remoteSession.Close()

	ch, in, err := nch.Accept()
	if err != nil {
		s.reply.RejectWithAcceptError(ctx, nch, err)
		return
	}
	defer ch.Close()

	sctx := newSessionContext(serverCtx, ch, remoteSession)
	for {
		select {
		case req := <-in:
			if req == nil {
				s.logger.DebugContext(ctx, "Client disconnected", "remote_addr", ccx.ServerConn.RemoteAddr())
				return
			}

			ok, err := s.dispatch(ctx, sctx, req)
			if err != nil {
				s.reply.ReplyError(ctx, req, err)
				return
			}
			s.reply.ReplyRequest(ctx, req, ok, nil)

		case execErr := <-sctx.waitExec:
			code := sshutils.ExitCodeFromExecError(execErr)
			s.logger.DebugContext(ctx, "Exec request complete", "code", code)
			s.reply.SendExitStatus(ctx, ch, code)
			return

		case <-ctx.Done():
			return
		}
	}
}

type sessionContext struct {
	*srv.ServerContext

	channel       ssh.Channel
	remoteSession *tracessh.Session
	waitExec      chan error
}

func newSessionContext(serverCtx *srv.ServerContext, ch ssh.Channel, remoteSession *tracessh.Session) *sessionContext {
	return &sessionContext{
		ServerContext: serverCtx,
		channel:       ch,
		remoteSession: remoteSession,
		waitExec:      make(chan error, 1),
	}
}

// dispatch executes an incoming request. If successful, it returns the ok value
// for the reply. Otherwise, it returns the error it encountered.
func (s *ForwardServer) dispatch(ctx context.Context, sctx *sessionContext, req *ssh.Request) (bool, error) {
	s.logger.DebugContext(ctx, "Dispatching client request", "request_type", req.Type)

	switch req.Type {
	case tracessh.EnvsRequest:
		s.logger.DebugContext(ctx, "Ignored request", "request_type", req.Type)
		return true, nil
	case sshutils.ExecRequest:
		return true, trace.Wrap(s.handleExec(ctx, sctx, req))
	case sshutils.EnvRequest:
		return true, trace.Wrap(s.handleEnv(ctx, sctx, req))
	default:
		s.logger.WarnContext(ctx, "Received unsupported SSH request", "request_type", req.Type)
		return false, nil
	}
}

// handleExec proxies the Git command between client and the target server.
func (s *ForwardServer) handleExec(ctx context.Context, sctx *sessionContext, req *ssh.Request) (err error) {
	var r sshutils.ExecReq
	defer func() {
		if err != nil {
			s.emitEvent(s.makeGitCommandEvent(sctx, r.Command, err))
		}
	}()

	if err := ssh.Unmarshal(req.Payload, &r); err != nil {
		return trace.Wrap(err, "failed to unmarshal exec request")
	}

	command, err := ParseSSHCommand(r.Command)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkSSHCommand(s.cfg.TargetServer, command); err != nil {
		return trace.Wrap(err)
	}
	recorder := NewCommandRecorder(ctx, *command)

	sctx.remoteSession.Stdout = sctx.channel
	sctx.remoteSession.Stderr = sctx.channel.Stderr()
	remoteStdin, err := sctx.remoteSession.StdinPipe()
	if err != nil {
		return trace.Wrap(err, "failed to open remote session")
	}
	go func() {
		defer remoteStdin.Close()
		if _, err := io.Copy(io.MultiWriter(remoteStdin, recorder), sctx.channel); err != nil {
			s.logger.WarnContext(ctx, "Failed to copy git command stdin", "error", err)
		}
	}()

	if err := sctx.remoteSession.Start(ctx, r.Command); err != nil {
		return trace.Wrap(err, "failed to start git command")
	}

	go func() {
		execErr := sctx.remoteSession.Wait()
		sctx.waitExec <- execErr
		s.emitEvent(s.makeGitCommandEventWithExecResult(sctx, recorder, execErr))
	}()
	return nil
}

func (s *ForwardServer) emitEvent(event apievents.AuditEvent) {
	if err := s.cfg.Emitter.EmitAuditEvent(s.cfg.ParentContext, event); err != nil {
		s.logger.WarnContext(s.cfg.ParentContext, "Failed to emit event",
			"error", err,
			"event_type", event.GetType(),
			"event_code", event.GetCode(),
		)
	}
}

func (s *ForwardServer) makeGitCommandEvent(sctx *sessionContext, command string, err error) *apievents.GitCommand {
	event := &apievents.GitCommand{
		Metadata: apievents.Metadata{
			Type: events.GitCommandEvent,
			Code: events.GitCommandCode,
		},
		UserMetadata:    sctx.Identity.GetUserMetadata(),
		SessionMetadata: sctx.GetSessionMetadata(),
		CommandMetadata: apievents.CommandMetadata{
			Command: command,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: sctx.ServerConn.RemoteAddr().String(),
			LocalAddr:  sctx.ServerConn.LocalAddr().String(),
		},
		ServerMetadata: s.TargetMetadata(),
	}
	if err != nil {
		event.Metadata.Code = events.GitCommandFailureCode
		event.Error = err.Error()
	}
	return event
}

func (s *ForwardServer) makeGitCommandEventWithExecResult(sctx *sessionContext, recorder CommandRecorder, execErr error) *apievents.GitCommand {
	event := s.makeGitCommandEvent(sctx, recorder.GetCommand().SSHCommand, execErr)

	event.ExitCode = strconv.Itoa(sshutils.ExitCodeFromExecError(execErr))
	event.Path = string(recorder.GetCommand().Repository)
	event.Service = recorder.GetCommand().Service

	actions, err := recorder.GetActions()
	if err != nil {
		s.logger.WarnContext(s.cfg.ParentContext, "Failed to get actions from Git command recorder. No actions will be recorded in the event.", "error", err)
	} else {
		event.Actions = actions
	}
	return event
}

// handleEnv sets env on the target server.
func (s *ForwardServer) handleEnv(ctx context.Context, sctx *sessionContext, req *ssh.Request) error {
	var e sshutils.EnvReqParams
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		return trace.Wrap(err)
	}
	s.logger.DebugContext(ctx, "Setting env on remote Git server", "name", e.Name, "value", e.Value)
	err := sctx.remoteSession.Setenv(ctx, e.Name, e.Value)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to set env on remote session", "error", err, "request", e)
	}
	return nil
}

func (s *ForwardServer) initRemoteConn(ctx context.Context, ccx *sshutils.ConnectionContext, identityCtx srv.IdentityContext) error {
	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(s.cfg.ParentContext)
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := s.makeRemoteSigner(ctx, s.cfg, identityCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	clientConfig := &ssh.ClientConfig{
		User: gitUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: s.verifyRemoteHost,
		Timeout:         netConfig.GetSSHDialTimeout(),
	}
	clientConfig.Ciphers = s.cfg.Ciphers
	clientConfig.KeyExchanges = s.cfg.KEXAlgorithms
	clientConfig.MACs = s.cfg.MACAlgorithms

	s.remoteClient, err = tracessh.NewClientConnWithDeadline(
		s.cfg.ParentContext,
		s.cfg.TargetConn,
		s.cfg.DstAddr.String(),
		clientConfig,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	ccx.AddCloser(s.remoteClient)
	return nil
}

func makeRemoteSigner(ctx context.Context, cfg *ForwardServerConfig, identityCtx srv.IdentityContext) (ssh.Signer, error) {
	switch cfg.TargetServer.GetSubKind() {
	case types.SubKindGitHub:
		return MakeGitHubSigner(ctx, GitHubSignerConfig{
			Server:                  cfg.TargetServer,
			TeleportUser:            identityCtx.TeleportUser,
			IdentityExpires:         identityCtx.CertValidBefore,
			GitHubUserID:            identityCtx.UnmappedIdentity.GitHubUserID,
			AuthPreferenceGetter:    cfg.AccessPoint,
			GitHubUserCertGenerator: cfg.AuthClient.IntegrationsClient(),
			Clock:                   cfg.Clock,
		})
	default:
		return nil, trace.BadParameter("unsupported subkind %q", cfg.TargetServer.GetSubKind())
	}
}

// Below functions implement srv.Server so git.ForwardServer can be used for
// srv.NewServerContext and srv.NewAuthHandlers.
// TODO(greedy52) decouple from srv.Server.

func (s *ForwardServer) Context() context.Context {
	return s.cfg.ParentContext
}
func (s *ForwardServer) TargetMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerNamespace: s.cfg.TargetServer.GetNamespace(),
		ServerAddr:      s.cfg.DstAddr.String(),
		ServerHostname:  s.cfg.TargetServer.GetHostname(),
		ForwardedBy:     s.cfg.HostUUID,
		ServerSubKind:   s.cfg.TargetServer.GetSubKind(),
	}
}
func (s *ForwardServer) GetInfo() types.Server {
	return s.cfg.TargetServer
}
func (s *ForwardServer) ID() string {
	return s.id
}
func (s *ForwardServer) HostUUID() string {
	return s.cfg.HostUUID
}
func (s *ForwardServer) GetNamespace() string {
	return s.cfg.TargetServer.GetNamespace()
}
func (s *ForwardServer) AdvertiseAddr() string {
	return s.clientConn.RemoteAddr().String()
}
func (s *ForwardServer) Component() string {
	return teleport.ComponentForwardingGit
}
func (s *ForwardServer) PermitUserEnvironment() bool {
	return false
}
func (s *ForwardServer) GetAccessPoint() srv.AccessPoint {
	return s.cfg.AccessPoint
}
func (s *ForwardServer) GetDataDir() string {
	return ""
}
func (s *ForwardServer) GetPAM() (*servicecfg.PAMConfig, error) {
	return nil, trace.NotImplemented("not supported for git forward server")
}
func (s *ForwardServer) GetClock() clockwork.Clock {
	return s.cfg.Clock
}
func (s *ForwardServer) UseTunnel() bool {
	return false
}
func (s *ForwardServer) GetBPF() bpf.BPF {
	return nil
}
func (s *ForwardServer) GetUserAccountingPaths() (utmp, wtmp, btmp string) {
	return
}
func (s *ForwardServer) GetLockWatcher() *services.LockWatcher {
	return s.cfg.LockWatcher
}
func (s *ForwardServer) GetCreateHostUser() bool {
	return false
}
func (s *ForwardServer) GetHostUsers() srv.HostUsers {
	return nil
}
func (s *ForwardServer) GetHostSudoers() srv.HostSudoers {
	return nil
}

type serverContextKey struct{}

const (
	gitUser = "git"
)
