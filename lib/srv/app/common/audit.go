/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// sensitiveHeaders is the set of header names (lowercase) that are stripped
// from recorded HTTP events to avoid leaking credentials.
var sensitiveHeaders = map[string]struct{}{
	"authorization": {},
	"cookie":        {},
	"set-cookie":    {},
	"x-auth-token":  {},
	"x-api-key":     {},
}

// filterHeaders returns a copy of h with sensitive headers removed.
// Headers whose (lowercase) name starts with "x-teleport-", "teleport-",
// or "x-forwarded-" are also stripped.
func filterHeaders(h http.Header) []*apievents.HTTPHeader {
	out := make([]*apievents.HTTPHeader, 0, len(h))
	for name, vals := range h {
		lower := strings.ToLower(name)
		if _, skip := sensitiveHeaders[lower]; skip {
			continue
		}
		if strings.HasPrefix(lower, "x-teleport-") ||
			strings.HasPrefix(lower, "teleport-") ||
			strings.HasPrefix(lower, "x-forwarded-") {
			continue
		}
		for _, v := range vals {
			out = append(out, &apievents.HTTPHeader{Name: name, Value: v})
		}
	}
	return out
}

// Audit defines an interface for app access audit events logger.
type Audit interface {
	// OnSessionStart is called when new app session starts.
	OnSessionStart(ctx context.Context, serverID string, identity *tlsca.Identity, app types.Application) error
	// OnSessionEnd is called when an app session ends.
	OnSessionEnd(ctx context.Context, serverID string, identity *tlsca.Identity, app types.Application) error
	// OnSessionChunk is called when a new session chunk is created.
	OnSessionChunk(ctx context.Context, serverID, chunkID string, identity *tlsca.Identity, app types.Application) error
	// OnRequest is called when an app request is sent during the session and a response is received.
	OnRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, status uint32, re *AWSResolvedEndpoint) error
	// OnDynamoDBRequest is called when app request for a DynamoDB API is sent and a response is received.
	OnDynamoDBRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, status uint32, re *AWSResolvedEndpoint) error
	// OnHTTPRequest is called when a proxied HTTP request is about to be forwarded.
	OnHTTPRequest(ctx context.Context, sessionCtx *SessionContext, requestID string, req *http.Request) error
	// OnHTTPRequestBodyChunk is called for each chunk of a proxied HTTP request body.
	OnHTTPRequestBodyChunk(ctx context.Context, sessionCtx *SessionContext, requestID string, chunkIndex int64, isLast bool, data []byte) error
	// OnHTTPResponse is called when a proxied HTTP response header is received.
	OnHTTPResponse(ctx context.Context, sessionCtx *SessionContext, requestID string, resp *http.Response, waitTimeMs int64) error
	// OnHTTPResponseBodyChunk is called for each chunk of a proxied HTTP response body.
	OnHTTPResponseBodyChunk(ctx context.Context, sessionCtx *SessionContext, requestID string, chunkIndex int64, isLast bool, data []byte) error
	// EmitEvent emits the provided audit event.
	EmitEvent(ctx context.Context, event apievents.AuditEvent) error
}

// AuditConfig is the audit events emitter configuration.
type AuditConfig struct {
	// Emitter is used to emit audit events.
	Emitter apievents.Emitter
	// Recorder is used to record session events.
	Recorder events.SessionPreparerRecorder
}

// Check validates the config.
func (c *AuditConfig) Check() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.Recorder == nil {
		return trace.BadParameter("missing Recorder")
	}
	return nil
}

// audit provides methods for emitting app access audit events.
type audit struct {
	// cfg is the audit events emitter configuration.
	cfg AuditConfig
	// log is used for logging
	log *slog.Logger
}

// NewAudit returns a new instance of the audit events emitter.
func NewAudit(config AuditConfig) (Audit, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &audit{
		cfg: config,
		log: slog.With(teleport.ComponentKey, "app:audit"),
	}, nil
}

func getSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return apievents.SessionMetadata{
		SessionID:        identity.RouteToApp.SessionID,
		WithMFA:          identity.MFAVerified,
		PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
	}
}

