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
	return New(FeatureResourceConstraintsV1)
}

type sshServerInfoGetter interface {
	GetProxyMode() bool
}

// ForSSHServer returns features that an SSH/Proxy server can support/participate in.
func ForSSHServer(g sshServerInfoGetter) *componentfeaturesv1.ComponentFeatures {
	// Resource Constraints are only supported for Proxy servers.
	if !g.GetProxyMode() {
		return New()
	}

	return New(FeatureResourceConstraintsV1)
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
