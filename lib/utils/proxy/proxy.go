/*
Copyright 2017 Gravitational, Inc.

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
package proxy

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"
)

// NewClientConnWithDeadline establishes new client connection with specified deadline
func NewClientConnWithDeadline(conn net.Conn, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	if config.Timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(config.Timeout))
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		return nil, err
	}
	if config.Timeout > 0 {
		conn.SetReadDeadline(time.Time{})
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// DialWithDeadline works around the case when net.DialWithTimeout
// succeeds, but key exchange hangs. Setting deadline on connection
// prevents this case from happening
func DialWithDeadline(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, addr, config.Timeout)
	if err != nil {
		return nil, err
	}
	return NewClientConnWithDeadline(conn, addr, config)
}

// A Dialer is a means for a client to establish a SSH connection.
type Dialer interface {
	// Dial establishes a client connection to a SSH server.
	Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error)
}

type directDial struct{}

// Dial calls ssh.Dial directly.
func (d directDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	return DialWithDeadline(network, addr, config)
}

type proxyDial struct {
	proxyHost string
}

// Dial first connects to a proxy, then uses the connection to establish a new
// SSH connection.
func (d proxyDial) Dial(network string, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	// build a proxy connection first
	pconn, err := dialProxy(d.proxyHost, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Timeout > 0 {
		pconn.SetReadDeadline(time.Now().Add(config.Timeout))
	}
	// do the same as ssh.Dial but pass in proxy connection
	c, chans, reqs, err := ssh.NewClientConn(pconn, addr, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.Timeout > 0 {
		pconn.SetReadDeadline(time.Time{})
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// DialerFromEnvironment returns a Dial function. If the https_proxy or http_proxy
// environment variable are set, it returns a function that will dial through
// said proxy server. If neither variable is set, it will connect to the SSH
// server directly.
func DialerFromEnvironment() Dialer {
	// try and get proxy addr from the environment
	proxyAddr := getProxyAddress()

	// if no proxy settings are in environment return regular ssh dialer,
	// otherwise return a proxy dialer
	if proxyAddr == "" {
		log.Debugf("[HTTP PROXY] No proxy set in environment, returning direct dialer.")
		return directDial{}
	}
	log.Debugf("[HTTP PROXY] Found proxy %q in environment, returning proxy dialer.", proxyAddr)
	return proxyDial{proxyHost: proxyAddr}
}

func dialProxy(proxyAddr string, addr string) (net.Conn, error) {
	ctx := context.Background()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		log.Warnf("[HTTP PROXY] Unable to dial to proxy: %v: %v", proxyAddr, err)
		return nil, trace.ConvertSystemError(err)
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}
	err = connectReq.Write(conn)
	if err != nil {
		log.Warnf("[HTTP PROXY] Unable to write to proxy: %v", err)
		return nil, trace.Wrap(err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		log.Warnf("[HTTP PROXY] Unable to read response: %v", err)
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		f := strings.SplitN(resp.Status, " ", 2)
		conn.Close()
		return nil, trace.BadParameter("Unable to proxy connection, StatusCode %v: %v", resp.StatusCode, f[1])
	}

	return conn, nil
}

func getProxyAddress() string {
	envs := []string{
		teleport.HTTPSProxy,
		strings.ToLower(teleport.HTTPSProxy),
		teleport.HTTPProxy,
		strings.ToLower(teleport.HTTPProxy),
	}

	for _, v := range envs {
		addr := os.Getenv(v)
		if addr != "" {
			proxyaddr, err := parse(addr)
			if err != nil {
				log.Debugf("[HTTP PROXY] Unable to parse environment variable %q: %q.", v, addr)
				continue
			}
			log.Debugf("[HTTP PROXY] Successfully parsed environment variable %q: %q to %q", v, addr, proxyaddr)
			return proxyaddr
		}
	}

	log.Debugf("[HTTP PROXY] No valid environment variables found.")
	return ""
}

// parse will extract the host:port of the proxy to dial to. If the
// value is not prefixed by "http", then it will prepend "http" and try.
func parse(addr string) (string, error) {
	proxyurl, err := url.Parse(addr)
	if err != nil || !strings.HasPrefix(proxyurl.Scheme, "http") {
		proxyurl, err = url.Parse("http://" + addr)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	return proxyurl.Host, nil
}
