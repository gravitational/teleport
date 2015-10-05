package boltrec

import (
	"testing"

	"github.com/gravitational/teleport/lib/recorder/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltRecSuite struct {
	r     *boltRecorder
	suite test.RecorderSuite
	dir   string
}

var _ = Suite(&BoltRecSuite{})

func (s *BoltRecSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.r, err = New(s.dir)
	c.Assert(err, IsNil)

	s.suite.R = s.r
}

func (s *BoltRecSuite) TearDownTest(c *C) {
	//c.Assert(s.r.Close(), IsNil)
}

func (s *BoltRecSuite) TestRecorder(c *C) {
	s.suite.Recorder(c)
}
