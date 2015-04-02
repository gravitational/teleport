// srv implements SSH server that supports multiplexing, tunneling and key-based auth
package srv

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/events"
	"github.com/gravitational/teleport/sshutils"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	// Server implements SSH server that uses configuration backend and certificate-based authentication:
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
)

type Server struct {
	addr        sshutils.Addr
	certChecker ssh.CertChecker
	b           backend.Backend
	rr          resolver
	elog        lunk.EventLogger
	srv         *sshutils.Server
	hostSigner  ssh.Signer
	shell       string
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

// New returns an unstarted server
func New(addr sshutils.Addr, signers []ssh.Signer, b backend.Backend, options ...ServerOption) (*Server, error) {
	s := &Server{
		addr: addr,
		b:    b,
		rr:   &backendResolver{b: b},
	}
	s.certChecker = ssh.CertChecker{IsAuthority: s.isAuthority}

	for _, o := range options {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	if s.elog == nil {
		s.elog = utils.NullEventLogger
	}
	srv, err := sshutils.NewServer(
		addr, s, signers, sshutils.AuthMethods{PublicKey: s.keyAuth},
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

func (s *Server) heartbeatPresence() {
	for {
		srv := backend.Server{
			ID:   strings.Replace(s.addr.Addr, ":", "_", -1),
			Addr: s.addr.Addr,
		}
		if err := s.b.UpsertServer(srv, 6*time.Second); err != nil {
			log.Warningf("failed to announce %#v presence: %v", srv, err)
		}
		time.Sleep(3 * time.Second)
	}
}

func (s *Server) getUserCAKey() (ssh.PublicKey, error) {
	key, err := s.b.GetUserCAPub()
	if err != nil {
		return nil, err
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA public key '%v', err: %v", string(key), err)
	}
	return pubKey, nil
}

// isAuthority is called during checking the client key, to see if the signing key is the real CA authority key.
func (s *Server) isAuthority(auth ssh.PublicKey) bool {
	key, err := s.getUserCAKey()
	if err != nil {
		log.Errorf("failed to retrieve user authority key, err: %v", err)
		return false
	}
	if !keysEqual(key, auth) {
		log.Warningf("authority signature check failed, signing keys mismatch")
		return false
	}
	return true
}

// userKeys returns keys registered for a given user in a configuration backend
func (s *Server) userKeys(user string) ([]ssh.PublicKey, error) {
	authKeys, err := s.b.GetUserKeys(user)
	if err != nil {
		log.Errorf("failed to retrieve user keys for %v, err: %v", user, err)
		return nil, err
	}
	out := []ssh.PublicKey{}
	for _, ak := range authKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(ak.Value)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse user public key for user=%v, id=%v, key='%v', err: %v", user, ak.ID, string(ak.Value), err)
		}
		out = append(out, key)
	}

	// TODO(klizhentas) for optional security, we may restrict access to web session keys for some hosts
	webKeys, err := s.b.GetWebSessions(user)
	if err != nil {
		return nil, err
	}
	for _, wk := range webKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(wk.Pub)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to parse user public key for user=%v, id=%v, key='%v', err: %v", user, string(wk.Pub), err)
		}
		out = append(out, key)
	}
	return out, nil
}

