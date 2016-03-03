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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"

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
	SessionID string `json:"sid"`
}

func newConnectHandler(req connectReq, ctx *sessionContext, site reversetunnel.RemoteSite) (*connectHandler, error) {
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := clt.GetServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var server *services.Server
	for i := range servers {
		node := servers[i]
		if node.ID == req.ServerID {
			server = &node
		}
	}
	if server == nil {
		return nil, trace.Wrap(
			teleport.NotFound(
				fmt.Sprintf("node '%v' not found", req.ServerID)))
	}
	if req.Login == "" {
		return nil, trace.Wrap(teleport.BadParameter("login", "missing login"))
	}
	if req.Term.W <= 0 || req.Term.H <= 0 {
		return nil, trace.Wrap(teleport.BadParameter("term", "bad term dimensions"))
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
	ctx    *sessionContext
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
	w.ws.Close()
	if w.up != nil {
		return w.up.Close()
	}
	return nil
}

func (w *connectHandler) connect(ws *websocket.Conn) {
	up, err := w.connectUpstream()
	if err != nil {
		log.Errorf("wsHandler: failed: %v", err)
		return
	}
	w.up = up
	w.ws = ws
	err = w.up.PipeShell(ws)
	log.Infof("pipe shell finished with: %v", err)
	ws.Write([]byte("\n\rdisconnected\n\r"))
}

func (w *connectHandler) resizePTYWindow(params session.TerminalParams) error {
	_, err := w.up.GetSession().SendRequest(
		sshutils.WindowChangeReq, false,
		ssh.Marshal(sshutils.WinChangeReqParams{
			W: uint32(params.W),
			H: uint32(params.H),
		}))
	return trace.Wrap(err)
}

func (w *connectHandler) connectUpstream() (*sshutils.Upstream, error) {
	methods, err := w.ctx.GetAuthMethods()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := w.site.ConnectToServer(w.server.Addr, w.req.Login, methods)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	up, err := sshutils.NewUpstream(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	up.GetSession().SendRequest(
		sshutils.SetEnvReq, false,
		ssh.Marshal(sshutils.EnvReqParams{
			Name:  sshutils.SessionEnvVar,
			Value: w.req.SessionID,
		}))
	up.GetSession().SendRequest(
		sshutils.PTYReq, false,
		ssh.Marshal(sshutils.PTYReqParams{
			W: uint32(w.req.Term.W),
			H: uint32(w.req.Term.H),
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
