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

package local

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// GetAppSession gets an application web session.
func (s *IdentityService) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.getSession(ctx, appsPrefix, sessionsPrefix, req.SessionID)
}

// GetSnowflakeSession gets an application web session.
func (s *IdentityService) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.getSession(ctx, snowflakePrefix, sessionsPrefix, req.SessionID)
}

// GetSAMLIdPSession gets a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) GetSAMLIdPSession(ctx context.Context, req types.GetSAMLIdPSessionRequest) (types.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.getSession(ctx, samlIdPPrefix, sessionsPrefix, req.SessionID)
}

func (s *IdentityService) getSession(ctx context.Context, keyParts ...string) (types.WebSession, error) {
	item, err := s.Get(ctx, backend.NewKey(keyParts...))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// maxSessionPageSize is the maximum number of app sessions allowed in a page
// returned by ListAppSessions
const maxSessionPageSize = 200

// ListAppSessions gets a paginated list of application web sessions.
func (s *IdentityService) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	return s.listSessions(ctx, pageSize, pageToken, user, appsPrefix, sessionsPrefix)
}

// GetSnowflakeSessions gets all Snowflake web sessions.
func (s *IdentityService) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	startKey := backend.ExactKey(snowflakePrefix, sessionsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]types.WebSession, len(result.Items))
	for i, item := range result.Items {
		session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = session
	}
	return out, nil
}

// ListSAMLIdPSessions gets a paginated list of SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) ListSAMLIdPSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	return s.listSessions(ctx, pageSize, pageToken, user, samlIdPPrefix, sessionsPrefix)
}

// listSessions gets a paginated list of sessions.
func (s *IdentityService) listSessions(ctx context.Context, pageSize int, pageToken, user string, keyPrefix ...string) ([]types.WebSession, string, error) {
	rangeStart := backend.NewKey(append(keyPrefix, pageToken)...)
	rangeEnd := backend.RangeEnd(backend.ExactKey(keyPrefix...))

	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > maxSessionPageSize {
		pageSize = maxSessionPageSize
	}

	// Increment pageSize to allow for the extra item represented by nextKey.
	// We skip this item in the results below.
	limit := pageSize + 1
	var out []types.WebSession

	if user == "" {
		// no filter provided get the range directly
		result, err := s.GetRange(ctx, rangeStart, rangeEnd, limit)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		out = make([]types.WebSession, 0, len(result.Items))
		for _, item := range result.Items {
			session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			out = append(out, session)
		}
	} else {
		// iterate over the sessions to filter only those matching the provided user
		if err := backend.IterateRange(ctx, s.Backend, rangeStart, rangeEnd, limit, func(items []backend.Item) (stop bool, err error) {
			for _, item := range items {
				if len(out) == limit {
					break
				}

				session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
				if err != nil {
					return false, trace.Wrap(err)
				}

				if session.GetUser() == user {
					out = append(out, session)
				}
			}

			return len(out) == limit, nil
		}); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	var nextKey string
	if len(out) > pageSize {
		nextKey = backend.GetPaginationKey(out[len(out)-1])
		// Truncate the last item that was used to determine next row existence.
		out = out[:pageSize]
	}

	return out, nextKey, nil
}

// UpsertAppSession creates an application web session.
func (s *IdentityService) UpsertAppSession(ctx context.Context, session types.WebSession) error {
	return s.upsertSession(ctx, session, appsPrefix, sessionsPrefix)
}

// UpsertSnowflakeSession creates a Snowflake web session.
func (s *IdentityService) UpsertSnowflakeSession(ctx context.Context, session types.WebSession) error {
	return s.upsertSession(ctx, session, snowflakePrefix, sessionsPrefix)
}

// UpsertSAMLIdPSession creates a SAMLIdP web session.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) UpsertSAMLIdPSession(ctx context.Context, session types.WebSession) error {
	return s.upsertSession(ctx, session, samlIdPPrefix, sessionsPrefix)
}

// upsertSession creates a web session.
func (s *IdentityService) upsertSession(ctx context.Context, session types.WebSession, keyPrefix ...string) error {
	rev := session.GetRevision()
	value, err := services.MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:      backend.NewKey(append(keyPrefix, session.GetName())...),
		Value:    value,
		Expires:  session.GetExpiryTime(),
		Revision: rev,
	}

	if _, err = s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAppSession removes an application web session.