// keyAuth implements SSH client authentication using public keys and is called by the server every time the client connects
func (s *Server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	cid := fmt.Sprintf("conn(%v->%v, user=%v)", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
	eventID := lunk.NewRootEventID()
	// log.Infof("%v auth attempt with key %v %v", cid, key.Type(), string(ssh.MarshalAuthorizedKey(key)))
	log.Infof("%v auth attempt with key %v", cid, key.Type())

	p, err := s.certChecker.Authenticate(conn, key)
	if err != nil {
		s.elog.Log(eventID, events.NewAuthAttempt(conn, key, false, err))
		log.Warningf("conn(%v->%v, user=%v) ERROR: Failed to authorize user %v, err: %v",
			conn.RemoteAddr(), conn.LocalAddr(), conn.User(), conn.User(), err)
		return nil, err
	}
	keys, err := s.userKeys(conn.User())
	if err != nil {
		log.Errorf("failed to retrieve user keys: %v", err)
		return nil, err
	}
	for _, k := range keys {
		if keysEqual(k, key) {
			log.Infof("%v SUCCESS auth", cid)
			s.elog.Log(eventID, events.NewAuthAttempt(conn, key, true, nil))
			return p, nil
		}
	}
	log.Infof("%v FAIL auth, no matching keys found", cid)
	return nil, fmt.Errorf("authentication failed")
}

// Close closes listening socket and stops accepting connections
func (s *Server) Close() error {
	return s.srv.Close()
}

func (s *Server) Start() error {
	go s.heartbeatPresence()
	return s.srv.Start()
}

func (s *Server) Wait() {
	s.srv.Wait()
}

func (s *Server) HandleRequest(r *ssh.Request) {
	log.Infof("recieved out-of-band request: %+v", r)
}

func (s *Server) HandleNewChan(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	cht := nch.ChannelType()
	switch cht {
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
		nch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", cht))
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
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(ch, conn)
		log.Infof("%v conn to channel copy closed, bytes transferred: %v, err: %v",
			ctx, written, err)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		written, err := io.Copy(conn, ch)
		log.Infof("%v channel to conn copy closed, bytes transferred: %v, err: %v",
			ctx, written, err)
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
			// this means that the session process stopped and desies to close the session and the channel
			// e.g. shell has stopped and won't send any data back to client any more
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
	switch req.Type {
	case "exec":
		// exec is a remote execution of a program, does not use PTY
		return s.handleExec(ch, req, ctx)
	case "pty-req":
		// SSH client asked to allocate PTY
		return s.handlePTYReq(ch, req, ctx)
	case "shell":
		// SSH client asked to launch shell, we allocate PTY and start shell session
		return s.handleShell(ch, req, ctx)
	case "env":
		// we currently ignore setting any environment variables via SSH for security purposes
		return s.handleEnv(ch, req, ctx)
	case "subsystem":
		// subsystems are SSH subsystems defined in http://tools.ietf.org/html/rfc4254 6.6
		// they are in essence SSH session extensions, allowing to implement new SSH commands
		return s.handleSubsystem(sconn, ch, req, ctx)
	case "window-change":
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
	sb, err := parseSubsystemRequest(req)
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

func (s *Server) handleShell(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleShell()", ctx)
	if ctx.getTerm() == nil {
		t, err := newTerm()
		if err != nil {
			log.Infof("handleShell failed to create term: %v", err)
			return err
		}
		ctx.setTerm(t)
	}
	t := ctx.getTerm()
	cmd := exec.Command(s.shell)
	// TODO(klizhentas) figure out linux user policy for launching shells, what user and environment
	// should we use to execute the shell? the simplest answer is to use current user and env, however
	// what if we are root?
	cmd.Env = []string{"TERM=xterm", fmt.Sprintf("HOME=%v", os.Getenv("HOME"))}
	if err := t.run(cmd); err != nil {
		log.Infof("%v failed to start shell: %v", ctx, err)
		return err
	}
	log.Infof("%v starting shell input/output streaming", ctx)
	// Pipe session to shell and visa-versa capturing input and output
	out := &bytes.Buffer{}
	// TODO(klizhentas) implement capturing as a thread safe factored out feature
	// what is important is that writes and reads to buffer should be protected
	// out contains captured command output
	go func() {
		written, err := io.Copy(io.MultiWriter(ch, out), t.pty)
		log.Infof("%v shell to channel copy closed, bytes written: %v, err: %v",
			ctx, written, err)
	}()
	go func() {
		written, err := io.Copy(t.pty, ch)
		log.Infof("%v channel to shell copy closed, bytes written: %v, err: %v",
			ctx, written, err)
	}()
	go func() {
		result, err := collectStatus(cmd, cmd.Wait())
		if err != nil {
			log.Errorf("%v wait failed: %v", ctx, err)
			ctx.emit(events.NewShell(s.shell, out, -1, err))
		}
		if result != nil {
			log.Infof("%v result collected: %v", ctx, result)
			ctx.emit(events.NewShell(s.shell, out, result.code, nil))
			ctx.sendResult(*result)
		}
	}()
	return nil
}

func (s *Server) handleEnv(ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v handleEnv()", ctx)
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

// constant time compare of the keys to avoid timing attacks
func keysEqual(ak, bk ssh.PublicKey) bool {
	a := ak.Marshal()
	b := bk.Marshal()
	return (len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1)
}
