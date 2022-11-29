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

package multiplexer

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// PROXY v1
	sampleProxyV1Line = "PROXY TCP4 127.0.0.1 127.0.0.2 12345 42\r\n"

	// PROXY v2

	// source=127.0.0.1:12345 destination=127.0.0.2:42
	sampleIPv4Addresses = []byte{0x7F, 0x00, 0x00, 0x01, 0x7F, 0x00, 0x00, 0x02, 0x30, 0x39, 0x00, 0x2A}
	// {0x21, 0x11, 0x00, 0x0C} - 4 bits version, 4 bits command, 4 bits address family, 4 bits protocol, 16 bits length
	sampleProxyV2Line = bytes.Join([][]byte{proxyV2Prefix, {0x21, 0x11, 0x00, 0x0C}, sampleIPv4Addresses}, nil)
	// Proxy line with LOCAL command
	sampleProxyV2LineLocal    = bytes.Join([][]byte{proxyV2Prefix, {0x20, 0x11, 0x00, 0x00}}, nil)
	sampleTLV                 = []byte{byte(PP2TypeTeleport), 0x00, 0x03, 0x01, 0x02, 0x03}
	sampleEmptyTLV            = []byte{byte(PP2TypeTeleport), 0x00, 0x00}
	sampleProxyV2LineTLV      = bytes.Join([][]byte{proxyV2Prefix, {0x21, 0x11, 0x00, 0x12}, sampleIPv4Addresses, sampleTLV}, nil)
	sampleProxyV2LineEmptyTLV = bytes.Join([][]byte{proxyV2Prefix, {0x21, 0x11, 0x00, 0x0F}, sampleIPv4Addresses, sampleEmptyTLV}, nil)
)

func TestReadProxyLine(t *testing.T) {
	t.Parallel()

	t.Run("empty line", func(t *testing.T) {
		_, err := ReadProxyLine(bufio.NewReader(bytes.NewReader([]byte{})))
		require.ErrorIs(t, err, io.EOF)
	})
	t.Run("malformed line", func(t *testing.T) {
		_, err := ReadProxyLine(bufio.NewReader(bytes.NewReader([]byte("JIBBERISH\r\n"))))
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
		_, err := ReadProxyLineV2(bufio.NewReader(bytes.NewReader(append(proxyV2Prefix, []byte("JIBBERISH")...))))
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
		require.Equal(t, 1, len(pl.TLVs))
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
		require.Equal(t, 1, len(pl.TLVs))
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
		assert.Equal(t, 28, len(b))
		assert.Equal(t, sampleProxyV2Line, b)

		pl2, err := ReadProxyLineV2(bufio.NewReader(bytes.NewBuffer(b)))
		assert.NoError(t, err)
		assert.Equal(t, TCP4, pl2.Protocol)
		assert.Equal(t, "127.0.0.1:12345", pl2.Source.String())
		assert.Equal(t, "127.0.0.2:42", pl2.Destination.String())
		assert.Equal(t, 0, len(pl2.TLVs))
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
		assert.Equal(t, 35, len(b))

		pl2, err := ReadProxyLineV2(bufio.NewReader(bytes.NewBuffer(b)))
		assert.NoError(t, err)
		assert.Equal(t, TCP4, pl2.Protocol)
		assert.Equal(t, "127.0.0.1:12345", pl2.Source.String())
		assert.Equal(t, "127.0.0.2:42", pl2.Destination.String())
		assert.Equal(t, 1, len(pl2.TLVs))
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
