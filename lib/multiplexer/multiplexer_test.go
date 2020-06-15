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

package multiplexer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type MuxSuite struct {
	signer ssh.Signer
}

var _ = fmt.Printf
var _ = check.Suite(&MuxSuite{})

func (s *MuxSuite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests()

	_, s.signer, err = utils.CreateCertificate("foo", ssh.HostCert)
	c.Assert(err, check.IsNil)
}

// TestMultiplexing tests basic use case of multiplexing TLS
// and SSH on the same listener socket
func (s *MuxSuite) TestMultiplexing(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: true,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "backend 1")
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	called := false
	sshHandler := sshutils.NewChanHandlerFunc(func(_ context.Context, _ *sshutils.ConnectionContext, nch ssh.NewChannel) {
		called = true
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		c.Assert(err, check.IsNil)
	})

	srv, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		sshHandler,
		[]ssh.Signer{s.signer},
		sshutils.AuthMethods{Password: pass("abc123")},
	)
	c.Assert(err, check.IsNil)
	go srv.Serve(mux.SSH())
	defer srv.Close()
	clt, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		Timeout:         time.Second,
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	})
	c.Assert(err, check.IsNil)
	defer clt.Close()

	// call new session to initiate opening new channel
	_, err = clt.NewSession()
	c.Assert(err, check.NotNil)
	// make sure the channel handler was called OK
	c.Assert(called, check.Equals, true)

	client := testClient(backend1)
	re, err := client.Get(backend1.URL)
	c.Assert(err, check.IsNil)
	defer re.Body.Close()
	bytes, err := ioutil.ReadAll(re.Body)
	c.Assert(err, check.IsNil)
	c.Assert(string(bytes), check.Equals, "backend 1")

	// Close mux, new requests should fail
	mux.Close()
	mux.Wait()

	// use new client to use new connection pool
	client = testClient(backend1)
	re, err = client.Get(backend1.URL)
	if err == nil {
		re.Body.Close()
	}
	c.Assert(err, check.NotNil)
}

// TestProxy tests Proxy line support protocol
func (s *MuxSuite) TestProxy(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: true,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, r.RemoteAddr)
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	remoteAddr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000}
	proxyLine := ProxyLine{
		Protocol:    TCP4,
		Source:      remoteAddr,
		Destination: net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000},
	}

	parsedURL, err := url.Parse(backend1.URL)
	c.Assert(err, check.IsNil)

	conn, err := net.Dial("tcp", parsedURL.Host)
	c.Assert(err, check.IsNil)
	defer conn.Close()
	// send proxy line first before establishing TLS connection
	_, err = fmt.Fprint(conn, proxyLine.String())
	c.Assert(err, check.IsNil)

	// upgrade connection to TLS
	tlsConn := tls.Client(conn, clientConfig(backend1))
	defer tlsConn.Close()

	// make sure the TLS call succeeded and we got remote address
	// correctly
	out, err := utils.RoundtripWithConn(tlsConn)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, remoteAddr.String())
}

// TestDisabledProxy makes sure the connection gets dropped
// when Proxy line support protocol is turned off
func (s *MuxSuite) TestDisabledProxy(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: false,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, r.RemoteAddr)
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	remoteAddr := net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000}
	proxyLine := ProxyLine{
		Protocol:    TCP4,
		Source:      remoteAddr,
		Destination: net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000},
	}

	parsedURL, err := url.Parse(backend1.URL)
	c.Assert(err, check.IsNil)

	conn, err := net.Dial("tcp", parsedURL.Host)
	c.Assert(err, check.IsNil)
	defer conn.Close()
	// send proxy line first before establishing TLS connection
	_, err = fmt.Fprint(conn, proxyLine.String())
	c.Assert(err, check.IsNil)

	// upgrade connection to TLS
	tlsConn := tls.Client(conn, clientConfig(backend1))
	defer tlsConn.Close()

	// make sure the TLS call failed
	_, err = utils.RoundtripWithConn(tlsConn)
	c.Assert(err, check.NotNil)
}

