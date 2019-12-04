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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
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
	remoteClient *ssh.Client

	// identityContext holds identity information about the user that has
	// authenticated on sconn (like system login, Teleport username, roles).
	identityContext srv.IdentityContext

	// userAgent is the SSH user agent that was forwarded to the proxy.
	userAgent agent.Agent

	// hostCertificate is the SSH host certificate this in-memory server presents
	// to the client.
	hostCertificate ssh.Signer

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
	auditLog        events.IAuditLog
	authService     auth.AccessPoint
	sessionRegistry *srv.SessionRegistry
	sessionServer   session.Service
	dataDir         string

	// closeContext and closeCancel are used to signal when the in-memory
	// server is closing and all blocking goroutines should unblock.
	closeContext context.Context
	closeCancel  context.CancelFunc

	clock clockwork.Clock

	// hostUUID is the UUID of the underlying proxy that the forwarding server
	// is running in.
	hostUUID string
}

// ServerConfig is the configuration needed to create an instance of a Server.
type ServerConfig struct {
	AuthClient      auth.ClientI
	UserAgent       agent.Agent
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
		id:              uuid.New(),
		targetConn:      c.TargetConn,
		serverConn:      utils.NewTrackingConn(serverConn),
		clientConn:      clientConn,
		userAgent:       c.UserAgent,
		hostCertificate: c.HostCertificate,
		useTunnel:       c.UseTunnel,
		address:         c.Address,
		authClient:      c.AuthClient,
		auditLog:        c.AuthClient,
		authService:     c.AuthClient,
		sessionServer:   c.AuthClient,
		dataDir:         c.DataDir,
		clock:           c.Clock,
		hostUUID:        c.HostUUID,
	}

	// Set the ciphers, KEX, and MACs that the in-memory server will send to the
	// client in its SSH_MSG_KEXINIT.
	s.ciphers = c.Ciphers
	s.kexAlgorithms = c.KEXAlgorithms
	s.macAlgorithms = c.MACAlgorithms

	s.sessionRegistry, err = srv.NewSessionRegistry(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Common auth handlers.
	s.authHandlers = &srv.AuthHandlers{
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component:       teleport.ComponentForwardingNode,
			trace.ComponentFields: logrus.Fields{},
		}),
		Server:      s,
		Component:   teleport.ComponentForwardingNode,
		AuditLog:    c.AuthClient,
		AccessPoint: c.AuthClient,
		FIPS:        c.FIPS,
	}

	// Common term handlers.
	s.termHandlers = &srv.TermHandlers{
		SessionRegistry: s.sessionRegistry,
	}

	// Create a close context that is used internally to signal when the server
	// is closing and for any blocking goroutines to unblock.
	s.closeContext, s.closeCancel = context.WithCancel(context.Background())

	return s, nil
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
	return defaults.Namespace
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

// EmitAuditEvent sends an event to the Audit Log.
func (s *Server) EmitAuditEvent(event events.Event, fields events.EventFields) {
	auditLog := s.GetAuditLog()
	if auditLog != nil {
		if err := auditLog.EmitAuditEvent(event, fields); err != nil {
			s.log.Error(err)
		}
	} else {
		s.log.Warn("SSH server has no audit log")
	}
}

// PermitUserEnvironment is always false because it's up the the remote host
// to decide if the user environment will be read or not.
func (s *Server) PermitUserEnvironment() bool {
	return false
}

// GetAuditLog returns the Audit Log for this cluster.
func (s *Server) GetAuditLog() events.IAuditLog {
	return s.auditLog
}

// GetAccessPoint returns an auth.AccessPoint for this cluster.
func (s *Server) GetAccessPoint() auth.AccessPoint {
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

// GetInfo returns a services.Server that represents this server.
func (s *Server) GetInfo() services.Server {
	return &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      s.ID(),
			Namespace: s.GetNamespace(),
		},
		Spec: services.ServerSpecV2{
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

	clusterConfig, err := s.GetAccessPoint().GetClusterConfig()
	if err != nil {
		s.log.Errorf("Unable to fetch cluster config: %v.", err)
		return
	}

	s.log.Debugf("Supported ciphers: %q.", s.ciphers)
	s.log.Debugf("Supported KEX algorithms: %q.", s.kexAlgorithms)
	s.log.Debugf("Supported MAC algorithms: %q.", s.macAlgorithms)

	sconn, chans, reqs, err := ssh.NewServerConn(s.serverConn, config)
	if err != nil {
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create server connection: %v.", err)
		return
	}
	s.sconn = sconn

	// Take connection and extract identity information for the user from it.
	s.identityContext, err = s.authHandlers.CreateIdentityContext(sconn)
	if err != nil {
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create server connection: %v.", err)
		return
	}

	// Connect and authenticate to the remote node.
	s.log.Debugf("Creating remote connection to %v@%v", sconn.User(), s.clientConn.RemoteAddr().String())
	s.remoteClient, err = s.newRemoteClient(sconn.User())
	if err != nil {
		// Reject the connection with an error so the client doesn't hang then
		// close the connection.
		s.rejectChannel(chans, err)
		sconn.Close()

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
		Interval:     clusterConfig.GetKeepAliveInterval(),
		MaxCount:     clusterConfig.GetKeepAliveCountMax(),
		CloseContext: s.closeContext,
		CloseCancel:  s.closeCancel,
	})

	go s.handleConnection(chans, reqs)
}

