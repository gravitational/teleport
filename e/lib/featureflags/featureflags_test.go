package featureflags

import (
	"testing"

	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestFlags(t *testing.T) { check.TestingT(t) }

type FlagsSuite struct {
}

var _ = check.Suite(&FlagsSuite{})

func (s *FlagsSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *FlagsSuite) TestUnmarshal(c *check.C) {
	type testCase struct {
		description string
		input       string
		expected    Flags
		err         error
	}
	testCases := []testCase{
		{
			description: "simple case",
			input:       `{"kind": "flags", "version": "v3", "metadata": {"name": "Teleport Commercial"}, "spec": {"usage": true, "k8s": true, "aws_account": "123", "aws_pid": "4"}}`,
			expected: MustNew("Teleport Commercial", SpecV3{
				ReportsUsage:       services.NewBool(true),
				SupportsKubernetes: services.NewBool(true),
				AWSAccountID:       "123",
				AWSProductID:       "4",
			}),
		},
		{
			description: "simple case with string booleans",
			input:       `{"kind": "flags", "version": "v3", "metadata": {"name": "flags"}, "spec": {"usage": "yes", "k8s": "yes", "aws_account": "123", "aws_pid": "4"}}`,
			expected: MustNew("flags", SpecV3{
				ReportsUsage:       services.NewBool(true),
				SupportsKubernetes: services.NewBool(true),
				AWSAccountID:       "123",
				AWSProductID:       "4",
			}),
		},
		{
			description: "failed validation - unknown version",
			input:       `{"kind": "flags", "version": "v2", "metadata": {"name": "flags"}, "spec": {"usage": "yes", "k8s": "yes", "aws_account": "123", "aws_pid": "4"}}`,
			err:         trace.BadParameter(""),
		},
		{
			description: "failed validation, bad types",
			input:       `{"kind": "flags", "version": "v3", "metadata": {"name": "flags"}, "spec": {"usage": 1, "k8s": "yes", "aws_account": 14, "aws_pid": "4"}}`,
			err:         trace.BadParameter(""),
		},
	}
	for _, tc := range testCases {
		comment := check.Commentf("test case %q", tc.description)
		out, err := Unmarshal([]byte(tc.input))
		if tc.err == nil {
			c.Assert(err, check.IsNil, comment)
			fixtures.DeepCompare(c, tc.expected, out)
			data, err := Marshal(out)
			c.Assert(err, check.IsNil, comment)
			out2, err := Unmarshal(data)
			c.Assert(err, check.IsNil, comment)
			fixtures.DeepCompare(c, tc.expected, out2)
		} else {
			c.Assert(err, check.FitsTypeOf, tc.err, comment)
		}
	}
}
