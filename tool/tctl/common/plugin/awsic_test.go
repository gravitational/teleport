// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package plugin

import (
	"context"
	"log/slog"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAWSICUserFilters(t *testing.T) {
	testCases := []struct {
		name            string
		labelValues     []string
		originValues    []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICUserSyncFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "single",
			labelValues:   []string{"a=alpha,b=bravo,c=charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
			},
		},
		{
			name: "multiple label filters",
			labelValues: []string{
				"a=alpha,b=bravo,c=charlie",
				"a=aardvark,b=a buzzing thing,c=big blue wobbly thing",
			},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
				{Labels: map[string]string{"a": "aardvark", "b": "a buzzing thing", "c": "big blue wobbly thing"}},
			},
		},
		{
			name:          "origin only",
			originValues:  []string{types.OriginOkta, types.OriginEntraID},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
		},
		{
			name: "complex",
			labelValues: []string{
				"a=alpha,b=bravo,c=charlie",
				"a=aardvark,b=a buzzing thing,c=big blue wobbly thing",
			},
			originValues:  []string{types.OriginOkta, types.OriginEntraID},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
				{Labels: map[string]string{"a": "aardvark", "b": "a buzzing thing", "c": "big blue wobbly thing"}},
				{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
		},
		{
			name:          "malformed label spec is an error",
			labelValues:   []string{"a=alpha,potato,c=charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICArgs{
				userLabels:  test.labelValues,
				userOrigins: test.originValues,
			}

			actualFilters, err := cliArgs.parseUserFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestAWSICGroupFilters(t *testing.T) {
	testCases := []struct {
		name            string
		nameValues      []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICResourceFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "multiple",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
			},
		},
		{
			name:          "malformed regex is an error",
			nameValues:    []string{"alpha", "^[)$", "charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICArgs{
				groupNameFilters: test.nameValues,
			}

			actualFilters, err := cliArgs.parseGroupFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestAWSICAccountFilters(t *testing.T) {
	testCases := []struct {
		name            string
		nameValues      []string
		idValues        []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICResourceFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "names only",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
			},
		},
		{
			name:          "ids only",
			idValues:      []string{"0123456789", "9876543210"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_Id{Id: "0123456789"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "9876543210"}},
			},
		},
		{
			name:          "complex",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			idValues:      []string{"0123456789", "9876543210"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "0123456789"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "9876543210"}},
			},
		},
		{
			name:          "malformed regex is an error",
			nameValues:    []string{"alpha", "^[)$", "charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICArgs{
				accountNameFilters: test.nameValues,
				accountIDFilters:   test.idValues,
			}

			actualFilters, err := cliArgs.parseAccountFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestSCIMBaseURLValidation(t *testing.T) {
	ctx := context.Background()

	requireURL := func(expectedURL string) require.ValueAssertionFunc {
		return func(subtestT require.TestingT, value any, _ ...any) {
			actualURL, ok := value.(*url.URL)
			require.True(subtestT, ok, "Expected value to be an *URL, got %T instead", value)
			require.Equal(subtestT, expectedURL, actualURL.String())
		}
	}

	testCases := []struct {
		name        string
		suppliedURL string
		forceURL    bool
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		{
			name:        "valid url",
			suppliedURL: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2",
			expectError: require.NoError,
			expectValue: requireURL("https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2"),
		},
		{
			name:        "fragments are stripped",
			suppliedURL: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2#spurious-fragment",
			expectError: require.NoError,
			expectValue: requireURL("https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2"),
		},
		{
			name:        "invalid AWS SCIM Base URLs are an error",
			suppliedURL: "https://scim.example.com/v2",
			expectError: require.Error,
		},
		{
			name:        "invalid AWS SCIM Base URL can be forced",
			suppliedURL: "https://scim.example.com/v2",
			forceURL:    true,
			expectValue: requireURL("https://scim.example.com/v2"),
			expectError: require.NoError,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICArgs{
				scimURL:      mustParseURL(test.suppliedURL),
				forceSCIMURL: test.forceURL,
			}

			err := cliArgs.validateSCIMBaseURL(ctx, slog.Default().With("test", t.Name()))
			test.expectError(t, err)
			if test.expectValue != nil {
				test.expectValue(t, cliArgs.scimURL)
			}
		})
	}
}
