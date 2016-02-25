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
	"net"
	"net/http"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/sshutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

// connectReq is a request to open interactive SSH
// connection to remote server
type connectReq struct {
	// Addr is a host:port pair of the server to connect to
	Addr string `json:"addr"`
	// User is linux username to connect as
	Login string `json:"login"`
	// Term sets PTY params like width and height
	Term connectTerm `json:"term"`
	// SessionID is a teleport session ID to join as
	SessionID string `json:"session_id"`
}

type connectTerm struct {
	H int `json:"h"`
	W int `json:"w"`
}

func newConnectHandler(req connectReq, ctx *sessionContext, site reversetunnel.RemoteSite) (*connectHandler, error) {
	if _, _, err := net.SplitHostPort(req.Addr); err != nil {
		return nil, trace.Wrap(teleport.BadParameter("addr", "bad address format"))
	}
	if req.Login == "" {
		return nil, trace.Wrap(teleport.BadParameter("login", "missing login"))
	}
	if req.Term.W <= 0 || req.Term.H <= 0 {
		return nil, trace.Wrap(teleport.BadParameter("term", "bad term dimensions"))
	}
	return &connectHandler{
		req:  req,
		ctx:  ctx,
		site: site,
	}, nil
}

// connectHandler is a websocket to SSH proxy handler
type connectHandler struct {
	ctx  *sessionContext
	site reversetunnel.RemoteSite
	up   *sshutils.Upstream
	req  connectReq
}

func (w *connectHandler) Close() error {
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
	err = w.up.PipeShell(ws)
	log.Infof("pipe shell finished with: %v", err)
	ws.Write([]byte("\n\rdisconnected\n\r"))
}

func (w *connectHandler) connectUpstream() (*sshutils.Upstream, error) {
	methods, err := w.ctx.GetAuthMethods()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := w.site.ConnectToServer(w.req.Addr, w.req.Login, methods)
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
