package events

import "gopkg.in/check.v1"
import "time"

type AuditApiTestSuite struct {
}

var _ = check.Suite(&AuditApiTestSuite{})

func (a *AuditApiTestSuite) TestFields(c *check.C) {
	now := time.Now().Round(time.Minute)

	f := EventFields{
		"one":  1,
		"name": "vincent",
		"time": now,
	}

	one, found := f.GetInt("one")
	c.Assert(found, check.Equals, true)
	c.Assert(one, check.Equals, 1)

	two, found := f.GetInt("two")
	c.Assert(found, check.Equals, false)
	c.Assert(two, check.Equals, 0)

	name, found := f.GetString("name")
	c.Assert(found, check.Equals, true)
	c.Assert(name, check.Equals, "vincent")

	city, found := f.GetString("city")
	c.Assert(found, check.Equals, found)
	c.Assert(city, check.Equals, "")

	t, found := f.GetTime("time")
	c.Assert(found, check.Equals, found)
	c.Assert(t, check.Equals, now)
}
