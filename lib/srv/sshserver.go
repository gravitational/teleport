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

// Package srv implements SSH server that supports multiplexing
// tunneling, SSH connections proxying and only supports Key based auth
package srv

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Server implements SSH server that uses configuration backend and
// certificate-based authentication
type Server struct {
	sync.Mutex

	addr          utils.NetAddr
	hostname      string
	certChecker   ssh.CertChecker
	resolver      resolver
	elog          events.Log
	srv           *sshutils.Server
	hostSigner    ssh.Signer
	shell         string
	authService   auth.AccessPoint
	reg           *sessionRegistry
	sessionServer rsession.Service
	rec           recorder.Recorder
	limiter       *limiter.Limiter

	labels      map[string]string                //static server labels
	cmdLabels   map[string]services.CommandLabel //dymanic server labels
	labelsMutex *sync.Mutex

	proxyMode bool
	proxyTun  reversetunnel.Server

	advertiseIP net.IP

	// server UUID gets generated once on the first start and never changes
	// usually stored in a file inside the data dir
	uuid string
}

// ServerOption is a functional option passed to the server
type ServerOption func(s *Server) error

// SetEventLogger sets structured event logger for this server
func SetEventLogger(e events.Log) ServerOption {
	return func(s *Server) error {
		s.elog = e
		return nil
	}
}

// SetShell sets default shell that will be executed for interactive
// sessions
func SetShell(shell string) ServerOption {
	return func(s *Server) error {
		s.shell = shell
		return nil
	}
}

// SetSessionServer represents realtime session registry server
func SetSessionServer(srv rsession.Service) ServerOption {
	return func(s *Server) error {
		s.sessionServer = srv
		return nil
	}
}

// SetRecorder records all
func SetRecorder(rec recorder.Recorder) ServerOption {
	return func(s *Server) error {
		s.rec = rec
		return nil
	}
}

// SetProxyMode starts this server in SSH proxying mode
func SetProxyMode(tsrv reversetunnel.Server) ServerOption {
	return func(s *Server) error {
		s.proxyMode = true
		s.proxyTun = tsrv
		return nil
	}
}

// SetLabels sets dynamic and static labels that server will report to the
// auth servers
func SetLabels(labels map[string]string,
	cmdLabels services.CommandLabels) ServerOption {
	return func(s *Server) error {
		for name, label := range cmdLabels {
			if label.Period < time.Second {
				label.Period = time.Second
				cmdLabels[name] = label
				log.Warningf("label period can't be less that 1 second. Period for label '%v' was set to 1 second", name)
			}
		}

		s.labels = labels
		s.cmdLabels = cmdLabels
		return nil
	}
}

// SetLimiter sets rate and connection limiter for this server
func SetLimiter(limiter *limiter.Limiter) ServerOption {
	return func(s *Server) error {
		s.limiter = limiter
		return nil
	}
}

