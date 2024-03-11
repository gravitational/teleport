package jira

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestErrorsUnmarshall(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		input          string
		expectedOutput ErrorResult
		assertErr      require.ErrorAssertionFunc
	}{
		{
			name:  "new error",
			input: `{"errorMessages":[], "errors": {"project": "project is required"}}`,
			expectedOutput: ErrorResult{
				Errors: Errors{Errors: map[string]string{"project": "project is required"}},
			},
			assertErr: require.NoError,
		},
		{
			name:  "legacy error",
			input: `{"errorMessages":["foo"],"errors":["bar", "baz"]}`,
			expectedOutput: ErrorResult{
				ErrorMessages: []string{"foo"},
				Errors:        Errors{LegacyErrors: []string{"bar", "baz"}},
			},
			assertErr: require.NoError,
		},
		{
			name:  "empty error",
			input: `{"errorMessages":["There was an error parsing JSON. Check that your request body is valid."]}`,
			expectedOutput: ErrorResult{
				ErrorMessages: []string{"There was an error parsing JSON. Check that your request body is valid."},
			},
			assertErr: require.NoError,
		},
		{
			name:           "malformed error",
			input:          `{"errorMessages":["Foo"],"errors":"This is a single string"}`,
			expectedOutput: ErrorResult{ErrorMessages: []string{"Foo"}},
			assertErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "This is a single string")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			data := []byte(tt.input)
			var result ErrorResult
			tt.assertErr(t, json.Unmarshal(data, &result))
			diff := cmp.Diff(tt.expectedOutput, result, cmpopts.EquateEmpty())
			require.Empty(t, diff)
		})
	}
}
