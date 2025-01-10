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
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils/cert"
)

func TestMain(m *testing.M) {
	InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSelfSignedCert(t *testing.T) {
	t.Parallel()

	creds, err := cert.GenerateSelfSignedCert([]string{"example.com"}, nil)
	require.NoError(t, err)
	signer, err := keys.ParsePrivateKey(creds.PrivateKey)
	require.NoError(t, err)
	pub, err := keys.ParsePublicKey(creds.PublicKey)
	require.NoError(t, err)
	require.Equal(t, signer.Public(), pub)
}

func TestRandomDuration(t *testing.T) {
	t.Parallel()

	expectedMin := time.Duration(0)
	expectedMax := time.Second * 10
	for i := 0; i < 50; i++ {
		dur := RandomDuration(expectedMax)
		require.GreaterOrEqual(t, dur, expectedMin)
		require.Less(t, dur, expectedMax)
	}
}

func TestRemoveFromSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		slice    []string
		target   string
		expected []string
	}{
		{name: "remove from empty", slice: []string{}, target: "a", expected: []string{}},
		{name: "remove only element", slice: []string{"a"}, target: "a", expected: []string{}},
		{name: "remove a", slice: []string{"a", "b"}, target: "a", expected: []string{"b"}},
		{name: "remove b", slice: []string{"a", "b"}, target: "b", expected: []string{"a"}},
		{name: "remove duplicate elements", slice: []string{"a", "a", "b"}, target: "a", expected: []string{"b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, RemoveFromSlice(tc.slice, tc.target))
		})
	}
}

// TestMinVersions tests versions compatibility checking
func TestMinVersions(t *testing.T) {
	t.Parallel()

	type tc struct {
		info      string
		client    string
		minClient string
	}
	successTestCases := []tc{
		{info: "client same as min version", client: "1.0.0", minClient: "1.0.0"},
		{info: "client newer than min version", client: "1.1.0", minClient: "1.0.0"},
		{info: "pre-releases clients are ok", client: "1.1.0-alpha.1", minClient: "1.0.0"},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			require.NoError(t, CheckMinVersion(testCase.client, testCase.minClient))
			assert.True(t, MeetsMinVersion(testCase.client, testCase.minClient), "MeetsMinVersion expected to succeed")
		})
	}

	failTestCases := []tc{
		{info: "client older than min version", client: "1.0.0", minClient: "1.1.0"},
		{info: "older pre-releases are no ok", client: "1.1.0-alpha.1", minClient: "1.1.0"},
	}
	for _, testCase := range failTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			fixtures.AssertBadParameter(t, CheckMinVersion(testCase.client, testCase.minClient))
			assert.False(t, MeetsMinVersion(testCase.client, testCase.minClient), "MeetsMinVersion expected to fail")
		})
	}
}

// TestMaxVersions tests versions compatibility checking
func TestMaxVersions(t *testing.T) {
	t.Parallel()

	type tc struct {
		info      string
		client    string
		maxClient string
	}
	successTestCases := []tc{
		{info: "client same as max version", client: "1.0.0", maxClient: "1.0.0"},
		{info: "client older than max version", client: "1.1.0", maxClient: "1.2.0"},
		{info: "pre-releases clients are ok", client: "1.0.0-alpha.1", maxClient: "1.0.0"},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			require.NoError(t, CheckMaxVersion(testCase.client, testCase.maxClient))
			assert.True(t, MeetsMaxVersion(testCase.client, testCase.maxClient), "MeetsMinVersion expected to succeed")
		})
	}

	failTestCases := []tc{
		{info: "client newer than max version", client: "1.3.0", maxClient: "1.1.0"},
		{info: "newer pre-releases are no ok", client: "1.1.0", maxClient: "1.1.0-alpha.1"},
	}
	for _, testCase := range failTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			fixtures.AssertBadParameter(t, CheckMaxVersion(testCase.client, testCase.maxClient))
			assert.False(t, MeetsMaxVersion(testCase.client, testCase.maxClient), "MeetsMinVersion expected to fail")
		})
	}
}

