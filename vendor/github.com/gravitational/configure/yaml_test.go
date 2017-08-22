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

func (s *YAMLSuite) TestParseTemplate(c *C) {
	type config struct {
		Data string `yaml:"data"`
	}
	os.Setenv("TEST_VAR1", "test var 1")

	raw := `data: {{env "TEST_VAR1"}}`

	var cfg config
	err := ParseYAML([]byte(raw), &cfg, EnableTemplating())
	c.Assert(err, IsNil)
	c.Assert(cfg.Data, Equals, "test var 1")
}
