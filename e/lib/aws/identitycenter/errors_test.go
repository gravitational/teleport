package identitycenter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type testTargetError struct {
	text string
}

func (err *testTargetError) Error() string {
	return err.text
}

func TestVisitErrorTree(t *testing.T) {
	testCases := []struct {
		name     string
		input    error
		expected []string
	}{
		{
			name: "nil",
		},
		{
			name:  "single_miss",
			input: errors.New("nope"),
		},
		{
			name:     "single_hit",
			input:    &testTargetError{"a hit"},
			expected: []string{"a hit"},
		},
		{
			name: "nested",
			input: trace.Wrap(
				trace.NewAggregate(
					errors.New("miss"),
					&testTargetError{"hit #1"},
					errors.New("another miss"),
					errors.Join(
						errors.New("nested miss"),
						&testTargetError{"hit #2"},
						&testTargetError{"hit #3"},
						errors.New("another nested miss"),
						trace.NewAggregate(
							&testTargetError{"hit #4"},
							trace.NewAggregate(
								&testTargetError{"hit #5"},
								&testTargetError{"hit #6"},
							),
							&testTargetError{"hit #7"},
						),
					),
					&testTargetError{"hit #8"},
					fmt.Errorf("Level 2: %w",
						fmt.Errorf("Level 3: %w",
							fmt.Errorf("Level 4: %w",
								&testTargetError{"hit #9"},
							),
						),
					),
					&testTargetError{"hit #10"},
				),
			),
			expected: []string{
				"hit #1", "hit #2", "hit #3", "hit #4", "hit #5",
				"hit #6", "hit #7", "hit #8", "hit #9", "hit #10",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var hits []string
			visitErrorTree(testCase.input, func(err error) {
				//nolint:errorlint // intentionally checking direct error type; nested errors will be discovered separately.
				if hit, ok := err.(*testTargetError); ok {
					hits = append(hits, hit.text)
				}
			})
			require.ElementsMatch(t, testCase.expected, hits)
		})
	}
}
