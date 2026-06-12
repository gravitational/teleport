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
	"bufio"
	"crypto/tls"
	"crypto/x509/pkix"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestKubeHTTPServer_StalledHandshakeClosed asserts a connection that completes the TCP handshake
// but never sends a ClientHello is closed by the server within the configured WriteTimeout.
func TestKubeHTTPServer_StalledHandshakeClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tlsCfg := newSelfSignedTLSConfig(t)
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close() })

		var handlerRan atomic.Bool
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handlerRan.Store(true) })
		srv := newKubeHTTPServer(h, logrus.New(), tlsCfg, nil)
		srv.ErrorLog = stdlog.New(io.Discard, "", 0)

		serveDone := make(chan error, 1)
		go func() {
			l := tls.NewListener(newSingleConnListener(server), tlsCfg)
			serveDone <- srv.Serve(l)
		}()

		// Read in a goroutine so we can probe non-blockingly.
		// Without this, a synchronous Read here would let synctest fast-forward past any later deadline,
		// hiding the case where the handshake bound is wrong.
		readDone := make(chan error, 1)
		go func() {
			buf := make([]byte, 1)
			_, err := client.Read(buf)
			readDone <- err
		}()

		time.Sleep(defaults.HandshakeReadDeadline + 1) // Connection stalled more than WriteTimeout
		synctest.Wait()

		require.False(t, handlerRan.Load(), "handler must not run for stalled connection")

		select {
		case err := <-readDone:
			require.Error(t, err, "server should have closed the stalled connection")
		default:
			t.Fatalf("server did not close stalled connection within %s", defaults.HandshakeReadDeadline+time.Second)
		}

		require.NoError(t, srv.Close())
		synctest.Wait()

		select {
		case err := <-serveDone:
			require.ErrorIs(t, err, http.ErrServerClosed)
		default:
			t.Fatal("Serve did not return after Close")
		}
	})
}

// TestKubeHTTPServer_LongHandlerSurvivesWriteTimeout asserts
// a handler that runs longer than WriteTimeout still delivers its response.
// This is the invariant the outer timeout-reset handler protects:
// WriteTimeout caps the TLS handshake but must not bound watch/exec/portforward streams.
func TestKubeHTTPServer_LongHandlerSurvivesWriteTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tlsCfg := newSelfSignedTLSConfig(t)
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close() })

		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(defaults.HandshakeReadDeadline + 1)
			_, _ = w.Write([]byte("OK"))
		})
		srv := newKubeHTTPServer(h, logrus.New(), tlsCfg, nil)
		srv.ErrorLog = stdlog.New(io.Discard, "", 0)

		tlsClient := tls.Client(client, &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         "localhost",
		})

		serveDone := make(chan error, 1)
		go func() {
			l := tls.NewListener(newSingleConnListener(server), tlsCfg)
			serveDone <- srv.Serve(l)
		}()

		handshakeDone := make(chan error, 1)
		go func() { handshakeDone <- tlsClient.Handshake() }()
		synctest.Wait()

		select {
		case err := <-handshakeDone:
			require.NoError(t, err)
		default:
			t.Fatal("TLS handshake did not complete")
		}

		req, err := http.NewRequest(http.MethodGet, "https://localhost/", nil)
		require.NoError(t, err)
		require.NoError(t, req.Write(tlsClient))

		// ReadResponse blocks until the handler returns.
		// synctest advances fake time across the handler's sleep.
		resp, err := http.ReadResponse(bufio.NewReader(tlsClient), req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, "OK", string(body))

		require.NoError(t, client.Close())
		require.NoError(t, srv.Close())
		synctest.Wait()

		select {
		case err := <-serveDone:
			require.ErrorIs(t, err, http.ErrServerClosed)
		default:
			t.Fatal("Serve did not return after Close")
		}
	})
}

func newSelfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	keyPEM, err := keys.MarshalPrivateKey(signer)
	require.NoError(t, err)
	certPEM, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Signer: signer,
		Entity: pkix.Name{CommonName: "test", Organization: []string{"test"}},
		TTL:    time.Hour,
	})
	require.NoError(t, err)
	keyPair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return &tls.Config{Certificates: []tls.Certificate{keyPair}}
}

// singleConnListener is a minimal net.Listener serving a single pre-built conn.
// In tests we feed it a net.Pipe conn because synctest can't advance fake time across goroutines blocked on syscall-level I/O.
type singleConnListener struct {
	conn   chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newSingleConnListener(c net.Conn) *singleConnListener {
	l := &singleConnListener{
		conn:   make(chan net.Conn, 1),
		closed: make(chan struct{}),
	}
	l.conn <- c
	return l
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conn:
		return c, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (*singleConnListener) Addr() net.Addr { return nil }
