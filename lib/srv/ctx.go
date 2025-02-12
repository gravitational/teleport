/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package srv

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/envutils"
	"github.com/gravitational/teleport/lib/utils/parse"
)

var ctxID int32

var (
	serverTX = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tx",
			Help: "Number of bytes transmitted.",
		},
	)
	serverRX = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rx",
			Help: "Number of bytes received.",
		},
	)
)

var (
	ErrNodeFileCopyingNotPermitted  = trace.AccessDenied("node does not allow file copying via SCP or SFTP")
	errCannotStartUnattendedSession = trace.AccessDenied("lacking privileges to start unattended session")
)

func init() {
	prometheus.MustRegister(serverTX)
	prometheus.MustRegister(serverRX)
}

// AccessPoint is the access point contract required by a Server
type AccessPoint interface {
	// Announcer adds methods used to announce presence
	authclient.Announcer

	// Semaphores provides semaphore operations
	types.Semaphores

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool) ([]types.CertAuthority, error)

	// ConnectionDiagnosticTraceAppender adds a method to append traces into ConnectionDiagnostics.
	services.ConnectionDiagnosticTraceAppender
}

// Server is regular or forwarding SSH server.
type Server interface {
	// StreamEmitter allows server to emit audit events and create
	// event streams for recording sessions
	events.StreamEmitter

	// ID is the unique ID of the server.
	ID() string

	// HostUUID is the UUID of the underlying host. For the forwarding
	// server this is the proxy the forwarding server is running in.
	HostUUID() string

	// GetNamespace returns the namespace the server was created in.
	GetNamespace() string

	// AdvertiseAddr is the publicly addressable address of this server.
	AdvertiseAddr() string

	// Component is the type of server, forwarding or regular.
	Component() string

	// PermitUserEnvironment returns if reading environment variables upon
	// startup is allowed.
	PermitUserEnvironment() bool

	// GetAccessPoint returns an AccessPoint for this cluster.
	GetAccessPoint() AccessPoint

	// GetDataDir returns data directory of the server
	GetDataDir() string

	// GetPAM returns PAM configuration for this server.
	GetPAM() (*servicecfg.PAMConfig, error)

	// GetClock returns a clock setup for the server
	GetClock() clockwork.Clock

	// GetInfo returns a services.Server that represents this server.
	GetInfo() types.Server

	// UseTunnel used to determine if this node has connected to this cluster
	// using reverse tunnel.
	UseTunnel() bool

	// GetBPF returns the BPF service used for enhanced session recording.
	GetBPF() bpf.BPF

	// Context returns server shutdown context
	Context() context.Context

	// GetUserAccountingPaths returns the path of the user accounting database and log. Returns empty for system defaults.
	GetUserAccountingPaths() (utmp, wtmp, btmp string)

	// GetLockWatcher gets the server's lock watcher.
	GetLockWatcher() *services.LockWatcher

	// GetCreateHostUser returns whether the node should create
	// temporary teleport users or not
	GetCreateHostUser() bool

	// GetHostUsers returns the HostUsers instance being used to manage
	// host user provisioning
	GetHostUsers() HostUsers

	// GetHostSudoers returns the HostSudoers instance being used to manage
	// sudoer file provisioning
	GetHostSudoers() HostSudoers

	// TargetMetadata returns metadata about the session target node.
	TargetMetadata() apievents.ServerMetadata
}

// IdentityContext holds all identity information associated with the user
// logged on the connection.
type IdentityContext struct {
	// UnmappedIdentity is the base identity of the user derived from the cert, without any
	// cross-cluster mapping applied.
	UnmappedIdentity *sshca.Identity

	// AccessPermit encodes the parameters/constraints associated with an authorized ssh access.
	AccessPermit *decisionpb.SSHAccessPermit

	// TeleportUser is the Teleport user associated with the connection.
	TeleportUser string

	// Impersonator is a user acting on behalf of other user
	Impersonator string

	// Login is the operating system user associated with the connection.
	Login string

	// CertAuthority is the Certificate Authority that signed the Certificate.
	CertAuthority types.CertAuthority

	// AccessChecker is used to check RBAC permissions.
	AccessChecker services.AccessChecker

	// UnmappedRoles lists the original roles of this Teleport user without
	// trusted-cluster-related role mapping being applied.
	UnmappedRoles []string

	// CertValidBefore is set to the expiry time of a certificate, or
	// empty, if cert does not expire
	CertValidBefore time.Time

	// RouteToCluster is derived from the certificate
	RouteToCluster string

	// ActiveRequests is active access request IDs
	ActiveRequests []string

	// DisallowReissue is a flag that, if set, instructs the auth server to
	// deny any requests from this identity to generate new certificates.
	DisallowReissue bool

	// Renewable indicates this certificate is renewable.
	Renewable bool

	// BotName is the name of the Machine ID bot this identity is associated
	// with, if any.
	BotName string

	// BotInstanceID is the unique identifier of the Machine ID bot instance
	// this identity is associated with, if any.
	BotInstanceID string

	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
}

