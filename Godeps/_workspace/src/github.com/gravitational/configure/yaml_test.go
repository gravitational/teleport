package configure

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/configure/test"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type YAMLSuite struct {
	test.ConfigSuite
}

var _ = Suite(&YAMLSuite{})

func (s *YAMLSuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
}

func (s *YAMLSuite) TestParseEnv(c *C) {
	raw := `string: string1
bool: true
int: -1
hex: 686578766172
map: {a: "b", c: "d", "e":f}
slice: [{a: "b", c: "d"}, {"e":f}]
nested:
   nested: nested
`

	var cfg test.Config
	err := ParseYAML([]byte(raw), &cfg)
	c.Assert(err, IsNil)
	s.CheckVariables(c, &cfg)
}
