// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"slices"

	"github.com/gravitational/trace"
)

const (
	// MSGraphDefaultLoginEndpoint is the endpoint under which Microsoft identity platform APIs are available.
	MSGraphDefaultLoginEndpoint = "https://login.microsoftonline.com"
	// MSDefaultGraphEndpoint is the endpoint under which Microsoft Graph API is available.
	MSGraphDefaultEndpoint = "https://graph.microsoft.com"
)

var (
	validLoginEndpoints = []string{
		"https://login.microsoftonline.com",
		"https://login.microsoftonline.us",
		"https://login.chinacloudapi.cn",
	}
	validGraphEndpoints = []string{
		"https://graph.microsoft.com",
		"https://graph.microsoft.us",
		"https://dod-graph.microsoft.us",
		"https://microsoftgraph.chinacloudapi.cn",
	}
)

// ValidateMSGraphAndLoginEndpoints checks if API endpoints point to one of the official deployments of
// the Microsoft identity platform and Microsoft Graph.
// https://learn.microsoft.com/en-us/graph/deployments
func ValidateMSGraphAndLoginEndpoints(loginEndpoint, graphEndpoint string) error {
	if err := ValidateMSGraphEndpoint(graphEndpoint); err != nil {
		return trace.Wrap(err)
	}

	if err := ValidateMSLoginEndpoint(loginEndpoint); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ValidateMSGraphEndpoint checks if the given endpoint points to a valid MS Graph endpoint.
// Note: An empty string is considered valid to maintain backward compatibility.
func ValidateMSGraphEndpoint(graphEndpoint string) error {
	if graphEndpoint != "" && !slices.Contains(validGraphEndpoints, graphEndpoint) {
		return trace.BadParameter("expected graph endpoint to be one of %q, got %q", validGraphEndpoints, graphEndpoint)
	}
	return nil
}

// ValidateMSLoginEndpoint checks if the given endpoint points to a valid MS login endpoint.
// Note: An empty string is considered valid to maintain backward compatibility.
func ValidateMSLoginEndpoint(loginEndpoint string) error {
	if loginEndpoint != "" && !slices.Contains(validLoginEndpoints, loginEndpoint) {
		return trace.BadParameter("expected login endpoint to be one of %q, got %q", validLoginEndpoints, loginEndpoint)
	}
	return nil
}

const (
	// EntraIDSecurityGroups represents security enabled Entra ID groups.
	EntraIDSecurityGroups = "security-groups"
	// EntraIDDirectoryRoles represents Entra ID directory roles.
	EntraIDDirectoryRoles = "directory-roles"
	// EntraIDAllGroups represents all types of Entra ID groups, including directory roles.
	EntraIDAllGroups = "all-groups"
)

// EntraIDGroupsTypes defines supported Entra ID
// group types for Entra ID groups proivder.
var EntraIDGroupsTypes = []string{
	EntraIDSecurityGroups,
	EntraIDDirectoryRoles,
	EntraIDAllGroups,
}

// checkAndSetDefaults validates fields on the Entra ID groups provider and
// returns an error for invalid configurations.
func (e *EntraIDGroupsProvider) checkAndSetDefaults() error {
	if e.GroupType != "" {
		if !slices.Contains(EntraIDGroupsTypes, e.GroupType) {
			return trace.BadParameter("expected group type to be one of %q, got %q", EntraIDGroupsTypes, e.GroupType)
		}
	}

	if err := ValidateMSGraphEndpoint(e.GraphEndpoint); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
