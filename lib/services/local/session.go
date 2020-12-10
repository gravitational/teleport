/*
Copyright 2020 Gravitational, Inc.

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

package local

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// GetAppSession gets an application web session.
func (s *IdentityService) GetAppSession(ctx context.Context, req services.GetAppSessionRequest) (services.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.Get(ctx, backend.Key(appsPrefix, sessionsPrefix, req.SessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value, services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// GetAppSessions gets all application web sessions.
func (s *IdentityService) GetAppSessions(ctx context.Context) ([]services.WebSession, error) {
	startKey := backend.Key(appsPrefix, sessionsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]services.WebSession, len(result.Items))
	for i, item := range result.Items {
		session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = session
	}
	return out, nil
}

// UpsertAppSession creates an application web session.
func (s *IdentityService) UpsertAppSession(ctx context.Context, session services.WebSession) error {
	value, err := services.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(appsPrefix, sessionsPrefix, session.GetName()),
		Value:   value,
		Expires: session.GetExpiryTime(),
	}

	if _, err = s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAppSession removes an application web session.
func (s *IdentityService) DeleteAppSession(ctx context.Context, req services.DeleteAppSessionRequest) error {
	if err := s.Delete(ctx, backend.Key(appsPrefix, sessionsPrefix, req.SessionID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllAppSessions removes all application web sessions.
func (s *IdentityService) DeleteAllAppSessions(ctx context.Context) error {
	startKey := backend.Key(appsPrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetWebSession returns a web session state described with req
func (s *IdentityService) GetWebSession(ctx context.Context, req services.GetWebSessionRequest) (services.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.Get(ctx, backend.Key(webPrefix, sessionsPrefix, req.SessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is for backwards compatibility to ensure we
	// always have these values
	session.SetUser(req.User)
	session.SetName(req.SessionID)
	return session, nil
}

// GetWebSessions gets all regular web sessions.
func (s *IdentityService) GetWebSessions(ctx context.Context) ([]services.WebSession, error) {
	startKey := backend.Key(webPrefix, sessionsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.WebSession, 0, len(result.Items))
	for _, item := range result.Items {
		session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, session)
	}
	return out, nil
}

// UpsertWebSession updates or inserts a web session for the given req.
// Session will be created with bearer token expiry time TTL, because
// it is expected that client will periodically update it
func (s *IdentityService) UpsertWebSession(ctx context.Context, session services.WebSession) error {
	// TODO(dmitri): session should already have the user and name (ID)
	// session.SetUser(req.User)
	// session.SetName(req.Name)
	value, err := services.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:     backend.Key(webPrefix, sessionsPrefix, session.GetName()),
		Value:   value,
		Expires: backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
	}
	_, err = s.Put(ctx, item)
	return trace.Wrap(err)
}

// DeleteWebSession deletes web session from the storage
func (s *IdentityService) DeleteWebSession(ctx context.Context, req services.DeleteWebSessionRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(s.Delete(ctx, backend.Key(webPrefix, sessionsPrefix, req.SessionID)))
}

// DeleteAllWebSessions removes all regular web sessions.
func (s *IdentityService) DeleteAllWebSessions(ctx context.Context) error {
	startKey := backend.Key(webPrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
