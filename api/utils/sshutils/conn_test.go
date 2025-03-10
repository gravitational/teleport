/*
Copyright 2022 Gravitational, Inc.

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

package sshutils

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/cryptopatch"
)

type server struct {
	listener net.Listener
	config   *ssh.ServerConfig
	handler  func(*ssh.ServerConn)

	cSigner ssh.Signer
	hSigner ssh.Signer
}

func (s *server) Run(errC chan error) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			errC <- err
			return
		}

		go func() {
			sconn, _, _, err := ssh.NewServerConn(conn, s.config)
			if err != nil {
				errC <- err
				return
			}
			s.handler(sconn)
		}()
	}
}

func (s *server) Stop() error {
	return s.listener.Close()
}

func generateSigner(t *testing.T) ssh.Signer {
	private, err := cryptopatch.GenerateRSAKey(rand.Reader, 2048)
	require.NoError(t, err)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	}

	privatePEM := pem.EncodeToMemory(block)
	signer, err := ssh.ParsePrivateKey(privatePEM)
	require.NoError(t, err)

	return signer
}

func (s *server) GetClient(t *testing.T) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request) {
	conn, err := net.Dial("tcp", s.listener.Addr().String())
	require.NoError(t, err)

	sconn, nc, r, err := ssh.NewClientConn(conn, "", &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.cSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.hSigner.PublicKey()),
	})
	require.NoError(t, err)

	return sconn, nc, r
}

func newServer(t *testing.T, handler func(*ssh.ServerConn)) *server {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	cSigner := generateSigner(t)
	hSigner := generateSigner(t)

	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(hSigner)

	return &server{
		listener: listener,
		config:   config,
		handler:  handler,
		cSigner:  cSigner,
		hSigner:  hSigner,
	}
}

// TestTransportError ensures ConnectProxyTransport does not block forever
// when an error occurs while opening the transport channel.
func TestTransportError(t *testing.T) {
	handlerErrC := make(chan error, 1)
	serverErrC := make(chan error, 1)

	server := newServer(t, func(sconn *ssh.ServerConn) {
		_, _, err := ConnectProxyTransport(sconn, &DialReq{
			Address: "test", ServerID: "test",
		}, false)
		handlerErrC <- err
	})

	go server.Run(serverErrC)
	t.Cleanup(func() { require.NoError(t, server.Stop()) })

	sconn1, nc, _ := server.GetClient(t)
	t.Cleanup(func() { require.Error(t, sconn1.Close()) })

	channel := <-nc
	require.Equal(t, constants.ChanTransport, channel.ChannelType())

	sconn1.Close()
	err := timeoutErrC(t, handlerErrC, time.Second*5)
	require.Error(t, err)

	sconn2, nc, _ := server.GetClient(t)
	t.Cleanup(func() { require.NoError(t, sconn2.Close()) })

	channel = <-nc
	require.Equal(t, constants.ChanTransport, channel.ChannelType())

	err = channel.Reject(ssh.ConnectionFailed, "test reject")
	require.NoError(t, err)

	err = timeoutErrC(t, handlerErrC, time.Second*5)
	require.Error(t, err)

	select {
	case err = <-serverErrC:
		require.FailNow(t, err.Error())
	default:
	}
}

func timeoutErrC(t *testing.T, errC <-chan error, d time.Duration) error {
	timeout := time.NewTimer(d)
	select {
	case err := <-errC:
		return err
	case <-timeout.C:
		require.FailNow(t, "failed to receive on err channel in time")
	}

	return nil
}