// New returns an unstarted server
func New(addr utils.NetAddr,
	hostname string,
	signers []ssh.Signer,
	authService auth.AccessPoint,
	dataDir string,
	advertiseIP net.IP,
	options ...ServerOption) (*Server, error) {

	// read the host UUID:
	uuid, err := utils.ReadOrMakeHostUUID(dataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		addr:        addr,
		authService: authService,
		resolver:    &backendResolver{authService: authService},
		hostname:    hostname,
		labelsMutex: &sync.Mutex{},
		advertiseIP: advertiseIP,
		uuid:        uuid,
	}
	s.limiter, err = limiter.NewLimiter(limiter.LimiterConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.certChecker = ssh.CertChecker{IsAuthority: s.isAuthority}

	for _, o := range options {
		if err := o(s); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	s.reg = newSessionRegistry(s)
	if s.elog == nil {
		s.elog = events.NullEventLogger
	}

	srv, err := sshutils.NewServer(
		addr, s, signers,
		sshutils.AuthMethods{PublicKey: s.keyAuth},
		sshutils.SetLimiter(s.limiter),
		sshutils.SetRequestHandler(s))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.srv = srv
	return s, nil
}

func (s *Server) logFields(fields map[string]interface{}) log.Fields {
	var component string
	if s.proxyMode {
		component = teleport.ComponentProxy
	} else {
		component = teleport.ComponentNode
	}
	return log.Fields{
		teleport.Component:       component,
		teleport.ComponentFields: fields,
	}
}

// Addr returns server address
func (s *Server) Addr() string {
	return s.srv.Addr()
}

// ID returns server ID
func (s *Server) ID() string {
	return s.uuid
}

func (s *Server) setAdvertiseIP(ip net.IP) {
	s.Lock()
	defer s.Unlock()
	s.advertiseIP = ip
}

func (s *Server) getAdvertiseIP() net.IP {
	s.Lock()
	defer s.Unlock()
	return s.advertiseIP
}

// AdvertiseAddr returns an address this server should be publicly accessible
// as, in "ip:host" form
func (s *Server) AdvertiseAddr() string {
	// se if we have explicit --advertise-ip option
	if s.getAdvertiseIP() == nil {
		return s.addr.Addr
	}
	_, port, _ := net.SplitHostPort(s.addr.Addr)
	return net.JoinHostPort(s.getAdvertiseIP().String(), port)
}

// registerServer attempts to register server in the cluster
func (s *Server) registerServer() error {
	srv := services.Server{
		ID:        s.ID(),
		Addr:      s.AdvertiseAddr(),
		Hostname:  s.hostname,
		Labels:    s.labels,
		CmdLabels: s.getCommandLabels(),
	}
	return trace.Wrap(s.authService.UpsertServer(srv, defaults.ServerHeartbeatTTL))
}

// heartbeatPresence periodically calls into the auth server to let everyone
// know we're up & alive
func (s *Server) heartbeatPresence() {
	for {
		if err := s.registerServer(); err != nil {
			log.Warningf("failed to announce %#v presence: %v", s, err)
		}
		sleepTime := defaults.ServerHeartbeatTTL/2 + utils.RandomDuration(defaults.ServerHeartbeatTTL/10)
		log.Infof("[SSH] will ping auth service in %v", sleepTime)
		time.Sleep(sleepTime)
	}
}

func (s *Server) updateLabels() {
	for name, label := range s.cmdLabels {
		go s.periodicUpdateLabel(name, label)
	}
}

func (s *Server) syncUpdateLabels() {
	for name, label := range s.cmdLabels {
		s.updateLabel(name, label)
	}
}

func (s *Server) updateLabel(name string, label services.CommandLabel) {
	out, err := exec.Command(label.Command[0], label.Command[1:]...).Output()
	if err != nil {
		log.Errorf(err.Error())
		label.Result = err.Error() + " output: " + string(out)
	} else {
		label.Result = strings.TrimSpace(string(out))
	}
	s.setCommandLabel(name, label)
}

func (s *Server) periodicUpdateLabel(name string, label services.CommandLabel) {
	for {
		s.updateLabel(name, label)
		time.Sleep(label.Period)
	}
}

func (s *Server) setCommandLabel(name string, value services.CommandLabel) {
	s.labelsMutex.Lock()
	defer s.labelsMutex.Unlock()
	s.cmdLabels[name] = value
}

func (s *Server) getCommandLabels() map[string]services.CommandLabel {
	s.labelsMutex.Lock()
	defer s.labelsMutex.Unlock()
	out := make(map[string]services.CommandLabel, len(s.cmdLabels))
	for key, val := range s.cmdLabels {
		out[key] = val
	}
	return out
}

func (s *Server) checkPermissionToLogin(cert ssh.PublicKey, teleportUser, osUser string) error {
	// find cert authority by it's key
	cas, err := s.authService.GetCertAuthorities(services.UserCA)
	if err != nil {
		return trace.Wrap(err)
	}

	var ca *services.CertAuthority
	for i := range cas {
		checkers, err := cas[i].Checkers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(cert, checker) {
				ca = cas[i]
				break
			}
		}
	}

	if ca == nil {
		return trace.Wrap(teleport.NotFound(
			fmt.Sprintf("not found authority for key %v", teleportUser),
		))
	}

	localDomain, err := s.authService.GetLocalDomain()
	if err != nil {
		return trace.Wrap(err)
	}

	// for local users, go and check their individual permissions
	if localDomain == ca.DomainName {
		log.Infof("%v is local authority", ca.DomainName)
		users, err := s.authService.GetUsers()
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("users: %v", users)
		for _, u := range users {
			if u.Name == teleportUser {
				for _, login := range u.AllowedLogins {
					if login == osUser {
						return nil
					}
				}
			}
		}
		return trace.Wrap(teleport.NotFound(
			fmt.Sprintf("not found local user entry for %v and os user %v for local authority %v",
				teleportUser, osUser, ca.ID())))
	}
	log.Infof("%v is remote authority", ca.DomainName)

	// for other authorities, check for authoritiy permissions
	for _, login := range ca.AllowedLogins {
		if login == osUser {
			return nil
		}
	}
	return trace.Wrap(teleport.NotFound(
		fmt.Sprintf("not found user entry for %v and os user %v for remote authority %v",
			teleportUser, osUser, ca.ID())))
}

// isAuthority is called during checking the client key, to see if the signing
// key is the real CA authority key.
func (s *Server) isAuthority(cert ssh.PublicKey) bool {
	// find cert authority by it's key
	cas, err := auth.RetryingClient(s.authService, 20).GetCertAuthorities(services.UserCA)
	if err != nil {
		log.Warningf("%v", err)
		return false
	}

	for i := range cas {
		checkers, err := cas[i].Checkers()
		if err != nil {
			log.Warningf("%v", err)
			return false
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(cert, checker) {
				return true
			}
		}
	}
	log.Warningf("no matching authority found")
	return false
}

// keyAuth implements SSH client authentication using public keys and is called
// by the server every time the client connects
func (s *Server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf("conn(%v->%v, user=%v)", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
	eventID := lunk.NewRootEventID()
	fingerprint := fmt.Sprintf("%v %v", key.Type(), sshutils.Fingerprint(key))
	log.Infof("%v auth attempt with key %v", cid, fingerprint)

	logger := log.WithFields(log.Fields{
		"local":       conn.LocalAddr(),
		"remote":      conn.RemoteAddr(),
		"user":        conn.User(),
		"fingerprint": fingerprint,
	})

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		logger.Warningf("server doesn't support provided key type")
		return nil, trace.Wrap(teleport.BadParameter("key", fmt.Sprintf("server doesn't support provided key type: %v", fingerprint)))
	}
	if len(cert.ValidPrincipals) == 0 {
		logger.Warningf("cert does not have valid principals")
		return nil, trace.Wrap(teleport.BadParameter("key", fmt.Sprintf("need a valid principal for key %v", fingerprint)))
	}
	if len(cert.KeyId) == 0 {
		logger.Warningf("cert does not have valid key id")
		return nil, trace.Wrap(teleport.BadParameter("key", fmt.Sprintf("need a valid key for key %v", fingerprint)))
	}
	teleportUser := cert.KeyId

	permissions, err := s.certChecker.Authenticate(conn, key)
	if err != nil {
		s.elog.Log(eventID, events.NewAuthAttempt(conn, key, false, err))
		logger.Warningf("authenticate err: %v", err)
		return nil, trace.Wrap(err)
	}
	if err := s.certChecker.CheckCert(teleportUser, cert); err != nil {
		logger.Warningf("failed to authenticate user, err: %v", err)
		return nil, trace.Wrap(err)
	}

	// this is the only way I know of to pass valid principal with the
	// connection
	permissions.Extensions[utils.CertTeleportUser] = teleportUser

	if s.proxyMode {
		return permissions, nil
	}

	err = s.checkPermissionToLogin(cert.SignatureKey, teleportUser, conn.User())
	if err != nil {
		logger.Warningf("authenticate user: %v", err)
		return nil, trace.Wrap(err)
	}
	return permissions, nil
}

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	return s.srv.Close()
}

