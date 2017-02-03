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

	ws *websocket.Conn
	up *sshutils.Upstream

	// site/cluster we're connected to
	site reversetunnel.RemoteSite
	// server we're connected to
	server services.Server
}

func (w *terminalHandler) Close() error {
	if w.ws != nil {
		w.ws.Close()
	}
	if w.up != nil {
		return w.up.Close()
	}
	return nil
}

// resizePTYWindow is called when a brower resizes its window. Now the node
// needs to be notified via SSH
func (w *terminalHandler) resizePTYWindow(params session.TerminalParams) error {
	_, err := w.up.GetSession().SendRequest(
		// send SSH "window resized" SSH request:
		sshutils.WindowChangeReq, false,
		ssh.Marshal(sshutils.WinChangeReqParams{
			W: uint32(params.W),
			H: uint32(params.H),
		}))
	return trace.Wrap(err)
}

// connectUpstream establishes the SSH connection to a requested SSH server (node)
func (t *terminalHandler) connectUpstream() (*sshutils.Upstream, error) {
	agent, err := t.ctx.GetAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer agent.Close()
	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := t.site.ConnectToServer(
		t.server.GetAddr(), t.params.Login, []ssh.AuthMethod{ssh.PublicKeys(signers...)})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	up, err := sshutils.NewUpstream(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this goroutine receives terminal window size changes in real time
	// and stores the last size (example: "100:25") as a special "prefix"
	// which gets added to future SSH reads by web clients.
	go func() {
		buff := make([]byte, 16)
		sshChan, _, err := up.GetClient().OpenChannel("x-teleport-request-resize-events", nil)
		for err == nil {
			n, err := sshChan.Read(buff)
			if err != nil {
				break
			}
			up.SetPrefix(buff[:n])
		}
		if err != nil {
			log.Error(err)
		}
	}()

	up.GetSession().SendRequest(
		sshutils.SetEnvReq, false,
		ssh.Marshal(sshutils.EnvReqParams{
			Name:  sshutils.SessionEnvVar,
			Value: string(t.params.SessionID),
		}))
	return up, nil
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
		up, err := t.connectUpstream()
		if err != nil {
			errToTerm(err, ws)
			return
		}
		t.up = up
		t.ws = ws

		// PipeShell will be piping inputs/output to/from SSH connection (to the node)
		// and the websocket (to a browser)
		err = t.up.PipeShell(utils.NewWebSockWrapper(ws, utils.WebSocketTextMode),
			&sshutils.PTYReqParams{
				W: uint32(t.params.Term.W),
				H: uint32(t.params.Term.H),
			})
		log.Infof("pipe shell finished with: %v", err)
	}

	// second version
	v2 := func(ws *websocket.Conn) {
		agent, err := t.ctx.GetAgent()
		if err != nil {
			errToTerm(err, ws)
			return
		}
		defer agent.Close()
		signers, err := agent.Signers()
		if err != nil {
			errToTerm(err, ws)
			return
		}
		host, _, err := net.SplitHostPort(t.server.GetAddr())
		if err != nil {
			errToTerm(err, ws)
			return
		}
		output := utils.NewWebSockWrapper(ws, utils.WebSocketTextMode)
		tc, err := client.NewClient(&client.Config{
			AuthMethods:   []ssh.AuthMethod{ssh.PublicKeys(signers...)},
			HostLogin:     t.params.Login,
			Namespace:     t.params.Namespace,
			Stdout:        output,
			Stderr:        output,
			Stdin:         ws,
			ProxyHostPort: t.params.ProxyHostPort,
			Host:          host,
		})
		if err != nil {
			errToTerm(err, ws)
			return
		}
		if err = tc.SSH(context.TODO(), nil, false); err != nil {
			errToTerm(err, ws)
			return
		}
	}

	if true {
		webSocketLoop = v2
	}

	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	ws := &websocket.Server{Handler: webSocketLoop}
	ws.ServeHTTP(w, r)
}
