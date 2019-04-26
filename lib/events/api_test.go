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

package events

import (
	"time"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

type AuditAPITestSuite struct{}

var _ = check.Suite(&AuditAPITestSuite{})

func (a *AuditAPITestSuite) TestFields(c *check.C) {
	now := time.Now().Round(time.Minute)

	f := EventFields{
		"one":  1,
		"name": "vincent",
		"time": now,
	}

	one := f.GetInt("one")
	c.Assert(one, check.Equals, 1)

	two := f.GetInt("two")
	c.Assert(two, check.Equals, 0)

	name := f.GetString("name")
	c.Assert(name, check.Equals, "vincent")

	city := f.GetString("city")
	c.Assert(city, check.Equals, "")

	t := f.GetTime("time")
	c.Assert(t, check.Equals, now)
}

func (a *AuditAPITestSuite) TestUpdateFields(c *check.C) {
	event := Event{
		Name: "test.event",
		Code: "TEST0001I",
	}
	fields := EventFields{
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	}
	c.Assert(UpdateEventFields(event, fields, clockwork.NewFakeClock(), utils.NewFakeUID()), check.IsNil)

	// Check the fields have been updated appropriately.
	c.Assert(fields, check.DeepEquals, EventFields{
		EventType:   event.Name,
		EventID:     fixtures.UUID,
		EventCode:   event.Code,
		EventTime:   time.Date(1984, time.April, 4, 0, 0, 0, 0, time.UTC),
		EventUser:   "test@example.com",
		LoginMethod: LoginMethodOIDC,
	})
}
