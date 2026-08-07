/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package ui

import (
	"github.com/gravitational/teleport/lib/utils/set"
)

// PrincipalSet holds the visible (granted + requestable) and granted-only
// principals for a single dimension of access (e.g., SSH logins, AWS role ARNs,
// db_names, db_users).
type PrincipalSet struct {
	// All contains both granted and requestable principals.
	All set.Set[string]
	// Granted contains only the principals the user can use without an
	// access request.
	Granted set.Set[string]
}
