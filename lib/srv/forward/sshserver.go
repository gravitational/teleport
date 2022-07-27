/*
Copyright 2017 Gravitational, Inc.

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

package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Server is a forwarding server. Server is used to create a single in-memory
// SSH server that will forward connections to a remote server. It's used along
// with the recording proxy to allow Teleport to record sessions with OpenSSH
// nodes at the proxy level.
//
// To create a forwarding server and serve a single SSH connection on it:
//
//   serverConfig := forward.ServerConfig{
//      ...
//   }
//   remoteServer, err := forward.New(serverConfig)
//   if err != nil {
//   	return nil, trace.Wrap(err)
//   }
//   go remoteServer.Serve()
//
//   conn, err := remoteServer.Dial()
//   if err != nil {
//   	return nil, trace.Wrap(err)
//   }
type Server struct {
	log *logrus.Entry

	id string

	// targetConn is the TCP connection to the remote host.
	targetConn net.Conn

	// clientConn is the client half of the pipe used to connect the client
	// and server.
	clientConn net.Conn

	// serverConn is the server half of the pipe used to connect the client and
	// server.
	serverConn net.Conn

	// sconn is an authenticated SSH connection from the server perspective.
	sconn *ssh.ServerConn

	// remoteClient exposes an API to SSH functionality like shells, port
	// forwarding, subsystems.
	remoteClient *tracessh.Client

	// connectionContext is used to construct ServerContext instances
	// and supports registration of connection-scoped resource closers.
	connectionContext *sshutils.ConnectionContext

	// identityContext holds identity information about the user that has
	// authenticated on sconn (like system login, Teleport username, roles).
	identityContext srv.IdentityContext

	// userAgent is the SSH user agent that was forwarded to the proxy.
	userAgent teleagent.Agent

	// hostCertificate is the SSH host certificate this in-memory server presents
	// to the client.
	hostCertificate ssh.Signer

	// StreamEmitter points to the auth service and emits audit events
	events.StreamEmitter

	// authHandlers are common authorization and authentication handlers shared
	// by the regular and forwarding server.
	authHandlers *srv.AuthHandlers
	// termHandlers are common terminal handlers shared by the regular and
	// forwarding server.
	termHandlers *srv.TermHandlers

	// useTunnel indicates of this server is connected over a reverse tunnel.
	useTunnel bool

	// address is the name of the host certificate.
	address string

	// ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	ciphers []string
	// kexAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	kexAlgorithms []string
	// macAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	macAlgorithms []string

	authClient      auth.ClientI
	authService     srv.AccessPoint
	sessionRegistry *srv.SessionRegistry
	sessionServer   session.Service
	dataDir         string

	clock clockwork.Clock

	// hostUUID is the UUID of the underlying proxy that the forwarding server
	// is running in.
	hostUUID string

	// closeContext and closeCancel are used to signal to the outside
	// world that this server is closed
	closeContext context.Context
	closeCancel  context.CancelFunc

	// parentContext is used to signal server closure
	parentContext context.Context

	// lockWatcher is the server's lock watcher.
	lockWatcher *services.LockWatcher
}

// ServerConfig is the configuration needed to create an instance of a Server.
type ServerConfig struct {
	AuthClient      auth.ClientI
	UserAgent       teleagent.Agent
	TargetConn      net.Conn
	SrcAddr         net.Addr
	DstAddr         net.Addr
	HostCertificate ssh.Signer

	// UseTunnel indicates of this server is connected over a reverse tunnel.
	UseTunnel bool

	// Address is the name of the host certificate.
	Address string

	// Ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string

	// KEXAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string

	// MACAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string

	// DataDir is a local data directory used for local server storage
	DataDir string

	// Clock is an optoinal clock to override default real time clock
	Clock clockwork.Clock

	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// HostUUID is the UUID of the underlying proxy that the forwarding server
	// is running in.
	HostUUID string

	// Emitter is audit events emitter
	Emitter events.StreamEmitter

	// ParentContext is a parent context, used to signal global
	// closure
	ParentContext context.Context

	// LockWatcher is a lock watcher.
	LockWatcher *services.LockWatcher
}

// CheckDefaults makes sure all required parameters are passed in.
func (s *ServerConfig) CheckDefaults() error {
	if s.AuthClient == nil {
		return trace.BadParameter("auth client required")
	}
	if s.DataDir == "" {
		return trace.BadParameter("missing parameter DataDir")
	}
	if s.UserAgent == nil {
		return trace.BadParameter("user agent required to connect to remote host")
	}
	if s.TargetConn == nil {
		return trace.BadParameter("connection to target connection required")
	}
	if s.SrcAddr == nil {
		return trace.BadParameter("source address required to identify client")
	}
	if s.DstAddr == nil {
		return trace.BadParameter("destination address required to connect to remote host")
	}
	if s.HostCertificate == nil {
		return trace.BadParameter("host certificate required to act on behalf of remote host")
	}
	if s.Clock == nil {
		s.Clock = clockwork.NewRealClock()
	}
	if s.Emitter == nil {
		return trace.BadParameter("missing parameter Emitter")
	}
	if s.ParentContext == nil {
		s.ParentContext = context.TODO()
	}
	if s.LockWatcher == nil {
		return trace.BadParameter("missing parameter LockWatcher")
	}
	return nil
}

// New creates a new unstarted Server.
func New(c ServerConfig) (*Server, error) {
	// Check and make sure we everything we need to build a forwarding node.
	err := c.CheckDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build a pipe connection to hook up the client and the server. we save both
	// here and will pass them along to the context when we create it so they
	// can be closed by the context.
	serverConn, clientConn := utils.DualPipeNetConn(c.SrcAddr, c.DstAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentForwardingNode,
			trace.ComponentFields: map[string]string{
				"src-addr": c.SrcAddr.String(),
				"dst-addr": c.DstAddr.String(),
			},
		}),
		id:              uuid.New().String(),
		targetConn:      c.TargetConn,
		serverConn:      utils.NewTrackingConn(serverConn),
		clientConn:      clientConn,
		userAgent:       c.UserAgent,
		hostCertificate: c.HostCertificate,
		useTunnel:       c.UseTunnel,
		address:         c.Address,
		authClient:      c.AuthClient,
		authService:     c.AuthClient,
		sessionServer:   c.AuthClient,
		dataDir:         c.DataDir,
		clock:           c.Clock,
		hostUUID:        c.HostUUID,
		StreamEmitter:   c.Emitter,
		parentContext:   c.ParentContext,
		lockWatcher:     c.LockWatcher,
	}

	// Set the ciphers, KEX, and MACs that the in-memory server will send to the
	// client in its SSH_MSG_KEXINIT.
	s.ciphers = c.Ciphers
	s.kexAlgorithms = c.KEXAlgorithms
	s.macAlgorithms = c.MACAlgorithms

	s.sessionRegistry, err = srv.NewSessionRegistry(srv.SessionRegistryConfig{
		Srv:                   s,
		SessionTrackerService: s.authClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Common auth handlers.
	authHandlerConfig := srv.AuthHandlerConfig{
		Server:      s,
		Component:   teleport.ComponentForwardingNode,
		Emitter:     c.Emitter,
		AccessPoint: c.AuthClient,
		FIPS:        c.FIPS,
		Clock:       c.Clock,
	}

	s.authHandlers, err = srv.NewAuthHandlers(&authHandlerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Common term handlers.
	s.termHandlers = &srv.TermHandlers{
		SessionRegistry: s.sessionRegistry,
	}

	// Create a close context that is used internally to signal when the server
	// is closing and for any blocking goroutines to unblock.
	s.closeContext, s.closeCancel = context.WithCancel(c.ParentContext)

	return s, nil
}

// Context returns parent context, used to signal
// that parent server has been closed
func (s *Server) Context() context.Context {
	return s.parentContext
}

// GetDataDir returns server local storage
func (s *Server) GetDataDir() string {
	return s.dataDir
}

// ID returns the ID of the proxy that creates the in-memory forwarding server.
func (s *Server) ID() string {
	return s.id
}

// HostUUID is the UUID of the underlying proxy that the forwarding server
// is running in.
func (s *Server) HostUUID() string {
	return s.hostUUID
}

// GetNamespace returns the namespace the forwarding server resides in.
func (s *Server) GetNamespace() string {
	return apidefaults.Namespace
}

// AdvertiseAddr is the address of the remote host this forwarding server is
// connected to.
func (s *Server) AdvertiseAddr() string {
	return s.clientConn.RemoteAddr().String()
}

// Component is the type of node this server is.
func (s *Server) Component() string {
	return teleport.ComponentForwardingNode
}

// PermitUserEnvironment is always false because it's up the the remote host
// to decide if the user environment will be read or not.
func (s *Server) PermitUserEnvironment() bool {
	return false
}

// GetAccessPoint returns a srv.AccessPoint for this cluster.
func (s *Server) GetAccessPoint() srv.AccessPoint {
	return s.authService
}

// GetSessionServer returns a session server.
func (s *Server) GetSessionServer() session.Service {
	return s.sessionServer
}

// GetPAM returns the PAM configuration for a server. Because the forwarding
// server runs in-memory, it does not support PAM.
func (s *Server) GetPAM() (*pam.Config, error) {
	return nil, trace.BadParameter("PAM not supported by forwarding server")
}

// UseTunnel used to determine if this node has connected to this cluster
// using reverse tunnel.
func (s *Server) UseTunnel() bool {
	return s.useTunnel
}

// GetBPF returns the BPF service used by enhanced session recording. BPF
// for the forwarding server makes no sense (it has to run on the actual
// node), so return a NOP implementation.
func (s Server) GetBPF() bpf.BPF {
	return &bpf.NOP{}
}

// GetCreateHostUser determines whether users should be created on the
// host automatically
func (s *Server) GetCreateHostUser() bool {
	return false
}

// GetHostUser returns the HostUsers instance being used to manage
// host user provisioning, unimplemented for the forwarder server.
func (s *Server) GetHostUsers() srv.HostUsers {
	return nil
}

// GetRestrictedSessionManager returns a NOP manager since for a
// forwarding server it makes no sense (it has to run on the actual
// node).
func (s Server) GetRestrictedSessionManager() restricted.Manager {
	return &restricted.NOP{}
}

// GetInfo returns a services.Server that represents this server.
func (s *Server) GetInfo() types.Server {
	return &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      s.ID(),
			Namespace: s.GetNamespace(),
		},
		Spec: types.ServerSpecV2{
			Addr: s.AdvertiseAddr(),
		},
	}
}

// Dial returns the client connection created by pipeAddrConn.
func (s *Server) Dial() (net.Conn, error) {
	return s.clientConn, nil
}

// GetClock returns server clock implementation
func (s *Server) GetClock() clockwork.Clock {
	return s.clock
}

// GetUtmpPath returns the optional override of the utmp and wtmp path.
// These values are never set for the forwarding server because utmp and wtmp
// are updated by the target server and not the forwarding server.
func (s *Server) GetUtmpPath() (string, string) {
	return "", ""
}

// GetLockWatcher gets the server's lock watcher.
func (s *Server) GetLockWatcher() *services.LockWatcher {
	return s.lockWatcher
}

func (s *Server) Serve() {
	config := &ssh.ServerConfig{}

	// Configure callback for user certificate authentication.
	config.PublicKeyCallback = s.authHandlers.UserKeyAuth

	// Set host certificate the in-memory server will present to clients.
	config.AddHostKey(s.hostCertificate)

	// Set the ciphers, KEX, and MACs that the client will send to the target
	// server in its SSH_MSG_KEXINIT.
	config.Ciphers = s.ciphers
	config.KeyExchanges = s.kexAlgorithms
	config.MACs = s.macAlgorithms

	netConfig, err := s.GetAccessPoint().GetClusterNetworkingConfig(s.Context())
	if err != nil {
		s.log.Errorf("Unable to fetch cluster config: %v.", err)
		return
	}

	s.log.Debugf("Supported ciphers: %q.", s.ciphers)
	s.log.Debugf("Supported KEX algorithms: %q.", s.kexAlgorithms)
	s.log.Debugf("Supported MAC algorithms: %q.", s.macAlgorithms)

	sconn, chans, reqs, err := ssh.NewServerConn(s.serverConn, config)
	if err != nil {
		s.userAgent.Close()
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create server connection: %v.", err)
		return
	}
	s.sconn = sconn

	ctx := context.Background()
	ctx, s.connectionContext = sshutils.NewConnectionContext(ctx, s.serverConn, s.sconn)

	// Take connection and extract identity information for the user from it.
	s.identityContext, err = s.authHandlers.CreateIdentityContext(sconn)
	if err != nil {
		s.userAgent.Close()
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create server connection: %v.", err)
		return
	}

	// Connect and authenticate to the remote node.
	s.log.Debugf("Creating remote connection to %v@%v", sconn.User(), s.clientConn.RemoteAddr().String())
	s.remoteClient, err = s.newRemoteClient(ctx, sconn.User())
	if err != nil {
		// Reject the connection with an error so the client doesn't hang then
		// close the connection.
		s.rejectChannel(chans, err.Error())
		sconn.Close()

		s.userAgent.Close()
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create remote connection: %v", err)
		return
	}

	// The keep-alive loop will keep pinging the remote server and after it has
	// missed a certain number of keep-alive requests it will cancel the
	// closeContext which signals the server to shutdown.
	go srv.StartKeepAliveLoop(srv.KeepAliveParams{
		Conns: []srv.RequestSender{
			s.sconn,
			s.remoteClient,
		},
		Interval:     netConfig.GetKeepAliveInterval(),
		MaxCount:     netConfig.GetKeepAliveCountMax(),
		CloseContext: ctx,
		CloseCancel:  func() { s.connectionContext.Close() },
	})

	go s.handleConnection(ctx, chans, reqs)
}

// Close will close all underlying connections that the forwarding server holds.
func (s *Server) Close() error {
	conns := []io.Closer{
		s.userAgent,
		s.sconn,
		s.clientConn,
		s.serverConn,
		s.targetConn,
		s.remoteClient,
		s.connectionContext,
	}

	var errs []error

	for _, c := range conns {
		if c == nil {
			continue
		}

		err := c.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	// Signal to the outside world that this server is closed
	s.closeCancel()

	return trace.NewAggregate(errs...)
}

// newRemoteSession will create and return a *ssh.Client and *ssh.Session
// with a remote host.
func (s *Server) newRemoteClient(ctx context.Context, systemLogin string) (*tracessh.Client, error) {
	// the proxy will use the agent that has been forwarded to it as the auth
	// method when connecting to the remote host
	if s.userAgent == nil {
		return nil, trace.AccessDenied("agent must be forwarded to proxy")
	}
	authMethod := ssh.PublicKeysCallback(s.userAgent.Signers)

	clientConfig := &ssh.ClientConfig{
		User: systemLogin,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: s.authHandlers.HostKeyAuth,
		Timeout:         apidefaults.DefaultDialTimeout,
	}

	// Ciphers, KEX, and MACs preferences are honored by both the in-memory
	// server as well as the client in the connection to the target node.
	clientConfig.Ciphers = s.ciphers
	clientConfig.KeyExchanges = s.kexAlgorithms
	clientConfig.MACs = s.macAlgorithms

	// Destination address is used to validate a connection was established to
	// the correct host. It must occur in the list of principals presented by
	// the remote server.
	dstAddr := net.JoinHostPort(s.address, "0")
	client, err := tracessh.NewClientConnWithDeadline(ctx, s.targetConn, dstAddr, clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (s *Server) handleConnection(ctx context.Context, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	defer s.log.Debugf("Closing forwarding server connected to %v and releasing resources.", s.sconn.LocalAddr())
	defer s.Close()

	for {
		select {
		// Global out-of-band requests.
		case newRequest := <-reqs:
			if newRequest == nil {
				return
			}
			go s.handleGlobalRequest(newRequest)
		// Channel requests.
		case newChannel := <-chans:
			if newChannel == nil {
				return
			}
			go s.handleChannel(ctx, newChannel)
		// If the server is closing (either the heartbeat failed or Close() was
		// called, exit out of the connection handler loop.
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) rejectChannel(chans <-chan ssh.NewChannel, errMessage string) {
	newChannel, ok := <-chans
	if !ok {
		return
	}
	if err := newChannel.Reject(ssh.ConnectionFailed, errMessage); err != nil {
		s.log.Errorf("Unable to reject and close connection.")
	}
}

func (s *Server) handleGlobalRequest(req *ssh.Request) {
	// Version requests are internal Teleport requests, they should not be
	// forwarded to the remote server.
	if req.Type == teleport.VersionRequest {
		err := req.Reply(true, []byte(teleport.Version))
		if err != nil {
			s.log.Debugf("Failed to reply to version request: %v.", err)
		}
		return
	}

	ok, payload, err := s.remoteClient.SendRequest(req.Type, req.WantReply, req.Payload)
	if err != nil {
		s.log.Warnf("Failed to forward global request %v: %v", req.Type, err)
		return
	}

	if req.WantReply {
		err = req.Reply(ok, payload)
		if err != nil {
			s.log.Warnf("Failed to reply to global request: %v: %v", req.Type, err)
		}
	}
}

func (s *Server) handleChannel(ctx context.Context, nch ssh.NewChannel) {
	channelType := nch.ChannelType()

	switch channelType {
	// Channels of type "session" handle requests that are involved in running
	// commands on a server, subsystem requests, and agent forwarding.
	case teleport.ChanSession:
		go s.handleSessionChannel(ctx, nch)

	// Channels of type "direct-tcpip" handles request for port forwarding.
	case teleport.ChanDirectTCPIP:
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			s.log.Errorf("Failed to parse request data: %v, err: %v", string(nch.ExtraData()), err)
			if err := nch.Reject(ssh.UnknownChannelType, "failed to parse direct-tcpip request"); err != nil {
				s.log.Warnf("Failed to reject channel: %v", err)
			}
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			s.log.Warnf("Unable to accept channel: %v", err)
			if err := nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err)); err != nil {
				s.log.Warnf("Failed to reject channel: %v", err)
			}
			return
		}
		go s.handleDirectTCPIPRequest(ctx, ch, req)
	default:
		if err := nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType)); err != nil {
			s.log.Warnf("Failed to reject channel of unknown type: %v", err)
		}
	}
}

// handleDirectTCPIPRequest handles port forwarding requests.
func (s *Server) handleDirectTCPIPRequest(ctx context.Context, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	// Create context for this channel. This context will be closed when
	// forwarding is complete.
	ctx, scx, err := srv.NewServerContext(ctx, s.connectionContext, s, s.identityContext)
	if err != nil {
		s.log.Errorf("Unable to create connection context: %v.", err)
		s.stderrWrite(ch, "Unable to create connection context.")
		return
	}
	scx.RemoteClient = s.remoteClient
	scx.ChannelType = teleport.ChanDirectTCPIP
	scx.SrcAddr = fmt.Sprintf("%v:%d", req.Orig, req.OrigPort)
	scx.DstAddr = fmt.Sprintf("%v:%d", req.Host, req.Port)
	defer scx.Close()

	ch = scx.TrackActivity(ch)

	// Check if the role allows port forwarding for this user.
	err = s.authHandlers.CheckPortForward(scx.DstAddr, scx)
	if err != nil {
		s.stderrWrite(ch, err.Error())
		return
	}

	s.log.Debugf("Opening direct-tcpip channel from %v to %v in context %v.", scx.SrcAddr, scx.DstAddr, scx.ID())
	defer s.log.Debugf("Completing direct-tcpip request from %v to %v in context %v.", scx.SrcAddr, scx.DstAddr, scx.ID())

	// Create "direct-tcpip" channel from the remote host to the target host.
	conn, err := s.remoteClient.Dial("tcp", scx.DstAddr)
	if err != nil {
		scx.Infof("Failed to connect to: %v: %v", scx.DstAddr, err)
		return
	}
	defer conn.Close()

	if err := s.EmitAuditEvent(s.closeContext, &apievents.PortForward{
		Metadata: apievents.Metadata{
			Type: events.PortForwardEvent,
			Code: events.PortForwardCode,
		},
		UserMetadata: s.identityContext.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  s.sconn.LocalAddr().String(),
			RemoteAddr: s.sconn.RemoteAddr().String(),
		},
		Addr: scx.DstAddr,
		Status: apievents.Status{
			Success: true,
		},
	}); err != nil {
		scx.WithError(err).Warn("Failed to emit port forward event.")
	}

	var wg sync.WaitGroup
	wch := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(ch, conn); err != nil {
			scx.Warningf("failed proxying data for port forwarding connection: %v", err)
		}
		ch.Close()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, ch); err != nil {
			scx.Warningf("failed proxying data for port forwarding connection: %v", err)
		}
		conn.Close()
	}()
	// block on wg in separate goroutine so that we
	// can select on wg and context cancellation.
	go func() {
		defer close(wch)
		wg.Wait()
	}()
	select {
	case <-wch:
	case <-ctx.Done():
	}
}

// handleSessionChannel handles accepting and forwarding a session channel from the client to
// the remote host. Once the session channel has been established, this function's loop handles
// all the "exec", "subsystem" and "shell" requests.
func (s *Server) handleSessionChannel(ctx context.Context, nch ssh.NewChannel) {
	// Create context for this channel. This context will be closed when the
	// session request is complete.
	// There is no need for the forwarding server to initiate disconnects,
	// based on teleport business logic, because this logic is already
	// done on the server's terminating side.
	ctx, scx, err := srv.NewServerContext(ctx, s.connectionContext, s, s.identityContext)
	if err != nil {
		s.log.Warnf("Server context setup failed: %v", err)
		if err := nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("server context setup failed: %v", err)); err != nil {
			s.log.Warnf("Failed to reject channel: %v", err)
		}
		return
	}

	scx.RemoteClient = s.remoteClient
	scx.ChannelType = teleport.ChanSession
	defer scx.Close()

	// Create a "session" channel on the remote host.  Note that we
	// create the remote session channel before accepting the local
	// channel request; this allows us to propagate the rejection
	// reason/message in the event the channel is rejected.
	remoteSession, err := s.remoteClient.NewSession(ctx)
	if err != nil {
		s.log.Warnf("Remote session open failed: %v", err)
		reason, msg := ssh.ConnectionFailed, fmt.Sprintf("remote session open failed: %v", err)
		if e, ok := trace.Unwrap(err).(*ssh.OpenChannelError); ok {
			reason, msg = e.Reason, e.Message
		}
		if err := nch.Reject(reason, msg); err != nil {
			s.log.Warnf("Failed to reject channel: %v", err)
		}
		return
	}
	scx.RemoteSession = remoteSession

	// Accept the session channel request
	ch, in, err := nch.Accept()
	if err != nil {
		s.log.Warnf("Unable to accept channel: %v", err)
		if err := nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err)); err != nil {
			s.log.Warnf("Failed to reject channel: %v", err)
		}
		return
	}
	scx.AddCloser(ch)

	ch = scx.TrackActivity(ch)

	s.log.Debugf("Opening session request to %v in context %v.", s.sconn.RemoteAddr(), scx.ID())
	defer s.log.Debugf("Closing session request to %v in context %v.", s.sconn.RemoteAddr(), scx.ID())

	for {
		// Update the context with the session ID.
		err := scx.CreateOrJoinSession(s.sessionRegistry)
		if err != nil {
			errorMessage := fmt.Sprintf("unable to update context: %v", err)
			scx.Errorf("%v", errorMessage)

			// Write the error to channel and close it.
			s.stderrWrite(ch, errorMessage)
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: teleport.RemoteCommandFailure}))
			if err != nil {
				scx.Errorf("Failed to send exit status %v", errorMessage)
			}
			return
		}

		select {
		case result := <-scx.SubsystemResultCh:
			// Subsystem has finished executing, close the channel and session.
			scx.Debugf("Subsystem execution result: %v", result.Err)
			return
		case req := <-in:
			if req == nil {
				// The client has closed or dropped the connection.
				scx.Debugf("Client %v disconnected", s.sconn.RemoteAddr())
				return
			}
			if err := s.dispatch(ctx, ch, req, scx); err != nil {
				s.replyError(ch, req, err)
				return
			}
			if req.WantReply {
				if err := req.Reply(true, nil); err != nil {
					scx.Errorf("failed sending OK response on %q request: %v", req.Type, err)
				}
			}
		case result := <-scx.ExecResultCh:
			scx.Debugf("Exec request (%q) complete: %v", result.Command, result.Code)

			// The exec process has finished and delivered the execution result, send
			// the result back to the client, and close the session and channel.
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(result.Code)}))
			if err != nil {
				scx.Infof("Failed to send exit status for %v: %v", result.Command, err)
			}

			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) dispatch(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *srv.ServerContext) error {
	scx.Debugf("Handling request %v, want reply %v.", req.Type, req.WantReply)

	// Certs with a join-only principal can only use a
	// subset of all the possible request types.
	if scx.JoinOnly {
		switch req.Type {
		case tracessh.TracingRequest:
			return s.handleTracingRequest(req, scx)
		case sshutils.PTYRequest:
			return s.termHandlers.HandlePTYReq(ch, req, scx)
		case sshutils.ShellRequest:
			return s.termHandlers.HandleShell(ctx, ch, req, scx)
		case sshutils.WindowChangeRequest:
			return s.termHandlers.HandleWinChange(ch, req, scx)
		case teleport.ForceTerminateRequest:
			return s.termHandlers.HandleForceTerminate(ch, req, scx)
		case sshutils.EnvRequest:
			// We ignore all SSH setenv requests for join-only principals.
			// SSH will send them anyway but it seems fine to silently drop them.
		case sshutils.SubsystemRequest:
			return s.handleSubsystem(ctx, ch, req, scx)
		case sshutils.AgentForwardRequest:
			// to maintain interoperability with OpenSSH, agent forwarding requests
			// should never fail, all errors should be logged and we should continue
			// processing requests.
			err := s.handleAgentForward(ch, req, scx)
			if err != nil {
				s.log.Debug(err)
			}
			return nil
		default:
			return trace.AccessDenied("attempted %v request in join-only mode", req.Type)
		}
	}

	switch req.Type {
	case tracessh.TracingRequest:
		return s.handleTracingRequest(req, scx)
	case sshutils.ExecRequest:
		return s.termHandlers.HandleExec(ctx, ch, req, scx)
	case sshutils.PTYRequest:
		return s.termHandlers.HandlePTYReq(ch, req, scx)
	case sshutils.ShellRequest:
		return s.termHandlers.HandleShell(ctx, ch, req, scx)
	case sshutils.WindowChangeRequest:
		return s.termHandlers.HandleWinChange(ch, req, scx)
	case teleport.ForceTerminateRequest:
		return s.termHandlers.HandleForceTerminate(ch, req, scx)
	case sshutils.EnvRequest:
		return s.handleEnv(ch, req, scx)
	case sshutils.SubsystemRequest:
		return s.handleSubsystem(ctx, ch, req, scx)
	case sshutils.X11ForwardRequest:
		return s.handleX11Forward(ctx, ch, req, scx)
	case sshutils.AgentForwardRequest:
		// to maintain interoperability with OpenSSH, agent forwarding requests
		// should never fail, all errors should be logged and we should continue
		// processing requests.
		err := s.handleAgentForward(ch, req, scx)
		if err != nil {
			s.log.Debug(err)
		}
		return nil
	default:
		return trace.BadParameter(
			"%v doesn't support request type '%v'", s.Component(), req.Type)
	}
}

func (s *Server) handleAgentForward(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	// Check if the user's RBAC role allows agent forwarding.
	err := s.authHandlers.CheckAgentForward(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Route authentication requests to the agent that was forwarded to the proxy.
	err = agent.ForwardToAgent(ctx.RemoteClient.Client, s.userAgent)
	if err != nil {
		return trace.Wrap(err)
	}

	// Make an "auth-agent-req@openssh.com" request on the remote host.
	err = agent.RequestAgentForwarding(ctx.RemoteSession)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// handleX11ChannelRequest accepts an X11 channel and forwards it back to the client.
// Servers which support X11 forwarding request a separate channel for serving each
// inbound connection on the X11 socket of the remote session.
func (s *Server) handleX11ChannelRequest(ctx context.Context, nch ssh.NewChannel) {
	// accept inbound X11 channel from server
	sch, sin, err := nch.Accept()
	if err != nil {
		s.log.Errorf("X11 channel fwd failed: %v", err)
		return
	}
	defer sch.Close()

	// setup outbound X11 channel to client
	cch, cin, err := s.sconn.OpenChannel(sshutils.X11ChannelRequest, nch.ExtraData())
	if err != nil {
		s.log.Errorf("X11 channel fwd failed: %v", err)
		return
	}
	defer cch.Close()

	// Forward ssh requests on the X11 channels until X11 forwarding is complete
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		err := sshutils.ForwardRequests(ctx, cin, sch)
		if err != nil {
			s.log.WithError(err).Debug("Failed to forward ssh request from client during X11 forwarding")
		}
	}()

	go func() {
		err := sshutils.ForwardRequests(ctx, sin, cch)
		if err != nil {
			s.log.WithError(err).Debug("Failed to forward ssh request from server during X11 forwarding")
		}
	}()

	if err := x11.Forward(ctx, cch, sch); err != nil {
		s.log.WithError(err).Debug("Encountered error during x11 forwarding")
	}
}

// handleX11Forward handles an X11 forwarding request from the client.
func (s *Server) handleX11Forward(ctx context.Context, ch ssh.Channel, req *ssh.Request, scx *srv.ServerContext) (err error) {
	event := &apievents.X11Forward{
		Status: apievents.Status{
			Success: true,
		},
		Metadata: apievents.Metadata{
			Type: events.X11ForwardEvent,
			Code: events.X11ForwardCode,
		},
		UserMetadata: s.identityContext.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			LocalAddr:  s.sconn.LocalAddr().String(),
			RemoteAddr: s.sconn.RemoteAddr().String(),
		},
	}

	defer func() {
		if err != nil {
			event.Metadata.Code = events.X11ForwardFailureCode
			event.Status.Success = false
			event.Status.Error = err.Error()
		}
		if trace.IsAccessDenied(err) {
			// denied X11 requests are ok from a protocol perspective so we
			// don't return them, just reply over ssh and emit the audit log.
			s.replyError(ch, req, err)
			err = nil
		}
		if err := s.EmitAuditEvent(ctx, event); err != nil {
			s.log.WithError(err).Warn("Failed to emit x11-forward event.")
		}
	}()

	// Check if the user's RBAC role allows X11 forwarding.
	if err := s.authHandlers.CheckX11Forward(scx); err != nil {
		return trace.Wrap(err)
	}

	// send X11 forwarding request to remote
	ok, err := sshutils.ForwardRequest(scx.RemoteSession, req)
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.AccessDenied("X11 forwarding request denied by server")
	}

	err = x11.ServeChannelRequests(ctx, s.remoteClient.Client, s.handleX11ChannelRequest)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) handleSubsystem(ctx context.Context, ch ssh.Channel, req *ssh.Request, serverContext *srv.ServerContext) error {
	subsystem, err := parseSubsystemRequest(req, serverContext)
	if err != nil {
		return trace.Wrap(err)
	}

	// start the requested subsystem, if it fails to start return result right away
	err = subsystem.Start(ctx, ch)
	if err != nil {
		serverContext.SendSubsystemResult(srv.SubsystemResult{
			Name: subsystem.subsytemName,
			Err:  trace.Wrap(err),
		})
		return trace.Wrap(err)
	}

	// wait for the subsystem to finish and return that result
	go func() {
		err := subsystem.Wait()
		serverContext.SendSubsystemResult(srv.SubsystemResult{
			Name: subsystem.subsytemName,
			Err:  trace.Wrap(err),
		})
	}()

	return nil
}

func (s *Server) handleTracingRequest(req *ssh.Request, ctx *srv.ServerContext) error {
	if _, err := ctx.RemoteSession.SendRequest(req.Type, false, req.Payload); err != nil {
		s.log.WithError(err).Debugf("Unable to set forward tracing context")
	}

	return nil
}

func (s *Server) handleEnv(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	var e sshutils.EnvReqParams
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		ctx.Error(err)
		return trace.Wrap(err, "failed to parse env request")
	}

	err := ctx.RemoteSession.Setenv(e.Name, e.Value)
	if err != nil {
		s.log.Debugf("Unable to set environment variable: %v: %v", e.Name, e.Value)
	}

	return nil
}

func (s *Server) replyError(ch ssh.Channel, req *ssh.Request, err error) {
	s.log.Error(err)
	// Terminate the error with a newline when writing to remote channel's
	// stderr so the output does not mix with the rest of the output if the remote
	// side is not doing additional formatting for extended data.
	// See github.com/gravitational/teleport/issues/4542
	message := utils.FormatErrorWithNewline(err)
	s.stderrWrite(ch, message)
	if req.WantReply {
		if err := req.Reply(false, []byte(message)); err != nil {
			s.log.Errorf("sending error reply on SSH channel: %v", err)
		}
	}
}

func (s *Server) stderrWrite(ch ssh.Channel, message string) {
	if _, err := ch.Stderr().Write([]byte(message)); err != nil {
		s.log.Errorf("failed writing to SSH stderr channel: %v", err)
	}
}

func parseSubsystemRequest(req *ssh.Request, ctx *srv.ServerContext) (*remoteSubsystem, error) {
	var r sshutils.SubsystemReq
	err := ssh.Unmarshal(req.Payload, &r)
	if err != nil {
		return nil, trace.BadParameter("failed to parse subsystem request: %v", err)
	}

	return parseRemoteSubsystem(context.Background(), r.Name, ctx), nil
}
