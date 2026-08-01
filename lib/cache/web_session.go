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
	"log/slog"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	sortcache "github.com/gravitational/teleport/lib/utils/sortcache/v2"
)

type webSessionIndex string

const webSessionNameIndex webSessionIndex = "name"

func newWebSessionCollection(upstream types.WebSessionInterface, w types.WatchKind) (*collection[types.WebSession, webSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WebSession")
	}

	return &collection[types.WebSession, webSessionIndex]{
		store: newStore(
			types.KindWebSession,
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

// webSessionCollection provides read access to cached regular web sessions.
// Its exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type webSessionCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	logger   *slog.Logger
	upstream types.WebSessionInterface
	col      *collection[types.WebSession, webSessionIndex]
}

// GetWebSession gets a regular web session.
func (c webSessionCollection) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWebSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, webSessionIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      webSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.upstream.Get(ctx, types.GetWebSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.upstream.Get(ctx, req); err == nil {
			c.logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
	}
	return out, trace.Wrap(err)
}

// GetWebSession gets a regular web session.
func (c *Cache) GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error) {
	return webSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.WebSession,
		col:      c.collections.webSessions,
	}.GetWebSession(ctx, req)
}

// appSessionIndexes is the index-handle set of the app session collection.
// The user index is a secondary index: its compound key (user, session id)
// is compared component-wise, so a user name containing a "/" can no longer
// collide with or leak into another user's range.
type appSessionIndexes struct {
	name *sortcache.Index[types.WebSession, string]
	user *sortcache.Index2[types.WebSession, string, string]
}

func newAppSessionCollection(upstream services.AppSessionReader, w types.WatchKind) (*typedCollection[types.WebSession, appSessionIndexes], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AppSession")
	}

	indexes := appSessionIndexes{
		name: sortcache.NewIndex("name", types.WebSession.GetName),
	}
	indexes.user = sortcache.NewSecondaryIndex("user", indexes.name, types.WebSession.GetUser)

	return &typedCollection[types.WebSession, appSessionIndexes]{
		// the primary (name) index is registered first: delete events carry
		// header-only values whose user field is empty, and deletes resolve
		// via the first index that matches.
		store: newTypedStore(types.KindAppSession, types.WebSession.Copy,
			indexes, indexes.name, indexes.user),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			out, err := stream.Collect(
				stream.FilterMap(
					clientutils.Resources(ctx,
						func(ctx context.Context, size int, startKey string) ([]types.WebSession, string, error) {
							return upstream.ListAppSessions(ctx, size, startKey, "")
						}),
					func(ws types.WebSession) (types.WebSession, bool) {
						if !loadSecrets {
							return ws.WithoutSecrets(), true
						}

						return ws, true
					}),
			)
			return out, trace.Wrap(err)
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

// appSessionCollection provides read access to cached application web
// sessions. Its exported methods are promoted onto every topology cache that
// embeds it; the reads are implemented exactly once here. It is a stateless
// value assembled inline by each of its consumers so that no shared
// scaffolding couples their lifetimes.
type appSessionCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	logger   *slog.Logger
	upstream services.AppSessionReader
	col      *typedCollection[types.WebSession, appSessionIndexes]
}

// GetAppSession gets an application web session.
func (c appSessionCollection) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAppSession")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		session, err := c.upstream.GetAppSession(ctx, req)
		return session, trace.Wrap(err)
	}

	session, err := getByIndex(rg, rg.store.indexes.name, req.SessionID)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.upstream.GetAppSession(ctx, req); err == nil {
			c.logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
		return nil, trace.Wrap(err)
	}

	return session.Copy(), nil
}

// GetAppSession gets an application web session.
func (c *Cache) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	return appSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.AppSession,
		col:      c.collections.appSessions,
	}.GetAppSession(ctx, req)
}

// ListAppSessions returns a page of application web sessions.
func (c appSessionCollection) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListAppSessions")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		out, next, err := c.upstream.ListAppSessions(ctx, pageSize, pageToken, user)
		return out, next, trace.Wrap(err)
	}

	// Adjust page size, so it can't be too large.
	const maxSessionPageSize = 200
	if pageSize <= 0 || pageSize > maxSessionPageSize {
		pageSize = maxSessionPageSize
	}

	// the page token is the session id at which the next page starts,
	// matching the upstream implementation's token format.
	var sessions iter.Seq[types.WebSession]
	if user == "" {
		sessions = rg.store.resources(rg.store.indexes.name.AscendFrom(rg.snapshot, pageToken))
	} else {
		sessions = rg.store.resources(rg.store.indexes.user.AscendPrefixFrom(rg.snapshot, user, pageToken))
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

// ListAppSessions returns a page of application web sessions.
func (c *Cache) ListAppSessions(ctx context.Context, pageSize int, pageToken, user string) ([]types.WebSession, string, error) {
	return appSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.AppSession,
		col:      c.collections.appSessions,
	}.ListAppSessions(ctx, pageSize, pageToken, user)
}

