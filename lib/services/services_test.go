/*
Copyright 2018 Gravitational, Inc.

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

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type ServicesSuite struct {
}

var _ = check.Suite(&ServicesSuite{})

func (s *ServicesSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

// TestOptions tests command options operations
func (s *ServicesSuite) TestOptions(c *check.C) {
	// test empty scenario
	out := AddOptions(nil)
	c.Assert(out, check.HasLen, 0)

	// make sure original option list is not affected
	in := []MarshalOption{}
	out = AddOptions(in, WithResourceID(1))
	c.Assert(out, check.HasLen, 1)
	c.Assert(in, check.HasLen, 0)
	cfg, err := CollectOptions(out)
	c.Assert(err, check.IsNil)
	c.Assert(cfg.ID, check.Equals, int64(1))

	// Add a couple of other parameters
	out = AddOptions(in, WithResourceID(2), SkipValidation(), WithVersion(V2))
	c.Assert(out, check.HasLen, 3)
	c.Assert(in, check.HasLen, 0)
	cfg, err = CollectOptions(out)
	c.Assert(err, check.IsNil)
	c.Assert(cfg.ID, check.Equals, int64(2))
	c.Assert(cfg.SkipValidation, check.Equals, true)
	c.Assert(cfg.Version, check.Equals, V2)
}

// TestCommandLabels tests command labels
func (s *ServicesSuite) TestCommandLabels(c *check.C) {
	var l CommandLabels
	out := l.Clone()
	c.Assert(out, check.HasLen, 0)

	label := &CommandLabelV2{Command: []string{"ls", "-l"}, Period: Duration(time.Second)}
	l = CommandLabels{"a": label}
	out = l.Clone()

	c.Assert(out, check.HasLen, 1)
	fixtures.DeepCompare(c, out["a"], label)

	// make sure it's not a shallow copy
	label.Command[0] = "/bin/ls"
	c.Assert(label.Command[0], check.Not(check.Equals), out["a"].GetCommand())
}
