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

package mcp

import (
	"encoding/json"
	"fmt"

	prototypes "github.com/gogo/protobuf/types"
	"github.com/mark3labs/mcp-go/mcp"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type baseResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      mcp.RequestId   `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func (m *baseResponse) getID() string {
	return idToString(m.ID)
}

func idToString(id mcp.RequestId) string {
	return fmt.Sprintf("%v", id)
}

type baseRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  mcp.MCPMethod     `json:"method"`
	ID      mcp.RequestId     `json:"id,omitempty"`
	Params  *apievents.Struct `json:"params,omitempty"`
}

func (m *baseRequest) protoParams() *prototypes.Struct {
	if m.Params == nil {
		return nil
	}
	return &m.Params.Struct
}

func (m *baseRequest) getID() string {
	return idToString(m.ID)
}

func (m *baseRequest) getName() (string, bool) {
	if m.Params == nil {
		return "", false
	}
	value, ok := m.Params.Fields["name"]
	if !ok {
		return "", false
	}
	x, ok := value.GetKind().(*prototypes.Value_StringValue)
	if !ok {
		return "", false
	}
	return x.StringValue, true
}

func makeToolAccessDeniedResponse(msg *baseRequest, authErr error) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      msg.ID,
		Result: &mcp.CallToolResult{
			Content: []mcp.Content{mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Error: %v. RBAC is enforced by your Teleport roles. Contact your Teleport Adminstrators for more details.", authErr),
			}},
			IsError: false,
		},
	}
}
