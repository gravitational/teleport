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
// srv implements SSH server that supports multiplexing, tunneling and key-based auth
package srv

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	"code.google.com/p/go-uuid/uuid"
	"github.com/codahale/lunk"
	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	// Server implements SSH server that uses configuration backend and certificate-based authentication:
	"golang.org/x/crypto/ssh/agent"
)

type Server struct {
	sync.Mutex
	addr        utils.NetAddr
	hostname    string
	certChecker ssh.CertChecker
	rr          resolver
	elog        lunk.EventLogger
	srv         *sshutils.Server
	hostSigner  ssh.Signer
	shell       string
	ap          auth.AccessPoint
	reg         *sessionRegistry
	se          rsession.SessionServer
	rec         recorder.Recorder

	labels      map[string]string                //static server labels
	cmdLabels   map[string]services.CommandLabel //dymanic server labels
	labelsMutex *sync.Mutex

	certificatesCache *services.CAService

	proxyMode bool
	proxyTun  reversetunnel.Server
}

type ServerOption func(s *Server) error

func SetEventLogger(e lunk.EventLogger) ServerOption {
	return func(s *Server) error {
		s.elog = e
		return nil
	}
}

func SetShell(shell string) ServerOption {
	return func(s *Server) error {
		s.shell = shell
		return nil
	}
}

func SetSessionServer(srv rsession.SessionServer) ServerOption {
	return func(s *Server) error {
		s.se = srv
		return nil
	}
}

func SetRecorder(rec recorder.Recorder) ServerOption {
	return func(s *Server) error {
		s.rec = rec
		return nil
	}
}

func SetProxyMode(tsrv reversetunnel.Server) ServerOption {
	return func(s *Server) error {
		s.proxyMode = true
		s.proxyTun = tsrv
		return nil
	}
}

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

// New returns an unstarted server
func New(addr utils.NetAddr, hostname string, signers []ssh.Signer,
	ap auth.AccessPoint, limiter *limiter.Limiter,
	dataDir string, options ...ServerOption) (*Server, error) {

	s := &Server{
		addr:        addr,
		ap:          ap,
		rr:          &backendResolver{ap: ap},
		hostname:    hostname,
		labelsMutex: &sync.Mutex{},
	}
	s.reg = newSessionRegistry(s)
	s.certChecker = ssh.CertChecker{IsAuthority: s.isAuthority}

	for _, o := range options {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	if s.elog == nil {
		s.elog = utils.NullEventLogger
	}

	var err error

	certCacheFile := "CertCache"
	if s.proxyMode {
		certCacheFile = "ProxyCertCache"
	}
	backend, err := boltbk.New(path.Join(dataDir, certCacheFile))
	if err != nil {
		return nil, err
	}
	s.certificatesCache = services.NewCAService(backend)

	srv, err := sshutils.NewServer(
		addr, s, signers,
		sshutils.AuthMethods{PublicKey: s.keyAuth},
		limiter,
		sshutils.SetRequestHandler(s))
	if err != nil {
		return nil, err
	}
	s.srv = srv
	return s, nil
}

func (s *Server) Addr() string {
	return s.srv.Addr()
}

func (s *Server) ID() string {
	return strings.Replace(s.addr.Addr, ":", "_", -1)
}

func (s *Server) heartbeatPresence() {
	for {
		func() {
			s.labelsMutex.Lock()
			defer s.labelsMutex.Unlock()
			srv := services.Server{
				ID:        s.ID(),
				Addr:      s.addr.Addr,
				Hostname:  s.hostname,
				Labels:    s.labels,
				CmdLabels: s.cmdLabels,
			}
			if err := s.ap.UpsertServer(srv, 6*time.Second); err != nil {
				log.Warningf("failed to announce %#v presence: %v", srv, err)
			}

			if err := s.updateTrustedCAKeys(); err != nil {
				log.Warningf(err.Error())
			}
		}()
		time.Sleep(3 * time.Second)
	}
}

func (s *Server) updateLabels() {
	for name, label := range s.cmdLabels {
		go s.updateLabel(name, label)
	}
}

func (s *Server) updateLabel(name string, label services.CommandLabel) {
	for {
		out, err := exec.Command(label.Command[0], label.Command[1:]...).Output()
		if err != nil {
			log.Errorf(err.Error())
			label.Result = err.Error() + " Output: " + string(out)
		} else {
			if out[len(out)-1] == 10 {
				out = out[:len(out)-1] // remove new line
			}
			label.Result = string(out)
		}
		s.labelsMutex.Lock()
		s.cmdLabels[name] = label
		s.labelsMutex.Unlock()
		time.Sleep(label.Period)
	}
}

func (s *Server) getTrustedCAKeys(fromCache bool) ([]ssh.PublicKey, error) {
	var keys []services.CertificateAuthority
	var err error
	if fromCache {
		keys, err = s.certificatesCache.GetRemoteCertificates(services.UserCert, "")
		if err != nil {
			log.Errorf(err.Error())
		}
	} else {
		keys, err = s.ap.GetTrustedCertificates(services.UserCert)
		if err != nil {
			log.Errorf(err.Error())
		}
	}

	var parsedKeys []ssh.PublicKey
	for _, key := range keys {
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(key.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA public key '%v', err: %v",
				string(key.PublicKey), err)
		}
		parsedKeys = append(parsedKeys, parsedKey)
	}
	return parsedKeys, nil
}

