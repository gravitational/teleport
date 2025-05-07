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
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAppSessions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for i := 0; i < 31; i++ {
		err := p.appSessionS.UpsertAppSession(t.Context(), &types.WebSessionV2{
			Kind:    types.KindWebSession,
			SubKind: types.KindAppSession,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "app-session" + strconv.Itoa(i+1),
			},
			Spec: types.WebSessionSpecV2{
				User: "fish",
			},
		})
		require.NoError(t, err)
	}

	for i := 0; i < 3; i++ {
		err := p.appSessionS.UpsertAppSession(t.Context(), &types.WebSessionV2{
			Kind:    types.KindWebSession,
			SubKind: types.KindAppSession,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "app-session" + strconv.Itoa(i+100),
			},
			Spec: types.WebSessionSpecV2{
				User: "llama",
			},
		})
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		expected, next, err := p.appSessionS.ListAppSessions(ctx, 0, "", "")
		assert.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, expected, 34)

		cached, next, err := p.cache.ListAppSessions(ctx, 0, "", "")
		assert.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, cached, 34)
	}, 15*time.Second, 100*time.Millisecond)

	session, err := p.cache.GetAppSession(ctx, types.GetAppSessionRequest{
		SessionID: "app-session100",
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, "llama", session.GetUser())

	session, err = p.cache.GetAppSession(ctx, types.GetAppSessionRequest{
		SessionID: "app-session1",
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, "fish", session.GetUser())

	var sessions []types.WebSession
	for pageToken := ""; ; {
		cached, next, err := p.cache.ListAppSessions(ctx, 1, pageToken, "llama")
		if !assert.NoError(t, err) {
			return
		}
		sessions = append(sessions, cached...)
		pageToken = next
		if next == "" {
			break
		}
	}
	assert.Len(t, sessions, 3)

	sessions = nil
	for pageToken := ""; ; {
		cached, next, err := p.cache.ListAppSessions(ctx, 7, pageToken, "fish")
		if !assert.NoError(t, err) {
			return
		}
		sessions = append(sessions, cached...)
		pageToken = next
		if next == "" {
			break
		}
	}
	assert.Len(t, sessions, 31)

	require.NoError(t, p.appSessionS.DeleteAllAppSessions(ctx))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		cached, next, err := p.cache.ListAppSessions(ctx, 0, "", "")
		assert.NoError(t, err)
		assert.Empty(t, next)
		assert.Empty(t, cached)
	}, 15*time.Second, 100*time.Millisecond)
}

func TestWebSessions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for i := 0; i < 31; i++ {
		err := p.webSessionS.Upsert(t.Context(), &types.WebSessionV2{
			Kind:    types.KindWebSession,
			SubKind: types.KindWebSession,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "web-session" + strconv.Itoa(i+1),
			},
			Spec: types.WebSessionSpecV2{
				User: "fish",
			},
		})
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		expected, err := p.webSessionS.List(ctx)
		assert.NoError(t, err)
		assert.Len(t, expected, 31)

		for _, session := range expected {
			cached, err := p.cache.GetWebSession(ctx, types.GetWebSessionRequest{SessionID: session.GetName()})
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(session, cached))
		}
	}, 15*time.Second, 100*time.Millisecond)

	require.NoError(t, p.webSessionS.DeleteAll(ctx))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for i := 0; i < 31; i++ {
			session, err := p.cache.GetWebSession(ctx, types.GetWebSessionRequest{SessionID: "web-session" + strconv.Itoa(i+1)})
			assert.Error(t, err)
			assert.Nil(t, session)
		}
	}, 15*time.Second, 100*time.Millisecond)
}

func TestSnowflakeSessions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for i := 0; i < 31; i++ {
		err := p.snowflakeSessionS.UpsertSnowflakeSession(t.Context(), &types.WebSessionV2{
			Kind:    types.KindWebSession,
			SubKind: types.KindSnowflakeSession,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "snow-session" + strconv.Itoa(i+1),
			},
			Spec: types.WebSessionSpecV2{
				User: "fish",
			},
		})
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		expected, err := p.snowflakeSessionS.GetSnowflakeSessions(ctx)
		assert.NoError(t, err)
		assert.Len(t, expected, 31)

		for _, session := range expected {
			cached, err := p.cache.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: session.GetName()})
			assert.NoError(t, err)
			assert.Empty(t, cmp.Diff(session, cached))
		}
	}, 15*time.Second, 100*time.Millisecond)

	require.NoError(t, p.snowflakeSessionS.DeleteAllSnowflakeSessions(ctx))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for i := 0; i < 31; i++ {
			session, err := p.cache.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: "snow-session" + strconv.Itoa(i+1)})
			assert.Error(t, err)
			assert.Nil(t, session)
		}
	}, 15*time.Second, 100*time.Millisecond)
}
