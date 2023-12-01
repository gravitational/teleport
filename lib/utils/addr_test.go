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

package utils

import (
	"net"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestParseHostPort(t *testing.T) {
	t.Parallel()

	// success
	addr, err := ParseHostPortAddr("localhost:22", -1)
	require.NoError(t, err)
	require.Equal(t, "tcp", addr.AddrNetwork)
	require.Equal(t, "localhost:22", addr.Addr)

	// scheme + existing port
	addr, err = ParseHostPortAddr("https://localhost", 443)
	require.NoError(t, err)
	require.Equal(t, "https", addr.AddrNetwork)
	require.Equal(t, "localhost:443", addr.Addr)

	// success
	addr, err = ParseHostPortAddr("localhost", 1111)
	require.NoError(t, err)
	require.Equal(t, "tcp", addr.AddrNetwork)
	require.Equal(t, "localhost:1111", addr.Addr)

	// missing port
	addr, err = ParseHostPortAddr("localhost", -1)
	require.Error(t, err)
	require.Nil(t, addr)

	// scheme + missing port
	_, err = ParseHostPortAddr("https://localhost", -1)
	require.Error(t, err)
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	var a NetAddr
	require.True(t, a.IsEmpty())
}

func TestParse(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("tcp://one:25/path")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "one:25", addr.Addr)
	require.Equal(t, "/path", addr.Path)
	require.Equal(t, "tcp://one:25", addr.FullAddress())
	require.False(t, addr.IsEmpty())
	require.Equal(t, "one", addr.Host())
	require.Equal(t, 25, addr.Port(0))
}

func TestParseIPV6(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("[::1]:49870")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "[::1]:49870", addr.Addr)
	require.Empty(t, addr.Path)
	require.Equal(t, "tcp://[::1]:49870", addr.FullAddress())
	require.False(t, addr.IsEmpty())
	require.Equal(t, "::1", addr.Host())
	require.Equal(t, 49870, addr.Port(0))

	// Just square brackets is also valid
	addr, err = ParseAddr("[::1]")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "[::1]", addr.Addr)
	require.Equal(t, "::1", addr.Host())
}

func TestParseEmptyPort(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("one")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "one", addr.Addr)
	require.Empty(t, addr.Path)
	require.Equal(t, "tcp://one", addr.FullAddress())
	require.False(t, addr.IsEmpty())
	require.Equal(t, "one", addr.Host())
	require.Equal(t, 443, addr.Port(443))
}

func TestParseHTTP(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("http://one:25/path")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "one:25", addr.Addr)
	require.Equal(t, "/path", addr.Path)
	require.Equal(t, "http://one:25", addr.FullAddress())
	require.False(t, addr.IsEmpty())
}

func TestParseDefaults(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("host:25")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, "host:25", addr.Addr)
	require.Equal(t, "tcp://host:25", addr.FullAddress())
	require.False(t, addr.IsEmpty())
}

func TestReplaceLocalhost(t *testing.T) {
	t.Parallel()

	var result string
	result = ReplaceLocalhost("10.10.1.1", "192.168.1.100:399")
	require.Equal(t, "10.10.1.1", result)
	result = ReplaceLocalhost("10.10.1.1:22", "192.168.1.100:399")
	require.Equal(t, "10.10.1.1:22", result)
	result = ReplaceLocalhost("127.0.0.1:22", "192.168.1.100:399")
	require.Equal(t, "192.168.1.100:22", result)
	result = ReplaceLocalhost("0.0.0.0:22", "192.168.1.100:399")
	require.Equal(t, "192.168.1.100:22", result)
	result = ReplaceLocalhost("[::]:22", "192.168.1.100:399")
	require.Equal(t, "192.168.1.100:22", result)
	result = ReplaceLocalhost("[::]:22", "[1::1]:399")
	require.Equal(t, "[1::1]:22", result)
}

func TestLocalAddrs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in       string
		expected bool
	}{
		{in: "127.0.0.1:5000", expected: true},
		{in: "localhost:5000", expected: true},
		{in: "127.0.0.2:5000", expected: true},
		{in: "tcp://127.0.0.2:5000", expected: true},
		{in: "tcp://10.0.0.3:5000", expected: false},
		{in: "tcp://hostname:5000", expected: false},
	}
	for i, testCase := range testCases {
		addr, err := ParseAddr(testCase.in)
		require.NoError(t, err)
		require.Equal(t, testCase.expected, addr.IsLocal(), "test case %v, %v should be local(%v)", i, testCase.in, testCase.expected)
	}
}

func TestGuessesIPAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		addrs    []net.Addr
		expected net.IP
		comment  string
	}{
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("10.0.100.80")},
				&net.IPAddr{IP: net.ParseIP("192.168.1.80")},
				&net.IPAddr{IP: net.ParseIP("172.16.0.0")},
				&net.IPAddr{IP: net.ParseIP("172.31.255.255")},
			},
			expected: net.ParseIP("10.0.100.80"),
			comment:  "prefers 10.0.0.0/8",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.168.1.80")},
				&net.IPAddr{IP: net.ParseIP("172.31.12.1")},
			},
			expected: net.ParseIP("192.168.1.80"),
			comment:  "prefers 192.168.0.0/16",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.167.255.255")},
				&net.IPAddr{IP: net.ParseIP("172.15.0.0")},
				&net.IPAddr{IP: net.ParseIP("172.32.1.1")},
				&net.IPAddr{IP: net.ParseIP("172.30.1.1")},
			},
			expected: net.ParseIP("172.30.1.1"),
			comment:  "identifies private IP by netmask",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("172.1.1.1")},
				&net.IPAddr{IP: net.ParseIP("172.30.0.1")},
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
			},
			expected: net.ParseIP("172.30.0.1"),
			comment:  "prefers 172.16.0.0/12",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.168.12.1")},
				&net.IPAddr{IP: net.ParseIP("192.168.12.2")},
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
			},
			expected: net.ParseIP("192.168.12.2"),
			comment:  "prefers last",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("::1")},
				&net.IPAddr{IP: net.ParseIP("fe80::af:6dff:fefd:150f")},
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
			},
			expected: net.ParseIP("52.35.21.180"),
			comment:  "ignores IPv6",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("::1")},
				&net.IPAddr{IP: net.ParseIP("fe80::af:6dff:fefd:150f")},
			},
			expected: net.ParseIP("127.0.0.1"),
			comment:  "falls back to ipv4 loopback",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.comment, func(t *testing.T) {
			ip := guessHostIP(testCase.addrs)
			require.Empty(t, cmp.Diff(ip, testCase.expected))
		})
	}
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in       *NetAddr
		expected string
	}{
		{in: &NetAddr{Addr: "localhost:5000"}, expected: "localhost:5000"},
		{in: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000"}, expected: "tcp://localhost:5000"},
		{in: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000", Path: "/path"}, expected: "tcp://localhost:5000/path"},
		{in: &NetAddr{AddrNetwork: "unix", Path: "/path"}, expected: "unix:///path"},
	}

	for i, testCase := range testCases {
		bytes, err := yaml.Marshal(testCase.in)
		require.NoError(t, err)
		require.Equal(t, testCase.expected, strings.TrimSpace(string(bytes)), "test case %v, %v should be marshaled to: %v", i, testCase.in, testCase.expected)
	}
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in       string
		expected *NetAddr
	}{
		{in: "localhost:5000", expected: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000"}},
		{in: "tcp://localhost:5000/path", expected: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000", Path: "/path"}},
		{in: "unix:///path", expected: &NetAddr{AddrNetwork: "unix", Addr: "/path"}},
	}

	for i, testCase := range testCases {
		addr := &NetAddr{}
		err := yaml.Unmarshal([]byte(testCase.in), addr)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(addr, testCase.expected), "test case %v, %v should be unmarshalled to: %v", i, testCase.in, testCase.expected)

	}
}

func TestParseMultiple(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in  []string
		out []NetAddr
	}{
		{
			in: []string{
				"https://localhost:3080",
				"tcp://example:587/path",
				"[::1]:465",
			},
			out: []NetAddr{
				{Addr: "localhost:3080", AddrNetwork: "https"},
				{Addr: "example:587", AddrNetwork: "tcp", Path: "/path"},
				{Addr: "[::1]:465", AddrNetwork: "tcp"},
			},
		},
	}
	for _, test := range tests {
		parsed, err := ParseAddrs(test.in)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(parsed, test.out))
	}
}
