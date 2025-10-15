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

package awscommon

import (
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidIntegratioName validates the integration name.
func ValidIntegratioName(name string) error {
	// AWS OIDC and Roles Anywhere Integrations can be used as source of credentials to access AWS Web/CLI.
	// For OIDC, this creates a new AppServer whose endpoint is <integrationName>.<proxyURL>, which can fail if integrationName is not a valid DNS Label.
	// For Roles Anywhere, this creates a AppServers for each Roles Anywhere Profile whose endpoint is <profileName>-<integrationName>.<proxyURL>, which can fail if integrationName is not a valid DNS Label.
	// Instead of failing when the integration is already created, it fails at creation time.
	if errs := validation.IsDNS1035Label(name); len(errs) > 0 {
		return trace.BadParameter("integration name %q must be a lower case valid DNS subdomain so that it can be used to allow Web/CLI access", name)
	}

	return nil
}
