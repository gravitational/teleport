// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type webSessionIndex string

const webSessionNameIndex webSessionIndex = "name"

func newWebSessionCollection(upstream types.WebSessionInterface, w types.WatchKind) (*collection[types.WebSession, webSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WebSession")
	}

	return &collection[types.WebSession, webSessionIndex]{
		store: newStore(
			types.WebSession.Copy,
			map[webSessionIndex]func(types.WebSession) string{
				webSessionNameIndex: types.WebSession.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			webSessions, err := upstream.List(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if !loadSecrets {
				for i := range webSessions {
					webSessions[i] = webSessions[i].WithoutSecrets()
				}
			}

			return webSessions, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WebSession {
			return &types.WebSessionV2{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetWebSession gets a regular web session.
func (c *Cache) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWebSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, webSessionIndex]{
		cache:      c,
		collection: c.collections.webSessions,
		index:      webSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.Config.WebSession.Get(ctx, types.GetWebSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.WebSession.Get(ctx, req); err == nil {
			c.Logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
	}
	return out, trace.Wrap(err)
}

type appSessionIndex string

const (
	appSessionNameIndex appSessionIndex = "name"
	appSessionUserIndex appSessionIndex = "user"
)

func newAppSessionCollection(upstream services.AppSession, w types.WatchKind) (*collection[types.WebSession, appSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AppSession")
	}

	return &collection[types.WebSession, appSessionIndex]{
		store: newStore(
			types.WebSession.Copy,
			map[appSessionIndex]func(types.WebSession) string{
				appSessionNameIndex: types.WebSession.GetName,
				appSessionUserIndex: func(r types.WebSession) string {
					return r.GetUser() + "/" + r.GetMetadata().Name
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			var startKey string
			var sessions []types.WebSession

			for {
				webSessions, nextKey, err := upstream.ListAppSessions(ctx, 0, startKey, "")
				if err != nil {
					return nil, trace.Wrap(err)
				}

				if !loadSecrets {
					for i := range webSessions {
						webSessions[i] = webSessions[i].WithoutSecrets()
					}
				}

				sessions = append(sessions, webSessions...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}
			return sessions, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WebSession {
			return &types.WebSessionV2{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetAppSession gets an application web session.
func (c *Cache) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAppSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, appSessionIndex]{
		cache:      c,
		collection: c.collections.appSessions,
		index:      appSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.Config.AppSession.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.AppSession.GetAppSession(ctx, req); err == nil {
			c.Logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
	}
	return out, trace.Wrap(err)
}

// ListAppSessions returns a page of application web sessions.
func (c *Cache) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAppSessions")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.appSessions)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := c.Config.AppSession.ListAppSessions(ctx, pageSize, pageToken, user)
		return out, next, trace.Wrap(err)
	}

	// Adjust page size, so it can't be too large.
	const maxSessionPageSize = 200
	if pageSize <= 0 || pageSize > maxSessionPageSize {
		pageSize = maxSessionPageSize
	}

	var sessions iter.Seq[types.WebSession]
	if user == "" {
		sessions = rg.store.resources(appSessionNameIndex, pageToken, "")
	} else {
		startKey := user + "/"
		endKey := sortcache.NextKey(startKey)
		if pageToken != "" {
			startKey += pageToken
		}

		sessions = rg.store.resources(appSessionUserIndex, startKey, endKey)
	}

	var out []types.WebSession
	for sess := range sessions {
		if len(out) == pageSize {
			return out, sess.GetName(), nil
		}

		out = append(out, sess.Copy())
	}

	return out, "", nil
}

type snowflakeSessionIndex string

const snowflakeSessionNameIndex snowflakeSessionIndex = "name"

func newSnowflakeSessionCollection(upstream services.SnowflakeSession, w types.WatchKind) (*collection[types.WebSession, snowflakeSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AppSession")
	}

	return &collection[types.WebSession, snowflakeSessionIndex]{
		store: newStore(
			types.WebSession.Copy,
			map[snowflakeSessionIndex]func(types.WebSession) string{
				snowflakeSessionNameIndex: types.WebSession.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			webSessions, err := upstream.GetSnowflakeSessions(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if !loadSecrets {
				for i := range webSessions {
					webSessions[i] = webSessions[i].WithoutSecrets()
				}
			}

			return webSessions, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WebSession {
			return &types.WebSessionV2{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetSnowflakeSession gets Snowflake web session.
func (c *Cache) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSnowflakeSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, snowflakeSessionIndex]{
		cache:      c,
		collection: c.collections.snowflakeSessions,
		index:      snowflakeSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.Config.SnowflakeSession.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.Config.SnowflakeSession.GetSnowflakeSession(ctx, req); err == nil {
			c.Logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
	}
	return out, trace.Wrap(err)
}
