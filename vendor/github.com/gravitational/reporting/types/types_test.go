/*
Copyright 2017 Gravitational, Inc.

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

package types

import (
	"testing"

	check "gopkg.in/check.v1"
)

func TestTypes(t *testing.T) { check.TestingT(t) }

type TypesSuite struct{}

var _ = check.Suite(&TypesSuite{})

func (s *TypesSuite) TestHeartbeat(c *check.C) {
	h := NewHeartbeat(
		Notification{
			Type:     NotificationUsage,
			Severity: SeverityWarning,
			Text:     "Usage limit exceeded",
			HTML:     "<div>Usage limit exceeded</div>",
		},
		Notification{
			Type:     NotificationTerms,
			Severity: SeverityError,
			Text:     "Terms of service violation",
			HTML:     "<div>Terms of service violation</div>",
		})
	bytes, err := MarshalHeartbeat(*h)
	c.Assert(err, check.IsNil)
	unmarshaled, err := UnmarshalHeartbeat(bytes)
	c.Assert(err, check.IsNil)
	c.Assert(len(unmarshaled.Spec.Notifications), check.Equals, 2)
	c.Assert(unmarshaled, check.DeepEquals, h)
}

func (s *TypesSuite) TestEmptyHeartbeat(c *check.C) {
	h := NewHeartbeat()
	bytes, err := MarshalHeartbeat(*h)
	c.Assert(err, check.IsNil)
	unmarshaled, err := UnmarshalHeartbeat(bytes)
	c.Assert(err, check.IsNil)
	c.Assert(len(unmarshaled.Spec.Notifications), check.Equals, 0)
	c.Assert(unmarshaled, check.DeepEquals, h)
}
