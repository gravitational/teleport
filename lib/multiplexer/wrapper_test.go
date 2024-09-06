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

package multiplexer

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPROXYEnabledListener_Accept(t *testing.T) {
	t.Parallel()

	clusterName := "teleport-test"
	tlsProxyCert, casGetter, jwtSigner := getTestCertCAsGetterAndSigner(t, clusterName)
	_, _ = tlsProxyCert, jwtSigner

	addr1 := net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 444}
	addr2 := net.TCPAddr{IP: net.ParseIP("5.4.3.2"), Port: 555}

	signedHeader, err := signPROXYHeader(&addr1, &addr2, clusterName, tlsProxyCert, jwtSigner)
	require.NoError(t, err)

	testCases := []struct {
		name               string
		proxyLine          []byte
		expectedRemoteAddr string
		expectedLocalAddr  string
		PROXYProtocolMode  PROXYProtocolMode
	}{
		{
			name:               "PROXY v1 header",
			proxyLine:          []byte(sampleProxyV1Line),
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
			PROXYProtocolMode:  PROXYProtocolOn,
		},
		{
			name:               "PROXY v1 header, unspecified mode",
			proxyLine:          []byte(sampleProxyV1Line),
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:0",
			PROXYProtocolMode:  PROXYProtocolUnspecified,
		},
		{
			name:               "unsigned PROXY v2 header",
			proxyLine:          sampleProxyV2Line,
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
			PROXYProtocolMode:  PROXYProtocolOn,
		},
		{
			name:               "signed PROXY v2 header",
			proxyLine:          signedHeader,
			expectedLocalAddr:  addr2.String(),
			expectedRemoteAddr: addr1.String(),
			PROXYProtocolMode:  PROXYProtocolOff,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { listener.Close() })

			proxyListener, err := NewPROXYEnabledListener(Config{
				Listener:            listener,
				Context:             context.Background(),
				ID:                  "test",
				PROXYProtocolMode:   tt.PROXYProtocolMode,
				CertAuthorityGetter: casGetter,
				LocalClusterName:    clusterName,
			})
			require.NoError(t, err, "Could not create new PROXY enabled listener")

			connChan := make(chan net.Conn)
			errChan := make(chan error)
			go func() {
				conn, err := proxyListener.Accept()
				if err != nil {
					errChan <- err
					return
				}
				connChan <- conn
			}()
			conn, err := net.Dial("tcp", proxyListener.Addr().String())
			require.NoError(t, err)
			defer conn.Close()

			_, err = conn.Write(tt.proxyLine)
			require.NoError(t, err)

			testData := append(sshPrefix, []byte("this is test data")...)
			_, err = conn.Write(testData) // Force PROXY listener to pass connection since it detected a real protocol (SSH)
			require.NoError(t, err)

			select {
			case conn := <-connChan:
				require.Equal(t, tt.expectedRemoteAddr, conn.RemoteAddr().String())
				require.Equal(t, tt.expectedLocalAddr, conn.LocalAddr().String())
			case err := <-errChan:
				require.NoError(t, err, "Received error while trying to accept connection")
			case <-time.After(time.Millisecond * 500):
				require.Fail(t, "Time out while accepting connection")
			}
		})
	}
}
