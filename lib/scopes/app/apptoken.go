// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package app

import "path"

// JWTAppTokenIssuer returns the "iss" claim of the JWT Teleport injects into requests to a
// scoped HTTP app in the form of <cluster-name>/<scope>: e.g. "teleport.example.com/staging" for an app in /staging.
func JWTAppTokenIssuer(clusterName, scope string) string {
	return clusterName + scope
}

// JWTAppTokenAudience returns the "aud" claim of the JWT Teleport injects into requests to a
// scoped HTTP app in the form of </scope/app-name>: e.g. "/staging/grafana".
func JWTAppTokenAudience(scope, appName string) string {
	return path.Join(scope, appName)
}
