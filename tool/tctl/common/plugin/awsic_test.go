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
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAWSICUserFilters(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		// GIVEN some arbitrary set of account name filter CLI args
		cliArgs := awsICArgs{
			userLabels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"},
		}

		// WHEN I attempt to convert the command-line args into filters
		actualFilters, err := cliArgs.parseUserFilters()

		// EXPECT the operation to succeed
		require.NoError(t, err)

		// EXPECT that the returned filters are an accurate representation of
		// the command line args, in arbitrary order
		expectedFilters := []*types.AWSICUserSyncFilter{
			{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
		}
		require.ElementsMatch(t, expectedFilters, actualFilters)
	})

	t.Run("empty lists are valid", func(*testing.T) {
		// GIVEN a cli arg set that doesn't specify any group filters
		cliArgs := awsICArgs{}

		// WHEN I attempt to convert the command-line args into filters
		filters, err := cliArgs.parseUserFilters()

		// EXPECT that the operation succeeds and returns an empty filter list
		require.NoError(t, err)
		require.Empty(t, filters)
	})

	t.Run("origin is applied", func(*testing.T) {
		// GIVEN a name filter with a malformed regex
		cliArgs := awsICArgs{
			userLabels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"},
			userOrigin: types.OriginOkta,
		}

		// WHEN I attempt to convert the command-line args into filters
		actualFilters, err := cliArgs.parseUserFilters()
		require.NoError(t, err)

		expectedFilters := []*types.AWSICUserSyncFilter{
			{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie", types.OriginLabel: types.OriginOkta}},
		}
		require.ElementsMatch(t, expectedFilters, actualFilters)
	})
}

func TestAWSICGroupFilters(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		// GIVEN some arbitrary set of account name filter CLI args
		cliArgs := awsICArgs{
			groupNameFilters: []string{"alpha", "bravo", "charlie"},
		}

		// WHEN I attempt to convert the command-line args into filters
		actualFilters, err := cliArgs.parseGroupFilters()

		// EXPECT the operation to succeed
		require.NoError(t, err)

		// EXPECT that the returned filters are an accurate representation of
		// the command line args, in arbitrary order
		expectedFilters := []*types.AWSICResourceFilter{
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
		}
		rand.Shuffle(len(expectedFilters), func(i, j int) {
			expectedFilters[i], expectedFilters[j] = expectedFilters[j], expectedFilters[i]
		})
		require.ElementsMatch(t, expectedFilters, actualFilters)
	})

	t.Run("empty lists are valid", func(*testing.T) {
		// GIVEN a cli arg set that doesn't specify any group filters
		cliArgs := awsICArgs{}

		// WHEN I attempt to convert the command-line args into filters
		filters, err := cliArgs.parseGroupFilters()

		// EXPECT that the operation succeeds and returns an empty filter list
		require.NoError(t, err)
		require.Empty(t, filters)
	})

	t.Run("regex errors are detected", func(*testing.T) {
		// GIVEN a name filter with a malformed regex
		cliArgs := awsICArgs{
			groupNameFilters: []string{"alpha", "^[)$", "charlie"},
		}

		// WHEN I attempt to convert the command-line args into filters
		_, err := cliArgs.parseGroupFilters()

		// EXPECT the operation to fail
		require.Error(t, err)
	})
}

func TestAWSICAccountFilters(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		// GIVEN some arbitrary combination of Name- and ID-based account filters
		// CLI args..
		cliArgs := awsICArgs{
			accountNameFilters: []string{"alpha", "bravo", "charlie"},
			accountIDFilters:   []string{"0123456789", "9876543210"},
		}

		// WHEN I attempt to convert the command-line args into filters
		actualFilters, err := cliArgs.parseAccountFilters()

		// EXPECT the operation to succeed
		require.NoError(t, err)

		// EXPECT that the returned filters are an accurate representation of
		// the command line args, in arbitrary order
		expectedFilters := []*types.AWSICResourceFilter{
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
			{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
			{Include: &types.AWSICResourceFilter_Id{Id: "0123456789"}},
			{Include: &types.AWSICResourceFilter_Id{Id: "9876543210"}},
		}
		rand.Shuffle(len(expectedFilters), func(i, j int) {
			expectedFilters[i], expectedFilters[j] = expectedFilters[j], expectedFilters[i]
		})
		require.ElementsMatch(t, expectedFilters, actualFilters)
	})

	t.Run("empty lists are valid", func(*testing.T) {
		// GIVEN a cli arg set that doesn't specify any account filters
		cliArgs := awsICArgs{}

		// WHEN I attempt to convert the command-line args into filters
		filters, err := cliArgs.parseAccountFilters()

		// EXPECT that the operation succeeds and returns an empty filter list
		require.NoError(t, err)
		require.Empty(t, filters)
	})

	t.Run("regex errors are detected", func(*testing.T) {
		// GIVEN a name filter with a malformed regex
		cliArgs := awsICArgs{
			accountNameFilters: []string{"alpha", "^[)$", "charlie"},
		}

		// WHEN I attempt to convert the command-line args into filters
		_, err := cliArgs.parseAccountFilters()

		// EXPECT the operation to fail
		require.Error(t, err)
	})
}