// ServerContext holds session specific context, such as SSH auth agents, PTYs,
// and other resources. SessionContext also holds a ServerContext which can be
// used to access resources on the underlying server. SessionContext can also
// be used to attach resources that should be closed once the session closes.
//
// Any events that need to be recorded should be emitted via session and not
// ServerContext directly. Failure to use the session emitted will result in
// incorrect event indexes that may ultimately cause events to be overwritten.
type ServerContext struct {
	// ConnectionContext is the parent context which manages connection-level
	// resources.
	*sshutils.ConnectionContext
	Logger *slog.Logger

	mu sync.RWMutex

	// env is a list of environment variables passed to the session.
	env map[string]string

	// srv is the server that is holding the context.
	srv Server

	// id is the server specific incremental session id.
	id int

	// term holds PTY if it was requested by the session.
	term Terminal

	// sessionID holds the session ID that will be used when a new
	// session is created.
	sessionID rsession.ID

	// session holds the active session (if there's an active one).
	session *session

	// closers is a list of io.Closer that will be called when session closes
	// this is handy as sometimes client closes session, in this case resources
	// will be properly closed and deallocated, otherwise they could be kept hanging.
	closers []io.Closer

	// Identity holds the identity of the user that is currently logged in on
	// the Conn.
	Identity IdentityContext

	// ExecResultCh is a Go channel which will be used to send and receive the
	// result of a "exec" request.
	ExecResultCh chan ExecResult

	// SubsystemResultCh is a Go channel which will be used to send and receive
	// the result of a "subsystem" request.
	SubsystemResultCh chan SubsystemResult

	// IsTestStub is set to true by tests.
	IsTestStub bool

	// execRequest is the command to be executed within this session context. Do
	// not get or set this field directly, use (Get|Set)ExecRequest.
	execRequest Exec

	// ClusterName is the name of the cluster current user is authenticated with.
	ClusterName string

	// SessionRecordingConfig holds the session recording configuration at the
	// time this context was created.
	SessionRecordingConfig types.SessionRecordingConfig

	// RemoteClient holds an SSH client to a remote server. Only used by the
	// recording proxy.
	RemoteClient *tracessh.Client

	// RemoteSession holds an SSH session to a remote server. Only used by the
	// recording proxy.
	RemoteSession *tracessh.Session

	// disconnectExpiredCert is set to time when/if the certificate should
	// be disconnected, set to empty if no disconnect is necessary
	disconnectExpiredCert time.Time

	// clientIdleTimeout is set to the timeout on
	// client inactivity, set to 0 if not setup
	clientIdleTimeout time.Duration

	// cancelContext signals closure to all outstanding operations
	cancelContext context.Context

	// cancel is called whenever server context is closed
	cancel context.CancelFunc

	// termAllocated is used to track if a terminal has been allocated. This has
	// to be tracked because the terminal is set to nil after it's "taken" in the
	// session. Terminals can be allocated for both "exec" or "session" requests.
	termAllocated bool

	// ttyName is the name of the TTY used for a session, ex: /dev/pts/0
	ttyName string

	// sshRequest is the SSH request that was issued by the client. Do not get or
	// set this field directly, use (Get|Set)SSHRequest instead.
	sshRequest *ssh.Request

	// cmd{r,w} are used to send the command from the parent process to the
	// child process.
	cmdr *os.File
	cmdw *os.File

	// cont{r,w} is used to send the continue signal from the parent process
	// to the child process.
	contr *os.File
	contw *os.File

	// ready{r,w} is used to send the ready signal from the child process
	// to the parent process.
	readyr *os.File
	readyw *os.File

	// killShell{r,w} are used to send kill signal to the child process
	// to terminate the shell.
	killShellr *os.File
	killShellw *os.File

	// ExecType holds the type of the channel or request. For example "session" or
	// "direct-tcpip". Used to create correct subcommand during re-exec.
	ExecType string

	// SrcAddr is the source address of the request. This the originator IP
	// address and port in an SSH "direct-tcpip" or "tcpip-forward" request. This
	// value is only populated for port forwarding requests.
	SrcAddr string

	// DstAddr is the destination address of the request. This is the host and
	// port to connect to in a "direct-tcpip" or "tcpip-forward" request. This
	// value is only populated for port forwarding requests.
	DstAddr string

	// allowFileCopying controls if remote file operations via SCP/SFTP are allowed
	// by the server.
	AllowFileCopying bool

	// JoinOnly is set if the connection was created using a join-only principal and may only be used to join other sessions.
	JoinOnly bool

	// ServerSubKind if the sub kind of the node this context is for.
	ServerSubKind string

	// approvedFileReq is an approved file transfer request that will only be
	// set when the session's pending file transfer request is approved.
	approvedFileReq *FileTransferRequest
}

