package web

import (
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type UsersSuite struct{}

var _ = check.Suite(&UsersSuite{})

func (s *UsersSuite) TestCheckUserParameters(c *check.C) {
	tests := []struct {
		user     string
		roles    []string
		isExpErr bool
	}{
		{
			"", []string{}, true,
		},
		{
			"mimi", []string{}, true,
		},
		{
			"", []string{"testrole"}, true,
		},
		{
			"mimi", []string{"testrole"}, false,
		},
	}

	for _, t := range tests {
		// isNew boolean doesn't matter for this test.
		req := saveUserRequest{true, t.user, t.roles}
		err := req.checkAndSetDefaults()

		if t.isExpErr {
			c.Assert(trace.IsBadParameter(err), check.Equals, true)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}
