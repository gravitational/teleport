/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package types

import (
	"regexp"

	"github.com/gravitational/trace"
)

// validGitHubOrganizationName filters the allowed characters in GitHub
// organization name.
//
// GitHub shows the following error when inputing an invalid org name:
// The name '_' may only contain alphanumeric characters or single hyphens, and
// cannot begin or end with a hyphen.
var validGitHubOrganizationName = regexp.MustCompile(`^[a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?$`)

// ValidateGitHubOrganizationName returns an error if a given string is not a
// valid GitHub organization name.
func ValidateGitHubOrganizationName(name string) error {
	const maxGitHubOrgNameLength = 39
	if len(name) > maxGitHubOrgNameLength {
		return trace.BadParameter("GitHub organization name cannot exceed %d characters", maxGitHubOrgNameLength)
	}
	return ValidateResourceName(validGitHubOrganizationName, name)
}
