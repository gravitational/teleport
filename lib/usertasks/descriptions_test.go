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
	for _, issueGroup := range [][]string{
		usertasksapi.DiscoverEC2IssueTypes,
		usertasksapi.DiscoverEKSIssueTypes,
		usertasksapi.DiscoverRDSIssueTypes,
	} {
		for _, issueType := range issueGroup {
			title, description := DescriptionForDiscoverEC2Issue(issueType)
			require.NotEmpty(t, title, "issue type %q is missing title in descriptions/%s.md file", issueType, issueType)
			require.NotEmpty(t, description, "issue type %q is missing description in descriptions/%s.md file", issueType, issueType)
		}
	}
}
