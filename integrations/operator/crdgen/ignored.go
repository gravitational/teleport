/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package crdgen

type stringSet map[string]struct{}

/*
Fields that we are ignoring when creating a CRD
Each entry represents the ignore fields using the resource name as the version

One of the reasons to ignore fields those fields is because they are readonly in Teleport
CRD do not support readonly logic
This should be removed when the following feature is implemented
https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#transition-rules
*/
var ignoredFields = map[string]stringSet{
	"UserSpecV2": {
		"LocalAuth": struct{}{}, // struct{}{} is used to signify "no value".
		"Expires":   struct{}{},
		"CreatedBy": struct{}{},
		"Status":    struct{}{},
	},
	"GithubConnectorSpecV3": {
		"TeamsToLogins": struct{}{}, // Deprecated field, removed since v11
	},
	"ServerSpecV2": {
		// Useless field for agentless servers, and potentially dangerous as it
		// allows remote exec on agentful nodes.
		"CmdLabels": struct{}{},
	},
	"TrustedClusterSpecV2": {
		"Roles": struct{}{}, // Deprecated, use RoleMap instead.
	},
}
