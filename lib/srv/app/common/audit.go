/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// Audit defines an interface for app access audit events logger.
type Audit interface {
	// TODO(gavin): move app session start/end event emitting out of tcpServer and into this interface as OnSessionStart/End
	// OnSessionChunk is called when a new session chunk is created.
	OnSessionChunk(ctx context.Context, sessionCtx *SessionContext, serverID string)
	// OnRequest is called when an app request is sent during the session.
	OnRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, res *http.Response, re *endpoints.ResolvedEndpoint)
	// EmitEvent emits the provided audit event.
	EmitEvent(ctx context.Context, event apievents.AuditEvent)
}

// AuditConfig is the audit events emitter configuration.
type AuditConfig struct {
	// Emitter is used to emit audit events.
	Emitter apievents.Emitter
}

// Check validates the config.
func (c *AuditConfig) Check() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
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
		log: logrus.WithField(trace.Component, "app:audit"),
	}, nil
}

// OnSessionChunk is called when a new session chunk is created.
func (a *audit) OnSessionChunk(ctx context.Context, sessionCtx *SessionContext, serverID string) {
	event := &apievents.AppSessionChunk{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionChunkEvent,
			Code:        events.AppSessionChunkCode,
			ClusterName: sessionCtx.Identity.RouteToApp.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerID:        serverID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sessionCtx.Identity.RouteToApp.SessionID,
			WithMFA:   sessionCtx.Identity.MFAVerified,
		},
		UserMetadata: sessionCtx.Identity.GetUserMetadata(),
		AppMetadata: apievents.AppMetadata{
			AppURI:        sessionCtx.App.GetURI(),
			AppPublicAddr: sessionCtx.App.GetPublicAddr(),
			AppName:       sessionCtx.App.GetName(),
		},
		SessionChunkID: sessionCtx.ChunkID,
	}
	a.EmitEvent(ctx, event)
}

// OnRequest is called when an app request is sent during the session.
func (a *audit) OnRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, res *http.Response, re *endpoints.ResolvedEndpoint) {
	if sessionCtx.App.IsDynamoDB() {
		a.onDynamoDBRequest(ctx, sessionCtx, req, res, re)
		return
	}
	event := &apievents.AppSessionRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionRequestEvent,
			Code: events.AppSessionRequestCode,
		},
		AppMetadata:        *MakeAppMetadata(sessionCtx.App),
		Method:             req.Method,
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		StatusCode:         uint32(res.StatusCode),
		AWSRequestMetadata: *MakeAWSRequestMetadata(req, re),
	}
	a.EmitEvent(ctx, event)
}

// onDynamoDBRequest is called when a DynamoDB app request is sent during the session.
func (a *audit) onDynamoDBRequest(ctx context.Context, sessionCtx *SessionContext, req *http.Request, res *http.Response, re *endpoints.ResolvedEndpoint) {
	jsonBody, err := io.ReadAll(req.Body)
	if err != nil {
		a.log.WithError(err).Warn("Failed to read DynamoDB request body.")
	}
	body := &apievents.Struct{}
	err = body.UnmarshalJSON(jsonBody)
	if err != nil {
		a.log.WithError(err).Warn("Failed to decode DynamoDB request JSON body.")
	}
	// get the API target from the request header, according to the API request format documentation:
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html#Programming.LowLevelAPI.RequestFormat
	target := req.Header.Get(awsutils.TargetHeader)
	event := &apievents.AppSessionDynamoDBRequest{
		Metadata: apievents.Metadata{
			Type: events.AppSessionDynamoDBRequestEvent,
			Code: events.AppSessionDynamoDBRequestCode,
		},
		UserMetadata:       sessionCtx.Identity.GetUserMetadata(),
		AppMetadata:        *MakeAppMetadata(sessionCtx.App),
		AWSRequestMetadata: *MakeAWSRequestMetadata(req, re),
		SessionChunkID:     sessionCtx.ChunkID,
		StatusCode:         uint32(res.StatusCode),
		Path:               req.URL.Path,
		RawQuery:           req.URL.RawQuery,
		Method:             req.Method,
		Target:             target,
		Body:               body,
	}
	a.EmitEvent(ctx, event)
}

// EmitEvent emits the provided audit event.
func (a *audit) EmitEvent(ctx context.Context, event apievents.AuditEvent) {
	if err := a.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		a.log.WithError(err).Errorf("Failed to emit audit event: %v.", event)
	}
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
func MakeAWSRequestMetadata(req *http.Request, awsEndpoint *endpoints.ResolvedEndpoint) *apievents.AWSRequestMetadata {
	if awsEndpoint == nil {
		return &apievents.AWSRequestMetadata{}
	}
	return &apievents.AWSRequestMetadata{
		AWSRegion:  awsEndpoint.SigningRegion,
		AWSService: awsEndpoint.SigningName,
		AWSHost:    req.Host,
	}
}