// Close will close all underlying connections that the forwarding server holds.
func (s *Server) Close() error {
	conns := []io.Closer{
		s.sconn,
		s.clientConn,
		s.serverConn,
		s.targetConn,
		s.remoteClient,
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

	// Signal to waiting goroutines that the server is closing (for example,
	// the keep alive loop).
	s.closeCancel()

	return trace.NewAggregate(errs...)
}

// newRemoteSession will create and return a *ssh.Client and *ssh.Session
// with a remote host.
func (s *Server) newRemoteClient(systemLogin string) (*ssh.Client, error) {
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
		Timeout:         defaults.DefaultDialTimeout,
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
	client, err := proxy.NewClientConnWithDeadline(s.targetConn, dstAddr, clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (s *Server) handleConnection(chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
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
			go s.handleChannel(newChannel)
		// If the server is closing (either the heartbeat failed or Close() was
		// called, exit out of the connection handler loop.
		case <-s.closeContext.Done():
			return
		}
	}
}

func (s *Server) rejectChannel(chans <-chan ssh.NewChannel, err error) {
	for newChannel := range chans {
		err := newChannel.Reject(ssh.ConnectionFailed, err.Error())
		if err != nil {
			s.log.Errorf("Unable to reject and close connection.")
		}
		return
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

func (s *Server) handleChannel(nch ssh.NewChannel) {
	channelType := nch.ChannelType()

	switch channelType {
	// Channels of type "session" handle requests that are involved in running
	// commands on a server, subsystem requests, and agent forwarding.
	case "session":
		ch, requests, err := nch.Accept()
		if err != nil {
			s.log.Warnf("Unable to accept channel: %v", err)
			nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go s.handleSessionRequests(ch, requests)
	// Channels of type "direct-tcpip" handles request for port forwarding.
	case "direct-tcpip":
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			s.log.Errorf("Failed to parse request data: %v, err: %v", string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType, "failed to parse direct-tcpip request")
			return
		}
		ch, _, err := nch.Accept()
		if err != nil {
			s.log.Warnf("Unable to accept channel: %v", err)
			nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go s.handleDirectTCPIPRequest(ch, req)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
	}
}

// handleDirectTCPIPRequest handles port forwarding requests.
func (s *Server) handleDirectTCPIPRequest(ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	srcAddr := fmt.Sprintf("%v:%d", req.Orig, req.OrigPort)
	dstAddr := fmt.Sprintf("%v:%d", req.Host, req.Port)

	// Create context for this channel. This context will be closed when
	// forwarding is complete.
	ctx, err := srv.NewServerContext(s, s.sconn, s.identityContext)
	if err != nil {
		ctx.Errorf("Unable to create connection context: %v.", err)
		ch.Stderr().Write([]byte("Unable to create connection context."))
		return
	}
	ctx.Connection = s.serverConn
	ctx.RemoteClient = s.remoteClient
	defer ctx.Close()

	// Check if the role allows port forwarding for this user.
	err = s.authHandlers.CheckPortForward(dstAddr, ctx)
	if err != nil {
		ch.Stderr().Write([]byte(err.Error()))
		return
	}

	s.log.Debugf("Opening direct-tcpip channel from %v to %v in context %v.", srcAddr, dstAddr, ctx.ID())
	defer s.log.Debugf("Completing direct-tcpip request from %v to %v in context %v.", srcAddr, dstAddr, ctx.ID())

	// Create "direct-tcpip" channel from the remote host to the target host.
	conn, err := s.remoteClient.Dial("tcp", dstAddr)
	if err != nil {
		ctx.Infof("Failed to connect to: %v: %v", dstAddr, err)
		return
	}
	defer conn.Close()

	// Emit a port forwarding audit event.
	s.EmitAuditEvent(events.PortForward, events.EventFields{
		events.PortForwardAddr:    dstAddr,
		events.PortForwardSuccess: true,
		events.EventLogin:         s.identityContext.Login,
		events.EventUser:          s.identityContext.TeleportUser,
		events.LocalAddr:          s.sconn.LocalAddr().String(),
		events.RemoteAddr:         s.sconn.RemoteAddr().String(),
	})

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(ch, conn)
		ch.Close()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(conn, ch)
		conn.Close()
	}()

	wg.Wait()
}

// handleSessionRequests handles out of band session requests once the session
// channel has been created this function's loop handles all the "exec",
// "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(ch ssh.Channel, in <-chan *ssh.Request) {
	// Create context for this channel. This context will be closed when the
	// session request is complete.
	// There is no need for the forwarding server to initiate disconnects,
	// based on teleport business logic, because this logic is already
	// done on the server's terminating side.
	ctx, err := srv.NewServerContext(s, s.sconn, s.identityContext)
	if err != nil {
		ctx.Errorf("Unable to create connection context: %v.", err)
		ch.Stderr().Write([]byte("Unable to create connection context."))
		return
	}
	ctx.Connection = s.serverConn
	ctx.RemoteClient = s.remoteClient
	ctx.AddCloser(ch)
	defer ctx.Close()

	// Create a "session" channel on the remote host.
	remoteSession, err := s.remoteClient.NewSession()
	if err != nil {
		ch.Stderr().Write([]byte(err.Error()))
		return
	}
	ctx.RemoteSession = remoteSession

	s.log.Debugf("Opening session request to %v in context %v.", s.sconn.RemoteAddr(), ctx.ID())
	defer s.log.Debugf("Closing session request to %v in context %v.", s.sconn.RemoteAddr(), ctx.ID())

	for {
		// Update the context with the session ID.
		err := ctx.CreateOrJoinSession(s.sessionRegistry)
		if err != nil {
			errorMessage := fmt.Sprintf("unable to update context: %v", err)
			ctx.Errorf("%v", errorMessage)

			// Write the error to channel and close it.
			ch.Stderr().Write([]byte(errorMessage))
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: teleport.RemoteCommandFailure}))
			if err != nil {
				ctx.Errorf("Failed to send exit status %v", errorMessage)
			}
			return
		}

		select {
		case result := <-ctx.SubsystemResultCh:
			// Subsystem has finished executing, close the channel and session.
			ctx.Debugf("Subsystem execution result: %v", result.Err)
			return
		case req := <-in:
			if req == nil {
				// The client has closed or dropped the connection.
				ctx.Debugf("Client %v disconnected", s.sconn.RemoteAddr())
				return
			}
			if err := s.dispatch(ch, req, ctx); err != nil {
				s.replyError(ch, req, err)
				return
			}
			if req.WantReply {
				req.Reply(true, nil)
			}
		case result := <-ctx.ExecResultCh:
			ctx.Debugf("Exec request (%q) complete: %v", result.Command, result.Code)

			// The exec process has finished and delivered the execution result, send
			// the result back to the client, and close the session and channel.
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(result.Code)}))
			if err != nil {
				ctx.Infof("Failed to send exit status for %v: %v", result.Command, err)
			}

			return
		}
	}
}

