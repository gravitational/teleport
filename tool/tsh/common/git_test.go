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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseGitURL(t *testing.T) {
	tests := []struct {
		inputURL string
		wantOrg  string
		wantRepo string
	}{
		{
			inputURL: "https://github.com/my-org-a/my-repo-a.git",
			wantOrg:  "my-org-a",
			wantRepo: "my-repo-a",
		},
		{
			inputURL: "org-123456789@github.com:my-org-b/my-repo-b.git",
			wantOrg:  "my-org-b",
			wantRepo: "my-repo-b",
		},
		{
			inputURL: "my-org-c/my-repo-c",
			wantOrg:  "my-org-c",
			wantRepo: "my-repo-c",
		},
	}

	for _, test := range tests {
		t.Run(test.inputURL, func(t *testing.T) {
			org, repo, ok := parseGitURL(test.inputURL)
			require.True(t, ok)
			require.Equal(t, test.wantOrg, org)
			require.Equal(t, test.wantRepo, repo)
		})
	}
}
