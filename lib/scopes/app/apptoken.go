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

import (
	"path"

	"github.com/gravitational/trace"

	apiscopes "github.com/gravitational/teleport/api/scopes"
)

// JWTAppTokenIssuer returns the "iss" claim of the JWT Teleport injects into requests to a
// scoped HTTP app in the form of <cluster-name>/<scope>: e.g. "teleport.example.com/staging" for an app in /staging.
func JWTAppTokenIssuer(clusterName, scope string) string {
	return clusterName + scope
}

// AppTokenAudience returns the "aud" claim of the Teleport injects into requests to a
// scoped HTTP apps and OIDC MCP apps in the form of </scope/app-name>: e.g. "/staging/grafana".
func AppTokenAudience(scope, appName string) string {
	return path.Join(scope, appName)
}

// OIDCAppTokenIssuerPrefix is the path which the OIDC issuers of scoped apps are served.
const OIDCAppTokenIssuerPrefix = "/apps/scope"

// OIDCAppTokenIssuerPath returns the path of the issuer of the OIDC ID tokens Teleport injects into requests
// to a scoped MCP app, e.g. "/apps/scope/staging/west/app-name" for the app linear in /staging/west, like the
// issuers of scoped AWS OIDC integrations.
func OIDCAppTokenIssuerPath(scope, appName string) (string, error) {
	if err := apiscopes.StrongValidate(scope); err != nil {
		return "", trace.Wrap(err)
	}
	if err := apiscopes.StrongValidateResourceName(appName); err != nil {
		return "", trace.Wrap(err)
	}
	return path.Join(OIDCAppTokenIssuerPrefix, scope, appName), nil
}
