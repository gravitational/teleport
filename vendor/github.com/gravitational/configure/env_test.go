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
package configure

import (
	"os"

	"github.com/gravitational/configure/test"
	"github.com/gravitational/log"
	. "gopkg.in/check.v1"
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
