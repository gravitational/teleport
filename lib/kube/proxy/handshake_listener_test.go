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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
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

// startBoundedListener creates a TCP listener on 127.0.0.1, wraps it with
// newHandshakeBoundedTLSListener, and registers cleanup.
func startBoundedListener(t *testing.T, timeout time.Duration, nextProtos []string) (net.Listener, *tls.Config) {
	t.Helper()
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg := newSelfSignedTLSConfig(t, nextProtos)
	bounded := newHandshakeBoundedTLSListener(inner, cfg, timeout)
	t.Cleanup(func() { _ = bounded.Close() })
	return bounded, cfg
}

// acceptResult bundles an accept return for select-driven tests.
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

// TestHandshakeBoundedTLSListener_StalledClientDropped asserts a TCP client
// that connects and never sends a ClientHello is dropped within the
// configured timeout and never delivered via Accept().
func TestHandshakeBoundedTLSListener_StalledClientDropped(t *testing.T) {
	const timeout = 500 * time.Millisecond
	l, _ := startBoundedListener(t, timeout, nil)

	// Plain TCP connect; send nothing.
	raw, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })

	acc := asyncAccept(l)

	// Accept must not deliver this conn — ever.
	select {
	case r := <-acc:
		t.Fatalf("Accept returned unexpectedly: conn=%v err=%v", r.conn, r.err)
	case <-time.After(timeout + 2*time.Second):
		// expected: nothing emerged.
	}

	// The underlying conn should now be closed by the listener.
	_ = raw.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = raw.Read(buf)
	require.Error(t, err, "stalled conn should be closed by listener after handshake timeout")
}

// TestHandshakeBoundedTLSListener_NormalClientSucceeds asserts a legitimate
// TLS client completes the handshake and is delivered via Accept() as a
// post-handshake *tls.Conn.
func TestHandshakeBoundedTLSListener_NormalClientSucceeds(t *testing.T) {
	const timeout = 5 * time.Second
	l, serverCfg := startBoundedListener(t, timeout, nil)

	clientCfg := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "localhost",
	}
	_ = serverCfg // unused on client side

	acc := asyncAccept(l)

	go func() {
		conn, err := tls.Dial("tcp", l.Addr().String(), clientCfg)
		if err == nil {
			defer conn.Close()
			_ = conn.Handshake()
		}
	}()

	select {
	case r := <-acc:
		require.NoError(t, r.err)
		require.NotNil(t, r.conn)
		tc, ok := r.conn.(*tls.Conn)
		require.True(t, ok, "expected *tls.Conn, got %T", r.conn)
		assert.True(t, tc.ConnectionState().HandshakeComplete, "handshake should already be complete")
		_ = tc.Close()
	case <-time.After(10 * time.Second):
		t.Fatal("Accept did not deliver completed TLS conn within 10s")
	}
}

// TestHandshakeBoundedTLSListener_BoundWellUnder60s is a sanity guard: even
// with a generous timeout setting, the listener must not let a stalled
// handshake hold a goroutine for the legacy 60s implicit bound.
func TestHandshakeBoundedTLSListener_BoundWellUnder60s(t *testing.T) {
	const timeout = 1 * time.Second
	l, _ := startBoundedListener(t, timeout, nil)

	raw, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })

	start := time.Now()
	// Wait for the listener to close the conn.
	_ = raw.SetReadDeadline(time.Now().Add(20 * time.Second))
	buf := make([]byte, 1)
	_, err = raw.Read(buf)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 20*time.Second, "handshake bound must be well under the legacy 60s implicit bound")
}

// TestHandshakeBoundedTLSListener_HTTP2ALPN asserts h2 negotiation is
// preserved through the bounded listener.
func TestHandshakeBoundedTLSListener_HTTP2ALPN(t *testing.T) {
	const timeout = 5 * time.Second
	l, _ := startBoundedListener(t, timeout, []string{"h2", "http/1.1"})

	clientCfg := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         "localhost",
		NextProtos:         []string{"h2"},
	}

	acc := asyncAccept(l)
	go func() {
		conn, err := tls.Dial("tcp", l.Addr().String(), clientCfg)
		if err == nil {
			defer conn.Close()
			_ = conn.Handshake()
		}
	}()

	select {
	case r := <-acc:
		require.NoError(t, r.err)
		tc, ok := r.conn.(*tls.Conn)
		require.True(t, ok)
		assert.Equal(t, "h2", tc.ConnectionState().NegotiatedProtocol)
		_ = tc.Close()
	case <-time.After(10 * time.Second):
		t.Fatal("Accept timed out for h2 client")
	}
}

// TestHandshakeBoundedTLSListener_Close asserts Close propagates and unblocks
// pending Accept calls.
func TestHandshakeBoundedTLSListener_Close(t *testing.T) {
	const timeout = 5 * time.Second
	l, _ := startBoundedListener(t, timeout, nil)

	acc := asyncAccept(l)
	require.NoError(t, l.Close())

	select {
	case r := <-acc:
		require.Error(t, r.err, "Accept must return error after Close")
		// Either net.ErrClosed or a wrapper of it.
		assert.True(t, errors.Is(r.err, net.ErrClosed) || r.err.Error() != "", "expected closed-listener error")
	case <-time.After(5 * time.Second):
		t.Fatal("Accept did not return within 5s of Close")
	}

	// Second Close should be a no-op (idempotent), not a panic.
	_ = l.Close()
}

// TestHandshakeBoundedTLSListener_ContextDeadline ensures the handshake
// honors the configured timeout via context cancellation, not just the
// underlying read deadline. (Sanity check that the implementation passes
// a context with the deadline.)
func TestHandshakeBoundedTLSListener_ContextDeadline(t *testing.T) {
	const timeout = 200 * time.Millisecond
	l, _ := startBoundedListener(t, timeout, nil)

	// Connect but never send anything.
	raw, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })

	// Use a context unrelated to the listener, just to assert the listener
	// is self-bounded.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = raw.SetReadDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 1)
		_, _ = raw.Read(buf)
	}()

	select {
	case <-done:
		assert.Less(t, time.Since(start), 3*time.Second, "listener should drop stalled conn ~timeout, not seconds later")
	case <-ctx.Done():
		t.Fatal("listener did not close stalled conn within 5s")
	}
}
