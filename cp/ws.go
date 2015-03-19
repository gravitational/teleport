package cp

import (
	"fmt"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/net/websocket"
	"net/http"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/sshutils"
)

// wsHandler
type wsHandler struct {
	authServers []string
	ctx         *ctx
	addr        string
	up          *sshutils.Upstream
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
	w.up.PipeShell(ws)
}

func (w *wsHandler) connectUpstream() (*sshutils.Upstream, error) {
	agent, err := w.ctx.clt.GetAgent()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %v", err)
	}
	signers, err := agent.Signers()
	if err != nil {
		return nil, fmt.Errorf("no signers: %v", err)
	}
	return sshutils.DialUpstream(w.ctx.user, w.addr, signers)
}

func (w *wsHandler) Handler() http.Handler {
	return websocket.Handler(w.connect)
}

func newWSHandler(host string, auth []string) *wsHandler {
	return &wsHandler{}
}
