package cp

import (
	"net/http"

	"github.com/gravitational/teleport/sshutils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/net/websocket"
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
	up, err := w.connectUpstream()
	if err != nil {
		log.Errorf("wsHandler: failed: %v", err)
		return
	}
	w.up = up
	err = w.up.PipeShell(ws)
	log.Infof("Pipe shell finished with: %v", err)
}

func (w *wsHandler) connectUpstream() (*sshutils.Upstream, error) {
	up, err := w.ctx.ConnectUpstream(w.addr)
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
