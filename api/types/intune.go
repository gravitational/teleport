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

// ValidateIntuneAPIEndpoints checks if API endpoints point to one of the official deployments of
// the Microsoft identity platform and Microsoft Graph.
// https://learn.microsoft.com/en-us/graph/deployments
func ValidateIntuneAPIEndpoints(loginEndpoint, graphEndpoint string) error {
	if loginEndpoint != "" && !slices.Contains(validLoginEndpoints, loginEndpoint) {
		return trace.BadParameter("login endpoint is not one of the official Microsoft Entra ID endpoints (see https://learn.microsoft.com/en-us/graph/deployments)")
	}

	if graphEndpoint != "" && !slices.Contains(validGraphEndpoints, graphEndpoint) {
		return trace.BadParameter("graph endpoint is not one of the official Microsoft Graph endpoints (see https://learn.microsoft.com/en-us/graph/deployments)")
	}

	return nil
}

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
