package configure

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/configure/test"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type CLISuite struct {
	test.ConfigSuite
}

var _ = Suite(&CLISuite{})

func (s *CLISuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
}

func (s *CLISuite) TestParseEnv(c *C) {
	args := []string{
		"--map=a:b,c:d,e:f",
		"--slice=a:b,c:d",
		"--slice=e:f",
		"--string=string1",
		"--nested=nested",
		"--int=-1",
		"--hex=686578766172",
		"--bool=true",
	}
	var cfg test.Config
	err := ParseCommandLine(&cfg, args)
	c.Assert(err, IsNil)
	s.CheckVariables(c, &cfg)
}
