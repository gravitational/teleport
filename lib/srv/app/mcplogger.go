package app

import (
	"encoding/json"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/types/events"
)

// mcpMessageToEvent handles a single JSON-RPC message and either returns audit event (possibly empty) or error.
func mcpMessageToEvent(event *events.AppSessionMCPRequest, line string) (bool, error) {
	var rawMessage json.RawMessage
	err := json.Unmarshal([]byte(line), &rawMessage)
	if err != nil {
		return false, trace.Wrap(err, "failed to parse as JSON")
	}

	var baseMessage struct {
		JSONRPC string        `json:"jsonrpc"`
		Method  mcp.MCPMethod `json:"method"`
		ID      any           `json:"id,omitempty"`
	}

	err = json.Unmarshal(rawMessage, &baseMessage)
	if err != nil {
		return false, trace.Wrap(err, "failed to parse message as JSON-RPC")
	}

	// Check for valid JSONRPC version
	if baseMessage.JSONRPC != mcp.JSONRPC_VERSION {
		return false, trace.BadParameter("Invalid JSON-RPC version %q", baseMessage.JSONRPC)
	}

	if baseMessage.ID == nil {
		// probably notification, we ignore those for now anyway.
		var notification mcp.JSONRPCNotification
		if err := json.Unmarshal(rawMessage, &notification); err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC notification")
		}
		return false, nil // Return nil for notifications
	}

	event.RPCMethod = string(baseMessage.Method)

	s := &events.Struct{}
	if err := s.UnmarshalJSON(rawMessage); err != nil {
		return false, trace.Wrap(err)
	}
	event.RPCParams = s

	switch baseMessage.Method {
	case mcp.MethodInitialize:
		var request mcp.InitializeRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC initialize")
		}
		return true, nil
	case mcp.MethodPing:
		var request mcp.PingRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return false, nil
	case mcp.MethodResourcesList:
		var request mcp.ListResourcesRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return true, nil
	case mcp.MethodResourcesTemplatesList:
		var request mcp.ListResourceTemplatesRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return true, nil
	case mcp.MethodResourcesRead:
		var request mcp.ReadResourceRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return true, nil
	case mcp.MethodPromptsList:
		var request mcp.ListPromptsRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return false, nil
	case mcp.MethodPromptsGet:
		var request mcp.GetPromptRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return false, nil
	case mcp.MethodToolsList:
		var request mcp.ListToolsRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
		return false, nil
	case mcp.MethodToolsCall:
		var request mcp.CallToolRequest
		err = json.Unmarshal(rawMessage, &request)
		if err != nil {
			return false, trace.Wrap(err, "failed to parse message as JSON-RPC ping")
		}
	default:
		return true, nil
	}
	return true, nil
}