// TestClickableURL tests clickable URL conversions
func TestClickableURL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		info string
		in   string
		out  string
	}{
		{info: "original URL is OK", in: "http://127.0.0.1:3000/hello", out: "http://127.0.0.1:3000/hello"},
		{info: "unspecified IPV6", in: "http://[::]:5050/howdy", out: "http://127.0.0.1:5050/howdy"},
		{info: "unspecified IPV4", in: "http://0.0.0.0:5050/howdy", out: "http://127.0.0.1:5050/howdy"},
		{info: "specified IPV4", in: "http://192.168.1.1:5050/howdy", out: "http://192.168.1.1:5050/howdy"},
		{info: "specified IPV6", in: "http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:5050/howdy", out: "http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:5050/howdy"},
		{info: "hostname", in: "http://example.com:3000/howdy", out: "http://example.com:3000/howdy"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.info, func(t *testing.T) {
			out := ClickableURL(testCase.in)
			require.Equal(t, testCase.out, out)
		})
	}
}

// TestParseAdvertiseAddr tests parsing of advertise address
func TestParseAdvertiseAddr(t *testing.T) {
	t.Parallel()

	type tc struct {
		info string
		in   string
		host string
		port string
	}
	successTestCases := []tc{
		{info: "ok address", in: "192.168.1.1", host: "192.168.1.1"},
		{info: "trim space", in: "   192.168.1.1    ", host: "192.168.1.1"},
		{info: "ok address and port", in: "192.168.1.1:22", host: "192.168.1.1", port: "22"},
		{info: "ok host", in: "localhost", host: "localhost"},
		{info: "ok host and port", in: "localhost:33", host: "localhost", port: "33"},
		{info: "ipv6 address", in: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", host: "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{info: "ipv6 address and port", in: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:443", host: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", port: "443"},
	}
	for _, testCase := range successTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			host, port, err := ParseAdvertiseAddr(testCase.in)
			require.NoError(t, err)
			require.Equal(t, testCase.host, host)
			require.Equal(t, testCase.port, port)
		})
	}

	failTestCases := []tc{
		{info: "multicast address", in: "224.0.0.0"},
		{info: "multicast address", in: "   224.0.0.0   "},
		{info: "ok address and bad port", in: "192.168.1.1:b"},
		{info: "missing host ", in: ":33"},
		{info: "missing port", in: "localhost:"},
	}
	for _, testCase := range failTestCases {
		t.Run(testCase.info, func(t *testing.T) {
			_, _, err := ParseAdvertiseAddr(testCase.in)
			fixtures.AssertBadParameter(t, err)
		})
	}
}

// TestGlobToRegexp tests replacement of glob-style wildcard values
// with regular expression compatible value
func TestGlobToRegexp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		comment string
		in      string
		out     string
	}{
		{
			comment: "simple values are not replaced",
			in:      "value-value",
			out:     "value-value",
		},
		{
			comment: "wildcard and start of string is replaced with regexp wildcard expression",
			in:      "*",
			out:     "(.*)",
		},
		{
			comment: "wildcard is replaced with regexp wildcard expression",
			in:      "a-*-b-*",
			out:     "a-(.*)-b-(.*)",
		},
		{
			comment: "special chars are quoted",
			in:      "a-.*-b-*$",
			out:     `a-\.(.*)-b-(.*)\$`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.comment, func(t *testing.T) {
			out := GlobToRegexp(testCase.in)
			require.Equal(t, testCase.out, out)
		})
	}
}