// OnSessionStart is called when new app session starts.
func (a *audit) OnSessionStart(ctx context.Context, serverID string, identity *tlsca.Identity, app types.Application) error {
	event := &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        serverID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: identity.LoginIP,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppName:       app.GetName(),
			AppTargetPort: uint32(identity.RouteToApp.TargetPort),
		},
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnSessionEnd is called when an app session ends.
func (a *audit) OnSessionEnd(ctx context.Context, serverID string, identity *tlsca.Identity, app types.Application) error {
	event := &apievents.AppSessionEnd{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionEndEvent,
			Code:        events.AppSessionEndCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        serverID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: identity.LoginIP,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppName:       app.GetName(),
			AppTargetPort: uint32(identity.RouteToApp.TargetPort),
		},
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnSessionChunk is called when a new session chunk is created.
func (a *audit) OnSessionChunk(ctx context.Context, serverID, chunkID string, identity *tlsca.Identity, app types.Application) error {
	event := &apievents.AppSessionChunk{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionChunkEvent,
			Code:        events.AppSessionChunkCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        serverID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppName:       app.GetName(),
			// Session chunks are not created for TCP apps, so there's no need to pass TargetPort here.
		},
		SessionChunkID: chunkID,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnRequest is called when an app request is sent during the session and a response is received.
func (a *audit) OnRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, status uint32, re *AWSResolvedEndpoint) error {
	event := &apievents.AppSessionRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionRequestEvent,
			Code: events.AppSessionRequestCode,
		},
		AppMetadata:        *MakeAppMetadata(sessionCtx.App),
		Method:             req.Method,
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		StatusCode:         status,
		AWSRequestMetadata: *MakeAWSRequestMetadata(req, re),
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnDynamoDBRequest is called when a DynamoDB app request is sent during the session.
func (a *audit) OnDynamoDBRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, status uint32, re *AWSResolvedEndpoint) error {
	// Try to read the body and JSON unmarshal it.
	// If this fails, we still want to emit the rest of the event info; the request event Body is nullable, so it's ok if body is left nil here.
	body, err := awsutils.UnmarshalRequestBody(req)
	if err != nil {
		a.log.WarnContext(ctx, "Failed to read request body as JSON, omitting the body from the audit event.", "error", err)
	}
	// get the API target from the request header, according to the API request format documentation:
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html#Programming.LowLevelAPI.RequestFormat
	target := req.Header.Get(awsutils.AmzTargetHeader)
	event := &apievents.AppSessionDynamoDBRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionDynamoDBRequestEvent,
			Code: events.AppSessionDynamoDBRequestCode,
		},
		UserMetadata:       sessionCtx.Identity.GetUserMetadata(),
		AppMetadata:        *MakeAppMetadata(sessionCtx.App),
		AWSRequestMetadata: *MakeAWSRequestMetadata(req, re),
		SessionChunkID:     sessionCtx.ChunkID,
		StatusCode:         status,
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		Method:             req.Method,
		Target:             target,
		Body:               body,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnHTTPRequest is called when a proxied HTTP request is about to be forwarded.
func (a *audit) OnHTTPRequest(ctx context.Context, sessionCtx *SessionContext, requestID string, req *http.Request) error {
	event := &apievents.AppSessionHTTPRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionHTTPRequestEvent,
			Code: events.AppSessionHTTPRequestCode,
		},
		UserMetadata: sessionCtx.Identity.GetUserMetadata(),
		AppMetadata:  *MakeAppMetadata(sessionCtx.App),
		RequestId:    requestID,
		Method:       req.Method,
		// Url already contains the query string; RawQuery is duplicated here for
		// HAR compatibility so consumers can access it without re-parsing the URL.
		Url:         req.URL.String(),
		HttpVersion: req.Proto,
		Headers:     filterHeaders(req.Header),
		RawQuery:    req.URL.RawQuery,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnHTTPRequestBodyChunk is called for each chunk of a proxied HTTP request body.
