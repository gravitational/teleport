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

package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/utils/socks"
)

// TestDialProxy tests the dialing mechanism of HTTP and SOCKS proxies.
func TestDialProxy(t *testing.T) {
	ctx := context.Background()
	dest := "remote-ip:3080"

	cases := []struct {
		proxy      func(chan error, net.Listener)
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
			proxy:      func(errChan chan error, l net.Listener) { close(errChan) },
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

			go tc.proxy(errChan, l)

			proxyURL, err := url.Parse(tc.scheme + "://" + l.Addr().String())
			require.NoError(t, err)

			conn, err := client.DialProxy(ctx, proxyURL, dest)
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
func serveSOCKSProxy(errChan chan error, l net.Listener) {
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

// serveHTTPProxy starts a limited HTTP proxy on the supplied listener.
// It performs the HTTP handshake then writes back the requested remote address.
func serveHTTPProxy(errChan chan error, l net.Listener) {
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
	}

	if err := s.Serve(l); !errors.Is(err, net.ErrClosed) {
		errChan <- trace.Wrap(err)
	}
}
