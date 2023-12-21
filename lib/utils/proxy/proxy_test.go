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

package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils/socks"
)

// TestDialProxy tests the dialing mechanism of HTTP and SOCKS proxies.
func TestDialProxy(t *testing.T) {
	ctx := context.Background()
	dest := "remote-ip:3080"

	tlsConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)

	cases := []struct {
		proxy      func(chan error, net.Listener, *tls.Config)
		scheme     string
		assertDial require.ErrorAssertionFunc
	}{
		{
			proxy:      serveSOCKSProxy,
			scheme:     "socks5",
			assertDial: require.NoError,
		},
		{
			proxy:      serveHTTPProxy,
			scheme:     "http",
			assertDial: require.NoError,
		},
		{
			proxy:      serveHTTPProxy,
			scheme:     "https",
			assertDial: require.NoError,
		},
		{
			proxy:      func(errChan chan error, l net.Listener, _ *tls.Config) { close(errChan) },
			scheme:     "unknown",
			assertDial: require.Error,
		},
	}

	for _, tc := range cases {
		t.Run(tc.scheme, func(t *testing.T) {
			errChan := make(chan error, 1)
			l, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, l.Close())
				err := <-errChan
				require.NoError(t, err)
			})

			var serverTLSConfig *tls.Config
			if tc.scheme == "https" {
				serverTLSConfig = tlsConfig.TLS
			}
			go tc.proxy(errChan, l, serverTLSConfig)

			proxyURL, err := url.Parse(tc.scheme + "://" + l.Addr().String())
			require.NoError(t, err)

			pool := x509.NewCertPool()
			pool.AddCert(tlsConfig.Certificate)
			clientTLSConfig := &tls.Config{
				RootCAs: pool,
			}

			conn, err := client.DialProxy(ctx, proxyURL, dest, client.WithTLSConfig(clientTLSConfig))
			tc.assertDial(t, err)

			if conn != nil {
				result := make([]byte, len(dest))
				_, err = io.ReadFull(conn, result)
				require.NoError(t, err)
				require.Equal(t, string(result), dest)
			}
		})
	}
}

// serveSOCKSProxy starts a limited SOCKS proxy on the supplied listener.
// It performs the SOCKS5 handshake then writes back the requested remote address.
func serveSOCKSProxy(errChan chan error, l net.Listener, _ *tls.Config) {
	defer close(errChan)

	for {
		conn, err := l.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				errChan <- trace.Wrap(err)
			}
			return
		}

		go func(conn net.Conn) {
			defer conn.Close()

			write := func(msg string) {
				if _, err := conn.Write([]byte(msg)); err != nil {
					errChan <- trace.Wrap(err)
					return
				}
			}

			if remoteAddr, err := socks.Handshake(conn); err != nil {
				write(err.Error())
			} else {
				write(remoteAddr)
			}
		}(conn)
	}
}

// serveHTTPProxy starts a limited HTTP/HTTPS proxy on the supplied listener.
// It performs the HTTP handshake then writes back the requested remote address.
func serveHTTPProxy(errChan chan error, l net.Listener, tlsConfig *tls.Config) {
	defer close(errChan)

	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(r.Host))
			} else {
				http.Error(w, "handshake error", http.StatusBadRequest)
			}
		}),
		TLSConfig: tlsConfig,
	}

	var err error
	if tlsConfig != nil {
		err = s.ServeTLS(l, "", "")
	} else {
		err = s.Serve(l)
	}

	if !errors.Is(err, net.ErrClosed) {
		errChan <- trace.Wrap(err)
	}
}