// NewServerContext creates a new *ServerContext which is used to pass and
// manage resources, and an associated context.Context which is canceled when
// the ServerContext is closed.  The ctx parameter should be a child of the ctx
// associated with the scope of the parent ConnectionContext to ensure that
// cancellation of the ConnectionContext propagates to the ServerContext.
func NewServerContext(ctx context.Context, parent *sshutils.ConnectionContext, srv Server, identityContext IdentityContext, monitorOpts ...func(*MonitorConfig)) (*ServerContext, error) {
	recConfig, err := srv.GetAccessPoint().GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if identityContext.AccessPermit == nil {
		fmt.Printf("---> nil access permit: %+v\n", identityContext)
	}

	cancelContext, cancel := context.WithCancel(ctx)
	child := &ServerContext{
		ConnectionContext:      parent,
		id:                     int(atomic.AddInt32(&ctxID, int32(1))),
		env:                    make(map[string]string),
		srv:                    srv,
		ExecResultCh:           make(chan ExecResult, 10),
		SubsystemResultCh:      make(chan SubsystemResult, 10),
		ClusterName:            parent.ServerConn.Permissions.Extensions[utils.CertTeleportClusterName],
		SessionRecordingConfig: recConfig,
		Identity:               identityContext,
		clientIdleTimeout:      durationToGoDuration(identityContext.AccessPermit.ClientIdleTimeout),
		cancelContext:          cancelContext,
		cancel:                 cancel,
		ServerSubKind:          srv.TargetMetadata().ServerSubKind,
	}

	child.Logger = slog.With(
		teleport.ComponentKey, srv.Component(),
		"local_addr", child.ServerConn.LocalAddr(),
		"remote_addr", child.ServerConn.RemoteAddr(),
		"login", child.Identity.Login,
		"teleport_user", child.Identity.TeleportUser,
		"id", child.id,
	)

	if identityContext.Login == teleport.SSHSessionJoinPrincipal {
		child.JoinOnly = true
	}

	authPref, err := srv.GetAccessPoint().GetAuthPreference(ctx)
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}

	child.disconnectExpiredCert = getDisconnectExpiredCertFromIdentityContext(
		identityContext.AccessChecker, authPref, &identityContext,
	)

	// Update log entry fields.
	if !child.disconnectExpiredCert.IsZero() {
		child.Logger = child.Logger.With("cert", child.disconnectExpiredCert)
	}
	if child.clientIdleTimeout != 0 {
		child.Logger = child.Logger.With("idle", child.clientIdleTimeout)
	}

	clusterName, err := srv.GetAccessPoint().GetClusterName()
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}

	monitorConfig := MonitorConfig{
		LockWatcher:           child.srv.GetLockWatcher(),
		LockTargets:           ComputeLockTargets(clusterName.GetClusterName(), srv.HostUUID(), identityContext),
		LockingMode:           identityContext.AccessChecker.LockingMode(authPref.GetLockingMode()),
		DisconnectExpiredCert: child.disconnectExpiredCert,
		ClientIdleTimeout:     child.clientIdleTimeout,
		Clock:                 child.srv.GetClock(),
		Tracker:               child,
		Conn:                  child.ServerConn,
		Context:               cancelContext,
		TeleportUser:          child.Identity.TeleportUser,
		Login:                 child.Identity.Login,
		ServerID:              child.srv.ID(),
		Logger:                child.Logger,
		Emitter:               child.srv,
		EmitterContext:        ctx,
	}
	for _, opt := range monitorOpts {
		opt(&monitorConfig)
	}
	err = StartMonitor(monitorConfig)
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}

	// Create pipe used to send command to child process.
	child.cmdr, child.cmdw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.cmdr)
	child.AddCloser(child.cmdw)

	// Create pipe used to signal continue to child process.
	child.contr, child.contw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.contr)
	child.AddCloser(child.contw)

	// Create pipe used to signal continue to parent process.
	child.readyr, child.readyw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.readyr)
	child.AddCloser(child.readyw)

	child.killShellr, child.killShellw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.killShellr)
	child.AddCloser(child.killShellw)

	return child, nil
}

// Parent grants access to the connection-level context of which this
// is a subcontext.  Useful for unambiguously accessing methods which
// this subcontext overrides (e.g. child.Parent().SetEnv(...)).
func (c *ServerContext) Parent() *sshutils.ConnectionContext {
	return c.ConnectionContext
}

// ID returns ID of this context
func (c *ServerContext) ID() int {
	return c.id
}

