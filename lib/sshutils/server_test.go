/*
Copyright 2015 Gravitational, Inc.

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
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/observability/tracing"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestStartStop(t *testing.T) {
	t.Parallel()

	_, signer, err := cert.CreateCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	called := false
	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		called = true

		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		assert.NoError(t, err)
	})

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{signer},
		AuthMethods{Password: pass("abc123")},
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	// Wait for SSH server to successfully shutdown, fail if it does not within
	// the timeout period.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		srv.Wait(ctx)
		require.NoError(t, ctx.Err())
	})

	clientConfig := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), clientConfig)
	require.NoError(t, err)
	defer clt.Close()

	// Call new session to initiate opening new channel. This should get
	// rejected and fail.
	_, err = clt.NewSession()
	require.Error(t, err)
	require.ErrorContains(t, err, "nothing to see here")
	require.True(t, called)

	require.NoError(t, srv.Close())
}

// TestShutdown tests graceul shutdown feature
func TestShutdown(t *testing.T) {
	t.Parallel()

	_, signer, err := cert.CreateCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	closeContext, cancel := context.WithCancel(context.TODO())
	fn := NewChanHandlerFunc(func(_ context.Context, ccx *ConnectionContext, nch ssh.NewChannel) {
		ch, _, err := nch.Accept()
		require.NoError(t, err)
		defer ch.Close()

		<-closeContext.Done()
		ccx.ServerConn.Close()
	})

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{signer},
		AuthMethods{Password: pass("abc123")},
		SetShutdownPollPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	clientConfig := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), clientConfig)
	require.NoError(t, err)
	defer clt.Close()

	// call new session to initiate opening new channel
	_, err = clt.NewSession()
	require.NoError(t, err)

	// context will timeout because there is a connection around
	ctx, ctxc := context.WithTimeout(context.TODO(), 50*time.Millisecond)
	defer ctxc()
	require.True(t, trace.IsConnectionProblem(srv.Shutdown(ctx)))

	// now shutdown will return
	cancel()
	ctx2, ctxc2 := context.WithTimeout(context.TODO(), time.Second)
	defer ctxc2()
	require.NoError(t, srv.Shutdown(ctx2))

	// shutdown is re-entrable
	ctx3, ctxc3 := context.WithTimeout(context.TODO(), time.Second)
	defer ctxc3()
	require.NoError(t, srv.Shutdown(ctx3))
}

func TestConfigureCiphers(t *testing.T) {
	t.Parallel()

	_, signer, err := cert.CreateCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		assert.NoError(t, err)
	})

	// create a server that only speaks aes128-ctr
	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{signer},
		AuthMethods{Password: pass("abc123")},
		SetCiphers([]string{"aes128-ctr"}),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	// client only speaks aes256-ctr, should fail
	cc := ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes256-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}
	_, err = ssh.Dial("tcp", srv.Addr(), &cc)
	require.Error(t, err, "cipher mismatch, should fail, got nil")

	// client only speaks aes128-ctr, should succeed
	cc = ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes128-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), &cc)
	require.NoError(t, err)
	defer clt.Close()
}

// TestHostSigner makes sure Teleport can not be started with a invalid host
// certificate. The main check is the certificate algorithms.
func TestHostSignerFIPS(t *testing.T) {
	t.Parallel()

	_, signer, err := cert.CreateCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	_, ellipticSigner, err := cert.CreateEllipticCertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		assert.NoError(t, err)
	})

	var tests = []struct {
		inSigner ssh.Signer
		inFIPS   bool
		assert   require.ErrorAssertionFunc
	}{
		// ECDSA when in FIPS mode should fail.
		{
			inSigner: ellipticSigner,
			inFIPS:   true,
			assert:   require.Error,
		},
		// RSA when in FIPS mode is okay.
		{
			inSigner: signer,
			inFIPS:   true,
			assert:   require.NoError,
		},
		// ECDSA when in not FIPS mode should succeed.
		{
			inSigner: ellipticSigner,
			inFIPS:   false,
			assert:   require.NoError,
		},
		// RSA when in not FIPS mode should succeed.
		{
			inSigner: signer,
			inFIPS:   false,
			assert:   require.NoError,
		},
	}
	for _, tt := range tests {
		_, err := NewServer(
			"test",
			utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
			fn,
			[]ssh.Signer{tt.inSigner},
			AuthMethods{Password: pass("abc123")},
			SetCiphers([]string{"aes128-ctr"}),
			SetFIPS(tt.inFIPS),
		)
		tt.assert(t, err)
	}
}

// TestConnectionWrapper_Read makes sure connectionWrapper can correctly process ProxyHelloSignature and PROXY protocol
// on the wire.
func TestConnectionWrapper_Read(t *testing.T) {
	testCases := []struct {
		desc     string
		sendData []byte
	}{
		{
			desc:     "Plain connection without any special headers",
			sendData: nil,
		},
		{
			desc:     "Sending ProxyHelloSignature",
			sendData: getProxyHelloSignaturePayload(t),
		},
		{
			desc:     "Sending PROXY header",
			sendData: getPROXYProtocolPayload(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { listener.Close() })

			go startSSHServer(t, listener)

			conn, err := net.Dial("tcp", listener.Addr().String())
			require.NoError(t, err)

			_, err = conn.Write(tc.sendData)
			require.NoError(t, err)

			sconn, nc, r, err := ssh.NewClientConn(conn, "", &ssh.ClientConfig{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         time.Second,
			})
			require.NoError(t, err)
			require.Equal(t, "SSH-2.0-Go", string(sconn.ServerVersion()))

			client := ssh.NewClient(sconn, nc, r)
			require.NoError(t, err)

			// Make sure SSH connection works correctly
			ok, response, err := client.SendRequest("echo", true, []byte("beep"))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, "beep", string(response))
		})
	}
}

func getPROXYProtocolPayload() []byte {
	proxyV2Prefix := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	// source=127.0.0.1:12345 destination=127.0.0.2:42
	sampleIPv4Addresses := []byte{0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0x30, 0x39, 0x00, 0x2A}
	// {0x21, 0x11, 0x00, 0x0C} - 4 bits version, 4 bits command, 4 bits address family, 4 bits protocol, 16 bits length
	sampleProxyV2Line := bytes.Join([][]byte{proxyV2Prefix, {0x21, 0x11, 0x00, 0x0C}, sampleIPv4Addresses}, nil)

	return sampleProxyV2Line
}

func getProxyHelloSignaturePayload(t *testing.T) []byte {
	t.Helper()

	hp := &apisshutils.HandshakePayload{
		ClientAddr:     "127.0.0.1:12345",
		TracingContext: tracing.PropagationContextFromContext(context.Background()),
	}
	payloadJSON, err := json.Marshal(hp)
	require.NoError(t, err)

	return []byte(fmt.Sprintf("%s%s\x00", constants.ProxyHelloSignature, payloadJSON))
}

func startSSHServer(t *testing.T, listener net.Listener) {
	nConn, err := listener.Accept()
	assert.NoError(t, err)

	t.Cleanup(func() { nConn.Close() })

	wConn := wrapConnection(nConn, nil, "", clockwork.NewRealClock(), logrus.New())

	block, _ := pem.Decode(fixtures.LocalhostKey)
	pkey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	assert.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(pkey)
	assert.NoError(t, err)

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	conn, _, reqs, err := ssh.NewServerConn(wConn, config)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	t.Cleanup(func() { conn.Close() })

	go func() {
		for newReq := range reqs {
			if newReq.Type == "echo" {
				err := newReq.Reply(true, newReq.Payload)
				assert.NoError(t, err)
				continue
			}
			err := newReq.Reply(false, nil)
			assert.NoError(t, err)
		}
	}()
}

func pass(need string) PasswordFunc {
	return func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if string(password) == need {
			return &ssh.Permissions{
				Extensions: map[string]string{
					utils.ExtIntCertType: utils.ExtIntCertTypeUser,
				},
			}, nil
		}
		return nil, fmt.Errorf("passwords don't match")
	}
}
