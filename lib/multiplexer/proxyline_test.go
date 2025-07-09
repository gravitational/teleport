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
	"bufio"
	"bytes"
	"context"
	"crypto/x509/pkix"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/tlsca"
)

var (
	// PROXY v1
	sampleProxyV1Line = "PROXY TCP4 127.0.0.1 127.0.0.2 12345 42\r\n"

	// PROXY v2

	// source=127.0.0.1:12345 destination=127.0.0.2:42
	sampleIPv4Addresses = []byte{0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0x30, 0x39, 0x00, 0x2A}
	// {0x21, 0x11, 0x00, 0x0C} - 4 bits version, 4 bits command, 4 bits address family, 4 bits protocol, 16 bits length
	sampleProxyV2Line = bytes.Join([][]byte{ProxyV2Prefix, {0x21, 0x11, 0x00, 0x0C}, sampleIPv4Addresses}, nil)
	// Proxy line with LOCAL command
	sampleProxyV2LineLocal    = bytes.Join([][]byte{ProxyV2Prefix, {0x20, 0x11, 0x00, 0x00}}, nil)
	sampleTLV                 = []byte{byte(PP2TypeTeleport), 0x00, 0x03, 0x01, 0x02, 0x03}
	sampleEmptyTLV            = []byte{byte(PP2TypeTeleport), 0x00, 0x00}
	sampleProxyV2LineTLV      = bytes.Join([][]byte{ProxyV2Prefix, {0x21, 0x11, 0x00, 0x12}, sampleIPv4Addresses, sampleTLV}, nil)
	sampleProxyV2LineEmptyTLV = bytes.Join([][]byte{ProxyV2Prefix, {0x21, 0x11, 0x00, 0x0F}, sampleIPv4Addresses, sampleEmptyTLV}, nil)
)

func TestReadProxyLine(t *testing.T) {
	t.Parallel()

	t.Run("empty line", func(t *testing.T) {
		_, err := ReadProxyLine(bufio.NewReader(bytes.NewReader([]byte{})))
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("malformed line", func(t *testing.T) {
		_, err := ReadProxyLine(bufio.NewReader(bytes.NewReader([]byte("GIBBERISH\r\n"))))
		require.ErrorIs(t, err, trace.BadParameter("malformed PROXY line protocol string"))
	})
	t.Run("successfully read proxy v1 line", func(t *testing.T) {
		pl, err := ReadProxyLine(bufio.NewReader(bytes.NewReader([]byte(sampleProxyV1Line))))
		require.NoError(t, err)

		require.Equal(t, TCP4, pl.Protocol)
		require.Equal(t, "127.0.0.1:12345", pl.Source.String())
		require.Equal(t, "127.0.0.2:42", pl.Destination.String())
		require.Nil(t, pl.TLVs)
		require.Equal(t, sampleProxyV1Line, pl.String())
	})
}

func TestReadProxyLineV2(t *testing.T) {
	t.Parallel()

	t.Run("empty line", func(t *testing.T) {
		_, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader([]byte{})))
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("too short line", func(t *testing.T) {
		_, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader([]byte{0x01, 0x02})))
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})
	t.Run("wrong PROXY v2 signature line", func(t *testing.T) {
		_, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader([]byte("malformed PROXY line protocol\r\n"))))
		require.ErrorContains(t, err, "unrecognized signature")
	})
	t.Run("malformed PROXY v2 header", func(t *testing.T) {
		_, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(append(ProxyV2Prefix, []byte("JIBBERISH")...))))
		require.ErrorContains(t, err, "unsupported version")
	})
	t.Run("successfully read proxy v2 line without TLV", func(t *testing.T) {
		pl, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(sampleProxyV2Line)))
		require.NoError(t, err)

		require.Equal(t, TCP4, pl.Protocol)
		require.Equal(t, "127.0.0.1:12345", pl.Source.String())
		require.Equal(t, "127.0.0.2:42", pl.Destination.String())
		require.Nil(t, pl.TLVs)

		b, err := pl.Bytes()
		require.NoError(t, err)
		require.Equal(t, sampleProxyV2Line, b)
	})
	t.Run("successfully read proxy v2 line with TLV", func(t *testing.T) {
		pl, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(sampleProxyV2LineTLV)))
		require.NoError(t, err)

		require.Equal(t, TCP4, pl.Protocol)
		require.Equal(t, "127.0.0.1:12345", pl.Source.String())
		require.Equal(t, "127.0.0.2:42", pl.Destination.String())
		require.NotNil(t, pl.TLVs)
		require.Len(t, pl.TLVs, 1)
		require.Equal(t, PP2TypeTeleport, pl.TLVs[0].Type)
		require.Equal(t, []byte{0x01, 0x02, 0x03}, pl.TLVs[0].Value)

		b, err := pl.Bytes()
		require.NoError(t, err)
		require.Equal(t, sampleProxyV2LineTLV, b)
	})
	t.Run("successfully read proxy v2 line with TLV that has empty value", func(t *testing.T) {
		pl, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(sampleProxyV2LineEmptyTLV)))
		require.NoError(t, err)

		require.Equal(t, TCP4, pl.Protocol)
		require.Equal(t, "127.0.0.1:12345", pl.Source.String())
		require.Equal(t, "127.0.0.2:42", pl.Destination.String())
		require.NotNil(t, pl.TLVs)
		require.Len(t, pl.TLVs, 1)
		require.Equal(t, PP2TypeTeleport, pl.TLVs[0].Type)
		require.Equal(t, []byte{}, pl.TLVs[0].Value)

		b, err := pl.Bytes()
		require.NoError(t, err)
		require.Equal(t, sampleProxyV2LineEmptyTLV, b)
	})
	t.Run("LOCAL command", func(t *testing.T) {
		pl, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(sampleProxyV2LineLocal)))
		require.NoError(t, err)
		require.Nil(t, pl)
	})
}

