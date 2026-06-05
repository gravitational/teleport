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
	"encoding/json"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// sanitizeMCPRequestParams removes all non-canonical "name" (e.g. "Name") from top-level "params"
// field of the MCP Request message. This is to prevent vulnerable upstream MCP servers and
// responding to malformed messages. This is a stripped down version of [sanitizeRawMCPRequest] and
// prevents the same issue.  For mcputils.JSONRPCRequest the blast radius is reduced because
// JSONRPCRequest.UnmarshalJSON is case-sensitive cutting out the edge cases like capitalized
// "method", "id" or "params" field.
func sanitizeMCPRequestParams(r *mcputils.JSONRPCRequest) {
	deleteNonCanonicalKey(r.Params, mcputils.ParamsNameKey)
}

// sanitizeRawMCPRequest is an extended version of [sanitizeMCPRequestParams] which works on raw JSON
// data. It works for both MCP Requests and Notifications and removes all non-canonical fields and
// parameters which have a potential of confusing the upstream server. The confusion is possible
// due to non-compliant implementation. One example is usage of the Go builtin "json" package which
// is case-insensitive.
//
// Currently, it handles following cases:
//
//  1. Trying to confuse upstream MCP server with, non-canonical "name" parameter. Without this
//     sanitization Teleport correctly allows the call allowed_tool, but the upstream server may
//     incorrectly execute the denied_tool if it incorrectly decodes "name" parameter:
//
//     {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"Name":"denied_tool","name":"allowed_tool"}}
//
//  2. Trying to confuse upstream MCP server with, non-canonical "params" filed. Without this
//     sanitization Teleport correctly allows the call of allowed_tool, but the upstream server may
//     incorrectly execute the denied_tool if it incorrectly decodes "params" field:
//
//     {"jsonrpc":"2.0","id":1,"method":"tools/call","Params":{"name":"denied_tool"},"params":{"name":"allowed_tool"}}
//
//  3. Trying to confuse upstream MCP server with, non-canonical "method" filed. Without this
//     sanitization Teleport correctly forwards the ping request, but the upstream server may incorrectly
//     decode it as a tools/call request and execute the denied_tool.
//
//     {"jsonrpc":"2.0","id":1,"method":"ping","Method":"tools/call","params":{"name":"denied_tool"}}
//
//  4. Trying to confuse upstream MCP server with, non-canonical "id" filed. Without this
//     sanitization Teleport correctly interprets the message as a notification (messages without
//     the "id" field are Requests according to the spec), but the upstream server may incorrectly
//     decode it as a tools/call request and execute the denied_tool.
//
//     {"jsonrpc":"2.0","ID":1,"method":"tools/call","params":{"name":"denied_tool"}}
//
// It is implemented in a way that prevents corruptions caused by rounding float-precision number
// or overflowing very large numbers.
//
// NOTE: The provided request, can be modified or replaced like in the builtin append function.
func sanitizeRawMCPRequest(request []byte) ([]byte, error) {
	if len(request) == 0 {
		return request, nil
	}

	// Use json.RawMessage, to not mess with the content, e.g. introduce floating-point
	// corruption caused by rounding.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(request, &m); err != nil {
		return nil, trace.Wrap(err, "unmarshaling MCP Request into a map")
	}

	// Delete non-canonical "id", "method", "params" top-level message fields.
	modifiedM := deleteNonCanonicalKey(m, mcputils.IDKey, mcputils.MethodKey, mcputils.ParamsKey)

	var modifiedParams bool
	if rawParams := m[mcputils.ParamsKey]; len(rawParams) > 0 {
		var params map[string]json.RawMessage
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, trace.Wrap(err, "unmarshaling MCP Request params into a map")
		}

		// Delete non-canonical "name" key from "params" field.
		modifiedParams = deleteNonCanonicalKey(params, mcputils.ParamsNameKey)

		if modifiedParams {
			var err error
			m[mcputils.ParamsKey], err = json.Marshal(params)
			if err != nil {
				return nil, trace.Wrap(err, "encoding MCP Request params map back into JSON")
			}
		}
	}

	if modifiedM || modifiedParams {
		var err error
		request, err = json.Marshal(m)
		if err != nil {
			return nil, trace.Wrap(err, "encoding MCP Request map back into JSON")
		}
	}
	return request, nil
}

func deleteNonCanonicalKey[V any](m map[string]V, canonicalKey ...string) (modified bool) {
	for k := range m {
		for _, ck := range canonicalKey {
			if strings.EqualFold(k, ck) && k != ck {
				modified = true
				delete(m, k)
			}
		}
	}
	return
}
