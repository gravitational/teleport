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
	"net/http"
	"time"

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
	User string `json:"user"`
	// TerminalHeight is a PTY terminal height
	TerminalHeight int `json:"terminal_height"`
	// TerminalWidth is a PTY terminal width
	TerminalWidth int `json:"terminal_width"`
	// SessionID is a teleport session ID to join as
	SessionID string `json:"session_id"`
}

// wsHandler is a websocket to SSH proxy handler
type wsHandler struct {
	ctx  *sessionContext
	site reversetunnel.RemoteSite
	up   *sshutils.Upstream
	req  connectReq
}

func (w *wsHandler) Close() error {
	if w.up != nil {
		return w.up.Close()
	}
	return nil
}

func (w *wsHandler) connect(ws *websocket.Conn) {
	for {
		up, err := w.connectUpstream()
		if err != nil {
			ws.Write([]byte(err.Error() + "\n"))
			log.Errorf("wsHandler: failed: %v", err)
			continue
		}
		w.up = up
		err = w.up.PipeShell(ws)

		log.Infof("pipe shell finished with: %v", err)
		time.Sleep(time.Millisecond * 300)
		ws.Write([]byte("\n\rdisconnected\n\r"))
	}
}

func (w *wsHandler) connectUpstream() (*sshutils.Upstream, error) {
	methods, err := w.ctx.GetAuthMethods()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := w.site.ConnectToServer(w.req.Addr, w.req.User, methods)
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
			W: uint32(w.req.TerminalWidth),
			H: uint32(w.req.TerminalHeight),
		}))
	return up, nil
}

func (w *wsHandler) Handler() http.Handler {
	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	return &websocket.Server{
		Handler: w.connect,
	}
}

func newWSHandler(host string, auth []string) *wsHandler {
	return &wsHandler{}
}