func (s *IdentityService) DeleteAppSession(ctx context.Context, req types.DeleteAppSessionRequest) error {
	if err := s.Delete(ctx, backend.NewKey(appsPrefix, sessionsPrefix, req.SessionID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSnowflakeSession removes a Snowflake web session.
func (s *IdentityService) DeleteSnowflakeSession(ctx context.Context, req types.DeleteSnowflakeSessionRequest) error {
	if err := s.Delete(ctx, backend.NewKey(snowflakePrefix, sessionsPrefix, req.SessionID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteSAMLIdPSession removes a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) DeleteSAMLIdPSession(ctx context.Context, req types.DeleteSAMLIdPSessionRequest) error {
	if err := s.Delete(ctx, backend.NewKey(samlIdPPrefix, sessionsPrefix, req.SessionID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteUserAppSessions removes all application web sessions for a particular user.
func (s *IdentityService) DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) error {
	var token string

	for {
		sessions, nextToken, err := s.ListAppSessions(ctx, maxSessionPageSize, token, req.Username)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, session := range sessions {
			err := s.DeleteAppSession(ctx, types.DeleteAppSessionRequest{SessionID: session.GetName()})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if nextToken == "" {
			break
		}

		token = nextToken
	}

	return nil
}

// DeleteUserSAMLIdPSessions removes all SAML IdP sessions for a particular user.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) DeleteUserSAMLIdPSessions(ctx context.Context, user string) error {
	var token string

	for {
		sessions, nextToken, err := s.ListSAMLIdPSessions(ctx, maxSessionPageSize, token, user)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, session := range sessions {
			err := s.DeleteSAMLIdPSession(ctx, types.DeleteSAMLIdPSessionRequest{SessionID: session.GetName()})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if nextToken == "" {
			break
		}

		token = nextToken
	}

	return nil
}

// DeleteAllAppSessions removes all application web sessions.
func (s *IdentityService) DeleteAllAppSessions(ctx context.Context) error {
	startKey := backend.ExactKey(appsPrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllSnowflakeSessions removes all Snowflake web sessions.
func (s *IdentityService) DeleteAllSnowflakeSessions(ctx context.Context) error {
	startKey := backend.ExactKey(snowflakePrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllSAMLIdPSessions removes all SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (s *IdentityService) DeleteAllSAMLIdPSessions(ctx context.Context) error {
	startKey := backend.ExactKey(samlIdPPrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// WebSessions returns the web sessions manager.
func (s *IdentityService) WebSessions() types.WebSessionInterface {
	return &webSessions{backend: s.Backend}
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
	session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// Make sure the requested user matches the session user.
	if req.User != session.GetUser() {
		return nil, trace.NotFound("session not found")
	}

	return session, trace.Wrap(err)
}

// List gets all regular web sessions.
func (r *webSessions) List(ctx context.Context) (out []types.WebSession, err error) {
	startKey := backend.ExactKey(webPrefix, sessionsPrefix)
	result, err := r.backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range result.Items {
		session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
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
	rev := session.GetRevision()
	value, err := services.MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	sessionMetadata := session.GetMetadata()
	item := backend.Item{
		Key:      webSessionKey(session.GetName()),
		Value:    value,
		Expires:  backend.EarliestExpiry(session.GetBearerTokenExpiryTime(), sessionMetadata.Expiry()),
		Revision: rev,
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
	startKey := backend.ExactKey(webPrefix, sessionsPrefix)
	return trace.Wrap(r.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)))
}

// DELETE IN 7.x.
// listLegacySessions lists web sessions under a legacy path /web/users/<user>/sessions/<id>
func (r *webSessions) listLegacySessions(ctx context.Context) ([]types.WebSession, error) {
	startKey := backend.ExactKey(webPrefix, usersPrefix)
	result, err := r.backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]types.WebSession, 0, len(result.Items))
	for _, item := range result.Items {
		idx := slices.Index(item.Key.Components(), sessionsPrefix)
		if idx != len(item.Key.Components())-2 {
			continue
		}
		session, err := services.UnmarshalWebSession(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, session)
	}
	return out, nil
}

type webSessions struct {
	backend backend.Backend
}

// WebTokens returns the web token manager.
func (s *IdentityService) WebTokens() types.WebTokenInterface {
	return &webTokens{backend: s.Backend}
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
	token, err := services.UnmarshalWebToken(item.Value, services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// List gets all web tokens.
func (r *webTokens) List(ctx context.Context) (out []types.WebToken, err error) {
	startKey := backend.ExactKey(webPrefix, tokensPrefix)
	result, err := r.backend.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range result.Items {
		token, err := services.UnmarshalWebToken(item.Value, services.WithRevision(item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, token)
	}
	return out, nil
}

// Upsert updates the existing or inserts a new web token.
func (r *webTokens) Upsert(ctx context.Context, token types.WebToken) error {
	rev := token.GetRevision()
	bytes, err := services.MarshalWebToken(token, services.WithVersion(types.V3))
	if err != nil {
		return trace.Wrap(err)
	}
	metadata := token.GetMetadata()
	item := backend.Item{
		Key:      webTokenKey(token.GetToken()),
		Value:    bytes,
		Expires:  metadata.Expiry(),
		Revision: rev,
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
	startKey := backend.ExactKey(webPrefix, tokensPrefix)
	if err := r.backend.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type webTokens struct {
	backend backend.Backend
}

func webSessionKey(sessionID string) backend.Key {
	return backend.NewKey(webPrefix, sessionsPrefix, sessionID)
}

func webTokenKey(token string) backend.Key {
	return backend.NewKey(webPrefix, tokensPrefix, token)
}
