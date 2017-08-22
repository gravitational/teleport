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
	"github.com/gravitational/configure/test"
	"github.com/gravitational/log"
	. "gopkg.in/check.v1"
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

func (s *CLISuite) TestKeyValRoundtrip(c *C) {
	vals := map[string]string{"a": "b", "c": "d"}
	kv := KeyVal(vals)

	kv2 := make(KeyVal)
	c.Assert(kv2.Set(kv.String()), IsNil)

	c.Assert(kv2, DeepEquals, kv)
}