func (a *audit) OnHTTPRequestBodyChunk(ctx context.Context, _ *SessionContext, requestID string, chunkIndex int64, isLast bool, data []byte) error {
	event := &apievents.AppSessionHTTPRequestBodyChunk{
		Metadata: apievents.Metadata{
			Type: events.AppSessionHTTPRequestBodyChunkEvent,
			Code: events.AppSessionHTTPRequestBodyChunkCode,
		},
		RequestId:  requestID,
		ChunkIndex: chunkIndex,
		IsLast:     isLast,
		Data:       data,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnHTTPResponse is called when a proxied HTTP response header is received.
// SendTimeMs and ReceiveTimeMs are left at zero: send time is not tracked at
// this layer, and receive time cannot be known until the body is fully read.
func (a *audit) OnHTTPResponse(ctx context.Context, sessionCtx *SessionContext, requestID string, resp *http.Response, waitTimeMs int64) error {
	event := &apievents.AppSessionHTTPResponse{
		Metadata: apievents.Metadata{
			Type: events.AppSessionHTTPResponseEvent,
			Code: events.AppSessionHTTPResponseCode,
		},
		AppMetadata: *MakeAppMetadata(sessionCtx.App),
		RequestId:   requestID,
		StatusCode:  uint32(resp.StatusCode),
		StatusText:  resp.Status,
		HttpVersion: resp.Proto,
		Headers:     filterHeaders(resp.Header),
		WaitTimeMs:  waitTimeMs,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnHTTPResponseBodyChunk is called for each chunk of a proxied HTTP response body.
func (a *audit) OnHTTPResponseBodyChunk(ctx context.Context, _ *SessionContext, requestID string, chunkIndex int64, isLast bool, data []byte) error {
	event := &apievents.AppSessionHTTPResponseBodyChunk{
		Metadata: apievents.Metadata{
			Type: events.AppSessionHTTPResponseBodyChunkEvent,
			Code: events.AppSessionHTTPResponseBodyChunkCode,
		},
		RequestId:  requestID,
		ChunkIndex: chunkIndex,
		IsLast:     isLast,
		Data:       data,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// EmitEvent emits the provided audit event.
func (a *audit) EmitEvent(ctx context.Context, e apievents.AuditEvent) error {
	preparedEvent, err := a.cfg.Recorder.PrepareSessionEvent(e)
	if err != nil {
		return trace.Wrap(err)
	}

	recErr := a.cfg.Recorder.RecordEvent(ctx, preparedEvent)
	event := preparedEvent.GetAuditEvent()
	var emitErr error
	switch event.GetType() {
	case events.AppSessionRequestEvent,
		events.AppSessionHTTPRequestEvent,
		events.AppSessionHTTPRequestBodyChunkEvent,
		events.AppSessionHTTPResponseEvent,
		events.AppSessionHTTPResponseBodyChunkEvent:
		// session-recording only; do not forward to the global audit emitter
	default:
		emitErr = a.cfg.Emitter.EmitAuditEvent(ctx, event)
	}

	return trace.NewAggregate(recErr, emitErr)
}

// MakeAppMetadata returns common server metadata for database session.
func MakeAppMetadata(app types.Application) *apievents.AppMetadata {
	return &apievents.AppMetadata{
		AppURI:        app.GetURI(),
		AppPublicAddr: app.GetPublicAddr(),
		AppName:       app.GetName(),
	}
}

// MakeAWSRequestMetadata is a helper to build AWSRequestMetadata from the provided request and endpoint.
// If the aws endpoint is nil, returns an empty request metadata.
func MakeAWSRequestMetadata(req *http.Request, awsEndpoint *AWSResolvedEndpoint) *apievents.AWSRequestMetadata {
	if awsEndpoint == nil {
		return &apievents.AWSRequestMetadata{}
	}

	return &apievents.AWSRequestMetadata{
		AWSRegion:      awsEndpoint.SigningRegion,
		AWSService:     awsEndpoint.SigningName,
		AWSHost:        req.URL.Host,
		AWSAssumedRole: GetAWSAssumedRole(req),
	}
}
