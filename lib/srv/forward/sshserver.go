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

	targetConn net.Conn
	clientConn net.Conn
	serverConn net.Conn

	// userAgent is the SSH user agent that was forwarded to the proxy.
	userAgent agent.Agent
	// userAgentChannel is the channel over which communication with the agent occurs.
	userAgentChannel ssh.Channel

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
}

// CheckDefaults makes sure all required parameters are passed in.
func (s *ServerConfig) CheckDefaults() error {
	if s.AuthClient == nil {
		return trace.BadParameter("auth client required")
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

	// take connection and build identity for the user from it to be passed
	// along with context
	identityContext, err := s.authHandlers.CreateIdentityContext(sconn)
	if err != nil {
		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create server connection: %v.", err)
		return
	}

	// build a remote session to the remote node
	s.log.Debugf("Creating remote connection to %v@%v", sconn.User(), s.clientConn.RemoteAddr().String())
	remoteClient, remoteSession, err := s.newRemoteSession(sconn.User())
	if err != nil {
		// reject the connection with an error so the client doesn't hang then
		// close the connection
		s.rejectChannel(chans, err)
		sconn.Close()

		s.targetConn.Close()
		s.clientConn.Close()
		s.serverConn.Close()

		s.log.Errorf("Unable to create remote connection: %v", err)
		return
	}

	// create server context for this connection, it's closed when the
	// connection is closed
	ctx := srv.NewServerContext(s, sconn, identityContext)

	ctx.RemoteClient = remoteClient
	ctx.RemoteSession = remoteSession
	ctx.SetAgent(s.userAgent, s.userAgentChannel)

	ctx.AddCloser(sconn)
	ctx.AddCloser(s.targetConn)
	ctx.AddCloser(s.serverConn)
	ctx.AddCloser(s.clientConn)
	ctx.AddCloser(remoteSession)
	ctx.AddCloser(remoteClient)

	s.log.Debugf("Created connection context %v", ctx.ID())

	// create a cancelable context and pass it to a keep alive loop. the keep
	// alive loop will keep pinging the remote server and after it has missed a
	// certain number of keep alive requests it will cancel the context which
	// will close any listening goroutines.
	heartbeatContext, cancel := context.WithCancel(context.Background())
	go s.keepAliveLoop(ctx, sconn, cancel)
	go s.handleConnection(ctx, heartbeatContext, sconn, chans, reqs)
}

// newRemoteSession will create and return a *ssh.Client and *ssh.Session
// with a remote host.
func (s *Server) newRemoteSession(systemLogin string) (*ssh.Client, *ssh.Session, error) {
	// the proxy will use the agent that has been forwarded to it as the auth
	// method when connecting to the remote host
	if s.userAgent == nil {
		return nil, nil, trace.AccessDenied("agent must be forwarded to proxy")
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
		return nil, nil, trace.Wrap(err)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return client, session, nil
}

func (s *Server) handleConnection(ctx *srv.ServerContext, heartbeatContext context.Context, sconn *ssh.ServerConn, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	defer s.log.Debugf("Closing connection context %v and releasing resources.", ctx.ID())
	defer ctx.Close()

	for {
		select {
		// global out-of-band requests
		case newRequest := <-reqs:
			if newRequest == nil {
				return
			}
			go s.handleGlobalRequest(ctx, newRequest)
		// channel requests
		case newChannel := <-chans:
			if newChannel == nil {
				return
			}
			go s.handleChannel(ctx, sconn, newChannel)
		// if the heartbeats failed, we close everything and cleanup
		case <-heartbeatContext.Done():
			return
		}
	}
}

func (s *Server) keepAliveLoop(ctx *srv.ServerContext, sconn *ssh.ServerConn, cancel context.CancelFunc) {
	var missed int

	// tick at 1/3 of the idle timeout duration
	keepAliveTick := time.NewTicker(defaults.DefaultIdleConnectionDuration / 3)
	defer keepAliveTick.Stop()

	for {
		select {
		case <-keepAliveTick.C:
			// send a keep alive to the target node and the client to ensure both are alive.
			proxyToNodeOk := s.sendKeepAliveWithTimeout(ctx.RemoteClient, defaults.ReadHeadersTimeout)
			proxyToClientOk := s.sendKeepAliveWithTimeout(sconn, defaults.ReadHeadersTimeout)
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

func (s *Server) handleGlobalRequest(ctx *srv.ServerContext, req *ssh.Request) {
	ok, err := ctx.RemoteSession.SendRequest(req.Type, req.WantReply, req.Payload)
	if err != nil {
		s.log.Warnf("Failed to forward global request %v: %v", req.Type, err)
		return
	}
	if req.WantReply {
		err = req.Reply(ok, nil)
		if err != nil {
			s.log.Warnf("Failed to reply to global request: %v: %v", req.Type, err)
		}
	}
}

func (s *Server) handleChannel(ctx *srv.ServerContext, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	channelType := nch.ChannelType()

	switch channelType {
	// a client requested the terminal size to be sent along with every
	// session message (Teleport-specific SSH channel for web-based terminals)
	case "x-teleport-request-resize-events":
		ch, _, _ := nch.Accept()
		go s.handleTerminalResize(sconn, ch)
	// interactive sessions
	case "session":
		ch, requests, err := nch.Accept()
		if err != nil {
			s.log.Warnf("Unable to accept channel: %v", err)
			nch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unable to accept channel: %v", err))
			return
		}
		go s.handleSessionRequests(ctx, sconn, ch, requests)
	// port forwarding
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
		go s.handleDirectTCPIPRequest(ctx, sconn, ch, req)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
	}
}

// handleDirectTCPIPRequest handles port forwarding requests.
func (s *Server) handleDirectTCPIPRequest(ctx *srv.ServerContext, sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	srcAddr := fmt.Sprintf("%v:%d", req.Orig, req.OrigPort)
	dstAddr := fmt.Sprintf("%v:%d", req.Host, req.Port)

	defer s.log.Debugf("Completing direct-tcpip request from %v to %v.", srcAddr, dstAddr)

	// check if the role allows port forwarding for this user
	err := s.authHandlers.CheckPortForward(dstAddr, ctx)
	if err != nil {
		ch.Stderr().Write([]byte(err.Error()))
		return
	}

	s.log.Debugf("Opening direct-tcpip channel from %v to %v.", srcAddr, dstAddr)

	conn, err := ctx.RemoteClient.Dial("tcp", dstAddr)
	if err != nil {
		ctx.Infof("Failed to connect to: %v: %v", dstAddr, err)
		return
	}
	defer conn.Close()

	// emit a port forwarding audit event
	s.EmitAuditEvent(events.PortForwardEvent, events.EventFields{
		events.PortForwardAddr:    dstAddr,
		events.PortForwardSuccess: true,
		events.EventLogin:         ctx.Identity.Login,
		events.LocalAddr:          sconn.LocalAddr().String(),
		events.RemoteAddr:         sconn.RemoteAddr().String(),
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

// handleTerminalResize is called by the web proxy via its SSH connection.
// when a web browser connects to the web API, the web proxy asks us,
// by creating this new SSH channel, to start injecting the terminal size
// into every SSH write back to it.
//
// this is the only way to make web-based terminal UI not break apart
// when window changes its size
func (s *Server) handleTerminalResize(sconn *ssh.ServerConn, ch ssh.Channel) {
	err := s.sessionRegistry.PushTermSizeToParty(sconn, ch)
	if err != nil {
		s.log.Warnf("Unable to push terminal size to party: %v", err)
	}
}

// handleSessionRequests handles out of band session requests once the session
// channel has been created this function's loop handles all the "exec",
// "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(ctx *srv.ServerContext, sconn *ssh.ServerConn, ch ssh.Channel, in <-chan *ssh.Request) {
	defer s.log.Debugf("Closing session request to %v.", sconn.RemoteAddr())
	defer ch.Close()

	s.log.Debugf("Opening session request to %v.", sconn.RemoteAddr())

	for {
		// update ctx with the session ID:
		err := ctx.CreateOrJoinSession(s.sessionRegistry)
		if err != nil {
			errorMessage := fmt.Sprintf("unable to update context: %v", err)
			ctx.Errorf("%v", errorMessage)

			// write the error to channel and close it
			ch.Stderr().Write([]byte(errorMessage))
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: teleport.RemoteCommandFailure}))
			if err != nil {
				ctx.Errorf("Failed to send exit status %v", errorMessage)
			}
			return
		}

		select {
		case result := <-ctx.SubsystemResultCh:
			// this means that subsystem has finished executing and
			// want us to close session and the channel
			ctx.Debugf("Subsystem execution result: %v", result.Err)

			return
		case req := <-in:
			if req == nil {
				// this will happen when the client closes/drops the connection
				ctx.Debugf("Client %v disconnected", sconn.RemoteAddr())
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

			// this means that exec process has finished and delivered the execution result,
			// we send it back and close the session
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
	// check if the user's RBAC role allows agent forwarding
	err := s.authHandlers.CheckAgentForward(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// route authentication requests to the agent that was forwarded to the proxy
	err = agent.ForwardToAgent(ctx.RemoteClient, ctx.GetAgent())
	if err != nil {
		return trace.Wrap(err)
	}

	// make an "auth-agent-req@openssh.com" request on the target node
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
