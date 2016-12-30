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
	"fmt"
	"net/http"

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

// connectReq is a request to open interactive SSH
// connection to remote server
type connectReq struct {
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
}

func newConnectHandler(req connectReq, ctx *SessionContext, site reversetunnel.RemoteSite) (*connectHandler, error) {
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
	return &connectHandler{
		req:    req,
		ctx:    ctx,
		site:   site,
		server: *server,
	}, nil
}

// connectHandler is a websocket to SSH proxy handler
type connectHandler struct {
	ctx    *SessionContext
	site   reversetunnel.RemoteSite
	up     *sshutils.Upstream
	req    connectReq
	ws     *websocket.Conn
	server services.Server
}

func (w *connectHandler) String() string {
	return fmt.Sprintf("connectHandler(%#v)", w.req)
}

func (w *connectHandler) Close() error {
	if w.ws != nil {
		w.ws.Close()
	}
	if w.up != nil {
		return w.up.Close()
	}
	return nil
}

// connect is called when a web browser wants to start piping an active terminal session
// io/out via the provided websocket
func (w *connectHandler) connect(ws *websocket.Conn) {
	// connectUpstream establishes an SSH connection to a requested node
	up, err := w.connectUpstream()
	if err != nil {
		log.Errorf("wsHandler: failed: %v", err)
		return
	}
	w.up = up
	w.ws = ws

	// PipeShell will be piping inputs/output to/from SSH connection (to the node)
	// and the websocket (to a browser)
	err = w.up.PipeShell(utils.NewWebSockWrapper(ws, utils.WebSocketTextMode),
		&sshutils.PTYReqParams{
			W: uint32(w.req.Term.W),
			H: uint32(w.req.Term.H),
		})

	log.Infof("pipe shell finished with: %v", err)
}

// resizePTYWindow is called when a brower resizes its window. Now the node
// needs to be notified via SSH
func (w *connectHandler) resizePTYWindow(params session.TerminalParams) error {
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
func (w *connectHandler) connectUpstream() (*sshutils.Upstream, error) {
	agent, err := w.ctx.GetAgent()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer agent.Close()
	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := w.site.ConnectToServer(
		w.server.GetAddr(), w.req.Login, []ssh.AuthMethod{ssh.PublicKeys(signers...)})
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
			Value: string(w.req.SessionID),
		}))
	return up, nil
}

func (w *connectHandler) Handler() http.Handler {
	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	return &websocket.Server{
		Handler: w.connect,
	}
}

func newWSHandler(host string, auth []string) *connectHandler {
	return &connectHandler{}
}
