/*
Copyright 2015 Gravitational, Inc.

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

package srv

import (
	"context"
	"fmt"
	"io"
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
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
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
	auth.Announcer

	// Semaphores provides semaphore operations
	types.Semaphores

	// GetClusterName returns cluster name
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)

	// GetClusterNetworkingConfig returns cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error)

	// GetSessionRecordingConfig returns session recording configuration.
	GetSessionRecordingConfig(ctx context.Context, opts ...services.MarshalOption) (types.SessionRecordingConfig, error)

	// GetAuthPreference returns the cluster authentication configuration.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)

	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)

	// GetCertAuthorities returns a list of cert authorities
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error)

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
	GetPAM() (*pam.Config, error)

	// GetClock returns a clock setup for the server
	GetClock() clockwork.Clock

	// GetInfo returns a services.Server that represents this server.
	GetInfo() types.Server

	// UseTunnel used to determine if this node has connected to this cluster
	// using reverse tunnel.
	UseTunnel() bool

	// GetBPF returns the BPF service used for enhanced session recording.
	GetBPF() bpf.BPF

	// GetRestrictedSessionManager returns the manager for restricting user activity
	GetRestrictedSessionManager() restricted.Manager

	// Context returns server shutdown context
	Context() context.Context

	// GetUtmpPath returns the path of the user accounting database and log. Returns empty for system defaults.
	GetUtmpPath() (utmp, wtmp string)

	// GetLockWatcher gets the server's lock watcher.
	GetLockWatcher() *services.LockWatcher

	// GetCreateHostUser returns whether the node should create
	// temporary teleport users or not
	GetCreateHostUser() bool

	// GetHostUser returns the HostUsers instance being used to manage
	// host user provisioning
	GetHostUsers() HostUsers

	// TargetMetadata returns metadata about the session target node.
	TargetMetadata() apievents.ServerMetadata
}

// IdentityContext holds all identity information associated with the user
// logged on the connection.
type IdentityContext struct {
	// TeleportUser is the Teleport user associated with the connection.
	TeleportUser string

	// Impersonator is a user acting on behalf of other user
	Impersonator string

	// Login is the operating system user associated with the connection.
	Login string

	// Certificate is the SSH user certificate bytes marshaled in the OpenSSH
	// authorized_keys format.
	Certificate *ssh.Certificate

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

	// Generation counts the number of times this identity's certificate has
	// been renewed.
	Generation uint64

	// AllowedResourceIDs lists the resources this identity should be allowed to
	// access
	AllowedResourceIDs []types.ResourceID
}

// ServerContext holds session specific context, such as SSH auth agents, PTYs,
// and other resources. SessionContext also holds a ServerContext which can be
// used to access resources on the underlying server. SessionContext can also
// be used to attach resources that should be closed once the session closes.
type ServerContext struct {
	// ConnectionContext is the parent context which manages connection-level
	// resources.
	*sshutils.ConnectionContext
	*log.Entry

	mu sync.RWMutex

	// env is a list of environment variables passed to the session.
	env map[string]string

	// srv is the server that is holding the context.
	srv Server

	// id is the server specific incremental session id.
	id int

	// term holds PTY if it was requested by the session.
	term Terminal

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

	// ChannelType holds the type of the channel. For example "session" or
	// "direct-tcpip". Used to create correct subcommand during re-exec.
	ChannelType string

	// SrcAddr is the source address of the request. This the originator IP
	// address and port in an SSH "direct-tcpip" request. This value is only
	// populated for port forwarding requests.
	SrcAddr string

	// DstAddr is the destination address of the request. This is the host and
	// port to connect to in a "direct-tcpip" request. This value is only
	// populated for port forwarding requests.
	DstAddr string

	// allowFileCopying controls if remote file operations via SCP/SFTP are allowed
	// by the server.
	AllowFileCopying bool

	// x11rdy{r,w} is used to signal from the child process to the
	// parent process when X11 forwarding is set up.
	x11rdyr *os.File
	x11rdyw *os.File

	// x11Config holds the xauth and XServer listener config for this session.
	x11Config *X11Config

	// JoinOnly is set if the connection was created using a join-only principal and may only be used to join other sessions.
	JoinOnly bool
}

// NewServerContext creates a new *ServerContext which is used to pass and
// manage resources, and an associated context.Context which is canceled when
// the ServerContext is closed.  The ctx parameter should be a child of the ctx
// associated with the scope of the parent ConnectionContext to ensure that
// cancellation of the ConnectionContext propagates to the ServerContext.
func NewServerContext(ctx context.Context, parent *sshutils.ConnectionContext, srv Server, identityContext IdentityContext, monitorOpts ...func(*MonitorConfig)) (context.Context, *ServerContext, error) {
	netConfig, err := srv.GetAccessPoint().GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	recConfig, err := srv.GetAccessPoint().GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
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
		clientIdleTimeout:      identityContext.AccessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		cancelContext:          cancelContext,
		cancel:                 cancel,
	}

	fields := log.Fields{
		"local":        child.ServerConn.LocalAddr(),
		"remote":       child.ServerConn.RemoteAddr(),
		"login":        child.Identity.Login,
		"teleportUser": child.Identity.TeleportUser,
		"id":           child.id,
	}
	child.Entry = log.WithFields(log.Fields{
		trace.Component:       child.srv.Component(),
		trace.ComponentFields: fields,
	})

	if identityContext.Login == teleport.SSHSessionJoinPrincipal {
		child.JoinOnly = true
	}

	authPref, err := srv.GetAccessPoint().GetAuthPreference(ctx)
	if err != nil {
		childErr := child.Close()
		return nil, nil, trace.NewAggregate(err, childErr)
	}
	disconnectExpiredCert := identityContext.AccessChecker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert())
	if !identityContext.CertValidBefore.IsZero() && disconnectExpiredCert {
		child.disconnectExpiredCert = identityContext.CertValidBefore
	}

	// Update log entry fields.
	if !child.disconnectExpiredCert.IsZero() {
		fields["cert"] = child.disconnectExpiredCert
	}
	if child.clientIdleTimeout != 0 {
		fields["idle"] = child.clientIdleTimeout
	}
	child.Entry = log.WithFields(log.Fields{
		trace.Component:       srv.Component(),
		trace.ComponentFields: fields,
	})

	lockTargets, err := ComputeLockTargets(srv, identityContext)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	monitorConfig := MonitorConfig{
		LockWatcher:           child.srv.GetLockWatcher(),
		LockTargets:           lockTargets,
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
		Entry:                 child.Entry,
		Emitter:               child.srv,
	}
	for _, opt := range monitorOpts {
		opt(&monitorConfig)
	}
	err = StartMonitor(monitorConfig)
	if err != nil {
		childErr := child.Close()
		return nil, nil, trace.NewAggregate(err, childErr)
	}

	// Create pipe used to send command to child process.
	child.cmdr, child.cmdw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.cmdr)
	child.AddCloser(child.cmdw)

	// Create pipe used to signal continue to child process.
	child.contr, child.contw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.contr)
	child.AddCloser(child.contw)

	// Create pipe used to get X11 forwarding ready signal from the child process.
	child.x11rdyr, child.x11rdyw, err = os.Pipe()
	if err != nil {
		childErr := child.Close()
		return nil, nil, trace.NewAggregate(err, childErr)
	}
	child.AddCloser(child.x11rdyr)
	child.AddCloser(child.x11rdyw)

	return ctx, child, nil
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
		return ""
	}
	return c.session.id
}

// GetServer returns the underlying server which this context was created in.
func (c *ServerContext) GetServer() Server {
	return c.srv
}

// CreateOrJoinSession will look in the SessionRegistry for the session ID. If
// no session is found, a new one is created. If one is found, it is returned.
func (c *ServerContext) CreateOrJoinSession(reg *SessionRegistry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// As SSH conversation progresses, at some point a session will be created and
	// its ID will be added to the environment
	ssid, found := c.getEnvLocked(sshutils.SessionEnvVar)
	if !found {
		return nil
	}
	// make sure whatever session is requested is a valid session
	id, err := rsession.ParseID(ssid)
	if err != nil {
		return trace.BadParameter("invalid session id")
	}

	// update ctx with the session if it exists
	if sess, found := reg.findSession(*id); found {
		c.session = sess
		c.Logger.Debugf("Will join session %v for SSH connection %v.", c.session.id, c.ServerConn.RemoteAddr())
	} else {
		c.Logger.Debugf("Will create new session for SSH connection %v.", c.ServerConn.RemoteAddr())
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
func (c *ServerContext) setSession(sess *session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = sess
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
		return ErrNodeFileCopyingNotPermitted
	}
	// Check if the user's RBAC role allows remote file operations.
	if !c.Identity.AccessChecker.CanCopyFiles() {
		return errRoleFileCopyingNotPermitted
	}

	return nil
}

// CheckSFTPAllowed returns an error if remote file operations via SCP
// or SFTP are not allowed by the user's role or the node's config, or
// if the user is not allowed to start unattended sessions.
func (c *ServerContext) CheckSFTPAllowed() error {
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
	if !canStart {
		return errCannotStartUnattendedSession
	}

	return nil
}

// OpenXServerListener opens a new XServer unix listener.
func (c *ServerContext) OpenXServerListener(x11Req x11.ForwardRequestPayload, displayOffset, maxDisplays int) error {
	l, display, err := x11.OpenNewXServerListener(displayOffset, maxDisplays, x11Req.ScreenNumber)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.setX11Config(&X11Config{
		XServerUnixSocket: l.Addr().String(),
		XAuthEntry: x11.XAuthEntry{
			Display: display,
			Proto:   x11Req.AuthProtocol,
			Cookie:  x11Req.AuthCookie,
		},
	})
	if err != nil {
		l.Close()
		return trace.Wrap(err)
	}

	c.AddCloser(l)

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
				// listener is closed
				return
			}

			go func() {
				defer xconn.Close()

				// If the session has not signaled that X11 forwarding is
				// fully set up yet, then ignore any incoming connections.
				// The client's session hasn't been fully set up yet so this
				// could potentially be a break-in attempt.
				if ok, err := c.x11Ready(); err != nil {
					c.Logger.WithError(err).Debug("Failed to get X11 ready status")
					return
				} else if !ok {
					c.Logger.WithError(err).Debug("Rejecting X11 request, XServer Proxy is not ready")
					return
				}

				xchan, sin, err := c.ServerConn.OpenChannel(sshutils.X11ChannelRequest, x11ChannelReqPayload)
				if err != nil {
					c.Logger.WithError(err).Debug("Failed to open a new X11 channel")
					return
				}
				defer xchan.Close()

				// Forward ssh requests on the X11 channels until X11 forwarding is complete
				ctx, cancel := context.WithCancel(c.cancelContext)
				defer cancel()

				go func() {
					err := sshutils.ForwardRequests(ctx, sin, c.RemoteSession)
					if err != nil {
						c.Logger.WithError(err).Debug("Failed to forward ssh request from server during X11 forwarding")
					}
				}()

				if err := x11.Forward(ctx, xconn, xchan); err != nil {
					c.Logger.WithError(err).Debug("Encountered error during X11 forwarding")
				}
			}()

			if x11Req.SingleConnection {
				l.Close()
				return
			}
		}
	}()

	return nil
}

// getX11Config gets the x11 config for this server session.
func (c *ServerContext) getX11Config() X11Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.x11Config != nil {
		return *c.x11Config
	}
	return X11Config{}
}

// setX11Config sets X11 config for the session, or returns an error if already set.
func (c *ServerContext) setX11Config(cfg *X11Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.x11Config != nil {
		return trace.AlreadyExists("X11 forwarding is already set up for this session")
	}

	c.x11Config = cfg
	return nil
}

// x11Ready returns whether the X11 unix listener is ready to accept connections.
func (c *ServerContext) x11Ready() (bool, error) {
	// Wait for child process to send signal (1 byte)
	// or EOF if signal was already received.
	_, err := io.ReadFull(c.x11rdyr, make([]byte, 1))
	if err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, trace.Wrap(err)
	}

	// signal received, close writer so future calls read EOF.
	if err := c.x11rdyw.Close(); err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
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
	// Never emit session data events for the proxy or from a Teleport node if
	// sessions are being recorded at the proxy (this would result in double
	// events).
	if c.GetServer().Component() == teleport.ComponentProxy {
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
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        c.GetServer().HostUUID(),
			ServerNamespace: c.GetServer().GetNamespace(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: string(c.SessionID()),
			WithMFA:   c.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified],
		},
		UserMetadata: c.Identity.GetUserMetadata(),
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
		c.WithError(err).Warn("Failed to emit session data event.")
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
func (c *ServerContext) SendExecResult(r ExecResult) {
	select {
	case c.ExecResultCh <- r:
	default:
		c.Infof("Blocked on sending exec result %v.", r)
	}
}

// SendSubsystemResult sends the result of running the subsystem over the
// SubsystemResultCh.
func (c *ServerContext) SendSubsystemResult(r SubsystemResult) {
	select {
	case c.SubsystemResultCh <- r:
	default:
		c.Info("Blocked on sending subsystem result.")
	}
}

func (c *ServerContext) String() string {
	return fmt.Sprintf("ServerContext(%v->%v, user=%v, id=%v)", c.ServerConn.RemoteAddr(), c.ServerConn.LocalAddr(), c.ServerConn.User(), c.id)
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
		traits, err := services.ExtractTraitsFromCert(c.Identity.Certificate)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for key, value := range localPAMConfig.Environment {
			expr, err := parse.NewExpression(value)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if expr.Namespace() != teleport.TraitExternalPrefix && expr.Namespace() != parse.LiteralNamespace {
				return nil, trace.BadParameter("PAM environment interpolation only supports external traits, found %q", value)
			}

			result, err := expr.Interpolate(traits)
			if err != nil {
				// If the trait isn't passed by the IdP due to misconfiguration
				// we fallback to setting a value which will indicate this.
				if trace.IsNotFound(err) {
					c.Logger.Warnf("Attempted to interpolate custom PAM environment with external trait %[1]q but received SAML response does not contain claim %[1]q", expr.Name())
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
		Environment:           buildEnvironment(c),
		PAMConfig:             pamConfig,
		IsTestStub:            c.IsTestStub,
		UaccMetadata:          *uaccMetadata,
		X11Config:             c.getX11Config(),
	}, nil
}

func (id *IdentityContext) GetUserMetadata() apievents.UserMetadata {
	return apievents.UserMetadata{
		Login:          id.Login,
		User:           id.TeleportUser,
		Impersonator:   id.Impersonator,
		AccessRequests: id.ActiveRequests,
	}
}

// buildEnvironment constructs a list of environment variables from
// cluster information.
func buildEnvironment(ctx *ServerContext) []string {
	var env []string

	// gather all dynamically defined environment variables
	ctx.VisitEnv(func(key, val string) {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	})

	// Parse the local and remote addresses to build SSH_CLIENT and
	// SSH_CONNECTION environment variables.
	remoteHost, remotePort, err := net.SplitHostPort(ctx.ServerConn.RemoteAddr().String())
	if err != nil {
		ctx.Logger.Debugf("Failed to split remote address: %v.", err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.ServerConn.LocalAddr().String())
		if err != nil {
			ctx.Logger.Debugf("Failed to split local address: %v.", err)
		} else {
			env = append(env,
				fmt.Sprintf("SSH_CLIENT=%s %s %s", remoteHost, remotePort, localPort),
				fmt.Sprintf("SSH_CONNECTION=%s %s %s %s", remoteHost, remotePort, localHost, localPort))
		}
	}

	// If a session has been created try and set TERM, SSH_TTY, and SSH_SESSION_ID.
	session := ctx.getSession()
	if session != nil {
		if session.term != nil {
			env = append(env, fmt.Sprintf("TERM=%v", session.term.GetTermType()))
			env = append(env, fmt.Sprintf("SSH_TTY=%s", session.term.TTY().Name()))
		}
		if session.id != "" {
			env = append(env, fmt.Sprintf("%s=%s", teleport.SSHSessionID, session.id))
		}
	}

	// Set some Teleport specific environment variables: SSH_TELEPORT_USER,
	// SSH_TELEPORT_HOST_UUID, and SSH_TELEPORT_CLUSTER_NAME.
	env = append(env, teleport.SSHTeleportHostUUID+"="+ctx.srv.ID())
	env = append(env, teleport.SSHTeleportClusterName+"="+ctx.ClusterName)
	env = append(env, teleport.SSHTeleportUser+"="+ctx.Identity.TeleportUser)

	return env
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
	utmpPath, wtmpPath := c.srv.GetUtmpPath()

	return &UaccMetadata{
		Hostname:   hostname,
		RemoteAddr: preparedAddr,
		UtmpPath:   utmpPath,
		WtmpPath:   wtmpPath,
	}, nil
}

// ComputeLockTargets computes lock targets inferred from a Server
// and an IdentityContext.
func ComputeLockTargets(s Server, id IdentityContext) ([]types.LockTarget, error) {
	clusterName, err := s.GetAccessPoint().GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lockTargets := []types.LockTarget{
		{User: id.TeleportUser},
		{Login: id.Login},
		{Node: s.HostUUID()},
		{Node: auth.HostFQDN(s.HostUUID(), clusterName.GetClusterName())},
		{MFADevice: id.Certificate.Extensions[teleport.CertExtensionMFAVerified]},
	}
	roles := apiutils.Deduplicate(append(id.AccessChecker.RoleNames(), id.UnmappedRoles...))
	lockTargets = append(lockTargets,
		services.RolesToLockTargets(roles)...,
	)
	lockTargets = append(lockTargets,
		services.AccessRequestsToLockTargets(id.ActiveRequests)...,
	)
	return lockTargets, nil
}

// SetRequest sets the ssh request that was issued by the client.
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

// GetRequest returns the ssh request that was issued by the client and saved on
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
