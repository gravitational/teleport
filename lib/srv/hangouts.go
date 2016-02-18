package srv

import (
	"encoding/json"
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
	remoteSrv, err := t.srv.hangoutsTun.GetSite(t.host)
	if err != nil {
		return trace.Wrap(err)
	}

	hostKey, osUser, authPort, nodePort := remoteSrv.GetHangoutInfo()
	if hostKey == nil {
		return trace.Errorf("No hostkey for that hangout")
	}

	targetPort := ""
	if t.port == "auth" {
		targetPort = authPort
	}
	if t.port == "node" {
		targetPort = nodePort
	}

	// find matching server in the list of servers for this site
	servers, err := remoteSrv.GetServers()
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
		HostKey: *hostKey,
		OSUser:  osUser,
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