func TestProxyLine_Bytes(t *testing.T) {
	t.Parallel()

	source4 := net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 12345,
	}
	destination4 := net.TCPAddr{
		IP:   net.ParseIP("127.0.0.2"),
		Port: 42,
	}

	source6 := net.TCPAddr{
		IP:   net.ParseIP("fe80::a00:27ff:fe8e:8aa8"),
		Port: 12345,
	}
	destination6 := net.TCPAddr{
		IP:   net.ParseIP("fe80::a00:27ff:fe8e:8aa8"),
		Port: 42,
	}

	t.Run("without TLV", func(t *testing.T) {
		pl := ProxyLine{
			Protocol:    TCP4,
			Source:      source4,
			Destination: destination4,
		}

		b, err := pl.Bytes()
		assert.NoError(t, err)
		assert.Len(t, b, 28)
		assert.Equal(t, sampleProxyV2Line, b)

		pl2, err := ReadProxyLineV2(bufio.NewReader(bytes.NewBuffer(b)))
		assert.NoError(t, err)
		assert.Equal(t, TCP4, pl2.Protocol)
		assert.Equal(t, "127.0.0.1:12345", pl2.Source.String())
		assert.Equal(t, "127.0.0.2:42", pl2.Destination.String())
		assert.Empty(t, pl2.TLVs)
	})

	t.Run("with TLV", func(t *testing.T) {
		pl := ProxyLine{
			Protocol:    TCP4,
			Source:      source4,
			Destination: destination4,
			TLVs: []TLV{{
				Type:  PP2TypeTeleport,
				Value: []byte("0123"),
			}},
		}

		b, err := pl.Bytes()
		assert.NoError(t, err)
		assert.Len(t, b, 35)

		pl2, err := ReadProxyLineV2(bufio.NewReader(bytes.NewBuffer(b)))
		assert.NoError(t, err)
		assert.Equal(t, TCP4, pl2.Protocol)
		assert.Equal(t, "127.0.0.1:12345", pl2.Source.String())
		assert.Equal(t, "127.0.0.2:42", pl2.Destination.String())
		assert.Len(t, pl2.TLVs, 1)
		assert.Equal(t, PP2TypeTeleport, pl2.TLVs[0].Type)
		assert.Equal(t, []byte("0123"), pl2.TLVs[0].Value)
	})

	t.Run("write-read proxy line conversion", func(t *testing.T) {
		tlv := TLV{
			Type:  PP2TypeTeleport,
			Value: []byte("test"),
		}
		testCases := []struct {
			desc      string
			proxyLine ProxyLine
		}{
			{
				desc:      "TCP4",
				proxyLine: ProxyLine{Protocol: TCP4, Source: source4, Destination: destination4, TLVs: nil},
			},
			{
				desc:      "TCP6",
				proxyLine: ProxyLine{Protocol: TCP6, Source: source6, Destination: destination6, TLVs: nil},
			},
			{
				desc:      "TCP4 with TLV",
				proxyLine: ProxyLine{Protocol: TCP4, Source: source4, Destination: destination4, TLVs: []TLV{tlv}},
			},
			{
				desc:      "TCP6 with TLV",
				proxyLine: ProxyLine{Protocol: TCP6, Source: source6, Destination: destination6, TLVs: []TLV{tlv}},
			},
		}

		for _, tt := range testCases {
			b, err := tt.proxyLine.Bytes()
			require.NoError(t, err)
			pl, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(b)))

			require.NoError(t, err)
			require.NotNil(t, pl)
			require.Equal(t, tt.proxyLine.Protocol, pl.Protocol)
			require.Equal(t, tt.proxyLine.Source.String(), pl.Source.String())
			require.Equal(t, tt.proxyLine.Destination.String(), pl.Destination.String())
			require.Equal(t, tt.proxyLine.TLVs, pl.TLVs)
		}
	})
}

