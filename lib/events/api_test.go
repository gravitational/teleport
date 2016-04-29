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