func (s *Server) updateTrustedCAKeys() error {
	keys, err := s.ap.GetTrustedCertificates(services.UserCert)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, key := range keys {
		err := s.certificatesCache.UpsertRemoteCertificate(key, CertificatesCacheTTL)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// isAuthority is called during checking the client key, to see if the signing
// key is the real CA authority key.
func (s *Server) isAuthority(auth ssh.PublicKey) bool {
	// Checking certificates from cache
	keys, err := s.getTrustedCAKeys(true)
	if err != nil {
		log.Errorf("failed to retrieve trused keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}

	// Checking certificates from auth server
	keys, err = s.getTrustedCAKeys(false)
	if err != nil {
		log.Errorf("failed to retrieve trused keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}

	return false
}

// keyAuth implements SSH client authentication using public keys and is called
// by the server every time the client connects
func (s *Server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf("conn(%v->%v, user=%v)", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
	eventID := lunk.NewRootEventID()
	log.Infof("%v auth attempt with key %v", cid, key.Type())

	p, err := s.certChecker.Authenticate(conn, key)
	if err != nil {
		s.elog.Log(eventID, events.NewAuthAttempt(conn, key, false, err))
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, err
	}
	if err := s.certChecker.CheckCert(conn.User(), key.(*ssh.Certificate)); err != nil {
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, trace.Wrap(err)
	}
	return p, nil
}

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	return s.srv.Close()
}

func (s *Server) Start() error {
	if !s.proxyMode {
		if len(s.cmdLabels) > 0 {
			s.updateLabels()
		}
		go s.heartbeatPresence()
	}
	return s.srv.Start()
}

func (s *Server) Wait() {
	s.srv.Wait()
}

func (s *Server) HandleRequest(r *ssh.Request) {
	log.Infof("recieved out-of-band request: %+v", r)
}

func (s *Server) HandleNewChan(sconn *ssh.ServerConn, nch ssh.NewChannel) {
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

	log.Infof("%v opened direct-tcpip channel: %#v", ctx, req)
	addr := fmt.Sprintf("%v:%d", req.Host, req.Port)
	log.Infof("%v connecting to %v", ctx, addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Infof("%v failed to connect to: %v, err: %v", ctx, addr, err)
		return
	}
	defer conn.Close()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(ch, conn)
		log.Infof("%v conn to channel copy closed, bytes transferred: %v, err: %v",
			ctx, written, err)
		ch.Close()
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(conn, ch)
		log.Infof("%v channel to conn copy closed, bytes transferred: %v, err: %v",
			ctx, written, err)
		conn.Close()
	}()
	wg.Wait()
	log.Infof("%v direct-tcp closed", ctx)
}

// handleSessionRequests handles out of band session requests once the session channel has been created
// this function's loop handles all the "exec", "subsystem" and "shell" requests.
func (s *Server) handleSessionRequests(sconn *ssh.ServerConn, ch ssh.Channel, in <-chan *ssh.Request) {
	// ctx holds the session context and all associated resources
	ctx := newCtx(s, sconn)
	log.Infof("%v opened session channel", ctx)

	// closeCh will close the connection and the context once the session closes
	var closeCh = func() {
		log.Infof("%v closing session channel", ctx)
		if err := ctx.Close(); err != nil {
			log.Infof("failed to close channel context: %v", err)
		}
		ch.Close()
	}
	for {
		select {
		case creq := <-ctx.close:
			// this means that the session process stopped and desires to close
			// the session and the channel e.g. shell has stopped and won't send
			// any data back to client any more
			log.Infof("close session request: %v", creq.reason)
			closeCh()
			return
		case req := <-in:
			if req == nil {
				// this will happen when the client closes/drops the connection
				log.Infof("%v client disconnected", ctx)
				closeCh()
				return
			}
			if err := s.dispatch(sconn, ch, req, ctx); err != nil {
				log.Infof("error dispatching request: %v, closing channel", err)
				replyError(req, err)
				closeCh()
				return
			}
			if req.WantReply {
				req.Reply(true, nil)
			}
		case result := <-ctx.result:
			// this means that exec process has finished and delivered the execution result, we send it back and close the session
			log.Infof("%v got execution result: %v", ctx, result)
			_, err := ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(result.code)}))
			if err != nil {
				log.Infof("%v %v failed to send exit status: %v", ctx, result.command, err)
			}
			closeCh()
			return
		}
	}
}

func (s *Server) fwdDispatch(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v dispatch(req=%v, wantReply=%v)", ctx, req.Type, req.WantReply)
	return fmt.Errorf("unsupported request type: %v", req.Type)
}

func (s *Server) dispatch(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v dispatch(req=%v, wantReply=%v)", ctx, req.Type, req.WantReply)
	if s.proxyMode {
		switch req.Type {
		case "subsystem":
			return s.handleSubsystem(sconn, ch, req, ctx)
		case "env":
			// we currently ignore setting any environment variables via SSH for security purposes
			return s.handleEnv(ch, req, ctx)
		default:
			return trace.Errorf("proxy doesn't support request type '%v'", req.Type)
		}
	}
	switch req.Type {
	case "exec":
		// exec is a remote execution of a program, does not use PTY
		return s.handleExec(ch, req, ctx)
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
		return fmt.Errorf("unsupported request type: %v", req.Type)
	}
}

func (s *Server) handleAgentForward(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	authChan, _, err := sconn.OpenChannel("auth-agent@openssh.com", nil)
	if err != nil {
		return err
	}
	log.Infof("%v opened agent channel", ctx)
	ctx.setAgent(agent.NewClient(authChan), authChan)
	return nil
}

func (s *Server) handleWinChange(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleWinChange()", ctx)
	t := ctx.getTerm()
	if t == nil {
		return fmt.Errorf("no PTY allocated for winChange")
	}
	return t.reqWinChange(req)
}

func (s *Server) handleSubsystem(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleSubsystem()", ctx)
	sb, err := parseSubsystemRequest(s, req)
	if err != nil {
		log.Infof("%v failed to parse subsystem request: %v", ctx, err)
		return err
	}
	go func() {
		if err := sb.execute(sconn, ch, req, ctx); err != nil {
			ctx.requestClose(fmt.Sprintf("%v failed to execute request, err: %v", ctx, err))
			log.Infof("%v failed to execute request, err: %v", err, ctx)
			return
		}
		ctx.requestClose(fmt.Sprintf("%v subsystem executed successfully", ctx))
	}()
	return nil
}

func (s *Server) handleShell(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleShell()", ctx)

	sid, ok := ctx.getEnv(sshutils.SessionEnvVar)
	if !ok {
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
		log.Errorf("%v handleEnv(err=%v)", s, err)
		return fmt.Errorf("failed to parse env request, error: %v", err)
	}
	log.Infof("%v handleEnv(%#v)", ctx, e)
	ctx.setEnv(e.Name, e.Value)
	return nil
}

func (s *Server) handlePTYReq(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handlePTYReq()", ctx)
	if term := ctx.getTerm(); term != nil {
		r, err := parsePTYReq(req)
		if err != nil {
			log.Infof("%v failed to parse PTY request: %v", ctx, err)
			replyError(req, err)
			return err
		}
		term.setWinsize(r.W, r.H)
		return nil
	}
	term, err := reqPTY(req)
	if err != nil {
		log.Infof("%v failed to allocate PTY: %v", ctx, err)
		replyError(req, err)
		return err
	}
	log.Infof("%v PTY allocated successfully", ctx)
	ctx.setTerm(term)
	return nil
}

func (s *Server) handleExec(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleExec()", ctx)
	e, err := parseExecRequest(req, ctx)
	if err != nil {
		log.Infof("%v failed to parse exec request: %v", ctx, err)
		replyError(req, err)
		return err
	}

	if scp.IsSCP(e.cmdName) {
		log.Infof("%v detected SCP command: %v", ctx, e.cmdName)
		if err := s.handleSCP(ch, req, ctx, e.cmdName); err != nil {
			log.Errorf("%v handleSCP() err: %v", ctx, err)
			return err
		}
		return ch.Close()
	}

	result, err := e.start(s.shell, ch)
	if err != nil {
		log.Infof("%v error starting command, %v", ctx, err)
		replyError(req, err)
	}
	if result != nil {
		log.Infof("%v %v result collected: %v", ctx, e, result)
		ctx.sendResult(*result)
	}
	// in case if result is nil and no error, this means that program is
	// running in the background
	go func() {
		log.Infof("%v %v waiting for result", ctx, e)
		result, err := e.wait()
		if err != nil {
			log.Infof("%v %v wait failed: %v", ctx, e, err)
		}
		if result != nil {
			log.Infof("%v %v result collected: %v", ctx, e, result)
			ctx.sendResult(*result)
		}
	}()
	return nil
}

func (s *Server) handleSCP(ch ssh.Channel, req *ssh.Request, ctx *ctx, args string) error {
	log.Infof("%v handleSCP(cmd=%v)", ctx, args)
	cmd, err := scp.ParseCommand(args)
	if err != nil {
		log.Errorf("%v failed to parse command: %v", ctx, cmd)
		return fmt.Errorf("failure: %v", err)
	}
	log.Infof("%v handleSCP(cmd=%#v)", ctx, cmd)
	srv, err := scp.New(*cmd)
	if err != nil {
		log.Errorf("%v failed to create scp server: %v", s, ctx)
		return err
	}
	// TODO(klizhentas) current version of handling exec is incorrect.
	// req.Reply should be sent as long as command start is done,
	// not at the end. This is my fix for SCP only:
	req.Reply(true, nil)
	if err := srv.Serve(ch); err != nil {
		log.Errorf("%v error serving: %v", ctx, err)
		return err
	}
	log.Infof("SCP serve finished", ctx)
	_, err = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{C: uint32(0)}))
	if err != nil {
		log.Infof("%v failed to send scp exit status: %v", ctx, err)
	}
	log.Infof("SCP sent exit status", ctx)
	return nil
}

func replyError(req *ssh.Request, err error) {
	if req.WantReply {
		req.Reply(false, []byte(err.Error()))
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

const (
	CertificatesCacheTTL = time.Minute * 30
)