func TestUnmarshalTLVs(t *testing.T) {
	testCases := []struct {
		desc         string
		input        []byte
		expectedErr  error
		expectedTLVs []TLV
	}{
		{
			desc:         "empty input",
			input:        []byte{},
			expectedErr:  nil,
			expectedTLVs: nil,
		},
		{
			desc:         "too short input",
			input:        []byte{byte(PP2TypeTeleport)},
			expectedErr:  ErrTruncatedTLV,
			expectedTLVs: nil,
		},
		{
			desc:         "too short input #2",
			input:        []byte{byte(PP2TypeTeleport), 0x00},
			expectedErr:  ErrTruncatedTLV,
			expectedTLVs: nil,
		},
		{
			desc:         "specified length is larger than TLV value",
			input:        []byte{byte(PP2TypeTeleport), 0x00, 0x04, 0x01, 0x02, 0x03},
			expectedErr:  ErrTruncatedTLV,
			expectedTLVs: nil,
		},
		{
			desc:         "specified length is smaller than TLV value",
			input:        []byte{byte(PP2TypeTeleport), 0x00, 0x02, 0x01, 0x02, 0x03},
			expectedErr:  ErrTruncatedTLV,
			expectedTLVs: nil,
		},
		{
			desc:        "successful TLV with value",
			input:       sampleTLV,
			expectedErr: nil,
			expectedTLVs: []TLV{
				{
					Type:  PP2TypeTeleport,
					Value: []byte{0x01, 0x02, 0x03},
				},
			},
		},
		{
			desc:        "successful empty TLV",
			input:       sampleEmptyTLV,
			expectedErr: nil,
			expectedTLVs: []TLV{
				{
					Type:  PP2TypeTeleport,
					Value: []byte{},
				},
			},
		},
		{
			desc:        "successful 2 TLVs",
			input:       append(sampleEmptyTLV, sampleTLV...),
			expectedErr: nil,
			expectedTLVs: []TLV{
				{
					Type:  PP2TypeTeleport,
					Value: []byte{},
				},
				{
					Type:  PP2TypeTeleport,
					Value: []byte{0x01, 0x02, 0x03},
				},
			},
		},
	}

	for _, tt := range testCases {
		tlvs, err := UnmarshalTLVs(tt.input)
		if tt.expectedErr != nil {
			require.ErrorIs(t, err, tt.expectedErr)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tt.expectedTLVs, tlvs)
	}
}

func TestProxyLine_AddSignature(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc      string
		inputTLVs []TLV
		signature string
		cert      string

		wantErr      string
		expectedTLVs []TLV
	}{
		{
			desc:         "missing signature bytes",
			inputTLVs:    nil,
			signature:    "",
			cert:         "abc",
			wantErr:      "missing signature",
			expectedTLVs: nil,
		},
		{
			desc:         "missing cert",
			inputTLVs:    nil,
			signature:    "abc",
			cert:         "",
			wantErr:      "missing signing certificate",
			expectedTLVs: nil,
		},
		{
			desc:      "no existing signature on proxy line",
			inputTLVs: nil,
			signature: "123",
			cert:      "456",
			wantErr:   "",
			expectedTLVs: []TLV{
				{Type: PP2TypeTeleport,
					Value: []byte{0x1, 0x0, 0x3, 0x34, 0x35, 0x36, 0x2, 0x0, 0x3, 0x31, 0x32, 0x33}},
			},
		},
		{
			desc:      "existing signature on proxy line",
			inputTLVs: []TLV{{Type: PP2TypeTeleport, Value: []byte{0x01, 0x02, 0x03}}},
			signature: "123",
			cert:      "456",
			wantErr:   "",
			expectedTLVs: []TLV{
				{Type: PP2TypeTeleport,
					Value: []byte{0x1, 0x0, 0x3, 0x34, 0x35, 0x36, 0x2, 0x0, 0x3, 0x31, 0x32, 0x33}},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			pl := ProxyLine{
				TLVs: tt.inputTLVs,
			}

			err := pl.AddTeleportTLVs([]byte(tt.signature), []byte(tt.cert), nil)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr, "Didn't find expected error")
			} else {
				require.NoError(t, err, "Unexpected error found")
			}

			require.Equal(t, tt.expectedTLVs, pl.TLVs, "Proxy line TLVs mismatch")
		})
	}
}

