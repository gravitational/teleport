/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package usertasks

import (
	"testing"

	"github.com/stretchr/testify/require"

	usertasksapi "github.com/gravitational/teleport/api/types/usertasks"
)

func TestAllDescriptions(t *testing.T) {
	for _, issueType := range usertasksapi.DiscoverEC2IssueTypes {
		require.NotEmpty(t, DescriptionForDiscoverEC2Issue(issueType), "issue type %q is missing descriptions/%s.md file", issueType, issueType)
	}
	for _, issueType := range usertasksapi.DiscoverEKSIssueTypes {
		require.NotEmpty(t, DescriptionForDiscoverEKSIssue(issueType), "issue type %q is missing descriptions/%s.md file", issueType, issueType)
	}
	for _, issueType := range usertasksapi.DiscoverRDSIssueTypes {
		require.NotEmpty(t, DescriptionForDiscoverRDSIssue(issueType), "issue type %q is missing descriptions/%s.md file", issueType, issueType)
	}
}