// Start starts server
func (s *Server) Start() error {
	if !s.proxyMode {
		if len(s.cmdLabels) > 0 {
			s.updateLabels()
		}
		go s.heartbeatPresence()
	}
	return s.srv.Start()
}

// Wait waits until server stops
func (s *Server) Wait() {
	s.srv.Wait()
}

// HandleRequest is a callback for out of band requests
func (s *Server) HandleRequest(r *ssh.Request) {
	log.Infof("recieved out-of-band request: %+v", r)
}

// HandleNewChan is called when new channel is opened
func (s *Server) HandleNewChan(_ net.Conn, sconn *ssh.ServerConn, nch ssh.NewChannel) {
	channelType := nch.ChannelType()
	if s.proxyMode {
		if channelType == "session" { // interactive sessions
			ch, requests, err := nch.Accept()
			if err != nil {
				log.Infof("could not accept channel (%s)", err)
			}
			go s.handleSessionRequests(sconn, ch, requests)
		} else {
			nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
		}
		return
	}

	switch channelType {
	case "session": // interactive sessions
		ch, requests, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleSessionRequests(sconn, ch, requests)
	case "direct-tcpip": //port forwarding
		req, err := sshutils.ParseDirectTCPIPReq(nch.ExtraData())
		if err != nil {
			log.Errorf("failed to parse request data: %v, err: %v", string(nch.ExtraData()), err)
			nch.Reject(ssh.UnknownChannelType, "failed to parse direct-tcpip request")
		}
		sshCh, _, err := nch.Accept()
		if err != nil {
			log.Infof("could not accept channel (%s)", err)
		}
		go s.handleDirectTCPIPRequest(sconn, sshCh, req)
	default:
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", channelType))
	}
}

