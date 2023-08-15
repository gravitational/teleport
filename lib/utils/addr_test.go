/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"fmt"
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
	require.Equal(t, addr.AddrNetwork, "tcp")
	require.Equal(t, addr.Addr, "localhost:22")

	// scheme + existing port
	addr, err = ParseHostPortAddr("https://localhost", 443)
	require.NoError(t, err)
	require.Equal(t, addr.AddrNetwork, "https")
	require.Equal(t, addr.Addr, "localhost:443")

	// success
	addr, err = ParseHostPortAddr("localhost", 1111)
	require.NoError(t, err)
	require.Equal(t, addr.AddrNetwork, "tcp")
	require.Equal(t, addr.Addr, "localhost:1111")

	// missing port
	addr, err = ParseHostPortAddr("localhost", -1)
	require.Error(t, err)
	require.Nil(t, addr)

	// scheme + missing port
	_, err = ParseHostPortAddr("https://localhost", -1)
	require.NotNil(t, err)
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	var a NetAddr
	require.Equal(t, a.IsEmpty(), true)
}

func TestParse(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("tcp://one:25/path")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "one:25")
	require.Equal(t, addr.Path, "/path")
	require.Equal(t, addr.FullAddress(), "tcp://one:25")
	require.Equal(t, addr.IsEmpty(), false)
	require.Equal(t, addr.Host(), "one")
	require.Equal(t, addr.Port(0), 25)
}

func TestParseIPV6(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("[::1]:49870")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "[::1]:49870")
	require.Equal(t, addr.Path, "")
	require.Equal(t, addr.FullAddress(), "tcp://[::1]:49870")
	require.Equal(t, addr.IsEmpty(), false)
	require.Equal(t, addr.Host(), "::1")
	require.Equal(t, addr.Port(0), 49870)

	// Just square brackets is also valid
	addr, err = ParseAddr("[::1]")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "[::1]")
	require.Equal(t, addr.Host(), "::1")
}

func TestParseEmptyPort(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("one")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "one")
	require.Equal(t, addr.Path, "")
	require.Equal(t, addr.FullAddress(), "tcp://one")
	require.Equal(t, addr.IsEmpty(), false)
	require.Equal(t, addr.Host(), "one")
	require.Equal(t, addr.Port(443), 443)
}

func TestParseHTTP(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("http://one:25/path")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "one:25")
	require.Equal(t, addr.Path, "/path")
	require.Equal(t, addr.FullAddress(), "http://one:25")
	require.Equal(t, addr.IsEmpty(), false)
}

func TestParseDefaults(t *testing.T) {
	t.Parallel()

	addr, err := ParseAddr("host:25")
	require.NoError(t, err)
	require.NotNil(t, addr)
	require.Equal(t, addr.Addr, "host:25")
	require.Equal(t, addr.FullAddress(), "tcp://host:25")
	require.Equal(t, addr.IsEmpty(), false)
}

func TestReplaceLocalhost(t *testing.T) {
	t.Parallel()

	var result string
	result = ReplaceLocalhost("10.10.1.1", "192.168.1.100:399")
	require.Equal(t, result, "10.10.1.1")
	result = ReplaceLocalhost("10.10.1.1:22", "192.168.1.100:399")
	require.Equal(t, result, "10.10.1.1:22")
	result = ReplaceLocalhost("127.0.0.1:22", "192.168.1.100:399")
	require.Equal(t, result, "192.168.1.100:22")
	result = ReplaceLocalhost("0.0.0.0:22", "192.168.1.100:399")
	require.Equal(t, result, "192.168.1.100:22")
	result = ReplaceLocalhost("[::]:22", "192.168.1.100:399")
	require.Equal(t, result, "192.168.1.100:22")
	result = ReplaceLocalhost("[::]:22", "[1::1]:399")
	require.Equal(t, result, "[1::1]:22")
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
		require.Equalf(t, addr.IsLocal(), testCase.expected,
			fmt.Sprintf("test case %v, %v should be local(%v)", i, testCase.in, testCase.expected))
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
		ip := guessHostIP(testCase.addrs)
		require.Empty(t, cmp.Diff(ip, testCase.expected), fmt.Sprintf(testCase.comment))
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
		require.Equalf(t, strings.TrimSpace(string(bytes)), testCase.expected,
			fmt.Sprintf("test case %v, %v should be marshaled to: %v", i, testCase.in, testCase.expected))
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
		require.Empty(t, cmp.Diff(addr, testCase.expected),
			fmt.Sprintf("test case %v, %v should be unmarshalled to: %v", i, testCase.in, testCase.expected))

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