func TestIsValidHostname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		hostname string
		assert   require.BoolAssertionFunc
	}{
		{
			name:     "normal hostname",
			hostname: "some-host-1.example.com",
			assert:   require.True,
		},
		{
			name:     "only lower case works",
			hostname: "only-lower-case-works",
			assert:   require.True,
		},
		{
			name:     "mixed upper case fails",
			hostname: "mixed-UPPER-CASE-fails",
			assert:   require.False,
		},
		{
			name:     "one component",
			hostname: "example",
			assert:   require.True,
		},
		{
			name:     "empty",
			hostname: "",
			assert:   require.False,
		},
		{
			name:     "invalid characters",
			hostname: "some spaces.example.com",
			assert:   require.False,
		},
		{
			name:     "empty label",
			hostname: "somewhere..example.com",
			assert:   require.False,
		},
		{
			name:     "label too long",
			hostname: strings.Repeat("x", 64) + ".example.com",
			assert:   require.False,
		},
		{
			name:     "hostname too long",
			hostname: strings.Repeat("x.", 256) + ".example.com",
			assert:   require.False,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, IsValidHostname(tc.hostname))
		})
	}
}

// TestReplaceRegexp tests regexp-style replacement of values
func TestReplaceRegexp(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		comment string
		expr    string
		replace string
		in      string
		out     string
		err     error
	}{
		{
			comment: "simple values are replaced directly",
			expr:    "value",
			replace: "value",
			in:      "value",
			out:     "value",
		},
		{
			comment: "no match returns explicit not found error",
			expr:    "value",
			replace: "value",
			in:      "val",
			err:     trace.NotFound(""),
		},
		{
			comment: "empty value is no match",
			expr:    "",
			replace: "value",
			in:      "value",
			err:     trace.NotFound(""),
		},
		{
			comment: "bad regexp results in bad parameter error",
			expr:    "^(($",
			replace: "value",
			in:      "val",
			err:     trace.BadParameter(""),
		},
		{
			comment: "full match is supported",
			expr:    "^value$",
			replace: "value",
			in:      "value",
			out:     "value",
		},
		{
			comment: "wildcard replaces to itself",
			expr:    "^(.*)$",
			replace: "$1",
			in:      "value",
			out:     "value",
		},
		{
			comment: "wildcard replaces to predefined value",
			expr:    "*",
			replace: "boo",
			in:      "different",
			out:     "boo",
		},
		{
			comment: "wildcard replaces empty string to predefined value",
			expr:    "*",
			replace: "boo",
			in:      "",
			out:     "boo",
		},
		{
			comment: "regexp wildcard replaces to itself",
			expr:    "^(.*)$",
			replace: "$1",
			in:      "value",
			out:     "value",
		},
		{
			comment: "partial conversions are supported",
			expr:    "^test-(.*)$",
			replace: "replace-$1",
			in:      "test-hello",
			out:     "replace-hello",
		},
		{
			comment: "partial conversions are supported",
			expr:    "^test-(.*)$",
			replace: "replace-$1",
			in:      "test-hello",
			out:     "replace-hello",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.comment, func(t *testing.T) {
			out, err := ReplaceRegexp(testCase.expr, testCase.replace, testCase.in)
			if testCase.err == nil {
				require.NoError(t, err)
				require.Equal(t, testCase.out, out)
			} else {
				require.IsType(t, testCase.err, err)
			}
		})
	}
}

// TestContainsExpansion tests whether string contains expansion value
func TestContainsExpansion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		comment  string
		val      string
		contains bool
	}{
		{
			comment:  "detect simple expansion",
			val:      "$1",
			contains: true,
		},
		{
			comment:  "escaping is honored",
			val:      "$$",
			contains: false,
		},
		{
			comment:  "escaping is honored",
			val:      "$$$$",
			contains: false,
		},
		{
			comment:  "escaping is honored",
			val:      "$$$$$",
			contains: false,
		},
		{
			comment:  "escaping and expansion",
			val:      "$$$$$1",
			contains: true,
		},
		{
			comment:  "expansion with brackets",
			val:      "${100}",
			contains: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.comment, func(t *testing.T) {
			contains := ContainsExpansion(testCase.val)
			require.Equal(t, testCase.contains, contains)
		})
	}
}

