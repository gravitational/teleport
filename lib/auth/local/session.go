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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GetAppSession gets an application web session.
func (s *IdentityService) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.Get(ctx, backend.Key(appsPrefix, sessionsPrefix, req.SessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// GetAppSessions gets all application web sessions.
func (s *IdentityService) GetAppSessions(ctx context.Context) ([]types.WebSession, error) {
	startKey := backend.Key(appsPrefix, sessionsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]types.WebSession, len(result.Items))
	for i, item := range result.Items {
		session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = session
	}
	return out, nil
}

// UpsertAppSession creates an application web session.
func (s *IdentityService) UpsertAppSession(ctx context.Context, session services.WebSession) error {
	value, err := resource.MarshalWebSession(session)
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

// WebSessions returns the web sessions manager.
func (s *IdentityService) WebSessions() WebSessions {
	return &webSessions{backend: s.Backend, log: s.log}
}

// Get returns the web session state described with req.
func (r *webSessions) Get(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := r.backend.Get(ctx, webSessionKey(req.SessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if session != nil {
		return session, nil
	}
	// DELETE IN 7.x:
	// Return web sessions from a legacy path under /web/users/<user>/sessions/<id>
	return getLegacyWebSession(ctx, r.backend, req.User, req.SessionID)
}

// List gets all regular web sessions.
func (r *webSessions) List(ctx context.Context) (out []types.WebSession, err error) {
	key := backend.Key(webPrefix, sessionsPrefix)
	result, err := r.backend.GetRange(ctx, key, backend.RangeEnd(key), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range result.Items {
		session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, session)
	}
	// DELETE IN 7.x:
	// Return web sessions from a legacy path under /web/users/<user>/sessions/<id>
	legacySessions, err := r.listLegacySessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(out, legacySessions...), nil
}

// Upsert updates the existing or inserts a new web session.
func (r *webSessions) Upsert(ctx context.Context, session types.WebSession) error {
	value, err := resource.MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:     webSessionKey(session.GetName()),
		Value:   value,
		Expires: backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
	}
	_, err = r.backend.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Delete deletes the web session specified with req from the storage.
func (r *webSessions) Delete(ctx context.Context, req types.DeleteWebSessionRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.backend.Delete(ctx, webSessionKey(req.SessionID)))
}

// DeleteAll removes all regular web sessions.
func (r *webSessions) DeleteAll(ctx context.Context) error {
	startKey := backend.Key(webPrefix, sessionsPrefix)
	return trace.Wrap(r.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// DELETE IN 7.x.
// listLegacySessions lists web sessions under a legacy path /web/users/<user>/sessions/<id>
func (r *webSessions) listLegacySessions(ctx context.Context) ([]types.WebSession, error) {
	startKey := backend.Key(webPrefix, usersPrefix)
	result, err := r.backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.WebSession, 0, len(result.Items))
	for _, item := range result.Items {
		suffix, _, err := baseTwoKeys(item.Key)
		if err != nil && trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if suffix != sessionsPrefix {
			continue
		}
		session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, session)
	}
	return out, nil
}

type webSessions struct {
	backend backend.Backend
	log     logrus.FieldLogger
}

// WebTokens returns the web token manager.
func (s *IdentityService) WebTokens() WebTokens {
	return &webTokens{backend: s.Backend, log: s.log}
}

// Get returns the web token described with req.
func (r *webTokens) Get(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := r.backend.Get(ctx, webTokenKey(req.Token))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := resource.UnmarshalWebToken(item.Value, resource.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// List gets all web tokens.
func (r *webTokens) List(ctx context.Context) (out []types.WebToken, err error) {
	key := backend.Key(webPrefix, tokensPrefix)
	result, err := r.backend.GetRange(ctx, key, backend.RangeEnd(key), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range result.Items {
		token, err := resource.UnmarshalWebToken(item.Value, resource.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, token)
	}
	return out, nil
}

// Upsert updates the existing or inserts a new web token.
func (r *webTokens) Upsert(ctx context.Context, token types.WebToken) error {
	bytes, err := resource.MarshalWebToken(token, resource.WithVersion(services.V3))
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := token.GetMetadata()
	item := backend.Item{
		Key:     webTokenKey(token.GetToken()),
		Value:   bytes,
		Expires: metadata.Expiry(),
	}
	_, err = r.backend.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Delete deletes the web token specified with req from the storage.
func (r *webTokens) Delete(ctx context.Context, req types.DeleteWebTokenRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.backend.Delete(ctx, webTokenKey(req.Token)))
}

// DeleteAll removes all web tokens.
func (r *webTokens) DeleteAll(ctx context.Context) error {
	startKey := backend.Key(webPrefix, tokensPrefix)
	if err := r.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type webTokens struct {
	backend backend.Backend
	log     logrus.FieldLogger
}

// DELETE in 7.x.
// getLegacySession returns the web session for the specified user/sessionID
// under a legacy path /web/users/<user>/sessions/<id>
func getLegacyWebSession(ctx context.Context, backend backend.Backend, user, sessionID string) (types.WebSession, error) {
	item, err := backend.Get(ctx, legacyWebSessionKey(user, sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := resource.UnmarshalWebSession(item.Value, resource.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is for backwards compatibility to ensure we
	// always have these values
	session.SetUser(user)
	session.SetName(sessionID)
	return session, nil
}

func webSessionKey(sessionID string) (key []byte) {
	return backend.Key(webPrefix, sessionsPrefix, sessionID)
}

func webTokenKey(token string) (key []byte) {
	return backend.Key(webPrefix, tokensPrefix, token)
}

func legacyWebSessionKey(user, sessionID string) (key []byte) {
	return backend.Key(webPrefix, usersPrefix, user, sessionsPrefix, sessionID)
}