// TestTimeout tests client timeout - client dials, but writes nothing
// make sure server hangs up
func (s *MuxSuite) TestTimeout(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	config := Config{
		Listener:            listener,
		ReadDeadline:        time.Millisecond,
		EnableProxyProtocol: true,
	}
	mux, err := New(config)
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, r.RemoteAddr)
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	parsedURL, err := url.Parse(backend1.URL)
	c.Assert(err, check.IsNil)

	conn, err := net.Dial("tcp", parsedURL.Host)
	c.Assert(err, check.IsNil)
	defer conn.Close()

	time.Sleep(config.ReadDeadline + 5*time.Millisecond)
	// upgrade connection to TLS
	tlsConn := tls.Client(conn, clientConfig(backend1))
	defer tlsConn.Close()

	// roundtrip should fail on the timeout
	_, err = utils.RoundtripWithConn(tlsConn)
	c.Assert(err, check.NotNil)
}

// TestUnknownProtocol make sure that multiplexer closes connection
// with unknown protocol
func (s *MuxSuite) TestUnknownProtocol(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: true,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	conn, err := net.Dial("tcp", listener.Addr().String())
	c.Assert(err, check.IsNil)
	defer conn.Close()

	// try plain HTTP
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
	c.Assert(err, check.IsNil)

	// connection should be closed
	_, err = conn.Read(make([]byte, 1))
	c.Assert(err, check.Equals, io.EOF)
}

// TestDisableSSH disables SSH
func (s *MuxSuite) TestDisableSSH(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: true,
		DisableSSH:          true,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "backend 1")
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	_, err = ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		Timeout:         time.Second,
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	})
	c.Assert(err, check.NotNil)

	// TLS requests will succeed
	client := testClient(backend1)
	re, err := client.Get(backend1.URL)
	c.Assert(err, check.IsNil)
	defer re.Body.Close()
	bytes, err := ioutil.ReadAll(re.Body)
	c.Assert(err, check.IsNil)
	c.Assert(string(bytes), check.Equals, "backend 1")

	// Close mux, new requests should fail
	mux.Close()
	mux.Wait()

	// use new client to use new connection pool
	client = testClient(backend1)
	re, err = client.Get(backend1.URL)
	if err == nil {
		re.Body.Close()
	}
	c.Assert(err, check.NotNil)
}

// TestDisableTLS tests scenario with disabled TLS
func (s *MuxSuite) TestDisableTLS(c *check.C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)

	mux, err := New(Config{
		Listener:            listener,
		EnableProxyProtocol: true,
		DisableTLS:          true,
	})
	c.Assert(err, check.IsNil)
	go mux.Serve()
	defer mux.Close()

	backend1 := &httptest.Server{
		Listener: mux.TLS(),
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "backend 1")
		}),
		},
	}
	backend1.StartTLS()
	defer backend1.Close()

	called := false
	sshHandler := sshutils.NewChanHandlerFunc(func(_ context.Context, _ *sshutils.ConnectionContext, nch ssh.NewChannel) {
		called = true
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		c.Assert(err, check.IsNil)
	})

	srv, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		sshHandler,
		[]ssh.Signer{s.signer},
		sshutils.AuthMethods{Password: pass("abc123")},
	)
	c.Assert(err, check.IsNil)
	go srv.Serve(mux.SSH())
	defer srv.Close()
	clt, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		Timeout:         time.Second,
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	})
	c.Assert(err, check.IsNil)
	defer clt.Close()

	// call new session to initiate opening new channel
	_, err = clt.NewSession()
	c.Assert(err, check.NotNil)
	// make sure the channel handler was called OK
	c.Assert(called, check.Equals, true)

	client := testClient(backend1)
	re, err := client.Get(backend1.URL)
	if err == nil {
		re.Body.Close()
	}
	c.Assert(err, check.NotNil)

	// Close mux, new requests should fail
	mux.Close()
	mux.Wait()
}

// clientConfig returns tls client config from test http server
// set up to listen on TLS
func clientConfig(srv *httptest.Server) *tls.Config {
	cert, err := x509.ParseCertificate(srv.TLS.Certificates[0].Certificate[0])
	if err != nil {
		panic(err)
	}

	certpool := x509.NewCertPool()
	certpool.AddCert(cert)
	return &tls.Config{
		RootCAs:    certpool,
		ServerName: fmt.Sprintf("%v", cert.IPAddresses[0].String()),
	}
}

// testClient is a test HTTP client set up for TLS
func testClient(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: clientConfig(srv),
		},
	}
}

func pass(need string) sshutils.PasswordFunc {
	return func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if string(password) == need {
			return nil, nil
		}
		return nil, fmt.Errorf("passwords don't match")
	}
}
