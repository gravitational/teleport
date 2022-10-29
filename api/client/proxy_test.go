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

package client

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/gravitational/teleport/lib/utils/socks"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestDialProxy tests the dialing mechanism of HTTP and SOCKS proxies.
func TestDialProxy(t *testing.T) {
	ctx := context.Background()
	dest := "remote-ip:3080"

	cases := []struct {
		proxy  func(net.Listener)
		scheme string
	}{
		{
			proxy:  serveSOCKSProxy,
			scheme: "socks5",
		},
		{
			proxy:  serveHTTPProxy,
			scheme: "http",
		},
	}

	for _, tc := range cases {
		t.Run(tc.scheme, func(t *testing.T) {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, l.Close())
			})

			go tc.proxy(l)

			proxyURL, err := url.Parse(tc.scheme + "://" + l.Addr().String())
			require.NoError(t, err)

			conn, err := DialProxy(ctx, proxyURL, dest)
			require.NoError(t, err)

			result := make([]byte, len(dest))
			_, err = io.ReadFull(conn, result)
			require.NoError(t, err)
			require.Equal(t, string(result), dest)
		})
	}
}

// serveSOCKSProxy starts a limited SOCKS proxy on the supplied listener.
// It performs the SOCKS5 handshake then writes back the requested remote address.
func serveSOCKSProxy(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Debugf("error accepting connection: %v", err)
			return
		}

		go func(conn net.Conn) {
			defer conn.Close()

			remoteAddr, err := socks.Handshake(conn)
			if err != nil {
				log.Debugf("handshake error: %v", err)
				return
			}
			if _, err := conn.Write([]byte(remoteAddr)); err != nil {
				log.Debugf("error writing to connection: %v", err)
				return
			}
		}(conn)
	}
}

// serveHTTPProxy starts a limited HTTP proxy on the supplied listener.
// It performs the HTTP handshake then writes back the requested remote address.
func serveHTTPProxy(l net.Listener) {
	s := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(r.Host))
			} else {
				http.Error(w, "handshake error", http.StatusBadRequest)
				return
			}
		}),
	}

	s.Serve(l)
}
