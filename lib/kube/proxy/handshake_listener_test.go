/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSelfSignedTLSConfig returns a *tls.Config bound to a freshly-generated
// self-signed certificate valid for 127.0.0.1.
func newSelfSignedTLSConfig(t *testing.T, nextProtos []string) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   nextProtos,
	}
}

// pipeListener is an in-memory net.Listener whose connections are halves of
// net.Pipe pairs. It plays nicely with testing/synctest: all blocking happens
// on channels and pipe buffers, which are durably blocking inside a bubble.
type pipeListener struct {
	accept chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		accept: make(chan net.Conn),
		closed: make(chan struct{}),
	}
}

func (p *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-p.accept:
		return c, nil
	case <-p.closed:
		return nil, net.ErrClosed
	}
}

func (p *pipeListener) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

func (p *pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// dial returns the client side of a freshly-connected pipe pair and hands
// the server side to the listener's Accept queue.
func (p *pipeListener) dial(t *testing.T) net.Conn {
	t.Helper()
	client, server := net.Pipe()
	select {
	case p.accept <- server:
	case <-p.closed:
		_ = client.Close()
		_ = server.Close()
		t.Fatal("listener closed before dial")
	}
	return client
}

// startBoundedListener creates a pipeListener, wraps it with
// newHandshakeBoundedTLSListener, and registers cleanup.
func startBoundedListener(t *testing.T, timeout time.Duration, nextProtos []string) (net.Listener, *pipeListener) {
	t.Helper()
	pl := newPipeListener()
	cfg := newSelfSignedTLSConfig(t, nextProtos)
	bounded := newHandshakeBoundedTLSListener(pl, cfg, timeout)
	t.Cleanup(func() { _ = bounded.Close() })
	return bounded, pl
}

type acceptResult struct {
	conn net.Conn
	err  error
}

func asyncAccept(l net.Listener) <-chan acceptResult {
	ch := make(chan acceptResult, 1)
	go func() {
		c, err := l.Accept()
		ch <- acceptResult{c, err}
	}()
	return ch
}

// TestHandshakeBoundedTLSListener_StalledClientDropped asserts a client that
// connects and never sends a ClientHello is dropped within the configured
// timeout and never delivered via Accept().
func TestHandshakeBoundedTLSListener_StalledClientDropped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 500 * time.Millisecond
		l, pl := startBoundedListener(t, timeout, nil)

		raw := pl.dial(t)
		t.Cleanup(func() { _ = raw.Close() })

		acc := asyncAccept(l)

		// Advance past timeout; once all goroutines are durably blocked,
		// fake time advances and the handshake context fires.
		time.Sleep(timeout + time.Second)
		synctest.Wait()

		select {
		case r := <-acc:
			t.Fatalf("Accept returned unexpectedly: conn=%v err=%v", r.conn, r.err)
		default:
		}

		buf := make([]byte, 1)
		_, err := raw.Read(buf)
		require.Error(t, err, "stalled conn should be closed by listener after handshake timeout")
	})
}

// TestHandshakeBoundedTLSListener_NormalClientSucceeds asserts a legitimate
// TLS client completes the handshake and is delivered via Accept() as a
// post-handshake *tls.Conn.
func TestHandshakeBoundedTLSListener_NormalClientSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 5 * time.Second
		l, pl := startBoundedListener(t, timeout, nil)

		client := pl.dial(t)
		t.Cleanup(func() { _ = client.Close() })

		tlsClient := tls.Client(client, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "localhost",
		})
		go func() { _ = tlsClient.Handshake() }()

		conn, err := l.Accept()
		require.NoError(t, err)
		require.NotNil(t, conn)
		tc, ok := conn.(*tls.Conn)
		require.True(t, ok, "expected *tls.Conn, got %T", conn)
		assert.True(t, tc.ConnectionState().HandshakeComplete, "handshake should already be complete")
		_ = tc.Close()
	})
}

// TestHandshakeBoundedTLSListener_BadHandshakeDropped asserts a client that
// sends garbage instead of a valid ClientHello fails the handshake fast and is
// not delivered via Accept(), without waiting for the timeout.
func TestHandshakeBoundedTLSListener_BadHandshakeDropped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 30 * time.Second
		l, pl := startBoundedListener(t, timeout, nil)

		raw := pl.dial(t)
		t.Cleanup(func() { _ = raw.Close() })

		acc := asyncAccept(l)

		// Garbage that will never parse as a TLS ClientHello.
		_, err := raw.Write([]byte("not a tls clienthello\n"))
		require.NoError(t, err)

		synctest.Wait()

		select {
		case r := <-acc:
			t.Fatalf("Accept returned unexpectedly: conn=%v err=%v", r.conn, r.err)
		default:
		}

		buf := make([]byte, 1)
		_, err = raw.Read(buf)
		require.Error(t, err, "bad-handshake conn should be closed by listener")
	})
}

// TestHandshakeBoundedTLSListener_HTTP2ALPN asserts h2 negotiation is
// preserved through the bounded listener.
func TestHandshakeBoundedTLSListener_HTTP2ALPN(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 5 * time.Second
		l, pl := startBoundedListener(t, timeout, []string{"h2", "http/1.1"})

		client := pl.dial(t)
		t.Cleanup(func() { _ = client.Close() })

		tlsClient := tls.Client(client, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "localhost",
			NextProtos:         []string{"h2"},
		})
		go func() { _ = tlsClient.Handshake() }()

		conn, err := l.Accept()
		require.NoError(t, err)
		tc, ok := conn.(*tls.Conn)
		require.True(t, ok)
		assert.Equal(t, "h2", tc.ConnectionState().NegotiatedProtocol)
		_ = tc.Close()
	})
}

// TestHandshakeBoundedTLSListener_Close asserts Close propagates and unblocks
// pending Accept calls.
func TestHandshakeBoundedTLSListener_Close(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 5 * time.Second
		l, _ := startBoundedListener(t, timeout, nil)

		acc := asyncAccept(l)
		synctest.Wait() // ensure the Accept goroutine is parked

		require.NoError(t, l.Close())

		r := <-acc
		require.Error(t, r.err, "Accept must return error after Close")
		assert.ErrorIs(t, r.err, net.ErrClosed)

		// Second Close should be a no-op (idempotent), not a panic.
		_ = l.Close()
	})
}
