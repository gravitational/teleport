package configure

import (
	"os"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/configure/test"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type EnvSuite struct {
	test.ConfigSuite
}

var _ = Suite(&EnvSuite{})

func (s *EnvSuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
}

func (s *EnvSuite) TestParseEnv(c *C) {
	vars := map[string]string{
		"TEST_STRING_VAR":    "string1",
		"TEST_NESTED_VAR":    "nested",
		"TEST_BOOL_VAR":      "true",
		"TEST_HEX_VAR":       "686578766172",
		"TEST_MAP_VAR":       `{"a":"b", "c":"d", "e":"f"}`,
		"TEST_SLICE_MAP_VAR": `[{"a":"b", "c":"d"}, {"e":"f"}]`,
		"TEST_INT_VAR":       "-1",
	}
	for k, v := range vars {
		c.Assert(os.Setenv(k, v), IsNil)
	}
	var cfg test.Config
	err := ParseEnv(&cfg)
	c.Assert(err, IsNil)
	s.CheckVariables(c, &cfg)
}
