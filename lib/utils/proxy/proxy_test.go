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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ProxySuite struct{}

var _ = check.Suite(&ProxySuite{})
var _ = fmt.Printf

func (s *ProxySuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *ProxySuite) TearDownSuite(c *check.C) {}
func (s *ProxySuite) SetUpTest(c *check.C)     {}
func (s *ProxySuite) TearDownTest(c *check.C)  {}

func (s *ProxySuite) TestDirectDial(c *check.C) {
	os.Unsetenv("https_proxy")
	os.Unsetenv("http_proxy")

	d := debugServer{}
	err := d.Start()
	c.Assert(err, check.IsNil)

	dialer := DialerFromEnvironment()
	client, err := dialer.Dial("tcp", d.Address(), &ssh.ClientConfig{})
	c.Assert(err, check.IsNil)

	session, err := client.NewSession()
	c.Assert(err, check.IsNil)
	defer session.Close()

	session.Run("date")
	session.Close()
	client.Close()

	c.Assert(d.Commands(), check.DeepEquals, []string{"date"})
}

func (s *ProxySuite) TestProxyDial(c *check.C) {
	dh := &debugHandler{}
	ts := httptest.NewServer(dh)
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	c.Assert(err, check.IsNil)
	os.Setenv("http_proxy", u.Host)

	ds := debugServer{}
	err = ds.Start()
	c.Assert(err, check.IsNil)

	dialer := DialerFromEnvironment()
	client, err := dialer.Dial("tcp", ds.Address(), &ssh.ClientConfig{})
	c.Assert(err, check.IsNil)

	session, err := client.NewSession()
	c.Assert(err, check.IsNil)
	defer session.Close()

	session.Run("date")
	session.Close()
	client.Close()

	c.Assert(ds.Commands(), check.DeepEquals, []string{"date"})
	c.Assert(dh.Count(), check.Equals, 1)
}

func (s *ProxySuite) TestGetProxyAddress(c *check.C) {
	var tests = []struct {
		inEnvName    string
		inEnvValue   string
		outProxyAddr string
	}{
		// 0 - valid, can be raw host:port
		{
			"http_proxy",
			"proxy:1234",
			"proxy:1234",
		},
		// 1 - valid, raw host:port works for https
		{
			"HTTPS_PROXY",
			"proxy:1234",
			"proxy:1234",
		},
		// 2 - valid, correct full url
		{
			"https_proxy",
			"https://proxy:1234",
			"proxy:1234",
		},
		// 3 - valid, http endpoint can be set in https_proxy
		{
			"https_proxy",
			"http://proxy:1234",
			"proxy:1234",
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		unsetEnv()
		os.Setenv(tt.inEnvName, tt.inEnvValue)
		p := getProxyAddress()
		unsetEnv()

		c.Assert(p, check.Equals, tt.outProxyAddr, comment)
	}
}

type debugServer struct {
	sync.Mutex

	addr     string
	commands []string
}

func (d *debugServer) Start() error {
	hostkey, err := d.generateHostKey()
	if err != nil {
		return err
	}

	freePorts, err := utils.GetFreeTCPPorts(10)
	if err != nil {
		return err
	}
	srvPort := freePorts[len(freePorts)-1]
	d.addr = "127.0.0.1:" + srvPort

	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(hostkey)

	listener, err := net.Listen("tcp", d.addr)
	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Debugf("Unable to accept: %v", err)
			}

			go d.handleConnection(conn, config)
		}
	}()

	return nil
}

func (d *debugServer) handleConnection(conn net.Conn, config *ssh.ServerConfig) error {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return err
	}
	go ssh.DiscardRequests(reqs)

	newchan := <-chans
	channel, requests, err := newchan.Accept()
	if err != nil {
		return err
	}

	req := <-requests
	err = d.handleRequest(channel, req)
	if err != nil {
		return err
	}

	channel.Close()
	sconn.Close()

	return nil
}

func (d *debugServer) handleRequest(channel ssh.Channel, req *ssh.Request) error {
	if req.Type != "exec" {
		req.Reply(false, nil)
		return trace.BadParameter("only exec type supported")
	}

	type execRequest struct {
		Command string
	}

	var e execRequest
	if err := ssh.Unmarshal(req.Payload, &e); err != nil {
		return err
	}

	out, err := exec.Command(e.Command).Output()
	if err != nil {
		return err
	}

	io.Copy(channel, bytes.NewReader(out))
	channel.Close()

	d.Lock()
	d.commands = append(d.commands, e.Command)
	d.Unlock()

	if req.WantReply {
		req.Reply(true, nil)
	}

	return nil
}

func (d *debugServer) generateHostKey() (ssh.Signer, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	var privateKeyBuffer bytes.Buffer
	err = pem.Encode(&privateKeyBuffer, privateKeyPEM)
	if err != nil {
		return nil, err
	}

	hostkey, err := ssh.ParsePrivateKey(privateKeyBuffer.Bytes())
	if err != nil {
		return nil, err
	}

	return hostkey, nil
}

func (d *debugServer) Commands() []string {
	d.Lock()
	defer d.Unlock()
	return d.commands
}

func (d *debugServer) Address() string {
	return d.addr
}

type debugHandler struct {
	sync.Mutex

	count int
}

func (d *debugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// validate http connect parameters
	if r.Method != http.MethodConnect {
		http.Error(w, fmt.Sprintf("%v not supported", r.Method), http.StatusInternalServerError)
		return
	}
	if r.Host == "" {
		http.Error(w, fmt.Sprintf("host not set"), http.StatusInternalServerError)
		return
	}

	// hijack request so we can get underlying connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "unable to hijack connection", http.StatusInternalServerError)
		return
	}
	sconn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// dial to host we want to proxy connection to
	dconn, err := net.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// write 200 OK to the source, but don't close the connection
	resp := &http.Response{
		Status:     "OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 0,
	}
	resp.Write(sconn)

	// copy from src to dst and dst to src
	done := make(chan bool)
	go func() {
		io.Copy(sconn, dconn)
		done <- true
	}()
	go func() {
		io.Copy(dconn, sconn)
		done <- true
	}()

	d.Lock()
	d.count = d.count + 1
	d.Unlock()

	// wait until done
	<-done
	<-done

	// close the connections
	sconn.Close()
	dconn.Close()
}

func (d *debugHandler) Count() int {
	d.Lock()
	defer d.Unlock()
	return d.count
}

func unsetEnv() {
	os.Unsetenv("http_proxy")
	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("https_proxy")
	os.Unsetenv("HTTPS_PROXY")
}