// TestMarshalYAML tests marshal/unmarshal of elements
func TestMarshalYAML(t *testing.T) {
	t.Parallel()

	type kv struct {
		Key string
	}
	testCases := []struct {
		comment  string
		val      interface{}
		expected interface{}
		isDoc    bool
	}{
		{
			comment: "simple yaml value",
			val:     "hello",
		},
		{
			comment: "list of yaml types",
			val:     []interface{}{"hello", "there"},
		},
		{
			comment:  "list of yaml documents",
			val:      []interface{}{kv{Key: "a"}, kv{Key: "b"}},
			expected: []interface{}{map[string]interface{}{"Key": "a"}, map[string]interface{}{"Key": "b"}},
			isDoc:    true,
		},
		{
			comment:  "list of pointers to yaml docs",
			val:      []interface{}{kv{Key: "a"}, &kv{Key: "b"}},
			expected: []interface{}{map[string]interface{}{"Key": "a"}, map[string]interface{}{"Key": "b"}},
			isDoc:    true,
		},
		{
			comment: "list of maps",
			val:     []interface{}{map[string]interface{}{"Key": "a"}, map[string]interface{}{"Key": "b"}},
			isDoc:   true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.comment, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := WriteYAML(buf, testCase.val)
			require.NoError(t, err)
			if testCase.isDoc {
				require.Contains(t, buf.String(), yamlDocDelimiter)
			}
			out, err := ReadYAML(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)
			if testCase.expected != nil {
				require.Equal(t, testCase.expected, out)
			} else {
				require.Equal(t, testCase.val, out)
			}
		})
	}
}

// TestReadToken tests reading token from file and as is
func TestTryReadValueAsFile(t *testing.T) {
	t.Parallel()

	tok, err := TryReadValueAsFile("token")
	require.Equal(t, "token", tok)
	require.NoError(t, err)

	_, err = TryReadValueAsFile("/tmp/non-existent-token-for-teleport-tests-not-found")
	fixtures.AssertNotFound(t, err)

	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	err = os.WriteFile(tokenPath, []byte("shmoken"), 0644)
	require.NoError(t, err)

	tok, err = TryReadValueAsFile(tokenPath)
	require.NoError(t, err)
	require.Equal(t, "shmoken", tok)
}

// TestStringsSet makes sure that nil slice returns empty set (less error prone)
func TestStringsSet(t *testing.T) {
	t.Parallel()

	out := StringsSet(nil)
	require.Empty(t, out)
	require.NotNil(t, out)
}

func TestReadAtMost(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		limit int64
		data  string
		err   error
	}{
		{name: "limit reached at 4", limit: 4, data: "hell", err: ErrLimitReached},
		{name: "limit reached at 5", limit: 5, data: "hello", err: ErrLimitReached},
		{name: "limit not reached", limit: 6, data: "hello", err: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader("hello")
			data, err := ReadAtMost(r, tc.limit)
			require.Equal(t, []byte(tc.data), data)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestByteCount(t *testing.T) {
	tt := []struct {
		name     string
		size     int64
		expected string
	}{
		{
			name:     "1 byte",
			size:     1,
			expected: "1 B",
		},
		{
			name:     "2 byte2",
			size:     2,
			expected: "2 B",
		},
		{
			name:     "1kb",
			size:     1000,
			expected: "1.0 kB",
		},
		{
			name:     "1mb",
			size:     1000_000,
			expected: "1.0 MB",
		},
		{
			name:     "1gb",
			size:     1000_000_000,
			expected: "1.0 GB",
		},
		{
			name:     "1tb",
			size:     1000_000_000_000,
			expected: "1.0 TB",
		},
		{
			name:     "1.6 kb",
			size:     1600,
			expected: "1.6 kB",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ByteCount(tc.size))
		})
	}
}
