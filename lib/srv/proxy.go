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

	"github.com/gravitational/teleport/lib/reversetunnel"
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
	siteName  string
	closeC    chan struct{}
	error     error
	closeOnce sync.Once
}

// parseProxySubsys looks at the requested subsystem name and returns a fully configured
// proxy subsystem
//
// proxy subsystem name can take the following forms:
//  "proxy:host:22"          - standard SSH request to connect to  host:22 on the 1st site
//  "proxy:@sitename"        - Teleport request to connect to an auth server for site with name 'sitename'
//  "proxy:host:22@sitename" - Teleport request to connect to host:22 on site 'sitename'
func parseProxySubsys(name string, srv *Server) (*proxySubsys, error) {
	log.Infof("parse_proxy_subsys(%s)", name)
	var (
		siteName   string
		host       string
		port       string
		paramError = trace.BadParameter("invalid format for proxy request: '%v', expected 'proxy:host:port@site'", name)
	)
	const prefix = "proxy:"
	// get rid of 'proxy:' prefix:
	if strings.Index(name, prefix) != 0 {
		return nil, trace.Wrap(paramError)
	}
	name = strings.TrimPrefix(name, prefix)
	// find the site name in the argument:
	parts := strings.Split(name, "@")
	if len(parts) > 1 {
		siteName = strings.Join(parts[1:], "@")
		name = parts[0]
	}
	// find host & port in the arguments:
	host, port, err := net.SplitHostPort(name)
	if siteName == "" && err != nil {
		return nil, trace.Wrap(paramError)
	}
	// validate siteName
	if siteName != "" && srv.proxyTun != nil {
		_, err := srv.proxyTun.GetSite(siteName)
		if err != nil {
			return nil, trace.BadParameter("unknown site '%s'", siteName)
		}
	}
	return &proxySubsys{
		srv:      srv,
		host:     host,
		port:     port,
		siteName: siteName,
		closeC:   make(chan struct{}),
	}, nil
}

func (t *proxySubsys) String() string {
	return fmt.Sprintf("proxySubsys(site=%s, host=%s, port=%s)", t.siteName, t.host, t.port)
}

// start is called by Golang's ssh when it needs to engage this sybsystem (typically to establish
// a mapping connection between a client & remote node we're proxying to)
func (t *proxySubsys) start(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Debugf("proxy_subsystem: execute(remote: %v, local: %v) for subsystem with (%s:%s)",
		sconn.RemoteAddr(), sconn.LocalAddr(), t.host, t.port)
	var (
		site   reversetunnel.RemoteSite
		err    error
		tunnel = t.srv.proxyTun
	)
	// get the site by name:
	if t.siteName != "" {
		site, err = tunnel.GetSite(t.siteName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// connecting to a specific host:
	if t.host != "" {
		// no site given? use the 1st one:
		if site == nil {
			sites := tunnel.GetSites()
			if len(sites) == 0 {
				log.Errorf("this proxy is not connected to any sites")
				return trace.Errorf("no connected sites")
			}
			site = sites[0]
			log.Infof("proxy_subsystem: no site specified. connecting to default: '%s'", site.GetName())
		}
		return t.proxyToHost(site, ch)
	}
	// connect to a site's auth server:
	return t.proxyToSite(site, ch)
}

// proxyToSite establishes a proxy connection from the connected SSH client to the
// auth server of the requested site
func (t *proxySubsys) proxyToSite(site reversetunnel.RemoteSite, ch ssh.Channel) error {
	var (
		err  error
		conn net.Conn
	)
	siteClient, err := site.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	authServers, err := siteClient.GetAuthServers()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, authServer := range authServers {
		conn, err = site.Dial("tcp", authServer.Addr)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infof("proxy_subsystem: connected to auth server: %v", authServer.Addr)
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
	return err
}

// proxyToHost establishes a proxy connection from the connected SSH client to the
// requested remote node (t.host:t.port) via the given site
func (t *proxySubsys) proxyToHost(site reversetunnel.RemoteSite, ch ssh.Channel) error {
	siteClient, err := site.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}
	servers, err := siteClient.GetNodes()
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
	conn, err := site.Dial("tcp", serverAddr)
	if err != nil {
		return trace.ConvertSystemError(err)
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
