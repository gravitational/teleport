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
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
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
	log *log.Entry

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
}

// ServerConfig is the configuration needed to create an instance of a Server.
type ServerConfig struct {
	AuthClient      auth.ClientI
	UserAgent       agent.Agent
	TargetConn      net.Conn
	SrcAddr         net.Addr
	DstAddr         net.Addr
	HostCertificate ssh.Signer

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

	return nil
}

// New creates a new unstarted Server.
func New(c ServerConfig) (*Server, error) {
	// check and make sure we everything we need to build a forwarding node
	err := c.CheckDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// build a pipe connection to hook up the client and the server. we save both
	// here and will pass them along to the context when we create it so they
	// can be closed by the context.
	serverConn, clientConn := utils.DualPipeNetConn(c.SrcAddr, c.DstAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		log: log.WithFields(log.Fields{
			trace.Component: teleport.ComponentForwardingNode,
			trace.ComponentFields: map[string]string{
				"src-addr": c.SrcAddr.String(),
				"dst-addr": c.DstAddr.String(),
			},
		}),
		id:              uuid.New(),
		targetConn:      c.TargetConn,
		serverConn:      serverConn,
		clientConn:      clientConn,
		userAgent:       c.UserAgent,
		hostCertificate: c.HostCertificate,
		authClient:      c.AuthClient,
		auditLog:        c.AuthClient,
		authService:     c.AuthClient,
		sessionServer:   c.AuthClient,
		dataDir:         c.DataDir,
	}

	s.sessionRegistry, err = srv.NewSessionRegistry(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// common auth handlers
	s.authHandlers = &srv.AuthHandlers{
		Entry: log.WithFields(log.Fields{
			trace.Component:       teleport.ComponentForwardingNode,
			trace.ComponentFields: log.Fields{},
		}),
		Server:      nil,
		Component:   teleport.ComponentForwardingNode,
		AuditLog:    c.AuthClient,
		AccessPoint: c.AuthClient,
	}

	// common term handlers
	s.termHandlers = &srv.TermHandlers{
		SessionRegistry: s.sessionRegistry,
	}

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
func (s *Server) EmitAuditEvent(eventType string, fields events.EventFields) {
	auditLog := s.GetAuditLog()
	if auditLog != nil {
		if err := auditLog.EmitAuditEvent(eventType, fields); err != nil {
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

// Dial returns the client connection created by pipeAddrConn.
func (s *Server) Dial() (net.Conn, error) {
	return s.clientConn, nil
}

func (s *Server) Serve() {
	config := &ssh.ServerConfig{
		PublicKeyCallback: s.authHandlers.UserKeyAuth,
	}
	config.AddHostKey(s.hostCertificate)

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

	// Create a cancelable context and pass it to a keep alive loop. The keep
	// alive loop will keep pinging the remote server and after it has missed a
	// certain number of keep alive requests it will cancel the context which
	// will close any listening goroutines.
	heartbeatContext, cancel := context.WithCancel(context.Background())
	go s.keepAliveLoop(cancel)
	go s.handleConnection(heartbeatContext, chans, reqs)
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

	if len(s.ciphers) > 0 {
		clientConfig.Ciphers = s.ciphers
	}
	if len(s.kexAlgorithms) > 0 {
		clientConfig.KeyExchanges = s.kexAlgorithms
	}
	if len(s.macAlgorithms) > 0 {
		clientConfig.MACs = s.macAlgorithms
	}

	dstAddr := s.targetConn.RemoteAddr().String()
	client, err := proxy.NewClientConnWithDeadline(s.targetConn, dstAddr, clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (s *Server) handleConnection(heartbeatContext context.Context, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
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
		// If the heartbeats failed, close everything and cleanup.
		case <-heartbeatContext.Done():
			return
		}
	}
}

func (s *Server) keepAliveLoop(cancel context.CancelFunc) {
	var missed int

	// tick at 1/3 of the idle timeout duration
	keepAliveTick := time.NewTicker(defaults.DefaultIdleConnectionDuration / 3)
	defer keepAliveTick.Stop()

	for {
		select {
		case <-keepAliveTick.C:
			// send a keep alive to the target node and the client to ensure both are alive.
			proxyToNodeOk := s.sendKeepAliveWithTimeout(s.remoteClient, defaults.ReadHeadersTimeout)
			proxyToClientOk := s.sendKeepAliveWithTimeout(s.sconn, defaults.ReadHeadersTimeout)
			if proxyToNodeOk && proxyToClientOk {
				missed = 0
				continue
			}

			// if we miss 3 in a row the connections dead, call cancel and cleanup
			missed += 1
			if missed == 3 {
				s.log.Infof("Missed %v keep alive messages, closing connection", missed)
				cancel()
				return
			}
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
	// Channels of type "session" handle requests that are invovled in running
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
	s.EmitAuditEvent(events.PortForwardEvent, events.EventFields{
		events.PortForwardAddr:    dstAddr,
		events.PortForwardSuccess: true,
		events.EventLogin:         s.identityContext.Login,
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
	ctx, err := srv.NewServerContext(s, s.sconn, s.identityContext)
	if err != nil {
		ctx.Errorf("Unable to create connection context: %v.", err)
		ch.Stderr().Write([]byte("Unable to create connection context."))
		return
	}
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

// RequestSender is an interface that impliments SendRequest. It is used so
// server and client connections can be passed to functions to send requests.
type RequestSender interface {
	// SendRequest is used to send a out-of-band request.
	SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error)
}

// sendKeepAliveWithTimeout sends a keepalive@openssh.com message to the remote
// client. A manual timeout is needed here because SendRequest will wait for a
// response forever.
func (s *Server) sendKeepAliveWithTimeout(conn RequestSender, timeout time.Duration) bool {
	errorCh := make(chan error, 1)

	go func() {
		_, _, err := conn.SendRequest(teleport.KeepAliveReqType, true, nil)
		errorCh <- err
	}()

	select {
	case err := <-errorCh:
		if err != nil {
			return false
		}
		return true
	case <-time.After(timeout):
		return false
	}
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
