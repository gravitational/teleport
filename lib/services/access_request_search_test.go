/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestNewAccessRequestSearchMatcherDegradesGracefullyOnUserLookupFailure(t *testing.T) {
	t.Parallel()

	const displayOnlyUsername = "123456"

	displayOnly, err := types.NewAccessRequest("display-only", displayOnlyUsername, "role")
	require.NoError(t, err)

	resolvedUser, err := types.NewUser(displayOnlyUsername)
	require.NoError(t, err)
	resolvedUsers := []*types.UserV2{resolvedUser.(*types.UserV2)}

	successLister := &testUserSearchLister{users: resolvedUsers}
	successMatcher := services.NewAccessRequestSearchMatcher(t.Context(), []string{"Jane"}, successLister)
	require.True(t, successMatcher(displayOnly.(*types.AccessRequestV3)), "display-only match should pass when user lookup resolves the requester")

	failedLister := &testUserSearchLister{
		users: resolvedUsers,
		err:   errors.New("backend unavailable"),
	}
	failedMatcher := services.NewAccessRequestSearchMatcher(t.Context(), []string{"Jane"}, failedLister)
	// Gracefully degrade to return false when user lookup fails.
	require.False(t, failedMatcher(displayOnly.(*types.AccessRequestV3)), "same display-only match should be skipped when user lookup fails")
}