func (s *Server) dispatch(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	ctx.Debugf("Handling request %v, want reply %v.", req.Type, req.WantReply)

	switch req.Type {
	case sshutils.ExecRequest:
		return s.termHandlers.HandleExec(ch, req, ctx)
	case sshutils.PTYRequest:
		return s.termHandlers.HandlePTYReq(ch, req, ctx)
	case sshutils.ShellRequest:
		return s.termHandlers.HandleShell(ch, req, ctx)
	case sshutils.WindowChangeRequest:
		return s.termHandlers.HandleWinChange(ch, req, ctx)
	case sshutils.EnvRequest:
		return s.handleEnv(ch, req, ctx)
	case sshutils.SubsystemRequest:
		return s.handleSubsystem(ch, req, ctx)
	case sshutils.AgentForwardRequest:
		// to maintain interoperability with OpenSSH, agent forwarding requests
		// should never fail, all errors should be logged and we should continue
		// processing requests.
		err := s.handleAgentForward(ch, req, ctx)
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
	err = agent.ForwardToAgent(ctx.RemoteClient, s.userAgent)
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

func (s *Server) handleSubsystem(ch ssh.Channel, req *ssh.Request, ctx *srv.ServerContext) error {
	subsystem, err := parseSubsystemRequest(req, ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// start the requested subsystem, if it fails to start return result right away
	err = subsystem.Start(ch)
	if err != nil {
		ctx.SendSubsystemResult(srv.SubsystemResult{
			Name: subsystem.subsytemName,
			Err:  trace.Wrap(err),
		})
		return trace.Wrap(err)
	}

	// wait for the subsystem to finish and return that result
	go func() {
		err := subsystem.Wait()
		ctx.SendSubsystemResult(srv.SubsystemResult{
			Name: subsystem.subsytemName,
			Err:  trace.Wrap(err),
		})
	}()

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
	message := []byte(utils.UserMessageFromError(err))
	ch.Stderr().Write(message)
	if req.WantReply {
		req.Reply(false, message)
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
