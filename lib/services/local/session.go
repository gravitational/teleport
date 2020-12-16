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

// GetWebSession returns a web session state for the given user and session id
func (s *IdentityService) GetWebSession(user, sid string) (services.WebSession, error) {
	session, err := getWebSession(context.TODO(), s.Backend, user, sid, webSessionKey(sid))
	if err == nil {
		return session, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return getWebSession(context.TODO(), s.Backend, user, sid, legacyWebSessionKey(user, sid))
}

// UpsertWebSession updates or inserts a web session for a user and session id
// the session will be created with bearer token expiry time TTL, because
// it is expected to be extended by the client before then.
func (s *IdentityService) UpsertWebSession(user, sid string, session services.WebSession) error {
	session.SetUser(user)
	session.SetName(sid)
	value, err := services.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:     backend.Key(webPrefix, sessionsPrefix, sid),
		Value:   value,
		Expires: backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
	}
	_, err = s.Put(context.TODO(), item)
	return trace.Wrap(err)
}

// DeleteWebSession deletes web session from the storage.
func (s *IdentityService) DeleteWebSession(user, sid string) error {
	if user == "" {
		return trace.BadParameter("missing username")
	}
	if sid == "" {
		return trace.BadParameter("missing session id")
	}
	err := s.Delete(context.TODO(), backend.Key(webPrefix, sessionsPrefix, sid))
	return trace.Wrap(err)
}

// WebSessions returns the web sessions manager.
func (s *IdentityService) WebSessions() services.WebSessionInterface {
	return &webSessions{identity: s}
}

// Get returns the web session state described with req.
func (r *webSessions) Get(ctx context.Context, req services.GetWebSessionRequest) (services.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := getWebSession(ctx, r.identity.Backend, req.User, req.SessionID, webSessionKey(req.SessionID))
	if err == nil {
		return session, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return getWebSession(ctx, r.identity.Backend, req.User, req.SessionID,
		legacyWebSessionKey(req.User, req.SessionID))
}

// List gets all regular web sessions.
func (r *webSessions) List(ctx context.Context) ([]services.WebSession, error) {
	startKey := backend.Key(webPrefix, sessionsPrefix)
	result, err := r.identity.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
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
	// DELETE in 6.x: return web sessions from a legacy path under /web/users/<user>/sessions/<id>
	legacySessions, err := r.listLegacySessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(out, legacySessions...), nil
}

// Upsert updates the existing or inserts a new web session.
// Session will be created with bearer token expiry time TTL, because
// it is expected that client will periodically update it
func (r *webSessions) Upsert(ctx context.Context, session services.WebSession) error {
	value, err := services.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:     webSessionKey(session.GetName()),
		Value:   value,
		Expires: backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
	}
	_, err = r.identity.Put(ctx, item)
	return trace.Wrap(err)
}

// Delete deletes the web session specified with req from the storage.
func (r *webSessions) Delete(ctx context.Context, req services.DeleteWebSessionRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.identity.Delete(ctx, webSessionKey(req.SessionID)))
}

// DeleteAll removes all regular web sessions.
func (r *webSessions) DeleteAll(ctx context.Context) error {
	startKey := backend.Key(webPrefix, sessionsPrefix)
	if err := r.identity.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getWebSession(ctx context.Context, backend backend.Backend, user, sessionID string, key []byte) (services.WebSession, error) {
	item, err := backend.Get(ctx, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// DELETE in 6.x.
	// this is for backwards compatibility to ensure we
	// always have these values
	session.SetUser(user)
	session.SetName(sessionID)
	return session, nil
}

// DELETE in 6.x
func (r *webSessions) listLegacySessions(ctx context.Context) ([]services.WebSession, error) {
	startKey := backend.Key(webPrefix, usersPrefix)
	result, err := r.identity.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.WebSession, 0, len(result.Items))
	for _, item := range result.Items {
		suffix, _, err := baseTwoKeys(item.Key)
		if err != nil && trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if suffix != sessionsPrefix {
			continue
		}
		session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(item.Value, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, session)
	}
	return out, nil
}

type webSessions struct {
	identity *IdentityService
}

func webSessionKey(sessionID string) (key []byte) {
	return backend.Key(webPrefix, sessionsPrefix, sessionID)
}

func legacyWebSessionKey(user, sessionID string) (key []byte) {
	return backend.Key(webPrefix, usersPrefix, user, sessionsPrefix, sessionID)
}
