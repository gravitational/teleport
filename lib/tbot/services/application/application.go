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

package application

import (
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	"github.com/gravitational/teleport/api/scopes"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/application")

// validateAppName validates an application service's app_name field. In scope
// mode the name must be a strongly valid scope-qualified name
// ("<scope>::<name>"); otherwise it must be a plain, non-qualified app name.
func validateAppName(name string, scoped bool) error {
	if scoped {
		sqn, err := scopes.ParseQualifiedName(name)
		if err != nil {
			return trace.BadParameter("app_name: %v", err)
		}

		if err := sqn.StrongValidate(); err != nil {
			return trace.BadParameter("app_name: %v", err)
		}

		return nil
	}

	qn, err := scopes.ParseOptionallyQualifiedName(name)
	if err != nil || qn.Scope != "" {
		return trace.BadParameter("app_name: can not be a scope-qualified name when not in scope mode")
	}

	return nil
}