// SessionID returns the ID of the session in the context.
func (c *ServerContext) SessionID() rsession.ID {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.session == nil {
		return c.sessionID
	}
	return c.session.id
}

// GetServer returns the underlying server which this context was created in.
func (c *ServerContext) GetServer() Server {
	return c.srv
}

// CreateOrJoinSession will look in the SessionRegistry for the session ID. If
// no session is found, a new one is created. If one is found, it is returned.
func (c *ServerContext) CreateOrJoinSession(ctx context.Context, reg *SessionRegistry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// As SSH conversation progresses, at some point a session will be created and
	// its ID will be added to the environment
	ssid, found := c.getEnvLocked(sshutils.SessionEnvVar)
	if !found {
		c.sessionID = rsession.NewID()
		c.Logger.DebugContext(ctx, "Will create new session for SSH connection")
		return nil
	}

	// make sure whatever session is requested is a valid session
	id, err := rsession.ParseID(ssid)
	if err != nil {
		return trace.BadParameter("invalid session ID")
	}

	// update ctx with the session if it exists
	if sess, found := reg.findSession(*id); found {
		c.sessionID = *id
		c.session = sess
		c.Logger.DebugContext(ctx, "Joining active SSH session", "session_id", c.session.id)
	} else {
		// TODO(capnspacehook): DELETE IN 17.0.0 - by then all supported
		// clients should only set TELEPORT_SESSION when they want to
		// join a session. Always return an error instead of using a
		// new ID.
		//
		// to prevent the user from controlling the session ID, generate
		// a new one
		c.sessionID = rsession.NewID()
		c.Logger.DebugContext(ctx, "Creating new SSH session")
	}

	return nil
}

// TrackActivity keeps track of all activity on ssh.Channel. The caller should
// use the returned ssh.Channel instead of the original one.
func (c *ServerContext) TrackActivity(ch ssh.Channel) ssh.Channel {
	return newTrackingChannel(ch, c)
}

// AddCloser adds any closer in ctx that will be called
// whenever server closes session channel
func (c *ServerContext) AddCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closers = append(c.closers, closer)
}

// GetTerm returns a Terminal.
func (c *ServerContext) GetTerm() Terminal {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.term
}

// SetTerm set a Terminal.
func (c *ServerContext) SetTerm(t Terminal) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.term = t
}

// VisitEnv grants visitor-style access to env variables.
func (c *ServerContext) VisitEnv(visit func(key, val string)) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// visit the parent env first since locally defined variables
	// effectively "override" parent defined variables.
	c.Parent().VisitEnv(visit)
	for key, val := range c.env {
		visit(key, val)
	}
}

// SetEnv sets a environment variable within this context.
func (c *ServerContext) SetEnv(key, val string) {
	c.mu.Lock()
	c.env[key] = val
	c.mu.Unlock()
}

// GetEnv returns a environment variable within this context.
func (c *ServerContext) GetEnv(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getEnvLocked(key)
}

func (c *ServerContext) getEnvLocked(key string) (string, bool) {
	val, ok := c.env[key]
	if ok {
		return val, true
	}
	return c.Parent().GetEnv(key)
}

// setSession sets the context's session
func (c *ServerContext) setSession(ctx context.Context, sess *session, ch ssh.Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = sess

	// inform the client of the session ID that is being used in a new
	// goroutine to reduce latency
	go func() {
		c.Logger.DebugContext(ctx, "Sending current session ID")
		_, err := ch.SendRequest(teleport.CurrentSessionIDRequest, false, []byte(sess.ID()))
		if err != nil {
			c.Logger.DebugContext(ctx, "Failed to send the current session ID", "error", err)
		}
	}()
}

// getSession returns the context's session
func (c *ServerContext) getSession() *session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

func (c *ServerContext) SetAllowFileCopying(allow bool) {
	c.AllowFileCopying = allow
}

// CheckFileCopyingAllowed returns an error if remote file operations via
// SCP or SFTP are not allowed by the user's role or the node's config.
func (c *ServerContext) CheckFileCopyingAllowed() error {
	// Check if remote file operations are disabled for this node.
	if !c.AllowFileCopying {
		return trace.Wrap(ErrNodeFileCopyingNotPermitted)
	}
	// Check if the user's RBAC role allows remote file operations.
	if !c.Identity.AccessChecker.CanCopyFiles() {
		return trace.Wrap(errRoleFileCopyingNotPermitted)
	}

	return nil
}

