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

	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/websocket"
)

// wsHandler
type wsHandler struct {
	ctx  Context
	addr string
	up   *sshutils.Upstream
	sid  string
}

func (w *wsHandler) Close() error {
	if w.up != nil {
		return w.up.Close()
	}
	return nil
}

func (w *wsHandler) connect(ws *websocket.Conn) {
	for {
		ws.Write([]byte("Enter username:\n\r"))
		username := ""
		buf := make([]byte, 1)
		for {
			ws.Read(buf)
			if (buf[0] != 10) && (buf[0] != 13) {
				username += string(buf)
				ws.Write(buf)
			} else {
				break
			}
		}
		ws.Write([]byte("\n\r"))

		up, err := w.connectUpstream(username)
		if err != nil {
			ws.Write([]byte(err.Error() + "\n"))
			log.Errorf("wsHandler: failed: %v", err)
			continue
		}
		w.up = up
		err = w.up.PipeShell(ws)

		log.Infof("Pipe shell finished with: %v", err)
		time.Sleep(time.Millisecond * 300)
		ws.Write([]byte("\n\rDisconnected\n\r"))
	}
}

func (w *wsHandler) connectUpstream(user string) (*sshutils.Upstream, error) {
	up, err := w.ctx.ConnectUpstream(w.addr, user)
	if err != nil {
		return nil, err
	}
	up.GetSession().SendRequest(
		sshutils.SetEnvReq, false,
		ssh.Marshal(sshutils.EnvReqParams{
			Name:  sshutils.SessionEnvVar,
			Value: w.sid,
		}))
	up.GetSession().SendRequest(
		sshutils.PTYReq, false,
		ssh.Marshal(sshutils.PTYReqParams{
			W: 120,
			H: 32,
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
