package srv

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/services"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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
		return nil, trace.Errorf("invalid format for proxy request: '%v', expected 'proxy:host:port'", name)
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

	// find matching server in the list of servers for this site
	clt, err := remoteSrv.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	servers, err := clt.GetServers()
	if err != nil {
		return trace.Wrap(err)
	}

	serverAddr := fmt.Sprintf("%v:%v", t.host, t.port)
	var server *services.Server
	for i := range servers {
		ip, port, err := net.SplitHostPort(servers[i].Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		// match either by hostname of ip, based on the match
		if (t.host == ip || t.host == servers[i].Hostname) && port == t.port {
			server = &servers[i]
			break
		}
	}
	if server == nil {
		return trace.Errorf("server %v not found", serverAddr)
	}

	// we must dial by server IP address because hostname
	// may not be actually DNS resolvable
	conn, err := remoteSrv.DialServer(server.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := io.Copy(ch, conn)
		if err != nil {
			log.Errorf(err.Error())
		}
		ch.Close()
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, ch)
		if err != nil {
			log.Errorf(err.Error())
		}
		conn.Close()
	}()

	wg.Wait()

	return nil
}