func TestProxyLine_VerifySignature(t *testing.T) {
	t.Parallel()
	const clusterName = "test-teleport"
	clock := clockwork.NewFakeClockAt(time.Now())
	tlsProxyCert, casGetter, jwtSigner := getTestCertCAsGetterAndSigner(t, clusterName)

	ip := "1.2.3.4"
	ipV6 := "::1"
	sAddr := net.TCPAddr{IP: net.ParseIP(ip), Port: 444}
	dAddr := net.TCPAddr{IP: net.ParseIP(ip), Port: 555}

	sAddrV6 := net.TCPAddr{IP: net.ParseIP(ipV6), Port: 888}
	dAddrV6 := net.TCPAddr{IP: net.ParseIP(ipV6), Port: 999}

	sAddrPseudo, err := getPseudoIPV4(sAddrV6)
	require.NoError(t, err)

	signature, err := jwtSigner.SignPROXYJWT(jwt.PROXYSignParams{
		ClusterName:        clusterName,
		SourceAddress:      sAddr.String(),
		DestinationAddress: dAddr.String(),
	})
	require.NoError(t, err)

	signatureV6, err := jwtSigner.SignPROXYJWT(jwt.PROXYSignParams{
		ClusterName:        clusterName,
		SourceAddress:      sAddrV6.String(),
		DestinationAddress: dAddrV6.String(),
	})
	require.NoError(t, err)

	signatureDowngrade, err := jwtSigner.SignPROXYJWT(jwt.PROXYSignParams{
		ClusterName:        clusterName,
		SourceAddress:      sAddrV6.String(),
		DestinationAddress: dAddr.String(),
	})
	require.NoError(t, err)

	wrongClusterSignature, err := jwtSigner.SignPROXYJWT(jwt.PROXYSignParams{
		ClusterName:        "wrong-cluster",
		SourceAddress:      sAddr.String(),
		DestinationAddress: dAddr.String(),
	})
	require.NoError(t, err)

	wrongSourceSignature, err := jwtSigner.SignPROXYJWT(jwt.PROXYSignParams{
		ClusterName:        clusterName,
		SourceAddress:      "4.3.2.1:1234",
		DestinationAddress: dAddr.String(),
	})
	require.NoError(t, err)

	ca, err := casGetter(context.Background(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	require.NoError(t, err)
	hostCACert := ca.GetTrustedTLSKeyPairs()[0].Cert

	_, wrongCACert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName: "wrong-cluster", Organization: []string{"wrong-cluster"}}, []string{"wrong-cluster"}, time.Hour)
	require.NoError(t, err)

	testCases := []struct {
		desc string

		sAddr            net.TCPAddr
		dAddr            net.TCPAddr
		originalSAddr    *net.TCPAddr
		hostCACert       []byte
		localClusterName string
		signature        string
		cert             []byte

		wantErr string
	}{
		{
			desc:             "wrong CA certificate",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       []byte(fixtures.TLSCACertPEM),
			localClusterName: clusterName,
			signature:        signature,
			cert:             tlsProxyCert,
			wantErr:          "certificate signed by unknown authority",
		},
		{
			desc:             "mangled signing certificate",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        signature,
			cert:             []byte{0x01},
			wantErr:          "x509: malformed certificate",
		},
		{
			desc:             "mangled signature",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        "42",
			cert:             tlsProxyCert,
			wantErr:          "compact JWS format must have three parts",
		},
		{
			desc:             "wrong signature (source address)",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        wrongSourceSignature,
			cert:             tlsProxyCert,
			wantErr:          "validation failed, invalid subject claim (sub)",
		},
		{
			desc:             "wrong signature (cluster)",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        wrongClusterSignature,
			cert:             tlsProxyCert,
			wantErr:          "validation failed, invalid issuer claim (iss)",
		},
		{
			desc:             "wrong CA cert",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       wrongCACert,
			localClusterName: clusterName,
			signature:        signature,
			cert:             tlsProxyCert,
			wantErr:          "certificate signed by unknown authority",
		},
		{
			desc:             "non local cluster",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: "different-cluster",
			signature:        signature,
			cert:             tlsProxyCert,
			wantErr:          "signing certificate is not signed by local cluster CA",
		},
		{
			desc:             "success",
			sAddr:            sAddr,
			dAddr:            dAddr,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        signature,
			cert:             tlsProxyCert,
			wantErr:          "",
		},
		{
			desc:             "success v6",
			sAddr:            sAddrV6,
			dAddr:            dAddrV6,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        signatureV6,
			cert:             tlsProxyCert,
			wantErr:          "",
		},
		{
			desc:             "success ipv6->ipv4 downgrade",
			sAddr:            sAddrPseudo,
			dAddr:            dAddr,
			originalSAddr:    &sAddrV6,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        signatureDowngrade,
			cert:             tlsProxyCert,
			wantErr:          "",
		},
		{
			desc:             "failure ipv6->ipv4 downgrade, pseudo source address does not match signed ipv6",
			sAddr:            sAddr,
			dAddr:            dAddr,
			originalSAddr:    &sAddrV6,
			hostCACert:       hostCACert,
			localClusterName: clusterName,
			signature:        signatureDowngrade,
			cert:             tlsProxyCert,
			wantErr:          "mismatched pseudo IPv4 source and original IPv6 in proxy line",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			pl := ProxyLine{
				Source:      tt.sAddr,
				Destination: tt.dAddr,
			}

			err := pl.AddTeleportTLVs([]byte(tt.signature), tt.cert, tt.originalSAddr)
			require.NoError(t, err)

			ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
				Type:        types.HostCA,
				ClusterName: clusterName,
				ActiveKeys: types.CAKeySet{
					TLS: []*types.TLSKeyPair{
						{
							Cert: tt.hostCACert,
							Key:  nil,
						},
					},
				},
			})
			require.NoError(t, err)

			mockCAGetter := func(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
				return ca, nil
			}

			err = pl.VerifySignature(context.Background(), mockCAGetter, tt.localClusterName, clock)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func FuzzReadProxyLineV1(f *testing.F) {
	f.Add([]byte(sampleProxyV1Line))

	f.Fuzz(func(t *testing.T, b []byte) {
		require.NotPanics(t, func() {
			_, _ = ReadProxyLine(bufio.NewReader(bytes.NewReader(b)))
		})
	})
}

func FuzzReadProxyLineV2(f *testing.F) {
	f.Add(sampleProxyV2LineTLV)
	f.Add(sampleProxyV2Line)
	f.Add(sampleProxyV2LineLocal)

	f.Fuzz(func(t *testing.T, b []byte) {
		require.NotPanics(t, func() {
			_, _ = ReadProxyLineV2(bufio.NewReader(bytes.NewReader(b)))
		})
	})
}

func FuzzProxyLine_Bytes(f *testing.F) {
	f.Add("TCP4", "127.0.0.1", "127.0.0.2", 12345, 42, sampleTLV)
	f.Add("TCP4", "127.0.0.5", "127.0.0.7", 442, 422, sampleTLV)
	f.Add("TCP4", "123.43.12.12", "211.54.120.3", 1, 2, sampleTLV)
	f.Add("TCP4", "123.43.12.12", "211.54.120.3", 1, 2, []byte{})
	f.Add("TCP6", "fe80::a00:27ff:fe8e:8aa8", "fe80::a00:27ff:fe8e:8aa8", 1, 2, sampleTLV)
	f.Add("TCP4", "fe80::a00:27ff:fe8e:8aa8", "fe80::a00:27ff:fe8e:8aa8", 1, 2, []byte{})

	f.Fuzz(func(t *testing.T, protocol, source, dest string, sourcePort, destPort int, tlv []byte) {
		sourceIP := net.ParseIP(source)
		destIP := net.ParseIP(dest)
		p := ProxyLine{
			Protocol:    protocol,
			Source:      net.TCPAddr{IP: sourceIP, Port: sourcePort, Zone: ""},
			Destination: net.TCPAddr{IP: destIP, Port: destPort, Zone: ""},
			TLVs:        nil,
		}

		b, err := p.Bytes()
		if err == nil {
			require.NotNil(t, b)

			p2, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(b)))
			require.NoError(t, err)
			require.Equal(t, p.Protocol, p2.Protocol)
		}
	})
}
