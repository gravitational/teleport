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
package cstrings

import (
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestStrings(t *testing.T) { TestingT(t) }

type USuite struct {
}

var _ = Suite(&USuite{})

func (s *USuite) TestSplit(c *C) {
	tcs := []struct {
		delim, escape rune
		input         string
		expect        []string
	}{
		{delim: ',', escape: '\\', input: "", expect: []string{}},
		{delim: ',', escape: '\\', input: "a", expect: []string{"a"}},
		{delim: ',', escape: '\\', input: "a,b", expect: []string{"a", "b"}},
		{delim: ',', escape: '\\', input: "a,b\\,cd", expect: []string{"a", "b\\,cd"}},
		{delim: ',', escape: '\\', input: "a,b\\,cd,e", expect: []string{"a", "b\\,cd", "e"}},
	}

	for i, t := range tcs {
		comment := Commentf(
			"test case #%v: delim: %c, escape: %v, input: '%v', expected: %#v",
			i, t.delim, t.escape, t.input, t.expect)
		out := Split(t.delim, t.escape, t.input)
		c.Assert(out, DeepEquals, t.expect, comment)
	}
}

func (s *USuite) TestDomain(c *C) {
	tcs := []struct {
		name     string
		expected bool
	}{
		{name: "domain.com", expected: true},
		{name: "domain com", expected: false},
		{name: "A-z.com", expected: true},
		{name: " ", expected: false},
		{name: ".", expected: false},
	}

	for i, t := range tcs {
		comment := Commentf(
			"test case #%v: name: %v expected %v",
			i, t.name, t.expected)
		valid := IsValidDomainName(t.name)
		c.Assert(valid, Equals, t.expected, comment)
	}
}

func (s *USuite) TestWithoutElement(c *C) {
	tcs := []struct {
		in   []string
		out  []string
		flag string
	}{
		{in: []string{}, out: []string{}, flag: ""},
		{in: []string{"a"}, out: []string{"a"}, flag: "--a"},
		{in: []string{"a", "--flag=b"}, out: []string{"a"}, flag: "--flag"},
		{in: []string{"a", "--flag", "b"}, out: []string{"a"}, flag: "--flag"},
		{in: []string{"a", "--flag", "b", "c"}, out: []string{"a", "c"}, flag: "--flag"},
	}

	for i, t := range tcs {
		comment := Commentf(
			"test case #%v: removing %v from %v should = %v",
			i, t.flag, t.in, t.out)
		out := WithoutFlag(t.in, t.flag)
		c.Assert(out, DeepEquals, t.out, comment)
	}
}

func (s *USuite) TestUser(c *C) {
	tcs := []struct {
		name     string
		expected bool
	}{
		{name: "Admin", expected: true},
		{name: "ce-nt.os", expected: true},
		{name: "ubuntu-2", expected: true},
		{name: "", expected: false},
		{name: " ", expected: false},
		{name: "-ab", expected: false},
		{name: "a/b", expected: false},
		{name: "a\tb", expected: false},
		{name: "a\nb", expected: false},
		{name: " ubuntu ?", expected: false},
		{name: strings.Repeat("a", 33), expected: false},
	}

	for i, t := range tcs {
		comment := Commentf(
			"test case #%v: name: %v expected %v",
			i, t.name, t.expected)
		valid := IsValidUnixUser(t.name)
		c.Assert(valid, Equals, t.expected, comment)
	}
}
