/*
Copyright 2023 Gravitational, Inc.

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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx := context.Background()
	clusterName := "teleport-test"
	tlsProxyCert, casGetter, jwtSigner := getTestCertCAsGetterAndSigner(t, clusterName)
	_, _ = tlsProxyCert, jwtSigner
	proxyListener, err := NewPROXYEnabledListener(Config{
		Listener:                    listener,
		Context:                     ctx,
		EnableExternalProxyProtocol: true,
		ID:                          "test",
		CertAuthorityGetter:         casGetter,
		LocalClusterName:            clusterName,
	})
	require.NoError(t, err, "Could not create new PROXY enabled listener")

	addr1 := net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 444}
	addr2 := net.TCPAddr{IP: net.ParseIP("5.4.3.2"), Port: 555}

	signedHeader, err := signPROXYHeader(&addr1, &addr2, clusterName, tlsProxyCert, jwtSigner)
	require.NoError(t, err)

	testCases := []struct {
		name               string
		proxyLine          []byte
		expectedRemoteAddr string
		expectedLocalAddr  string
	}{
		{
			name:               "PROXY v1 header",
			proxyLine:          []byte(sampleProxyV1Line),
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
		},
		{
			name:               "unsigned PROXY v2 header",
			proxyLine:          sampleProxyV2Line,
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
		},
		{
			name:               "signed PROXY v2 header",
			proxyLine:          signedHeader,
			expectedLocalAddr:  addr2.String(),
			expectedRemoteAddr: addr1.String(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
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