func (s *Server) handleDirectTCPIPRequest(sconn *ssh.ServerConn, ch ssh.Channel, req *sshutils.DirectTCPIPReq) {
	// ctx holds the session context and all associated resources
	ctx := newCtx(s, sconn)
	ctx.addCloser(ch)
	defer ctx.Close()

	ctx.Infof("opened direct-tcpip channel: %#v", req)
	addr := fmt.Sprintf("%v:%d", req.Host, req.Port)
	ctx.Infof("connecting to %v", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		ctx.Infof("failed to connect to: %v, err: %v", addr, err)
		return
	}
	defer conn.Close()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(ch, conn)
		ctx.Infof("conn to channel copy closed, bytes transferred: %v, err: %v",
			written, err)
		ch.Close()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(conn, ch)
		ctx.Infof("channel to conn copy closed, bytes transferred: %v, err: %v",
			ctx, written, err)
		conn.Close()
	}()
	wg.Wait()
	ctx.Infof("direct-tcp closed")
}

// handleSessionRequests handles out of band session requests once the session channel has been created
// this function's loop handles all the "exec", "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(sconn *ssh.ServerConn, ch ssh.Channel, in <-chan *ssh.Request) {
	// ctx holds the session context and all associated resources
	ctx := newCtx(s, sconn)
	ctx.Infof("opened session channel")

	// closeCh will close the connection and the context once the session closes
	var closeCh = func() {
		ctx.Infof("closing session channel")
		if err := ctx.Close(); err != nil {
			ctx.Infof("failed to close channel context: %v", err)
		}
		ch.Close()
	}
	for {
		select {
		case creq := <-ctx.subsystemResultC:
			// this means that subsystem has finished executing and
			// want us to close session and the channel
			ctx.Infof("close session request: %v", creq.err)
			closeCh()
			return
		case req := <-in:
			if req == nil {
				// this will happen when the client closes/drops the connection
				ctx.Infof("client disconnected")
				closeCh()
				return
			}
			if err := s.dispatch(sconn, ch, req, ctx); err != nil {
				ctx.Infof("error dispatching request: %#v", err)
				replyError(ch, req, err)
				closeCh()
				return
			}
			if req.WantReply {
				req.Reply(true, nil)
			}
		case result := <-ctx.result:
			// this means that exec process has finished and delivered the execution result, we send it back and close the session
			ctx.Infof("got execution result: %v", result)
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(result.code)}))
			if err != nil {
				ctx.Infof("%v failed to send exit status: %v", result.command, err)
			}
			closeCh()
			return
		}
	}
}

func (s *Server) fwdDispatch(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("dispatch(req=%v, wantReply=%v)", req.Type, req.WantReply)
	return trace.Errorf("unsupported request type: %v", req.Type)
}

