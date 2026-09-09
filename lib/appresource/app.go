/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package appresource

import "github.com/gravitational/teleport/api/types"

// GovernsApp returns true when app_resources rules apply to app. The rules
// do not apply to TCP, MCP, AWS console, Azure, GCP, LLM, and Identity
// Center apps.
func GovernsApp(app types.Application) bool {
	return !app.IsTCP() && !app.IsMCP() && !app.IsAWSConsole() && !app.IsAzureCloud() &&
		!app.IsGCP() && !app.IsLLM() && app.GetSubKind() != types.KindIdentityCenterAccount
}