// CheckSFTPAllowed returns an error if remote file operations via SCP
// or SFTP are not allowed by the user's role or the node's config, or
// if the user is not allowed to start unattended sessions.
func (c *ServerContext) CheckSFTPAllowed(registry *SessionRegistry) error {
	if err := c.CheckFileCopyingAllowed(); err != nil {
		return trace.Wrap(err)
	}

	// ensure moderated session policies allow starting an unattended session
	policySets := c.Identity.AccessChecker.SessionPolicySets()
	checker := auth.NewSessionAccessEvaluator(policySets, types.SSHSessionKind, c.Identity.TeleportUser)
	canStart, _, err := checker.FulfilledFor(nil)
	if err != nil {
		return trace.Wrap(err)
	}
	// canStart will be true for non-moderated sessions. If canStart is false, check to
	// see if the request has been approved through a moderated session next.
	if canStart {
		return nil
	}
	if registry == nil {
		return trace.Wrap(errCannotStartUnattendedSession)
	}

	approved, err := registry.isApprovedFileTransfer(c)
	if err != nil {
		return trace.Wrap(err)
	}
	if !approved {
		return trace.Wrap(errCannotStartUnattendedSession)
	}

	return nil
}

// OpenXServerListener opens a new XServer unix listener.
func (c *ServerContext) HandleX11Listener(ctx context.Context, l net.Listener, singleConnection bool) error {
	display, err := x11.ParseDisplayFromUnixSocket(l.Addr().String())
	if err != nil {
		return trace.Wrap(err)
	}
	c.Parent().SetEnv(x11.DisplayEnv, display.String())

	// Prepare X11 channel request payload
	originHost, originPort, err := net.SplitHostPort(c.ServerConn.LocalAddr().String())
	if err != nil {
		return trace.Wrap(err)
	}
	originPortI, err := strconv.Atoi(originPort)
	if err != nil {
		return trace.Wrap(err)
	}
	x11ChannelReqPayload := ssh.Marshal(x11.ChannelRequestPayload{
		OriginatorAddress: originHost,
		OriginatorPort:    uint32(originPortI),
	})

	go func() {
		for {
			xconn, err := l.Accept()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					c.Logger.DebugContext(ctx, "Encountered error accepting XServer connection", "error", err)
				}
				return
			}

			go func() {
				defer xconn.Close()

				xchan, sin, err := c.ServerConn.OpenChannel(x11.ChannelRequest, x11ChannelReqPayload)
				if err != nil {
					c.Logger.DebugContext(ctx, "Failed to open a new X11 channel", "error", err)
					return
				}
				defer xchan.Close()

				// Forward ssh requests on the X11 channels until X11 forwarding is complete
				ctx, cancel := context.WithCancel(c.cancelContext)
				defer cancel()

				go func() {
					err := sshutils.ForwardRequests(ctx, sin, c.RemoteSession)
					if err != nil {
						c.Logger.DebugContext(ctx, "Failed to forward ssh request from server during X11 forwarding", "error", err)
					}
				}()

				if err := utils.ProxyConn(ctx, xconn, xchan); err != nil {
					c.Logger.DebugContext(ctx, "Encountered error during X11 forwarding", "error", err)
				}
			}()

			if singleConnection {
				l.Close()
				return
			}
		}
	}()

	return nil
}

// takeClosers returns all resources that should be closed and sets the properties to null
// we do this to avoid calling Close() under lock to avoid potential deadlocks
func (c *ServerContext) takeClosers() []io.Closer {
	// this is done to avoid any operation holding the lock for too long
	c.mu.Lock()
	defer c.mu.Unlock()

	closers := []io.Closer{}
	if c.term != nil {
		closers = append(closers, c.term)
		c.term = nil
	}
	closers = append(closers, c.closers...)
	c.closers = nil
	return closers
}

// When the ServerContext (connection) is closed, emit "session.data" event
// containing how much data was transmitted and received over the net.Conn.
func (c *ServerContext) reportStats(conn utils.Stater) {
	// We may not want to record session data for this connection context, e.g. if this is
	// for a networking subprocess tied to a shell process.
	if c.SessionRecordingConfig.GetMode() == types.RecordOff {
		return
	}

	// Never emit session data events for the proxy or from a Teleport node if
	// sessions are being recorded at the proxy (this would result in double
	// events).
	// Do not emit session data for git commands as they have their own events.
	if c.GetServer().Component() == teleport.ComponentProxy ||
		c.GetServer().Component() == teleport.ComponentForwardingGit {
		return
	}
	if services.IsRecordAtProxy(c.SessionRecordingConfig.GetMode()) &&
		c.GetServer().Component() == teleport.ComponentNode {
		return
	}

	// Get the TX and RX bytes.
	txBytes, rxBytes := conn.Stat()

	// Build and emit session data event. Note that TX and RX are reversed
	// below, that is because the connection is held from the perspective of
	// the server not the client, but the logs are from the perspective of the
	// client.
	sessionDataEvent := &apievents.SessionData{
		Metadata: apievents.Metadata{
			Index: events.SessionDataIndex,
			Type:  events.SessionDataEvent,
			Code:  events.SessionDataCode,
		},
		ServerMetadata:  c.srv.TargetMetadata(),
		SessionMetadata: c.GetSessionMetadata(),
		UserMetadata:    c.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: c.ServerConn.RemoteAddr().String(),
		},
		BytesTransmitted: rxBytes,
		BytesReceived:    txBytes,
	}
	if !c.srv.UseTunnel() {
		sessionDataEvent.ConnectionMetadata.LocalAddr = c.ServerConn.LocalAddr().String()
	}
	if err := c.GetServer().EmitAuditEvent(c.GetServer().Context(), sessionDataEvent); err != nil {
		c.Logger.WarnContext(c.GetServer().Context(), "Failed to emit session data event", "error", err)
	}

	// Emit TX and RX bytes to their respective Prometheus counters.
	serverTX.Add(float64(txBytes))
	serverRX.Add(float64(rxBytes))
}

