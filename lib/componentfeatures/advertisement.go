/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package componentfeatures

import (
	componentfeaturesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/componentfeatures/v1"
	"github.com/gravitational/teleport/api/types"
)

// ForAuthServer returns features that an Auth server can support/participate in.
func ForAuthServer() *componentfeaturesv1.ComponentFeatures {
	return New(
		FeatureResourceConstraintsV1,
		FeatureResourceConstraintsSSHV1,
	)
}

// ForSSHServer returns features that an SSH server can support/participate in.
func ForSSHServer() *componentfeaturesv1.ComponentFeatures {
	return New(
		FeatureResourceConstraintsV1,
		FeatureResourceConstraintsSSHV1,
	)
}

type appServerInfoGetter interface {
	GetApp() types.Application
}

// ForAppServer returns features that an App server can support/participate in.
func ForAppServer(g appServerInfoGetter) *componentfeaturesv1.ComponentFeatures {
	// Resource Constraints are only supported for AWS Console apps.
	if app := g.GetApp(); !app.IsAWSConsole() {
		return New()
	}

	return New(FeatureResourceConstraintsV1)
}

// SupportsAppForwardedIdentityV1 reports whether app can be served with a Proxy-forwarded client identity, which is the
// set of applications [FeatureAppForwardedIdentityV1] covers. It supports LLM apps and apps with no subkind that the
// Application Service serves as HTTP, unless an exclusion matches. Any other subkind or protocol is unsupported until it
// is added here.
func SupportsAppForwardedIdentityV1(app types.Application) bool {
	switch {
	case app.GetCloud() != "" || app.IsAWSConsole():
		// Cloud apps need the upstream identity carried in an app certificate, such as the AWS role ARN.
		return false
	case app.GetIntegration() != "":
		// The Proxy serves integration-backed apps itself, using the integration's credentials.
		return false
	case app.GetClientCertMode() != types.AppClientCertModeDisabled:
		// Managed upstream client certificates are issued from the original app certificate.
		return false
	case app.Origin() == types.OriginOkta:
		// The Proxy redirects Okta apps to Okta instead of forwarding them.
		return false
	default:
		// Plain HTTP apps have no marker of their own, so the excluded apps above look like them, and so would a new
		// kind of app with no subkind that is served as HTTP. Exclude such an app above, or it is supported.
		plainHTTP := app.GetSubKind() == "" && app.GetProtocol() == types.ApplicationProtocolHTTP
		return plainHTTP || app.GetSubKind() == types.SubKindLLM
	}
}
