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

package mcp

import (
	"strings"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// sanitizeMCPRequestParams removes all non-canonical "name" (e.g. "Name") from top-level "params"
// field of the MCP Request message. This is to prevent vulnerable upstream MCP servers and
// responding to malformed messages. This is a stripped down version of [sanitizeRawMCPRequest] and
// prevents the same issue.  For mcputils.JSONRPCRequest the blast radius is reduced because
// JSONRPCRequest.UnmarshalJSON is case-sensitive cutting out the edge cases like capitalized
// "method", "id" or "params" field.
func sanitizeMCPRequestParams(params mcputils.JSONRPCParams) {
	for k := range params {
		if nonCanonicalEqual(k, mcputils.ParamsNameKey) {
			delete(params, k)
		}
	}
}

func nonCanonicalEqual(a, b string) bool {
	return strings.EqualFold(a, b) && a != b
}