func (c *ServerContext) Close() error {
	// If the underlying connection is holding tracking information, report that
	// to the audit log at close.
	if stats, ok := c.NetConn.(*utils.TrackingConn); ok {
		defer c.reportStats(stats)
	}

	// Unblock any goroutines waiting until session is closed.
	c.cancel()

	// Close and release all resources.
	err := closeAll(c.takeClosers()...)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CancelContext is a context associated with server context,
// closed whenever this server context is closed
func (c *ServerContext) CancelContext() context.Context {
	return c.cancelContext
}

// CancelFunc gets the context.CancelFunc associated with
// this context.  Not a substitute for calling the
// ServerContext.Close method.
func (c *ServerContext) CancelFunc() context.CancelFunc {
	return c.cancel
}

// SendExecResult sends the result of execution of the "exec" command over the
// ExecResultCh.
func (c *ServerContext) SendExecResult(ctx context.Context, r ExecResult) {
	select {
	case c.ExecResultCh <- r:
	default:
		c.Logger.InfoContext(ctx, "Blocked on sending exec result", "code", r.Code, "command", r.Command)
	}
}

// SendSubsystemResult sends the result of running the subsystem over the
// SubsystemResultCh.
func (c *ServerContext) SendSubsystemResult(ctx context.Context, r SubsystemResult) {
	select {
	case c.SubsystemResultCh <- r:
	default:
		c.Logger.InfoContext(ctx, "Blocked on sending subsystem result")
	}
}

func (c *ServerContext) String() string {
	return fmt.Sprintf("ServerContext(%v->%v, user=%v, id=%v)", c.ServerConn.RemoteAddr(), c.ServerConn.LocalAddr(), c.ServerConn.User(), c.id)
}

func (c *ServerContext) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("remote_addr", c.ServerConn.RemoteAddr().String()),
		slog.String("local_addr", c.ServerConn.LocalAddr().String()),
		slog.String("user", c.ServerConn.User()),
		slog.Int("id", c.id),
	)
}

func getPAMConfig(c *ServerContext) (*PAMConfig, error) {
	// PAM should be disabled.
	if c.srv.Component() != teleport.ComponentNode {
		return nil, nil
	}

	localPAMConfig, err := c.srv.GetPAM()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We use nil/empty to figure out if PAM is disabled later.
	if !localPAMConfig.Enabled {
		return nil, nil
	}

	// If the identity has roles, extract the role names.
	roleNames := c.Identity.AccessChecker.RoleNames()

	// Fill in the environment variables from the config and interpolate them if needed.
	environment := make(map[string]string)
	environment["TELEPORT_USERNAME"] = c.Identity.TeleportUser
	environment["TELEPORT_LOGIN"] = c.Identity.Login
	environment["TELEPORT_ROLES"] = strings.Join(roleNames, " ")
	if localPAMConfig.Environment != nil {
		for key, value := range localPAMConfig.Environment {
			expr, err := parse.NewTraitsTemplateExpression(value)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			varValidation := func(namespace, name string) error {
				if namespace != teleport.TraitExternalPrefix {
					return trace.BadParameter("PAM environment interpolation only supports external traits, found %q", value)
				}
				return nil
			}

			result, err := expr.Interpolate(varValidation, c.Identity.UnmappedIdentity.Traits)
			if err != nil {
				// If the trait isn't passed by the IdP due to misconfiguration
				// we fallback to setting a value which will indicate this.
				if trace.IsNotFound(err) {
					c.Logger.WarnContext(
						c.CancelContext(),
						"Attempted to interpolate custom PAM environment with external trait but received SAML response does not contain claim",
						"error", err,
					)
					continue
				}

				return nil, trace.Wrap(err)
			}

			environment[key] = strings.Join(result, " ")
		}
	}

	return &PAMConfig{
		UsePAMAuth:  localPAMConfig.UsePAMAuth,
		ServiceName: localPAMConfig.ServiceName,
		Environment: environment,
	}, nil
}

