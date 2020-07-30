package web

import (
	"testing"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type UsersSuite struct{}

var _ = check.Suite(&UsersSuite{})

func TestUsers(t *testing.T) { check.TestingT(t) }

func (s *UsersSuite) TestCheckUserParameters(c *check.C) {
	tests := []struct {
		user     requestUser
		isExpErr bool
	}{
		{
			requestUser{"", []string{}}, true,
		},
		{
			requestUser{"mimi", []string{}}, true,
		},
		{
			requestUser{"", []string{"testrole"}}, true,
		},
		{
			requestUser{"mimi", []string{"testrole"}}, false,
		},
	}

	for _, t := range tests {
		err := checkUserParameters(t.user)

		if t.isExpErr {
			c.Assert(trace.IsBadParameter(err), check.Equals, true)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}
