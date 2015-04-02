package memlog

import (
	"testing"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
)

func TestMem(t *testing.T) { TestingT(t) }

type MemSuite struct {
	l *logger
}

var _ = Suite(&MemSuite{})

func (s *MemSuite) SetUpTest(c *C) {
	s.l = New().(*logger)
}

func (s *MemSuite) TestSetGet(c *C) {
	out := s.l.LastEvents()
	c.Assert(out, DeepEquals, []interface{}{})

	_, err := s.l.Write([]byte(`"a"`))
	c.Assert(err, IsNil)

	_, err = s.l.Write([]byte(`"b"`))
	c.Assert(err, IsNil)

	out = s.l.LastEvents()
	c.Assert(out, DeepEquals, []interface{}{"b", "a"})
}
