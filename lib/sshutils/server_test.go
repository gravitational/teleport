/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sshutils

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestStartStop(t *testing.T) {
	t.Parallel()

	_, signer, err := cert.CreateTestEd25519Certificate("foo", ssh.HostCert)
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
		StaticHostSigners(signer),
		AuthMethods{Password: pass("abcdef123456")},
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
		Auth:            []ssh.AuthMethod{ssh.Password("abcdef123456")},
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

	_, signer, err := cert.CreateTestEd25519Certificate("foo", ssh.HostCert)
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
		StaticHostSigners(signer),
		AuthMethods{Password: pass("abcdef123456")},
		SetShutdownPollPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	clientConfig := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abcdef123456")},
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

	_, signer, err := cert.CreateTestEd25519Certificate("foo", ssh.HostCert)
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
		StaticHostSigners(signer),
		AuthMethods{Password: pass("abcdef123456")},
		SetCiphers([]string{"aes128-ctr"}),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	// client only speaks aes256-ctr, should fail
	cc := ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes256-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abcdef123456")},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}
	_, err = ssh.Dial("tcp", srv.Addr(), &cc)
	require.Error(t, err, "cipher mismatch, should fail, got nil")

	// client only speaks aes128-ctr, should succeed
	cc = ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes128-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abcdef123456")},
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

	_, signer, err := cert.CreateTestRSACertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	_, ellipticSigner, err := cert.CreateTestECDSACertificate("foo", ssh.HostCert)
	require.NoError(t, err)

	_, ed25519Signer, err := cert.CreateTestEd25519Certificate("foo", ssh.HostCert)
	require.NoError(t, err)

	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		assert.NoError(t, err)
	})

	tests := []struct {
		inSigner ssh.Signer
		inFIPS   bool
		assert   require.ErrorAssertionFunc
	}{
		// Ed25519 when in FIPS mode should fail.
		{
			inSigner: ed25519Signer,
			inFIPS:   true,
			assert:   require.Error,
		},
		// ECDSA when in FIPS mode is okay.
		{
			inSigner: ellipticSigner,
			inFIPS:   true,
			assert:   require.NoError,
		},
		// RSA when in FIPS mode is okay.
		{
			inSigner: signer,
			inFIPS:   true,
			assert:   require.NoError,
		},
		// Ed25519 when in not FIPS mode should succeed.
		{
			inSigner: ed25519Signer,
			inFIPS:   false,
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
			StaticHostSigners(tt.inSigner),
			AuthMethods{Password: pass("abcdef123456")},
			SetCiphers([]string{"aes128-ctr"}),
			SetFIPS(tt.inFIPS),
		)
		tt.assert(t, err)
	}
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

func TestDynamicHostSigners(t *testing.T) {
	t.Parallel()

	certFoo, signerFoo, err := cert.CreateTestEd25519Certificate("foo", ssh.HostCert)
	require.NoError(t, err)

	certBar, signerBar, err := cert.CreateTestEd25519Certificate("bar", ssh.HostCert)
	require.NoError(t, err)

	var activeSigner atomic.Pointer[ssh.Signer]
	activeSigner.Store(&signerFoo)

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nc ssh.NewChannel) {
			err := nc.Reject(ssh.UnknownChannelType, ssh.UnknownChannelType.String())
			assert.NoError(t, err)
		}),
		func() []ssh.Signer { return []ssh.Signer{*activeSigner.Load()} },
		AuthMethods{NoClient: true},
		SetShutdownPollPeriod(10*time.Millisecond),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())
	t.Cleanup(func() { _ = srv.Close() })

	dial := func(pub ssh.PublicKey) error {
		clt, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{
			HostKeyCallback: ssh.FixedHostKey(pub),
		})
		if clt != nil {
			defer clt.Close()
		}
		return err
	}

	require.NoError(t, dial(certFoo))
	require.ErrorContains(t, dial(certBar), "ssh: host key mismatch")

	activeSigner.Store(&signerBar)

	require.NoError(t, dial(certBar))
	require.ErrorContains(t, dial(certFoo), "ssh: host key mismatch")
}
