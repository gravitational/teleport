package configure

import (
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

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
