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
	addrV6 := net.TCPAddr{IP: net.ParseIP("::1"), Port: 999}
	markedAddrV6 := addrV6
	markedAddrV6.Port = 0

	signedHeader, err := signPROXYHeader(signPROXYHeaderInput{
		source:         &addr1,
		destination:    &addr2,
		clusterName:    clusterName,
		signingCert:    tlsProxyCert,
		signer:         jwtSigner,
		allowDowngrade: false,
	})
	require.NoError(t, err)

	signedHeaderDowngrade, err := signPROXYHeader(signPROXYHeaderInput{
		source:         &addrV6,
		destination:    &addr2,
		clusterName:    clusterName,
		signingCert:    tlsProxyCert,
		signer:         jwtSigner,
		allowDowngrade: true,
	})
	require.NoError(t, err)

	testCases := []struct {
		name               string
		proxyLine          []byte
		expectedRemoteAddr string
		expectedLocalAddr  string
		proxyMode          PROXYProtocolMode
		allowDowngrade     bool
	}{
		{
			name:               "PROXY v1 header",
			proxyLine:          []byte(sampleProxyV1Line),
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
			proxyMode:          PROXYProtocolOn,
			allowDowngrade:     false,
		},
		{
			name:               "PROXY v1 header, unspecified mode",
			proxyLine:          []byte(sampleProxyV1Line),
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:0",
			proxyMode:          PROXYProtocolUnspecified,
			allowDowngrade:     false,
		},
		{
			name:               "unsigned PROXY v2 header",
			proxyLine:          sampleProxyV2Line,
			expectedLocalAddr:  "127.0.0.2:42",
			expectedRemoteAddr: "127.0.0.1:12345",
			proxyMode:          PROXYProtocolOn,
			allowDowngrade:     false,
		},
		{
			name:               "signed PROXY v2 header",
			proxyLine:          signedHeader,
			expectedLocalAddr:  addr2.String(),
			expectedRemoteAddr: addr1.String(),
			proxyMode:          PROXYProtocolOff,
			allowDowngrade:     false,
		},
		{
			name:               "signed PROXY v2 header on, mixed version in downgrade mode",
			proxyLine:          signedHeaderDowngrade,
			expectedLocalAddr:  addr2.String(),
			expectedRemoteAddr: markedAddrV6.String(),
			proxyMode:          PROXYProtocolOn,
			allowDowngrade:     true,
		},
		{
			name:               "signed PROXY v2 header unspecified, mixed version in downgrade mode",
			proxyLine:          signedHeaderDowngrade,
			expectedLocalAddr:  addr2.String(),
			expectedRemoteAddr: markedAddrV6.String(),
			proxyMode:          PROXYProtocolUnspecified,
			allowDowngrade:     true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			proto := "tcp"
			listenAddr := "127.0.0.1:0"
			listener, err := net.Listen(proto, listenAddr)
			require.NoError(t, err)
			t.Cleanup(func() { listener.Close() })

			proxyListener, err := NewPROXYEnabledListener(Config{
				Listener:            listener,
				Context:             context.Background(),
				ID:                  "test",
				PROXYProtocolMode:   tt.proxyMode,
				PROXYAllowDowngrade: tt.allowDowngrade,
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
			conn, err := net.Dial(proto, proxyListener.Addr().String())
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

func TestWrapperConn(t *testing.T) {
	const proto = "tcp"
	const listenAddr = "127.0.0.1:0"
	listener, err := net.Listen(proto, listenAddr)
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	connCh := make(chan net.Conn)
	errCh := make(chan error)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errCh <- err
			return

		}

		connCh <- conn
	}()

	_, err = net.Dial(proto, listener.Addr().String())
	require.NoError(t, err)

	var c net.Conn
	select {
	case c = <-connCh:
	case err := <-errCh:
		require.NoError(t, err, "Received error while trying to accept connection")
	case <-time.After(time.Millisecond * 500):
		require.Fail(t, "Time out while accepting connection")
	}

	// Without a proxy line the RemoteAddr should just be a passthru to the underlying
	// net.Conn
	connNoProxy := Conn{
		Conn:     c,
		protocol: ProtoProxy,
	}
	require.Equal(t, c.RemoteAddr().String(), connNoProxy.RemoteAddr().String())

	// With a proxy line, the RemoteAddr should return the source address
	// present in the proxy line. This test case also guards against a
	// regression where genuine class E source addresses in a ProxyLine were
	// being ignored in favor of a nil original address in the TLVs
	classEAddr := net.TCPAddr{
		IP: []byte{classEPrefix, 0, 0, 1},
	}

	connWithProxy := Conn{
		Conn:     c,
		protocol: ProtoProxy,
		proxyLine: &ProxyLine{
			Protocol: proto,
			Source:   classEAddr,
		},
	}
	require.Equal(t, classEAddr.String(), connWithProxy.RemoteAddr().String())

	// With a downgraded proxy line , the RemoteAddr should return the original
	// source address present in the TLVs.
	downgradeAddr := net.TCPAddr{
		IP: []byte{192, 168, 0, 1},
	}
	tlvs, err := MarshalTLVs([]TLV{
		{
			Type:  PP2Type(PP2TeleportSubtypeOriginalAddr),
			Value: []byte(downgradeAddr.String()),
		},
	})
	require.NoError(t, err)
	connWithDowngrade := Conn{
		Conn:     c,
		protocol: ProtoProxy,
		proxyLine: &ProxyLine{
			Protocol: proto,
			Source:   classEAddr,
			TLVs: []TLV{
				{
					Type:  PP2TypeTeleport,
					Value: tlvs,
				},
			},
		},
	}

	require.Equal(t, downgradeAddr.String(), connWithDowngrade.RemoteAddr().String())
}
