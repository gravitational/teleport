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
	prototypes "github.com/gogo/protobuf/types"
	"github.com/mark3labs/mcp-go/mcp"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type baseMessage struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  mcp.MCPMethod     `json:"method"`
	ID      any               `json:"id,omitempty"`
	Params  *apievents.Struct `json:"params,omitempty"`
}

func (m *baseMessage) getName() (string, bool) {
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

func shouldEmitMCPEvent(method mcp.MCPMethod) bool {
	switch method {
	case mcp.MethodPing,
		mcp.MethodResourcesList,
		mcp.MethodResourcesTemplatesList,
		mcp.MethodPromptsList,
		mcp.MethodToolsList:
		return false
	default:
		return true
	}
}
