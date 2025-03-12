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
	"strings"
)

//go:embed descriptions/*.md
var descriptionsFS embed.FS

func loadIssueTitleDescription(issueType string) (string, string) {
	filename := fmt.Sprintf("descriptions/%s.md", issueType)
	bs, err := descriptionsFS.ReadFile(filename)
	if err != nil {
		return "", ""
	}

	documentParts := strings.SplitN(string(bs), "\n", 2)
	if len(documentParts) != 2 {
		return "", ""
	}

	title := documentParts[0]
	if !strings.HasPrefix(title, "# ") {
		return "", ""
	}
	title = title[2:]

	description := strings.TrimSpace(documentParts[1])

	return title, description
}

// DescriptionForDiscoverEC2Issue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverEC2Issue(issueType string) (string, string) {
	return loadIssueTitleDescription(issueType)
}

// DescriptionForDiscoverEKSIssue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverEKSIssue(issueType string) (string, string) {
	return loadIssueTitleDescription(issueType)
}

// DescriptionForDiscoverRDSIssue returns the description of the issue and fixing steps.
// The returned string contains a markdown document.
// If issue type is not recognized or doesn't have a specific description, them an empty string is returned.
func DescriptionForDiscoverRDSIssue(issueType string) (string, string) {
	return loadIssueTitleDescription(issueType)
}
