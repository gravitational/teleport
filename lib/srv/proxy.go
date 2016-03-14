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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// proxySubsys implements an SSH subsystem for proxying listening sockets from
// remote hosts to a proxy client (AKA port mapping)
type proxySubsys struct {
	srv       *Server
	host      string
	port      string
	closeC    chan struct{}
	error     error
	closeOnce sync.Once
}

// parseProxySubsys looks at the requested subsystem name and returns a fully configured
// proxy subsystem
//
// proxy subsystem name can take the following forms:
//
//  "proxy:host:22"          - standard SSH request to connect to  host:22 on the 1st site
//  "proxy:sitename@"        - Teleport request to connect to an auth server for site with name 'sitename'
//  "proxy:sitename@host:22" - Teleport request to connect to host:22 on site 'sitename'
//
func parseProxySubsys(name string, srv *Server) (*proxySubsys, error) {
	log.Debugf("parsing proxy request to %v", name)
	out := strings.Split(name, ":")
	if len(out) != 3 {
		return nil, trace.Wrap(teleport.BadParameter(
			"proxy", fmt.Sprintf("invalid format for proxy request: '%v', expected 'proxy:host:port'", name)))
	}
	return &proxySubsys{
		srv:    srv,
		host:   out[1],
		port:   out[2],
		closeC: make(chan struct{}),
	}, nil
}

func (t *proxySubsys) String() string {
	return fmt.Sprintf("proxySubsys(host=%v, port=%v)", t.host, t.port)
}

// start is called by Golang's ssh when it needs to engage this sybsystem (typically to establish
// a mapping connection between a client & remote node we're proxying to)
func (t *proxySubsys) start(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
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
		return trace.Wrap(teleport.ConvertSystemError(err))
	}
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer ch.Close()
		_, err = io.Copy(ch, conn)
	}()
	go func() {
		var err error
		defer func() {
			t.close(err)
		}()
		defer conn.Close()
		_, err = io.Copy(conn, ch)

	}()

	return nil
}

func (t *proxySubsys) close(err error) {
	t.closeOnce.Do(func() {
		t.error = err
		close(t.closeC)
	})
}

func (t *proxySubsys) wait() error {
	<-t.closeC
	return t.error
}
