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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// unmarshalResponse is a helper that unmarshalls a raw message to an
// jsonrpc.Response.
func unmarshalResponse(rawMessage string) (*jsonrpc.Response, error) {
	message, err := jsonrpc.DecodeMessage([]byte(rawMessage))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, ok := message.(*jsonrpc.Response)
	if !ok {
		return nil, trace.BadParameter("message is not a response")
	}
	return response, nil
}

// IDString returns the string representation of the ID.
func IDString(id jsonrpc.ID) string {
	if !id.IsValid() {
		return ""
	}
	return fmt.Sprintf("%v", id.Raw())
}

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