func (s *Server) dispatch(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("dispatch(req=%v, wantReply=%v)", req.Type, req.WantReply)
	if s.proxyMode {
		switch req.Type {
		case "subsystem":
			return s.handleSubsystem(sconn, ch, req, ctx)
		case "env":
			// we currently ignore setting any environment variables via SSH for security purposes
			return s.handleEnv(ch, req, ctx)
		default:
			return trace.Wrap(
				teleport.BadParameter("reqType",
					fmt.Sprintf("proxy doesn't support request type '%v'", req.Type)))
		}
	}
	switch req.Type {
	case "exec":
		// exec is a remote execution of a program, does not use PTY
		return s.handleExec(sconn, ch, req, ctx)
	case sshutils.PTYReq:
		// SSH client asked to allocate PTY
		return s.handlePTYReq(ch, req, ctx)
	case "shell":
		// SSH client asked to launch shell, we allocate PTY and start shell session
		return s.handleShell(sconn, ch, req, ctx)
	case "env":
		// we currently ignore setting any environment variables via SSH for security purposes
		return s.handleEnv(ch, req, ctx)
	case "subsystem":
		// subsystems are SSH subsystems defined in http://tools.ietf.org/html/rfc4254 6.6
		// they are in essence SSH session extensions, allowing to implement new SSH commands
		return s.handleSubsystem(sconn, ch, req, ctx)
	case sshutils.WindowChangeReq:
		return s.handleWinChange(ch, req, ctx)
	case "auth-agent-req@openssh.com":
		// This happens when SSH client has agent forwarding enabled, in this case
		// client sends a special request, in return SSH server opens new channel
		// that uses SSH protocol for agent drafted here:
		// https://tools.ietf.org/html/draft-ietf-secsh-agent-02
		// the open ssh proto spec that we implement is here:
		// http://cvsweb.openbsd.org/cgi-bin/cvsweb/src/usr.bin/ssh/PROTOCOL.agent
		return s.handleAgentForward(sconn, ch, req, ctx)
	default:
		return trace.Wrap(
			teleport.BadParameter("reqType",
				fmt.Sprintf("proxy doesn't support request type '%v'", req.Type)))
	}
}

func (s *Server) handleAgentForward(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	authChan, _, err := sconn.OpenChannel("auth-agent@openssh.com", nil)
	if err != nil {
		return err
	}
	ctx.Infof("opened agent channel")
	ctx.setAgent(agent.NewClient(authChan), authChan)
	return nil
}

func (s *Server) handleWinChange(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("handleWinChange()")
	params, err := parseWinChange(req)
	if err != nil {
		return trace.Wrap(err)
	}
	term := ctx.getTerm()
	if term != nil {
		return trace.Wrap(term.setWinsize(*params))
	}
	sid, ok := ctx.getEnv(sshutils.SessionEnvVar)
	if !ok {
		return trace.Wrap(
			teleport.BadParameter("pty", "no PTY allocated for winChange"))
	}
	err = s.reg.notifyWinChange(sid, *params)
	return trace.Wrap(err)
}

func (s *Server) handleSubsystem(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("handleSubsystem()")
	sb, err := parseSubsystemRequest(s, req)
	if err != nil {
		ctx.Infof("%v failed to parse subsystem request: %v", err)
		return trace.Wrap(err)
	}
	// starting subsystem is blocking to the client,
	// while collecting its result and waiting is not blocking
	if err := sb.start(sconn, ch, req, ctx); err != nil {
		ctx.Infof("failed to execute request, err: %v", err)
		ctx.sendSubsystemResult(trace.Wrap(err))
		return trace.Wrap(err)
	}
	go func() {
		err := sb.wait()
		log.Infof("%v finished with result: %v", sb, err)
		ctx.sendSubsystemResult(trace.Wrap(err))
	}()
	return nil
}

func (s *Server) handleShell(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("handleShell()")

	sid, ok := ctx.getEnv(sshutils.SessionEnvVar)
	if !ok || sid == "" {
		sid = uuid.New()
	}
	return s.reg.joinShell(sid, sconn, ch, req, ctx)
}

func (s *Server) emit(eid lunk.EventID, e lunk.Event) {
	s.elog.Log(eid, e)
}