type snowflakeSessionIndex string

const snowflakeSessionNameIndex snowflakeSessionIndex = "name"

func newSnowflakeSessionCollection(upstream services.SnowflakeSession, w types.WatchKind) (*collection[types.WebSession, snowflakeSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter upstream")
	}

	return &collection[types.WebSession, snowflakeSessionIndex]{
		store: newStore(
			types.KindSnowflakeSession,
			types.WebSession.Copy,
			map[snowflakeSessionIndex]func(types.WebSession) string{
				snowflakeSessionNameIndex: types.WebSession.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			// TODO(okraport): DELETE IN v21.0.0, replace with regular collect
			webSessions, err := clientutils.CollectWithFallback(ctx, upstream.ListSnowflakeSessions, upstream.GetSnowflakeSessions)
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
	return snowflakeSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.SnowflakeSession,
		col:      c.collections.snowflakeSessions,
	}.GetSnowflakeSession(ctx, req)
}

// snowflakeSessionCollection provides read access to cached Snowflake web
// sessions. Its exported methods are promoted onto every topology cache that
// embeds it; the reads are implemented exactly once here. It is a stateless
// value assembled inline by each of its consumers so that no shared
// scaffolding couples their lifetimes.
type snowflakeSessionCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	logger   *slog.Logger
	upstream services.SnowflakeSession
	col      *collection[types.WebSession, snowflakeSessionIndex]
}

// GetSnowflakeSession gets Snowflake web session.
func (c snowflakeSessionCollection) GetSnowflakeSession(ctx context.Context, req types.GetSnowflakeSessionRequest) (types.WebSession, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetSnowflakeSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, snowflakeSessionIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      snowflakeSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.upstream.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if sess, err := c.upstream.GetSnowflakeSession(ctx, req); err == nil {
			c.logger.DebugContext(ctx, "Cache was forced to load session from upstream",
				"session_kind", sess.GetSubKind(),
				"session", sess.GetName(),
			)
			return sess, nil
		}
	}
	return out, trace.Wrap(err)
}

// RangeSnowflakeSessions returns Snowflake session resources within the range [start, end).
func (c snowflakeSessionCollection) RangeSnowflakeSessions(ctx context.Context, start, end string) iter.Seq2[types.WebSession, error] {
	lister := genericLister[types.WebSession, snowflakeSessionIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        snowflakeSessionNameIndex,
		upstreamList: c.upstream.ListSnowflakeSessions,
		nextToken:    types.WebSession.GetName,
		// TODO(lokraszewski): DELETE IN v21.0.0
		fallbackGetter: c.upstream.GetSnowflakeSessions,
	}

	return func(yield func(types.WebSession, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/RangeSnowflakeSessions")
		defer span.End()

		for db, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(db, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}

// RangeSnowflakeSessions returns Snowflake session resources within the range [start, end).
func (c *Cache) RangeSnowflakeSessions(ctx context.Context, start, end string) iter.Seq2[types.WebSession, error] {
	return snowflakeSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.SnowflakeSession,
		col:      c.collections.snowflakeSessions,
	}.RangeSnowflakeSessions(ctx, start, end)
}

// ListSnowflakeSessions returns a page of Snowflake session resources.
func (c snowflakeSessionCollection) ListSnowflakeSessions(ctx context.Context, limit int, startKey string) ([]types.WebSession, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListSnowflakeSessions")
	defer span.End()

	lister := genericLister[types.WebSession, snowflakeSessionIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        snowflakeSessionNameIndex,
		upstreamList: c.upstream.ListSnowflakeSessions,
		nextToken: func(a types.WebSession) string {
			return a.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}

// ListSnowflakeSessions returns a page of Snowflake session resources.
func (c *Cache) ListSnowflakeSessions(ctx context.Context, limit int, startKey string) ([]types.WebSession, string, error) {
	return snowflakeSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.SnowflakeSession,
		col:      c.collections.snowflakeSessions,
	}.ListSnowflakeSessions(ctx, limit, startKey)
}

// GetSnowflakeSessions returns all Snowflake session resources.
func (c snowflakeSessionCollection) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetSnowflakeSessions")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		sessions, err := c.upstream.GetSnowflakeSessions(ctx)
		return sessions, trace.Wrap(err)
	}

	out := make([]types.WebSession, 0, rg.store.len())
	for a := range rg.store.resources(snowflakeSessionNameIndex, "", "") {
		out = append(out, a.Copy())
	}

	return out, nil
}

// GetSnowflakeSessions returns all Snowflake session resources.
func (c *Cache) GetSnowflakeSessions(ctx context.Context) ([]types.WebSession, error) {
	return snowflakeSessionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.SnowflakeSession,
		col:      c.collections.snowflakeSessions,
	}.GetSnowflakeSessions(ctx)
}
