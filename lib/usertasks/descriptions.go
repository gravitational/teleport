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
	"embed"
	"fmt"
)

//go:embed descriptions/*.md
var descriptionsFS embed.FS

func loadIssueDescription(issueType string) string {
	filename := fmt.Sprintf("descriptions/%s.md", issueType)
	bs, err := descriptionsFS.ReadFile(filename)
	if err != nil {
		return ""
	}
	return string(bs)
}

// DescriptionForDiscoverEC2Issue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverEC2Issue(issueType string) string {
	return loadIssueDescription(issueType)
}

// DescriptionForDiscoverEKSIssue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverEKSIssue(issueType string) string {
	return loadIssueDescription(issueType)
}

// DescriptionForDiscoverRDSIssue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverRDSIssue(issueType string) string {
	return loadIssueDescription(issueType)
}
