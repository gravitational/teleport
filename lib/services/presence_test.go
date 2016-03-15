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

package services

import (
	"time"

	"gopkg.in/check.v1"
)

type PresenceSuite struct {
}

var _ = check.Suite(&PresenceSuite{})

func (s *PresenceSuite) TestServerLabels(c *check.C) {
	emptyLabels := make(map[string]string)
	// empty
	server := &Server{}
	c.Assert(server.LabelsMap(), check.DeepEquals, emptyLabels)
	c.Assert(server.LabelsString(), check.Equals, "")
	c.Assert(server.MatchAgainst(emptyLabels), check.Equals, true)
	c.Assert(server.MatchAgainst(map[string]string{"a": "b"}), check.Equals, false)

	// more complex
	server = &Server{
		Labels: map[string]string{
			"role": "database",
		},
		CmdLabels: map[string]CommandLabel{
			"time": CommandLabel{
				Period:  time.Second,
				Command: []string{"time"},
				Result:  "now",
			},
		},
	}
	c.Assert(server.LabelsMap(), check.DeepEquals, map[string]string{
		"role": "database",
		"time": "now",
	})
	c.Assert(server.LabelsString(), check.Equals, "role=database,time=now")
	c.Assert(server.MatchAgainst(emptyLabels), check.Equals, true)
	c.Assert(server.MatchAgainst(map[string]string{"a": "b"}), check.Equals, false)
	c.Assert(server.MatchAgainst(map[string]string{"role": "database"}), check.Equals, true)
	c.Assert(server.MatchAgainst(map[string]string{"time": "now"}), check.Equals, true)
	c.Assert(server.MatchAgainst(map[string]string{"time": "now", "role": "database"}), check.Equals, true)
}
