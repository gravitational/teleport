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

package web

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

// terminalRequest describes a request to crate a web-based terminal
// to a remote SSH server
type terminalRequest struct {
	// ServerID is a server id to connect to
	ServerID string `json:"server_id"`
	// User is linux username to connect as
	Login string `json:"login"`
	// Term sets PTY params like width and height
	Term session.TerminalParams `json:"term"`
	// SessionID is a teleport session ID to join as
	SessionID session.ID `json:"sid"`
	// Namespace is node namespace
	Namespace string `json:"namespace"`
	// Proxy server address
	ProxyHostPort string `json:"-"`
}

// newTerminal creates a web-based terminal based on WebSockets and returns a new
// terminalHandler
func newTerminal(req terminalRequest,
	ctx *SessionContext,
	site reversetunnel.RemoteSite) (*terminalHandler, error) {

	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := clt.GetNodes(req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var server *services.Server
	for i := range servers {
		node := servers[i]
		if node.GetName() == req.ServerID {
			server = &node
		}
	}
	if server == nil {
		return nil, trace.NotFound("node '%v' not found", req.ServerID)
	}
	if req.Login == "" {
		return nil, trace.BadParameter("login: missing login")
	}
	if req.Term.W <= 0 || req.Term.H <= 0 {
		return nil, trace.BadParameter("term: bad term dimensions")
	}
	return &terminalHandler{
		params: req,
		ctx:    ctx,
		site:   site,
		server: *server,
	}, nil
}

// terminalHandler connects together an SSH session with a web-based
// terminal via a web socket.
type terminalHandler struct {
	// params describe the terminal configuration
	params terminalRequest

	// ctx is a web session context for the currently logged in user
	ctx *SessionContext

	// ws is the websocket which is connected to stdin/out/err of the terminal shell
	ws *websocket.Conn

	// site/cluster we're connected to
	site reversetunnel.RemoteSite
	// server we're connected to
	server services.Server

	// sshClient is initialized after an SSH connection to a node is established
	sshSession *ssh.Session
}

func (t *terminalHandler) Close() error {
	if t.ws != nil {
		t.ws.Close()
	}
	if t.sshSession != nil {
		t.sshSession.Close()
	}
	return nil
}

// resizePTYWindow is called when a brower resizes its window. Now the node
// needs to be notified via SSH
func (t *terminalHandler) resizePTYWindow(params session.TerminalParams) error {
	if t.sshSession == nil {
		return nil
	}
	_, err := t.sshSession.SendRequest(
		// send SSH "window resized" SSH request:
		sshutils.WindowChangeReq,
		// no response needed
		false,
		ssh.Marshal(sshutils.WinChangeReqParams{
			W: uint32(params.W),
			H: uint32(params.H),
		}))
	if err != nil {
		log.Error(err)
	}
	return trace.Wrap(err)
}

// Run creates a new websocket connection to the SSH server and runs
// the "loop" piping the input/output of the SSH session into the
// js-based terminal.
func (t *terminalHandler) Run(w http.ResponseWriter, r *http.Request) {
	errToTerm := func(err error, w io.Writer) {
		fmt.Fprintf(w, "%s\n\r", err.Error())
		log.Error(err)
	}
	webSocketLoop := func(ws *websocket.Conn) {
		// get user's credentials using the SSH agent implementation which
		// retreives them directly from auth server API:
		agent, err := t.ctx.GetAgent()
		if err != nil {
			errToTerm(err, ws)
			return
		}
		defer agent.Close()
		principal, auth, err := t.getUserCredentials(agent)
		if err != nil {
			errToTerm(err, ws)
			return
		}
		// create teleport client:
		output := utils.NewWebSockWrapper(ws, utils.WebSocketTextMode)
		tc, err := client.NewClient(&client.Config{
			SkipLocalAuth:    true,
			AuthMethods:      []ssh.AuthMethod{auth},
			DefaultPrincipal: principal,
			HostLogin:        t.params.Login,
			Namespace:        t.params.Namespace,
			Stdout:           output,
			Stderr:           output,
			Stdin:            ws,
			ProxyHostPort:    t.params.ProxyHostPort,
			Host:             t.server.GetHostname(),
			Env:              map[string]string{sshutils.SessionEnvVar: string(t.params.SessionID)},
			HostKeyCallback:  func(string, net.Addr, ssh.PublicKey) error { return nil },
			ClientAddr:       r.RemoteAddr,
		})
		if err != nil {
			errToTerm(err, ws)
			return
		}
		// this callback will execute when a shell is created, it will give
		// us a reference to ssh.Client object
		tc.OnShellCreated = func(s *ssh.Session, c *ssh.Client, _ io.ReadWriteCloser) (bool, error) {
			t.sshSession = s
			t.resizePTYWindow(t.params.Term)
			go t.pullServerTermsize(c, output)
			return false, nil
		}
		if err = tc.SSH(context.TODO(), nil, false); err != nil {
			errToTerm(err, ws)
			return
		}
	}
	// this is to make sure we close web socket connections once
	// sessionContext that owns them expires
	t.ctx.AddClosers(t)
	defer t.ctx.RemoveCloser(t)

	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	ws := &websocket.Server{Handler: webSocketLoop}
	ws.ServeHTTP(w, r)
}

// getUserCredentials retreives the SSH credentials (certificate) for the currently logged in user
// from the auth server API.
//
func (t *terminalHandler) getUserCredentials(agent auth.AgentCloser) (string, ssh.AuthMethod, error) {
	var (
		cert *ssh.Certificate
		pub  ssh.PublicKey
	)
	// this loop-over-keys is only needed to find an ssh.Certificate (so we can pull
	// the 1st valid principal out of it)
	keys, err := agent.List()
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	for _, k := range keys {
		pub, _, _, _, err = ssh.ParseAuthorizedKey([]byte(k.String()))
		if err != nil {
			log.Warn(err)
			continue
		}
		cert, _ = pub.(*ssh.Certificate)
		if cert != nil {
			break
		}
	}
	// take the principal (SSH username) out of the returned certificate
	if cert == nil {
		return "", nil, trace.Errorf("unable to retreive the user certificate")
	}
	signers, err := agent.Signers()
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	return cert.ValidPrincipals[0], ssh.PublicKeys(signers...), nil
}

// this goroutine receives terminal window size changes in real time
// and stores the last size (example: "100:25") as a special "prefix"
// which gets added to future SSH reads by web clients.
func (t *terminalHandler) pullServerTermsize(c *ssh.Client, ws *utils.WebSockWrapper) {
	var buff [16]byte
	sshChan, _, err := c.OpenChannel("x-teleport-request-resize-events", nil)
	for err == nil {
		n, err := sshChan.Read(buff[:])
		if err != nil {
			break
		}
		ws.SetPrefix(buff[:n])
	}
}
