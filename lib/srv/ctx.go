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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/uacc"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
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

func init() {
	prometheus.MustRegister(serverTX)
	prometheus.MustRegister(serverRX)
}

// Server is regular or forwarding SSH server.
type Server interface {
	// Emitter allows server to emit audit events and create
	// event streams for recording sessions
	events.StreamEmitter

	// ID is the unique ID of the server.
	ID() string

	// HostUUID is the UUID of the underlying host. For the the forwarding
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

	// GetAccessPoint returns an auth.AccessPoint for this cluster.
	GetAccessPoint() auth.AccessPoint

	// GetSessionServer returns a session server.
	GetSessionServer() rsession.Service

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

	// Context returns server shutdown context
	Context() context.Context

	// GetUtmpPath returns the path of the user accounting database and log. Returns empty for system defaults.
	GetUtmpPath() (utmp, wtmp string)
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

	// Certificate is the SSH user certificate bytes marshalled in the OpenSSH
	// authorized_keys format.
	Certificate *ssh.Certificate

	// CertAuthority is the Certificate Authority that signed the Certificate.
	CertAuthority types.CertAuthority

	// RoleSet is the roles this Teleport user is associated with. RoleSet is
	// used to check RBAC permissions.
	RoleSet services.RoleSet

	// CertValidBefore is set to the expiry time of a certificate, or
	// empty, if cert does not expire
	CertValidBefore time.Time

	// RouteToCluster is derived from the certificate
	RouteToCluster string
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

	// ExecRequest is the command to be executed within this session context.
	ExecRequest Exec

	// ClusterName is the name of the cluster current user is authenticated with.
	ClusterName string

	// SessionRecordingConfig holds the session recording configuration at the
	// time this context was created.
	SessionRecordingConfig types.SessionRecordingConfig

	// RemoteClient holds a SSH client to a remote server. Only used by the
	// recording proxy.
	RemoteClient *ssh.Client

	// RemoteSession holds a SSH session to a remote server. Only used by the
	// recording proxy.
	RemoteSession *ssh.Session

	// clientLastActive records the last time there was activity from the client
	clientLastActive time.Time

	// disconnectExpiredCert is set to time when/if the certificate should
	// be disconnected, set to empty if no disconect is necessary
	disconnectExpiredCert time.Time

	// clientIdleTimeout is set to the timeout on
	// on client inactivity, set to 0 if not setup
	clientIdleTimeout time.Duration

	// cancelContext signals closure to all outstanding operations
	cancelContext context.Context

	// cancel is called whenever server context is closed
	cancel context.CancelFunc

	// termAllocated is used to track if a terminal has been allocated. This has
	// to be tracked because the terminal is set to nil after it's "taken" in the
	// session. Terminals can be allocated for both "exec" or "session" requests.
	termAllocated bool

	// request is the request that was issued by the client
	request *ssh.Request

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
	// address and port in a SSH "direct-tcpip" request. This value is only
	// populated for port forwarding requests.
	SrcAddr string

	// DstAddr is the destination address of the request. This is the host and
	// port to connect to in a "direct-tcpip" request. This value is only
	// populated for port forwarding requests.
	DstAddr string
}

// NewServerContext creates a new *ServerContext which is used to pass and
// manage resources, and an associated context.Context which is canceled when
// the ServerContext is closed.  The ctx parameter should be a child of the ctx
// associated with the scope of the parent ConnectionContext to ensure that
// cancellation of the ConnectionContext propagates to the ServerContext.
func NewServerContext(ctx context.Context, parent *sshutils.ConnectionContext, srv Server, identityContext IdentityContext) (context.Context, *ServerContext, error) {
	clusterConfig, err := srv.GetAccessPoint().GetClusterConfig()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

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
		clientIdleTimeout:      identityContext.RoleSet.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		cancelContext:          cancelContext,
		cancel:                 cancel,
	}

	disconnectExpiredCert := identityContext.RoleSet.AdjustDisconnectExpiredCert(clusterConfig.GetDisconnectExpiredCert())
	if !identityContext.CertValidBefore.IsZero() && disconnectExpiredCert {
		child.disconnectExpiredCert = identityContext.CertValidBefore
	}

	fields := log.Fields{
		"local":        child.ServerConn.LocalAddr(),
		"remote":       child.ServerConn.RemoteAddr(),
		"login":        child.Identity.Login,
		"teleportUser": child.Identity.TeleportUser,
		"id":           child.id,
	}
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

	if !child.disconnectExpiredCert.IsZero() || child.clientIdleTimeout != 0 {
		mon, err := NewMonitor(MonitorConfig{
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
		})
		if err != nil {
			child.Close()
			return nil, nil, trace.Wrap(err)
		}
		go mon.Start()
	}

	// Create pipe used to send command to child process.
	child.cmdr, child.cmdw, err = os.Pipe()
	if err != nil {
		child.Close()
		return nil, nil, trace.Wrap(err)
	}
	child.AddCloser(child.cmdr)
	child.AddCloser(child.cmdw)

	// Create pipe used to signal continue to child process.
	child.contr, child.contw, err = os.Pipe()
	if err != nil {
		child.Close()
		return nil, nil, trace.Wrap(err)
	}
	child.AddCloser(child.contr)
	child.AddCloser(child.contw)

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
	_, err := rsession.ParseID(ssid)
	if err != nil {
		return trace.BadParameter("invalid session id")
	}

	findSession := func() (*session, bool) {
		reg.mu.Lock()
		defer reg.mu.Unlock()
		return reg.findSessionLocked(rsession.ID(ssid))
	}

	// update ctx with a session ID
	c.session, _ = findSession()
	if c.session == nil {
		log.Debugf("Will create new session for SSH connection %v.", c.ServerConn.RemoteAddr())
	} else {
		log.Debugf("Will join session %v for SSH connection %v.", c.session.id, c.ServerConn.RemoteAddr())
	}

	return nil
}

