/*
Copyright 2016 Gravitational, Inc.

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

// proxySubsys implements an SSH subsystem for proxying listening sockets from
// remote hosts to a proxy client (AKA port mapping)
type proxySubsys struct {
	srv  *Server
	host string
	port string
}

func parseProxySubsys(name string, srv *Server) (*proxySubsys, error) {
	log.Debugf("prasing proxy request to %s", name)
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

// execute is called by Golang's ssh when it needs to engage this sybsystem (typically to establish
// a mapping connection between a client & remote node we're proxying to)
func (t *proxySubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Debugf("proxySubsys.execute(remote: %v, local: %v) for subsystem with (%s:%s)",
		sconn.RemoteAddr(), sconn.LocalAddr(), t.host, t.port)

	// TODO: currently "teleport sites" are not exposed to end users (tctl, tsh, etc)
	// and there's always only one "site" and that's the one the auth service runs on
	remoteSrv, err := t.srv.proxyTun.FindSimilarSite(t.host)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := remoteSrv.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	servers, err := clt.GetServers()
	if err != nil {
		return trace.Wrap(err)
	}
	// enumerate and try to find a server with a matching name
	serverAddr := net.JoinHostPort(t.host, t.port)
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
	if server != nil {
		serverAddr = server.Addr
	}

	// we must dial by server IP address because hostname
	// may not be actually DNS resolvable
	conn, err := remoteSrv.Dial("tcp", serverAddr)
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