func (s *Server) handleEnv(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	var e sshutils.EnvReqParams
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		ctx.Errorf("handleEnv(err=%v)", err)

		return trace.Wrap(err, "failed to parse env request")
	}
	ctx.Infof("handleEnv(%#v)", e)
	ctx.setEnv(e.Name, e.Value)
	return nil
}

func (s *Server) handlePTYReq(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("handlePTYReq()")

	if term := ctx.getTerm(); term != nil {
		r, err := parsePTYReq(req)
		if err != nil {
			return trace.Wrap(err)
		}
		params, err := rsession.NewTerminalParamsFromUint32(r.W, r.H)
		if err != nil {
			return trace.Wrap(err)
		}
		term.setWinsize(*params)
		sid, ok := ctx.getEnv(sshutils.SessionEnvVar)
		if ok {
			if err := s.reg.notifyWinChange(sid, *params); err != nil {
				log.Infof("notifyWinChange: %v", err)
			}
		}
		return nil
	}
	log.Infof("handlePTYReq(new terminal)")
	term, params, err := requestPTY(req)
	if err != nil {
		return trace.Wrap(err)
	}
	sid, ok := ctx.getEnv(sshutils.SessionEnvVar)
	if ok {
		if err := s.reg.notifyWinChange(sid, *params); err != nil {
			log.Infof("notifyWinChange: %v", err)
		}
	}
	ctx.setTerm(term)
	ctx.Infof("PTY allocated successfully")
	return nil
}

func (s *Server) handleExec(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	ctx.Infof("handleExec()")
	e, err := parseExecRequest(req, ctx)
	if err != nil {
		ctx.Infof("failed to parse exec request: %v", err)
		replyError(ch, req, err)
		return trace.Wrap(err)
	}

	if scp.IsSCP(e.cmdName) {
		ctx.Infof("detected SCP command: %v", e.cmdName)
		if err := s.handleSCP(ch, req, ctx, e.cmdName); err != nil {
			ctx.Warningf("handleSCP() err: %v", err)
			return trace.Wrap(err)
		}
		return ch.Close()
	}

	result, err := e.start(sconn, s.shell, ch)
	if err != nil {
		ctx.Infof("error starting command, %v", err)
		replyError(ch, req, err)
	}
	if result != nil {
		ctx.Infof("%v result collected: %v", e, result)
		ctx.sendResult(*result)
	}
	// in case if result is nil and no error, this means that program is
	// running in the background
	go func() {
		ctx.Infof("%v waiting for result", e)
		result, err := e.wait()
		if err != nil {
			ctx.Infof("%v wait failed: %v", e, err)
		}
		if result != nil {
			ctx.Infof("%v result collected: %v", e, result)
			ctx.sendResult(*result)
		}
	}()
	return nil
}

func (s *Server) handleSCP(ch ssh.Channel, req *ssh.Request, ctx *ctx, args string) error {
	ctx.Infof("handleSCP(cmd=%v)", args)
	cmd, err := scp.ParseCommand(args)
	if err != nil {
		ctx.Warningf("failed to parse command: %v", cmd)
		return trace.Wrap(err, fmt.Sprintf("failure to parse command '%v'", cmd))
	}
	ctx.Infof("handleSCP(cmd=%#v)", cmd)
	srv, err := scp.New(*cmd)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(klizhentas) current version of handling exec is incorrect.
	// req.Reply should be sent as long as command start is done,
	// not at the end. This is my fix for SCP only:
	req.Reply(true, nil)
	if err := srv.Serve(ch); err != nil {
		return trace.Wrap(err)
	}
	ctx.Infof("SCP serve finished")
	_, err = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(0)}))
	if err != nil {
		ctx.Infof("failed to send scp exit status: %v", err)
	}
	ctx.Infof("SCP sent exit status")
	return nil
}

func replyError(ch ssh.Channel, req *ssh.Request, err error) {
	message := []byte(utils.UserMessageFromError(err))
	io.Copy(ch.Stderr(), bytes.NewBuffer(message))
	if req.WantReply {
		req.Reply(false, message)
	}
}

func closeAll(closers ...io.Closer) error {
	var err error
	for _, cl := range closers {
		if cl == nil {
			continue
		}
		if e := cl.Close(); e != nil {
			log.Infof("%T close failure: %v", cl, e)
			err = e
		}
	}
	return err
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}
