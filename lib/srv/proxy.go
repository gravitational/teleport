package srv

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

// proxySubsys is an SSH subsystem for easy proxyneling through proxy server
// This subsystem creates a new TCP connection and connects ssh channel
// with this connection
type proxySubsys struct {
	srv  *Server
	host string
	port string
}

func parseProxySubsys(name string, srv *Server) (*proxySubsys, error) {
	out := strings.Split(name, ":")
	if len(out) != 3 {
		return nil, fmt.Errorf("invalid format for proxy request: '%v', expected 'proxy:host:port'", name)
	}
	return &proxySubsys{
		srv:  srv,
		host: out[1],
		port: out[2],
	}, nil
}

func (t *proxySubsys) String() string {
	return fmt.Sprintf("proxySubsys(host=%v, port=%v)", t.host, t.port)
}

func (t *proxySubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	remoteSrv, err := t.srv.proxyTun.FindSimilarSite(t.host)
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := remoteSrv.DialServer(t.host + ":" + t.port)

	go io.Copy(ch, conn)
	io.Copy(conn, ch)
	return nil
}
