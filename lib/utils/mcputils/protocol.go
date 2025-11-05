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

package mcputils

import (
	"encoding/json"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// Type definitions from both mcp-go/client/transport or mcp-go are not suitable
// for our reverse proxy use, thus this file redefines them.
//
// TODO(greedy52) switch to official golang lib or official go SDK if they offer
// the level of handling we need. Same goes for other helpers like StdioXXX.

// JSONRPCParams defines params for request or notification.
// TODO(greedy52) handle metadata
type JSONRPCParams map[string]any

// GetEventParams returns the apievents.Struct for auditing.
func (p JSONRPCParams) GetEventParams() *apievents.Struct {
	if p == nil {
		return nil
	}

	eventParams, _ := apievents.EncodeMap(p)
	return eventParams
}

// GetName returns the "name" param.
func (p JSONRPCParams) GetName() (string, bool) {
	if p == nil {
		return "", false
	}
	name, ok := p["name"].(string)
	return name, ok
}

// BaseJSONRPCMessage is a base message that includes all fields for MCP
// protocol.
//
// Note that json.RawMessage is used to keep the original content when
// marshaling it again. json.RawMessage can also be easily unmarshalled to user
// defined types when needed. Same applies to other types in this file.
type BaseJSONRPCMessage struct {
	// JSONRPC specifies the version of JSONRPC.
	JSONRPC string `json:"jsonrpc"`
	// ID is the ID for request and response. ID is nil for notification.
	ID mcp.RequestId `json:"id"`
	// Method is the request or notification method. Method is empty for response.
	Method mcp.MCPMethod `json:"method,omitempty"`
	// Params is the params for request and notification.
	Params JSONRPCParams `json:"params,omitempty"`
	// Result is the response result.
	Result json.RawMessage `json:"result,omitempty"`
	// Error is the response error.
	Error json.RawMessage `json:"error,omitempty"`
}

// IsNotification returns true if the message is a notification.
func (m *BaseJSONRPCMessage) IsNotification() bool {
	return m.ID.IsNil()
}

// IsRequest returns true if the message is a request.
func (m *BaseJSONRPCMessage) IsRequest() bool {
	return !m.ID.IsNil() && m.Method != ""
}

// IsResponse returns if the message is a response.
func (m *BaseJSONRPCMessage) IsResponse() bool {
	return !m.ID.IsNil() && (m.Result != nil || m.Error != nil)
}

// MakeNotification converts the base message to JSONRPCNotification.
func (m *BaseJSONRPCMessage) MakeNotification() *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: m.JSONRPC,
		Method:  m.Method,
		Params:  m.Params,
	}
}

// MakeRequest converts the base message to JSONRPCRequest.
func (m *BaseJSONRPCMessage) MakeRequest() *JSONRPCRequest {
	return &JSONRPCRequest{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Method:  m.Method,
		Params:  m.Params,
	}
}

// MakeResponse converts the base message to JSONRPCResponse.
func (m *BaseJSONRPCMessage) MakeResponse() *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Result:  m.Result,
		Error:   m.Error,
	}
}

// JSONRPCNotification defines a MCP notification.
//
// https://modelcontextprotocol.io/specification/2025-03-26/basic#notifications
type JSONRPCNotification struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  mcp.MCPMethod `json:"method"`
	Params  JSONRPCParams `json:"params,omitempty"`
}

// JSONRPCRequest defines a MCP request.
//
// https://modelcontextprotocol.io/specification/2025-03-26/basic#requests
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  mcp.MCPMethod `json:"method"`
	ID      mcp.RequestId `json:"id"`
	Params  JSONRPCParams `json:"params,omitempty"`
}

// JSONRPCResponse defines an MCP response.
//
// By protocol spec, responses are further sub-categorized as either successful
// results or errors. Either a result or an error MUST be set. A response MUST
// NOT set both.
//
// https://modelcontextprotocol.io/specification/2025-03-26/basic#responses
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      mcp.RequestId   `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// GetListToolResult assumes the result is for mcp.MethodToolsList and returns
// the corresponding go object.
func (r *JSONRPCResponse) GetListToolResult() (*mcp.ListToolsResult, error) {
	var listResult mcp.ListToolsResult
	if err := json.Unmarshal([]byte(r.Result), &listResult); err != nil {
		return nil, trace.Wrap(err)
	}
	return &listResult, nil
}

// GetInitializeResult assumes the result is for mcp.MethodInitialize and
// returns the corresponding go object.
func (r *JSONRPCResponse) GetInitializeResult() (*mcp.InitializeResult, error) {
	var result mcp.InitializeResult
	if err := json.Unmarshal(r.Result, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

// unmarshalResponse is a helper that unmarshalls a raw message to an
// JSONRPCResponse.
func unmarshalResponse(rawMessage string) (*JSONRPCResponse, error) {
	var base BaseJSONRPCMessage
	if err := json.Unmarshal([]byte(rawMessage), &base); err != nil {
		return nil, trace.Wrap(err)
	}
	if !base.IsResponse() {
		return nil, trace.BadParameter("message is not a response")
	}
	return base.MakeResponse(), nil
}

const (
	// MethodNotificationInitialized defines the method used for "initialized"
	// notification. This notification is sent by the client after it receives
	// the initialize response.
	MethodNotificationInitialized = "notifications/initialized"
)
