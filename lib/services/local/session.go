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
	"iter"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
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
// Deprecated: Prefer paginated variant such as [IdentityService.ListSnowflakeSessions]
func (s *IdentityService) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	out, err := stream.Collect(s.rangeSessions(ctx, "", "", "", snowflakePrefix, sessionsPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// ListSnowflakeSessions gets a paginated list of Snowflake web sessions.
func (s *IdentityService) ListSnowflakeSessions(ctx context.Context, pageSize int, pageToken string) ([]types.WebSession, string, error) {
	return s.listSessions(ctx, pageSize, pageToken, "", snowflakePrefix, sessionsPrefix)
}

// RangeSnowflakeSessions returns Snowflake web sessions within the range [start, end).
func (s *IdentityService) RangeSnowflakeSessions(ctx context.Context, start, end string) iter.Seq2[types.WebSession, error] {
	return s.rangeSessions(ctx, start, end, "", snowflakePrefix, sessionsPrefix)
}

// listSessions gets a paginated list of sessions.
func (s *IdentityService) listSessions(ctx context.Context, pageSize int, pageToken, user string, keyPrefix ...string) ([]types.WebSession, string, error) {
	// Adjust page size, so it can't be too large.
	if pageSize <= 0 || pageSize > maxSessionPageSize {
		pageSize = maxSessionPageSize
	}

	return generic.CollectPageAndCursor(
		s.rangeSessions(ctx, pageToken, "", user, keyPrefix...),
		pageSize,
		types.WebSession.GetName,
	)
}

func (s *IdentityService) rangeSessions(ctx context.Context, start, end string, user string, keyPrefix ...string) iter.Seq2[types.WebSession, error] {
	mapFn := func(item backend.Item) (types.WebSession, bool) {
		// TODO(okraport): Do not unmarshal the expiry and instead rely on the unmarshalled fields.
		// This is because currently the backend expiry is the minima of Expires and BearerTokenExpires.
		// Address this and revisit unmarshal opts.
		session, err := services.UnmarshalWebSession(item.Value,
			services.WithRevision(item.Revision))
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to unmarshal web session",
				"key", item.Key,
				"error", err,
			)
			return nil, false
		}

		if user != "" && session.GetUser() != user {
			return session, false
		}

		return session, true
	}

	sessionKey := backend.NewKey(keyPrefix...)
	startKey := sessionKey.AppendKey(backend.KeyFromString(start))
	endKey := backend.RangeEnd(sessionKey)
	if end != "" {
		endKey = sessionKey.AppendKey(backend.KeyFromString(end)).ExactKey()
	}

	return stream.TakeWhile(
		stream.FilterMap(
			s.Backend.Items(ctx, backend.ItemsParams{
				StartKey: startKey,
				EndKey:   endKey,
			}),
			mapFn,
		),
		func(session types.WebSession) bool {
			// The range is not inclusive of the end key, so return early
			// if the end has been reached.
			return end == "" || session.GetName() < end
		})
}

// UpsertAppSession creates an application web session.
func (s *IdentityService) UpsertAppSession(ctx context.Context, session types.WebSession) error {
	return s.upsertSession(ctx, session, appsPrefix, sessionsPrefix)
}

// UpsertSnowflakeSession creates a Snowflake web session.
func (s *IdentityService) UpsertSnowflakeSession(ctx context.Context, session types.WebSession) error {
	return s.upsertSession(ctx, session, snowflakePrefix, sessionsPrefix)
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

// GetWebToken returns the web token described with req.
func (r *IdentityService) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := r.Get(ctx, webTokenKey(req.Token))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := services.UnmarshalWebToken(item.Value, services.WithRevision(item.Revision))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// GetWebTokens gets all web tokens.
// Deprecated: Prefer paginated variant such as [ListWebTokens] or [RangeWebTokens]
func (r *IdentityService) GetWebTokens(ctx context.Context) (out []types.WebToken, err error) {
	tokens, err := stream.Collect(r.RangeWebTokens(ctx, "", ""))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tokens, nil
}

// ListWebTokens returns a page of web tokens
func (r *IdentityService) ListWebTokens(ctx context.Context, limit int, start string) ([]types.WebToken, string, error) {
	return generic.CollectPageAndCursor(r.RangeWebTokens(ctx, start, ""), limit, types.WebToken.GetToken)
}

// RangeWebTokens returns web tokens within the range [start, end).
func (r *IdentityService) RangeWebTokens(ctx context.Context, start, end string) iter.Seq2[types.WebToken, error] {
	mapFn := func(item backend.Item) (types.WebToken, bool) {
		token, err := services.UnmarshalWebToken(item.Value,
			services.WithRevision(item.Revision))
		if err != nil {
			slog.WarnContext(ctx, "Failed to unmarshal web token",
				"key", item.Key,
				"error", err,
			)
			return nil, false
		}
		return token, true
	}

	tokenKey := backend.NewKey(webPrefix, tokensPrefix)
	startKey := tokenKey.AppendKey(backend.KeyFromString(start))
	endKey := backend.RangeEnd(tokenKey)
	if end != "" {
		endKey = tokenKey.AppendKey(backend.KeyFromString(end)).ExactKey()
	}

	return stream.TakeWhile(
		stream.FilterMap(
			r.Items(ctx, backend.ItemsParams{
				StartKey: startKey,
				EndKey:   endKey,
			}),
			mapFn,
		),
		func(token types.WebToken) bool {
			// The range is not inclusive of the end key, so return early
			// if the end has been reached.
			return end == "" || token.GetToken() < end
		})
}

// UpsertWebToken updates the existing or inserts a new web token.
func (r *IdentityService) UpsertWebToken(ctx context.Context, token types.WebToken) error {
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
	_, err = r.Put(ctx, item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteWebToken deletes the web token specified with req from the storage.
func (r *IdentityService) DeleteWebToken(ctx context.Context, req types.DeleteWebTokenRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.Delete(ctx, webTokenKey(req.Token)))
}

// DeleteAllWebTokens removes all web tokens.
func (r *IdentityService) DeleteAllWebTokens(ctx context.Context) error {
	startKey := backend.ExactKey(webPrefix, tokensPrefix)
	if err := r.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func webSessionKey(sessionID string) backend.Key {
	return backend.NewKey(webPrefix, sessionsPrefix, sessionID)
}

func webTokenKey(token string) backend.Key {
	return backend.NewKey(webPrefix, tokensPrefix, token)
}
