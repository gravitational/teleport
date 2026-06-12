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

package transform

import "github.com/gravitational/teleport/api/types"

// DefaultRedactionRules returns the redaction rules for migrated config review
// output.
func DefaultRedactionRules() []RedactRule {
	return []RedactRule{
		{Path: []string{"teleport", "auth_token"}, Mode: RedactFull},
		{Path: []string{"teleport", "token"}, Mode: RedactFull},
		{Path: []string{"teleport", "join_params", "token_secret"}, Mode: RedactFull},
		{Path: []string{"teleport", "join_params", "bound_keypair", "registration_secret_value"}, Mode: RedactFull},
		{Path: []string{"teleport", "join_params", "token_name"}, Mode: RedactTokenName},
	}
}

func maskTokenName(name string) string {
	return types.MaskTokenName(name)
}