// ExecCommand takes a *ServerContext and extracts the parts needed to create
// an *execCommand which can be re-sent to Teleport.
func (c *ServerContext) ExecCommand() (*ExecCommand, error) {
	// If the identity has roles, extract the role names.
	roleNames := c.Identity.AccessChecker.RoleNames()

	// Extract the command to be executed. This only exists if command execution
	// (exec or shell) is being requested, port forwarding has no command to
	// execute.
	var command string
	if execRequest, err := c.GetExecRequest(); err == nil {
		command = execRequest.GetCommand()
	}

	// Extract the request type. This only exists for command execution (exec
	// or shell), port forwarding requests have no request type.
	var requestType string
	if sshRequest, err := c.GetSSHRequest(); err == nil {
		requestType = sshRequest.Type
	}

	uaccMetadata, err := newUaccMetadata(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pamConfig, err := getPAMConfig(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the execCommand that will be sent to the child process.
	return &ExecCommand{
		Command:               command,
		DestinationAddress:    c.DstAddr,
		Username:              c.Identity.TeleportUser,
		Login:                 c.Identity.Login,
		Roles:                 roleNames,
		Terminal:              c.termAllocated || command == "",
		TerminalName:          c.ttyName,
		ClientAddress:         c.ServerConn.RemoteAddr().String(),
		RequestType:           requestType,
		PermitUserEnvironment: c.srv.PermitUserEnvironment(),
		UserCreatedByTeleport: c.UserCreatedByTeleport,
		Environment:           buildEnvironment(c),
		PAMConfig:             pamConfig,
		IsTestStub:            c.IsTestStub,
		UaccMetadata:          *uaccMetadata,
	}, nil
}

func eventDeviceMetadataFromIdentity(ident *sshca.Identity) *apievents.DeviceMetadata {
	if ident == nil {
		return nil
	}

	if ident.DeviceID == "" && ident.DeviceAssetTag == "" && ident.DeviceCredentialID == "" {
		return nil
	}

	return &apievents.DeviceMetadata{
		DeviceId:     ident.DeviceID,
		AssetTag:     ident.DeviceAssetTag,
		CredentialId: ident.DeviceCredentialID,
	}
}

func (id *IdentityContext) GetUserMetadata() apievents.UserMetadata {
	userKind := apievents.UserKind_USER_KIND_HUMAN
	if id.BotName != "" {
		userKind = apievents.UserKind_USER_KIND_BOT
	}

	return apievents.UserMetadata{
		Login:          id.Login,
		User:           id.TeleportUser,
		Impersonator:   id.Impersonator,
		AccessRequests: id.ActiveRequests,
		TrustedDevice:  eventDeviceMetadataFromIdentity(id.UnmappedIdentity),
		UserKind:       userKind,
		BotName:        id.BotName,
		BotInstanceID:  id.BotInstanceID,
	}
}

// buildEnvironment constructs a list of environment variables from
// cluster information.
func buildEnvironment(ctx *ServerContext) []string {
	env := &envutils.SafeEnv{}

	// Parse the local and remote addresses to build SSH_CLIENT and
	// SSH_CONNECTION environment variables.
	remoteHost, remotePort, err := net.SplitHostPort(ctx.ServerConn.RemoteAddr().String())
	if err != nil {
		ctx.Logger.DebugContext(ctx.CancelContext(), "Failed to split remote address", "error", err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.ServerConn.LocalAddr().String())
		if err != nil {
			ctx.Logger.DebugContext(ctx.CancelContext(), "Failed to split local address", "error", err)
		} else {
			env.AddTrusted("SSH_CLIENT", fmt.Sprintf("%s %s %s", remoteHost, remotePort, localPort))
			env.AddTrusted("SSH_CONNECTION", fmt.Sprintf("%s %s %s %s", remoteHost, remotePort, localHost, localPort))
		}
	}

	// If a session has been created try and set TERM, SSH_TTY, and SSH_SESSION_ID.
	session := ctx.getSession()
	if session != nil {
		if session.term != nil {
			env.AddTrusted("TERM", session.term.GetTermType())
			env.AddTrusted("SSH_TTY", session.term.TTYName())
		}
		if session.id != "" {
			env.AddTrusted(teleport.SSHSessionID, string(session.id))
		}
	}

	// Set some Teleport specific environment variables: SSH_TELEPORT_USER,
	// SSH_TELEPORT_HOST_UUID, and SSH_TELEPORT_CLUSTER_NAME.
	env.AddTrusted(teleport.SSHTeleportHostUUID, ctx.srv.ID())
	env.AddTrusted(teleport.SSHTeleportClusterName, ctx.ClusterName)
	env.AddTrusted(teleport.SSHTeleportUser, ctx.Identity.TeleportUser)

	// At the end gather all dynamically defined environment variables
	ctx.VisitEnv(env.AddUnique)

	return *env
}

func closeAll(closers ...io.Closer) error {
	var errs []error

	for _, cl := range closers {
		if cl == nil {
			continue
		}

		err := cl.Close()
		if err == nil {
			continue
		}

		errs = append(errs, err)
	}

	return trace.NewAggregate(errs...)
}

func newUaccMetadata(c *ServerContext) (*UaccMetadata, error) {
	addr := c.ConnectionContext.ServerConn.Conn.RemoteAddr()
	hostname, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	preparedAddr, err := uacc.PrepareAddr(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	utmpPath, wtmpPath, btmpPath := c.srv.GetUserAccountingPaths()

	return &UaccMetadata{
		Hostname:   hostname,
		RemoteAddr: preparedAddr,
		UtmpPath:   utmpPath,
		WtmpPath:   wtmpPath,
		BtmpPath:   btmpPath,
	}, nil
}

// ComputeLockTargets computes lock targets inferred from the clusterName, serverID and IdentityContext.
func ComputeLockTargets(clusterName, serverID string, id IdentityContext) []types.LockTarget {
	lockTargets := []types.LockTarget{
		{User: id.TeleportUser},
		{Login: id.Login},
		{Node: serverID, ServerID: serverID},
		{Node: authclient.HostFQDN(serverID, clusterName), ServerID: authclient.HostFQDN(serverID, clusterName)},
	}
	if mfaDevice := id.UnmappedIdentity.MFAVerified; mfaDevice != "" {
		lockTargets = append(lockTargets, types.LockTarget{MFADevice: mfaDevice})
	}
	if trustedDevice := id.UnmappedIdentity.DeviceID; trustedDevice != "" {
		lockTargets = append(lockTargets, types.LockTarget{Device: trustedDevice})
	}
	roles := apiutils.Deduplicate(append(id.AccessChecker.RoleNames(), id.UnmappedRoles...))
	lockTargets = append(lockTargets, services.RolesToLockTargets(roles)...)
	lockTargets = append(lockTargets, services.AccessRequestsToLockTargets(id.ActiveRequests)...)
	return lockTargets
}

// SetSSHRequest sets the ssh request that was issued by the client.
// Will return an error if called more than once for a single server context.
func (c *ServerContext) SetSSHRequest(e *ssh.Request) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sshRequest != nil {
		return trace.AlreadyExists("sshRequest has already been set")
	}
	c.sshRequest = e
	return nil
}

// GetSSHRequest returns the ssh request that was issued by the client and saved on
// this ServerContext by SetExecRequest, or an error if it has not been set.
func (c *ServerContext) GetSSHRequest() (*ssh.Request, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.sshRequest == nil {
		return nil, trace.NotFound("sshRequest has not been set")
	}
	return c.sshRequest, nil
}

// SetExecRequest sets the command to be executed within this session context.
// Will return an error if called more than once for a single server context.
func (c *ServerContext) SetExecRequest(e Exec) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.execRequest != nil {
		return trace.AlreadyExists("execRequest has already been set")
	}
	c.execRequest = e
	return nil
}

// GetExecRequest returns the exec request that is to be executed within this
// session context, or an error if it has not been set.
func (c *ServerContext) GetExecRequest() (Exec, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.execRequest == nil {
		return nil, trace.NotFound("execRequest has not been set")
	}
	return c.execRequest, nil
}

func (c *ServerContext) GetSessionMetadata() apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        string(c.SessionID()),
		WithMFA:          c.Identity.UnmappedIdentity.MFAVerified,
		PrivateKeyPolicy: string(c.Identity.UnmappedIdentity.PrivateKeyPolicy),
	}
}

func (c *ServerContext) GetPortForwardEvent(evType, code, addr string) apievents.PortForward {
	sconn := c.ConnectionContext.ServerConn
	return apievents.PortForward{
		Metadata: apievents.Metadata{
			Type: evType,
			Code: code,
		},
		UserMetadata: c.Identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  sconn.LocalAddr().String(),
			RemoteAddr: sconn.RemoteAddr().String(),
		},
		Addr: addr,
		Status: apievents.Status{
			Success: true,
		},
	}
}

func (c *ServerContext) setApprovedFileTransferRequest(req *FileTransferRequest) {
	c.mu.Lock()
	c.approvedFileReq = req
	c.mu.Unlock()
}

// ConsumeApprovedFileTransferRequest will return the approved file transfer
// request for this session if there is one present. Note that if an
// approved request is returned future calls to this method will return
// nil to prevent an approved request getting reused incorrectly.
func (c *ServerContext) ConsumeApprovedFileTransferRequest() *FileTransferRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := c.approvedFileReq
	c.approvedFileReq = nil

	return req
}
