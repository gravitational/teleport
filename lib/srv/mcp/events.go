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
	"context"
	"fmt"

	prototypes "github.com/gogo/protobuf/types"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

type baseMessage struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  mcp.MCPMethod     `json:"method"`
	ID      any               `json:"id,omitempty"`
	Params  *apievents.Struct `json:"params,omitempty"`
}

func (m *baseMessage) protoParams() *prototypes.Struct {
	if m.Params == nil {
		return nil
	}
	return &m.Params.Struct
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

// TODO(greedy52) refactor its own struct?
func emitStartEvent(ctx context.Context, session *sessionCtx) {
	emitEvent(ctx, session, &apievents.MCPSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.MCPSessionStartEvent,
			Code:        events.MCPSessionStartCode,
			ClusterName: session.identity.RouteToApp.ClusterName,
		},
		ServerMetadata:     getSessionServerMetadata(session),
		SessionMetadata:    getSessionMetadata(&session.identity),
		UserMetadata:       session.identity.GetUserMetadata(),
		ConnectionMetadata: getSessionConnectionMetadata(session),
		AppMetadata:        getSessionAppMetadata(session),
	})
}

func emitEndEvent(ctx context.Context, session *sessionCtx) {
	emitEvent(ctx, session, &apievents.MCPSessionEnd{
		Metadata: apievents.Metadata{
			Type:        events.MCPSessionEndEvent,
			Code:        events.MCPSessionEndCode,
			ClusterName: session.identity.RouteToApp.ClusterName,
		},
		ServerMetadata:     getSessionServerMetadata(session),
		SessionMetadata:    getSessionMetadata(&session.identity),
		UserMetadata:       session.identity.GetUserMetadata(),
		ConnectionMetadata: getSessionConnectionMetadata(session),
		AppMetadata:        getSessionAppMetadata(session),
	})
}

func emitNotificationEvent(ctx context.Context, session *sessionCtx, msg *baseMessage) {
	emitEvent(ctx, session, &apievents.MCPSessionNotification{
		Metadata: apievents.Metadata{
			Type:        events.MCPSessionNotificationEvent,
			Code:        events.MCPSessionNotificationCode,
			ClusterName: session.identity.RouteToApp.ClusterName,
		},
		SessionMetadata: getSessionMetadata(&session.identity),
		UserMetadata:    session.identity.GetUserMetadata(),
		AppMetadata:     getSessionAppMetadata(session),
		JSONRPC: &apievents.MCPJSONRPCDetails{
			Version: msg.JSONRPC,
			Method:  string(msg.Method),
			Params:  msg.Params,
		},
	})
}

func emitRequestEvent(ctx context.Context, session *sessionCtx, msg *baseMessage, err error) {
	// TODO(greedy52) is this assumption safe?
	if msg.ID == nil {
		emitNotificationEvent(ctx, session, msg)
		return
	}

	event := &apievents.MCPSessionRequest{
		Metadata: apievents.Metadata{
			Type:        events.MCPSessionRequestEvent,
			Code:        events.MCPSessionRequestCode,
			ClusterName: session.identity.RouteToApp.ClusterName,
		},
		SessionMetadata: getSessionMetadata(&session.identity),
		UserMetadata:    session.identity.GetUserMetadata(),
		AppMetadata:     getSessionAppMetadata(session),
		Status: apievents.Status{
			Success: true,
		},
		JSONRPC: &apievents.MCPJSONRPCDetails{
			Version: msg.JSONRPC,
			Method:  string(msg.Method),
			ID:      fmt.Sprintf("%v", msg.ID),
			Params:  msg.Params,
		},
	}

	if name, ok := msg.getName(); ok {
		event.JSONRPC.MethodName = name
	}
	if err != nil {
		event.Metadata.Code = events.MCPSessionRequestFailureCode
		event.Status.Success = false
		event.Status.Error = err.Error()
	}
	emitEvent(ctx, session, event)
}

func emitEvent(ctx context.Context, session *sessionCtx, event apievents.AuditEvent) {
	if err := session.emitter.EmitAuditEvent(ctx, event); err != nil {
		session.log.DebugContext(ctx, "Failed to emit audit event", "error", err)
	}
}

func getSessionServerMetadata(session *sessionCtx) apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerID:        session.serverID,
		ServerNamespace: apidefaults.Namespace,
	}
}

func getSessionConnectionMetadata(session *sessionCtx) apievents.ConnectionMetadata {
	return apievents.ConnectionMetadata{
		RemoteAddr: session.identity.LoginIP,
	}
}

func getSessionAppMetadata(session *sessionCtx) apievents.AppMetadata {
	return apievents.AppMetadata{
		AppURI:        session.app.GetURI(),
		AppPublicAddr: session.app.GetPublicAddr(),
		AppName:       session.app.GetName(),
		AppTargetPort: uint32(session.identity.RouteToApp.TargetPort),
	}
}

func getSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        identity.RouteToApp.SessionID,
		WithMFA:          identity.MFAVerified,
		PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
	}
}
