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
	jsoniter "github.com/json-iterator/go"
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
	ID mcp.RequestId `json:"id,omitempty"`
	// Method is the request or notification method. Method is empty for response.
	Method string `json:"method,omitempty"`
	// Params is the params for request and notification.
	Params JSONRPCParams `json:"params,omitempty"`
	// Result is the response result.
	Result json.RawMessage `json:"result,omitempty"`
	// Error is the response error.
	Error json.RawMessage `json:"error,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler with case-sensitive field matching.
// This ensures that json.Unmarshal automatically enforces case sensitivity for
// this type.
func (m *BaseJSONRPCMessage) UnmarshalJSON(data []byte) error {
	type Alias BaseJSONRPCMessage
	aux := (*Alias)(m)
	return trace.Wrap(caseSensitiveJSONConfig.Unmarshal(data, aux))
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
	Method  string        `json:"method"`
	Params  JSONRPCParams `json:"params,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler with case-sensitive field matching.
// This ensures that json.Unmarshal automatically enforces case sensitivity for
// this type.
func (n *JSONRPCNotification) UnmarshalJSON(data []byte) error {
	type Alias JSONRPCNotification
	aux := (*Alias)(n)
	return trace.Wrap(caseSensitiveJSONConfig.Unmarshal(data, aux))
}

// JSONRPCRequest defines a MCP request.
//
// https://modelcontextprotocol.io/specification/2025-03-26/basic#requests
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	ID      mcp.RequestId `json:"id,omitempty"`
	Params  JSONRPCParams `json:"params,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler with case-sensitive field matching.
// This ensures that json.Unmarshal automatically enforces case sensitivity for
// this type.
func (r *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	type Alias JSONRPCRequest
	aux := (*Alias)(r)
	return trace.Wrap(caseSensitiveJSONConfig.Unmarshal(data, aux))
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

// UnmarshalJSON implements json.Unmarshaler with case-sensitive field matching.
// This ensures that json.Unmarshal automatically enforces case sensitivity for
// this type.
func (r *JSONRPCResponse) UnmarshalJSON(data []byte) error {
	type Alias JSONRPCResponse
	aux := (*Alias)(r)
	return trace.Wrap(caseSensitiveJSONConfig.Unmarshal(data, aux))
}

// GetListToolResult assumes the result is for MethodToolsList and returns
// the corresponding go object.
func (r *JSONRPCResponse) GetListToolResult() (*mcp.ListToolsResult, error) {
	var listResult mcp.ListToolsResult
	if err := UnmarshalJSONRPCMessage(r.Result, &listResult); err != nil {
		return nil, trace.Wrap(err)
	}
	return &listResult, nil
}

// GetInitializeResult assumes the result is for MethodInitialize and
// returns the corresponding go object.
func (r *JSONRPCResponse) GetInitializeResult() (*mcp.InitializeResult, error) {
	var result mcp.InitializeResult
	if err := UnmarshalJSONRPCMessage(r.Result, &result); err != nil {
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

// UnmarshalJSONRPCMessage performs case-sensitive JSON umarshal.
func UnmarshalJSONRPCMessage(data []byte, v any) error {
	return trace.Wrap(caseSensitiveJSONConfig.Unmarshal(data, v))
}

// caseSensitiveJSONConfig is used to decode JSON RPC messages. The config is
// based on jsoniter.ConfigCompatibleWithStandardLibrary with the addition of
// CaseSensitive enabled.
var caseSensitiveJSONConfig = jsoniter.Config{
	EscapeHTML:             true,
	SortMapKeys:            true,
	ValidateJsonRawMessage: true,
	CaseSensitive:          true,
}.Froze()

const (
	// MethodInitialize initiates connection and negotiates protocol capabilities.
	MethodInitialize = "initialize"

	// MethodPing verifies connection liveness between client and server.
	MethodPing = "ping"

	// MethodResourcesList lists all available server resources.
	MethodResourcesList = "resources/list"

	// MethodResourcesTemplatesList provides URI templates for constructing resource URIs.
	MethodResourcesTemplatesList = "resources/templates/list"

	// MethodResourcesRead retrieves content of a specific resource by URI.
	MethodResourcesRead = "resources/read"

	// MethodPromptsList lists all available prompt templates.
	MethodPromptsList = "prompts/list"

	// MethodPromptsGet retrieves a specific prompt template with filled parameters.
	MethodPromptsGet = "prompts/get"

	// MethodToolsList lists all available executable tools.
	MethodToolsList = "tools/list"

	// MethodToolsCall invokes a specific tool with provided parameters.
	MethodToolsCall = "tools/call"

	// MethodSetLogLevel configures the minimum log level for client
	MethodSetLogLevel = "logging/setLevel"

	// MethodElicitationCreate requests additional information from the user during interactions.
	MethodElicitationCreate = "elicitation/create"

	// MethodListRoots requests roots list from the client during interactions.
	MethodListRoots = "roots/list"

	// MethodSamplingCreateMessage is sent by server to request client to sample messages from LLM.
	MethodSamplingCreateMessage = "sampling/createMessage"

	// MethodNotificationResourcesListChanged notifies when the list of available resources changes.
	MethodNotificationResourcesListChanged = "notifications/resources/list_changed"

	// MethodNotificationResourceUpdated notifies when a resource changes.
	MethodNotificationResourceUpdated = "notifications/resources/updated"

	// MethodNotificationPromptsListChanged notifies when the list of available prompt templates changes.
	MethodNotificationPromptsListChanged = "notifications/prompts/list_changed"

	// MethodNotificationToolsListChanged notifies when the list of available tools changes.
	MethodNotificationToolsListChanged = "notifications/tools/list_changed"

	// MethodNotificationRootsListChanged notifies when the list of available roots changes.
	MethodNotificationRootsListChanged = "notifications/roots/list_changed"

	// MethodNotificationInitialized defines the method used for "initialized"
	// notification. This notification is sent by the client after it receives
	// the initialize response.
	MethodNotificationInitialized = "notifications/initialized"
)
