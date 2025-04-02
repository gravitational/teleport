package app

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// mcpMessageToEvent handles a single JSON-RPC message and either returns audit event (possibly empty) or error.
func mcpMessageToEvent(line string, userMeta apievents.UserMetadata, sessionMeta apievents.SessionMetadata) (apievents.AuditEvent, bool, error) {
	var baseMessage struct {
		JSONRPC string            `json:"jsonrpc"`
		Method  string            `json:"method"`
		ID      any               `json:"id,omitempty"`
		Params  *apievents.Struct `json:"params,omitempty"`
	}
	if err := json.Unmarshal([]byte(line), &baseMessage); err != nil {
		return nil, false, trace.Wrap(err, "failed to parse MCP message")
	}
	shouldEmit := shouldEmitMCPEvent(mcp.MCPMethod(baseMessage.Method))
	if baseMessage.ID == nil {
		return &apievents.AppSessionMCPNotification{
			UserMetadata:    userMeta,
			SessionMetadata: sessionMeta,
			Metadata: apievents.Metadata{
				Type: events.AppSessionMCPNotificationEvent,
				Code: events.AppSessionMCPNotificationCode,
			},
			JSONRPC:   baseMessage.JSONRPC,
			RPCMethod: baseMessage.Method,
			RPCParams: baseMessage.Params,
		}, shouldEmit, nil
	}
	return &apievents.AppSessionMCPRequest{
		UserMetadata:    userMeta,
		SessionMetadata: sessionMeta,
		Metadata: apievents.Metadata{
			Type: events.AppSessionMCPRequestEvent,
			Code: events.AppSessionMCPRequestCode,
		},
		JSONRPC:   baseMessage.JSONRPC,
		RPCMethod: baseMessage.Method,
		RPCID:     fmt.Sprintf("%v", baseMessage.ID),
		RPCParams: baseMessage.Params,
	}, shouldEmit, nil
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