// TrackActivity keeps track of all activity on ssh.Channel. The caller should
// use the returned ssh.Channel instead of the original one.
func (c *ServerContext) TrackActivity(ch ssh.Channel) ssh.Channel {
	return newTrackingChannel(ch, c)
}

// GetClientLastActive returns time when client was last active
func (c *ServerContext) GetClientLastActive() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientLastActive
}

// UpdateClientActivity sets last recorded client activity associated with this context
// either channel or session
func (c *ServerContext) UpdateClientActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientLastActive = c.srv.GetClock().Now().UTC()
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
	sessionDataEvent := &events.SessionData{
		Metadata: events.Metadata{
			Index: events.SessionDataIndex,
			Type:  events.SessionDataEvent,
			Code:  events.SessionDataCode,
		},
		ServerMetadata: events.ServerMetadata{
			ServerID:        c.GetServer().HostUUID(),
			ServerNamespace: c.GetServer().GetNamespace(),
		},
		SessionMetadata: events.SessionMetadata{
			SessionID: string(c.SessionID()),
			WithMFA:   c.Identity.Certificate.Extensions[teleport.CertExtensionMFAVerified],
		},
		UserMetadata: events.UserMetadata{
			User:         c.Identity.TeleportUser,
			Login:        c.Identity.Login,
			Impersonator: c.Identity.Impersonator,
		},
		ConnectionMetadata: events.ConnectionMetadata{
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

// ProxyPublicAddress tries to get the public address from the first
// available proxy. if public_address is not set, fall back to the hostname
// of the first proxy we get back.
func (c *ServerContext) ProxyPublicAddress() string {
	proxyHost := "<proxyhost>:3080"

	if c.srv == nil {
		return proxyHost
	}

	proxies, err := c.srv.GetAccessPoint().GetProxies()
	if err != nil {
		c.Errorf("Unable to retrieve proxy list: %v", err)
	}

	if len(proxies) > 0 {
		proxyHost = proxies[0].GetPublicAddr()
		if proxyHost == "" {
			proxyHost = fmt.Sprintf("%v:%v", proxies[0].GetHostname(), defaults.HTTPListenPort)
			c.Debugf("public_address not set for proxy, returning proxyHost: %q", proxyHost)
		}
	}

	return proxyHost
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
	var roleNames []string
	if len(c.Identity.RoleSet) > 0 {
		roleNames = c.Identity.RoleSet.RoleNames()
	}

	// Fill in the environment variables from the config and interpolate them if needed.
	environment := make(map[string]string)
	environment["TELEPORT_USERNAME"] = c.Identity.TeleportUser
	environment["TELEPORT_LOGIN"] = c.Identity.Login
	environment["TELEPORT_ROLES"] = strings.Join(roleNames, " ")
	if localPAMConfig.Environment != nil {
		user, err := c.srv.GetAccessPoint().GetUser(c.Identity.TeleportUser, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		traits := user.GetTraits()

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
					log.Warnf("Attempted to interpolate custom PAM environment with external trait %[1]q but received SAML response does not contain claim %[1]q", expr.Name())
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
	var roleNames []string
	if len(c.Identity.RoleSet) > 0 {
		roleNames = c.Identity.RoleSet.RoleNames()
	}

	// Extract the command to be executed. This only exists if command execution
	// (exec or shell) is being requested, port forwarding has no command to
	// execute.
	var command string
	if c.ExecRequest != nil {
		command = c.ExecRequest.GetCommand()
	}

	// Extract the request type. This only exists for command execution (exec
	// or shell), port forwarding requests have no request type.
	var requestType string
	if c.request != nil {
		requestType = c.request.Type
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
		RequestType:           requestType,
		PermitUserEnvironment: c.srv.PermitUserEnvironment(),
		Environment:           buildEnvironment(c),
		PAMConfig:             pamConfig,
		IsTestStub:            c.IsTestStub,
		UaccMetadata:          *uaccMetadata,
	}, nil
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
		log.Debugf("Failed to split remote address: %v.", err)
	} else {
		localHost, localPort, err := net.SplitHostPort(ctx.ServerConn.LocalAddr().String())
		if err != nil {
			log.Debugf("Failed to split local address: %v.", err)
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
	// SSH_SESSION_WEBPROXY_ADDR, SSH_TELEPORT_HOST_UUID, and
	// SSH_TELEPORT_CLUSTER_NAME.
	env = append(env, teleport.SSHSessionWebproxyAddr+"="+ctx.ProxyPublicAddress())
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
