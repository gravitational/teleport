package filter

import (
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/scim2/filter-parser/v2"
	"github.com/stretchr/testify/require"
)

func requireBadParamWith(text string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsBadParameter(err), "Expected BadParameter, got %v", err)
		require.Contains(t, err.Error(), text)
	}
}

// TestParseFilter asserts that the filter parser and validator only supports
// the bare minimum functionality required for working with Okta. Other
// functionality can be added as necessary
func TestParseFilter(t *testing.T) {
	testCases := []struct {
		name             string
		text             string
		expectError      require.ErrorAssertionFunc
		expectExpression filter.Expression
	}{
		{
			name:        "simple",
			text:        `userName eq "scooby@mystery-machine.org"`,
			expectError: require.NoError,
			expectExpression: &filter.AttributeExpression{
				AttributePath: filter.AttributePath{
					AttributeName: "userName",
				},
				Operator:     filter.EQ,
				CompareValue: "scooby@mystery-machine.org",
			},
		},
		{
			name:        "non username attr",
			text:        `email eq "root@example.com"`,
			expectError: require.NoError,
			expectExpression: &filter.AttributeExpression{
				AttributePath: filter.AttributePath{
					AttributeName: "email",
				},
				Operator:     filter.EQ,
				CompareValue: "root@example.com",
			},
		}, {
			name:             "unsupported attr paths",
			text:             `userName.first eq "root@example.com"`,
			expectError:      requireBadParamWith("attribute paths"),
			expectExpression: nil,
		}, {
			name:             "unsupported not expression",
			text:             `not (userName eq "root@example.com")`,
			expectError:      requireBadParamWith("negation"),
			expectExpression: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f, err := ParseFilter(testCase.text)
			testCase.expectError(t, err)
			require.Equal(t, testCase.expectExpression, f)
		})
	}
}

// TestDetectUnsupportedComparisonOperators asserts that the filter validator
// rejects all operators not required for Okta integration. Adding support for
// new operators will be left until they are required by a SCIM client.
func TestDetectUnsupportedComparisonOperators(t *testing.T) {
	unsupportedOperators := []filter.CompareOperator{
		filter.PR,
		filter.NE,
		filter.CO,
		filter.SW,
		filter.EW,
		filter.GT,
		filter.LT,
		filter.GE,
		filter.LE,
	}
	for _, op := range unsupportedOperators {
		t.Run(string(op), func(t *testing.T) {
			operand := ""
			if op != filter.PR {
				operand = `"scooby@mystery-machine.org"`
			}
			text := fmt.Sprintf(`userName %s %s`, op, operand)
			_, err := ParseFilter(text)
			requireBadParamWith(string(op))(t, err)
		})
	}
}

// TestDetectUnsupportedLogicalOperators asserts that the filter validator
// rejects all operators not required for Okta integration. Adding support for
// new operators will be left until they are required by a SCIM client.
func TestDetectUnsupportedLogicalOperators(t *testing.T) {
	unsupportedOperators := []filter.LogicalOperator{
		filter.AND,
		filter.OR,
	}
	for _, op := range unsupportedOperators {
		t.Run(string(op), func(t *testing.T) {
			text := fmt.Sprintf(`userName eq "scooby" %s userName eq "shaggy"`, op)
			_, err := ParseFilter(text)
			requireBadParamWith(string(op))(t, err)
		})
	}
}

func TestFilterEval(t *testing.T) {
	f, err := ParseFilter(`userName eq "scooby@mystery-machine.org"`)
	require.NoError(t, err)

	t.Run("pass", func(t *testing.T) {
		attributes := map[string]string{
			"userName": "scooby@mystery-machine.org",
		}

		err = EvaluateFilter(f, attributes)
		require.NoError(t, err)
	})

	t.Run("fail", func(t *testing.T) {
		attributes := map[string]string{
			"userName": "shaggy@mystery-machine.org",
		}

		err = EvaluateFilter(f, attributes)
		require.Error(t, err)
	})

}
