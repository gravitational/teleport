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
	"net/http"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// RequestWithContext defines context for a request for auditing purpose.
type RequestWithContext struct {
	// Request is the HTTP request sent to upstream.
	*http.Request
	// SessionContext is the app session context.
	SessionContext *SessionContext
	// Status is the response status code.
	Status uint32
	// ResolvedEndpoint is the AWS resolved endpoint.
	ResolvedEndpoint *endpoints.ResolvedEndpoint
	// SigningCtx is the AWS signing context.
	SigningCtx *awsutils.SigningCtx
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
	OnRequest(ctx context.Context, req *RequestWithContext) error
	// OnDynamoDBRequest is called when app request for a DynamoDB API is sent and a response is received.
	OnDynamoDBRequest(ctx context.Context, req *RequestWithContext) error
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
	log logrus.FieldLogger
}

// NewAudit returns a new instance of the audit events emitter.
func NewAudit(config AuditConfig) (Audit, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &audit{
		cfg: config,
		log: logrus.WithField(teleport.ComponentKey, "app:audit"),
	}, nil
}

func getSessionMetadata(identity *tlsca.Identity) apievents.SessionMetadata {
	return identity.GetSessionMetadata(identity.RouteToApp.SessionID)
}

// OnSessionStart is called when new app session starts.
func (a *audit) OnSessionStart(ctx context.Context, serverID string, identity *tlsca.Identity, app types.Application) error {
	event := &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		ServerMetadata:  MakeServerMetadata(serverID),
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: identity.LoginIP,
		},
		AppMetadata: MakeAppMetadata(app),
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
		ServerMetadata:  MakeServerMetadata(serverID),
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: identity.LoginIP,
		},
		AppMetadata: MakeAppMetadata(app),
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
		ServerMetadata:  MakeServerMetadata(serverID),
		SessionMetadata: getSessionMetadata(identity),
		UserMetadata:    identity.GetUserMetadata(),
		AppMetadata:     MakeAppMetadata(app),
		SessionChunkID:  chunkID,
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnRequest is called when an app request is sent during the session and a response is received.
func (a *audit) OnRequest(ctx context.Context, req *RequestWithContext) error {
	event := &apievents.AppSessionRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionRequestEvent,
			Code: events.AppSessionRequestCode,
		},
		AppMetadata:        MakeAppMetadata(req.SessionContext.App),
		Method:             req.Method,
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		StatusCode:         req.Status,
		AWSRequestMetadata: MakeAWSRequestMetadata(req.Request, req.ResolvedEndpoint),
		AWSSessionMetadata: req.SigningCtx.MakeSessionMetadata(),
	}
	return trace.Wrap(a.EmitEvent(ctx, event))
}

// OnDynamoDBRequest is called when a DynamoDB app request is sent during the session.
func (a *audit) OnDynamoDBRequest(ctx context.Context, req *RequestWithContext) error {
	// Try to read the body and JSON unmarshal it.
	// If this fails, we still want to emit the rest of the event info; the request event Body is nullable, so it's ok if body is left nil here.
	body, err := awsutils.UnmarshalRequestBody(req.Request)
	if err != nil {
		a.log.WithError(err).Warn("Failed to read request body as JSON, omitting the body from the audit event.")
	}
	// get the API target from the request header, according to the API request format documentation:
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html#Programming.LowLevelAPI.RequestFormat
	target := req.Header.Get(awsutils.AmzTargetHeader)
	event := &apievents.AppSessionDynamoDBRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionDynamoDBRequestEvent,
			Code: events.AppSessionDynamoDBRequestCode,
		},
		UserMetadata:       req.SessionContext.Identity.GetUserMetadata(),
		AppMetadata:        MakeAppMetadata(req.SessionContext.App),
		AWSRequestMetadata: MakeAWSRequestMetadata(req.Request, req.ResolvedEndpoint),
		SessionChunkID:     req.SessionContext.ChunkID,
		StatusCode:         req.Status,
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		Method:             req.Method,
		Target:             target,
		Body:               body,
		AWSSessionMetadata: req.SigningCtx.MakeSessionMetadata(),
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
	// AppSessionRequest events should only go to session recording
	if event.GetType() != events.AppSessionRequestEvent {
		emitErr = a.cfg.Emitter.EmitAuditEvent(ctx, event)
	}

	return trace.NewAggregate(recErr, emitErr)
}

// MakeAppMetadata returns common server metadata for database session.
func MakeAppMetadata(app types.Application) apievents.AppMetadata {
	return apievents.AppMetadata{
		AppURI:        app.GetURI(),
		AppPublicAddr: app.GetPublicAddr(),
		AppName:       app.GetName(),
	}
}

// MakeAWSRequestMetadata is a helper to build AWSRequestMetadata from the provided request and endpoint.
// If the aws endpoint is nil, returns an empty request metadata.
func MakeAWSRequestMetadata(req *http.Request, awsEndpoint *endpoints.ResolvedEndpoint) apievents.AWSRequestMetadata {
	if awsEndpoint == nil || req == nil {
		return apievents.AWSRequestMetadata{}
	}

	return apievents.AWSRequestMetadata{
		AWSRegion:      awsEndpoint.SigningRegion,
		AWSService:     awsEndpoint.SigningName,
		AWSHost:        req.URL.Host,
		AWSAssumedRole: GetAWSAssumedRole(req),
	}
}

// MakeServerMetadata is a helper to build ServerMetadata for app session
// events.
func MakeServerMetadata(serverID string) apievents.ServerMetadata {
	return apievents.ServerMetadata{
		ServerVersion:   teleport.Version,
		ServerID:        serverID,
		ServerNamespace: apidefaults.Namespace,
	}
}
