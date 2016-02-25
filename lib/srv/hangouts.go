package srv

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// hangoutsSubsys is an SSH subsystem for easy proxyneling through proxy server
// This subsystem creates a new TCP connection and connects ssh channel
// with this connection
type hangoutsSubsys struct {
	srv  *Server
	host string
	port string
}

type HangoutEndpointInfo struct {
	HostKey services.CertAuthority
	OSUser  string
}

func parseHangoutsSubsys(name string, srv *Server) (*hangoutsSubsys, error) {
	out := strings.Split(name, ":")
	if len(out) != 3 {
		return nil, trace.Errorf("invalid format for proxy request: '%v', expected 'proxy:host:port'", name)
	}
	return &hangoutsSubsys{
		srv:  srv,
		host: out[1],
		port: out[2],
	}, nil
}

func (t *hangoutsSubsys) String() string {
	return fmt.Sprintf("hangoutsSubsys(host=%v, port=%v)", t.host, t.port)
}

func (t *hangoutsSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	remoteSrv, err := t.srv.proxyTun.GetSite(t.host)
	if err != nil {
		return trace.Wrap(err)
	}

	//hostKey, osUser, authPort, nodePort := remoteSrv.GetHangoutInfo()
	hangoutInfo, err := remoteSrv.GetHangoutInfo()
	if err != nil {
		return trace.Wrap(err)
	}

	targetPort := ""
	if t.port == utils.HangoutAuthPortAlias {
		targetPort = hangoutInfo.AuthPort
	}
	if t.port == utils.HangoutNodePortAlias {
		targetPort = hangoutInfo.NodePort
	}

	// find matching server in the list of servers for this site
	clt, err := remoteSrv.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	servers, err := auth.RetryingClient(clt, 20).GetServers()
	if err != nil {
		return trace.Wrap(err)
	}

	var server *services.Server
	for i := range servers {
		log.Infof("%v %v", servers[i].Addr, servers[i].Hostname)
		ip, port, err := net.SplitHostPort(servers[i].Addr)
		if err != nil {
			return trace.Wrap(err)
		}
		// match either by hostname of ip, based on the match
		if (t.host == ip || t.host == servers[i].Hostname) && targetPort == port {
			server = &servers[i]
			break
		}
	}
	if server == nil {
		return trace.Errorf("server %v:%v not found", t.host, t.port)
	}

	// send target server host key so user can check the server
	endpointInfo := HangoutEndpointInfo{
		HostKey: *(hangoutInfo.HostKey),
		OSUser:  hangoutInfo.OSUser,
	}
	data, err := json.Marshal(endpointInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = ch.Write(data)
	if err != nil {
		return trace.Wrap(err)
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
