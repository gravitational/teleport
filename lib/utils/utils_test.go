/*
Copyright 2015-2019 Gravitational, Inc.

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
	"bytes"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/fixtures"

	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	InitLoggerForTests()
	os.Exit(m.Run())
}

func TestUtils(t *testing.T) { check.TestingT(t) }

type UtilsSuite struct {
}

var _ = check.Suite(&UtilsSuite{})

// TestCapitalize tests capitalize function
func (s *UtilsSuite) TestCapitalize(c *check.C) {
	type testCase struct {
		in  string
		out string
	}
	cases := []testCase{
		{in: "hello there", out: "Hello there"},
		{in: " ", out: " "},
		{in: "", out: ""},
	}
	for i, tc := range cases {
		comment := check.Commentf("Test case %v", i)
		c.Assert(Capitalize(tc.in), check.Equals, tc.out, comment)
	}
}

// TestLinear tests retry logic
func (s *UtilsSuite) TestLinear(c *check.C) {
	r, err := NewLinear(LinearConfig{
		Step: time.Second,
		Max:  3 * time.Second,
	})
	c.Assert(err, check.IsNil)
	c.Assert(r.Duration(), check.Equals, time.Duration(0))
	r.Inc()
	c.Assert(r.Duration(), check.Equals, time.Second)
	r.Inc()
	c.Assert(r.Duration(), check.Equals, 2*time.Second)
	r.Inc()
	c.Assert(r.Duration(), check.Equals, 3*time.Second)
	r.Inc()
	c.Assert(r.Duration(), check.Equals, 3*time.Second)
	r.Reset()
	c.Assert(r.Duration(), check.Equals, time.Duration(0))
}

func (s *UtilsSuite) TestHostUUID(c *check.C) {
	// call twice, get same result
	dir := c.MkDir()
	uuid, err := ReadOrMakeHostUUID(dir)
	c.Assert(uuid, check.HasLen, 36)
	c.Assert(err, check.IsNil)
	uuidCopy, err := ReadOrMakeHostUUID(dir)
	c.Assert(err, check.IsNil)
	c.Assert(uuid, check.Equals, uuidCopy)

	// call with a read-only dir, make sure to get an error
	uuid, err = ReadOrMakeHostUUID("/bad-location")
	c.Assert(err, check.NotNil)
	c.Assert(uuid, check.Equals, "")
	c.Assert(err.Error(), check.Matches, "^.*no such file or directory.*$")

	// newlines are getting ignored
	dir = c.MkDir()
	id := "id-with-newline\n"
	err = ioutil.WriteFile(filepath.Join(dir, HostUUIDFile), []byte(id), 0666)
	c.Assert(err, check.IsNil)
	out, err := ReadHostUUID(dir)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.Equals, strings.TrimSpace(id))
}

func (s *UtilsSuite) TestSelfSignedCert(c *check.C) {
	creds, err := GenerateSelfSignedCert([]string{"example.com"})
	c.Assert(err, check.IsNil)
	c.Assert(creds, check.NotNil)
	c.Assert(len(creds.PublicKey)/100, check.Equals, 4)
	c.Assert(len(creds.PrivateKey)/100, check.Equals, 16)
}

func (s *UtilsSuite) TestRandomDuration(c *check.C) {
	expectedMin := time.Duration(0)
	expectedMax := time.Second * 10
	for i := 0; i < 50; i++ {
		dur := RandomDuration(expectedMax)
		c.Assert(dur >= expectedMin, check.Equals, true)
		c.Assert(dur < expectedMax, check.Equals, true)
	}
}

func (s *UtilsSuite) TestMiscFunctions(c *check.C) {
	// SliceContainsStr
	c.Assert(SliceContainsStr([]string{"two", "one"}, "one"), check.Equals, true)
	c.Assert(SliceContainsStr([]string{"two", "one"}, "five"), check.Equals, false)
	c.Assert(SliceContainsStr([]string(nil), "one"), check.Equals, false)

	// Deduplicate
	c.Assert(Deduplicate([]string{}), check.DeepEquals, []string{})
	c.Assert(Deduplicate([]string{"a", "b"}), check.DeepEquals, []string{"a", "b"})
	c.Assert(Deduplicate([]string{"a", "b", "b", "a", "c"}), check.DeepEquals, []string{"a", "b", "c"})

	// RemoveFromSlice
	c.Assert(RemoveFromSlice([]string{}, "a"), check.DeepEquals, []string{})
	c.Assert(RemoveFromSlice([]string{"a"}, "a"), check.DeepEquals, []string{})
	c.Assert(RemoveFromSlice([]string{"a", "b"}, "a"), check.DeepEquals, []string{"b"})
	c.Assert(RemoveFromSlice([]string{"a", "b"}, "b"), check.DeepEquals, []string{"a"})
	c.Assert(RemoveFromSlice([]string{"a", "a", "b"}, "a"), check.DeepEquals, []string{"b"})
}

// TestVersions tests versions compatibility checking
func (s *UtilsSuite) TestVersions(c *check.C) {
	testCases := []struct {
		info      string
		client    string
		minClient string
		err       error
	}{
		{info: "client older than min version", client: "1.0.0", minClient: "1.1.0", err: trace.BadParameter("")},
		{info: "client same as min version", client: "1.0.0", minClient: "1.0.0"},
		{info: "client newer than min version", client: "1.1.0", minClient: "1.0.0"},
		{info: "pre-releases clients are ok", client: "1.1.0-alpha.1", minClient: "1.0.0"},
		{info: "older pre-releases are no ok", client: "1.1.0-alpha.1", minClient: "1.1.0", err: trace.BadParameter("")},
	}
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %q", i, testCase.info)
		err := CheckVersions(testCase.client, testCase.minClient)
		if testCase.err == nil {
			c.Assert(err, check.IsNil, comment)
		} else {
			c.Assert(err, check.FitsTypeOf, testCase.err, comment)
		}
	}
}

// TestClickableURL tests clickable URL conversions
func (s *UtilsSuite) TestClickableURL(c *check.C) {
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
	}
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %q", i, testCase.info)
		out := ClickableURL(testCase.in)
		c.Assert(out, check.Equals, testCase.out, comment)
	}
}

// TestParseSessionsURI parses sessions URI
func (s *UtilsSuite) TestParseSessionsURI(c *check.C) {
	testCases := []struct {
		info string
		in   string
		url  *url.URL
		err  error
	}{
		{info: "local default file system URI", in: "/home/log", url: &url.URL{Scheme: teleport.SchemeFile, Path: "/home/log"}},
		{info: "explicit filesystem URI", in: "file:///home/log", url: &url.URL{Scheme: teleport.SchemeFile, Path: "/home/log"}},
		{info: "S3 URI", in: "s3://my-bucket", url: &url.URL{Scheme: teleport.SchemeS3, Host: "my-bucket"}},
	}
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %q", i, testCase.info)
		out, err := ParseSessionsURI(testCase.in)
		if testCase.err == nil {
			c.Assert(err, check.IsNil, comment)
			c.Assert(out, check.DeepEquals, testCase.url)
		} else {
			c.Assert(err, check.FitsTypeOf, testCase.err, comment)
		}
	}
}

// TestParseAdvertiseAddr tests parsing of advertise address
func (s *UtilsSuite) TestParseAdvertiseAddr(c *check.C) {
	testCases := []struct {
		info string
		in   string
		host string
		port string
		err  error
	}{
		{info: "ok address", in: "192.168.1.1", host: "192.168.1.1"},
		{info: "trim space", in: "   192.168.1.1    ", host: "192.168.1.1"},
		{info: "multicast address", in: "224.0.0.0", err: trace.BadParameter("")},
		{info: "multicast address", in: "   224.0.0.0   ", err: trace.BadParameter("")},
		{info: "ok address and port", in: "192.168.1.1:22", host: "192.168.1.1", port: "22"},
		{info: "ok address and bad port", in: "192.168.1.1:b", err: trace.BadParameter("")},
		{info: "ok host", in: "localhost", host: "localhost"},
		{info: "ok host and port", in: "localhost:33", host: "localhost", port: "33"},
		{info: "missing host ", in: ":33", err: trace.BadParameter("")},
		{info: "missing port", in: "localhost:", err: trace.BadParameter("")},
		{info: "ipv6 address", in: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", host: "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{info: "ipv6 address and port", in: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:443", host: "2001:0db8:85a3:0000:0000:8a2e:0370:7334", port: "443"},
	}
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %q", i, testCase.info)
		host, port, err := ParseAdvertiseAddr(testCase.in)
		if testCase.err == nil {
			c.Assert(err, check.IsNil, comment)
			c.Assert(host, check.Equals, testCase.host)
			c.Assert(port, check.Equals, testCase.port)
		} else {
			c.Assert(err, check.FitsTypeOf, testCase.err, comment)
		}
	}
}

// TestGlobToRegexp tests replacement of glob-style wildcard values
// with regular expression compatible value
func (s *UtilsSuite) TestGlobToRegexp(c *check.C) {
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
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %v", i, testCase.comment)
		out := GlobToRegexp(testCase.in)
		c.Assert(out, check.Equals, testCase.out, comment)
	}
}

// TestReplaceRegexp tests regexp-style replacement of values
func (s *UtilsSuite) TestReplaceRegexp(c *check.C) {
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
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %v", i, testCase.comment)
		out, err := ReplaceRegexp(testCase.expr, testCase.replace, testCase.in)
		if testCase.err == nil {
			c.Assert(err, check.IsNil, comment)
			c.Assert(out, check.Equals, testCase.out, comment)
		} else {
			comment := check.Commentf("test case %v %v, expected type %T, got type %T", i, testCase.comment, testCase.err, err)
			c.Assert(err, check.FitsTypeOf, testCase.err, comment)
		}
	}
}

// TestContainsExpansion tests whether string contains expansion value
func (s *UtilsSuite) TestContainsExpansion(c *check.C) {
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
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %v", i, testCase.comment)
		contains := ContainsExpansion(testCase.val)
		c.Assert(contains, check.Equals, testCase.contains, comment)
	}
}

// TestMarshalYAML tests marshal/unmarshal of elements
func (s *UtilsSuite) TestMarshalYAML(c *check.C) {
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
	for i, testCase := range testCases {
		comment := check.Commentf("test case %v %v", i, testCase.comment)
		buf := &bytes.Buffer{}
		err := WriteYAML(buf, testCase.val)
		c.Assert(err, check.IsNil, comment)
		if testCase.isDoc {
			c.Assert(bytes.Contains(buf.Bytes(),
				[]byte(yamlDocDelimiter)), check.Equals, true,
				check.Commentf("test case %v: expected to find --- in %q", testCase.comment, buf.String()))
		}
		out, err := ReadYAML(bytes.NewReader(buf.Bytes()))
		c.Assert(err, check.IsNil, comment)
		if testCase.expected != nil {
			fixtures.DeepCompare(c, out, testCase.expected)
		} else {
			fixtures.DeepCompare(c, out, testCase.val)
		}
	}
}

// TestReadToken tests reading token from file and as is
func (s *UtilsSuite) TestReadToken(c *check.C) {
	tok, err := ReadToken("token")
	c.Assert(tok, check.Equals, "token")
	c.Assert(err, check.IsNil)

	_, err = ReadToken("/tmp/non-existent-token-for-teleport-tests-not-found")
	fixtures.ExpectNotFound(c, err)

	dir := c.MkDir()
	tokenPath := filepath.Join(dir, "token")
	err = ioutil.WriteFile(tokenPath, []byte("shmoken"), 0644)
	c.Assert(err, check.IsNil)

	tok, err = ReadToken(tokenPath)
	c.Assert(err, check.IsNil)
	c.Assert(tok, check.Equals, "shmoken")
}

// TestStringsSet makes sure that nil slice returns empty set (less error prone)
func (s *UtilsSuite) TestStringsSet(c *check.C) {
	out := StringsSet(nil)
	c.Assert(out, check.HasLen, 0)
	c.Assert(out, check.NotNil)
}

// TestRepeatReader tests repeat reader
func (s *UtilsSuite) TestRepeatReader(c *check.C) {
	type tc struct {
		repeat   byte
		count    int
		expected string
	}
	tcs := []tc{
		{
			repeat:   byte('a'),
			count:    1,
			expected: "a",
		},
		{
			repeat:   byte('a'),
			count:    0,
			expected: "",
		},
		{
			repeat:   byte('a'),
			count:    3,
			expected: "aaa",
		},
	}
	for _, tc := range tcs {
		data, err := ioutil.ReadAll(NewRepeatReader(tc.repeat, tc.count))
		c.Assert(err, check.IsNil)
		c.Assert(string(data), check.Equals, tc.expected)
	}
}
