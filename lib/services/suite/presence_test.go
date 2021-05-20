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

package suite

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type PresenceSuite struct {
}

var _ = check.Suite(&PresenceSuite{})

func (s *PresenceSuite) TestServerLabels(c *check.C) {
	emptyLabels := make(map[string]string)
	// empty
	server := &types.ServerV2{}
	c.Assert(server.GetAllLabels(), check.DeepEquals, emptyLabels)
	c.Assert(server.LabelsString(), check.Equals, "")
	c.Assert(server.MatchAgainst(emptyLabels), check.Equals, true)
	c.Assert(server.MatchAgainst(map[string]string{"a": "b"}), check.Equals, false)

	// more complex
	server = &types.ServerV2{
		Metadata: types.Metadata{
			Labels: map[string]string{
				"role": "database",
			},
		},
		Spec: types.ServerSpecV2{
			CmdLabels: map[string]types.CommandLabelV2{
				"time": types.CommandLabelV2{
					Period:  services.NewDuration(time.Second),
					Command: []string{"time"},
					Result:  "now",
				},
			},
		},
	}

	c.Assert(server.GetAllLabels(), check.DeepEquals, map[string]string{
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
