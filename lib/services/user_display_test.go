/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// fakeUserGetter is a UserGetter backed by an in-memory map. It records lookup
// counts per username so tests can assert the dedupe contract.
type fakeUserGetter struct {
	users   map[string]types.User
	failFor map[string]error // username -> error returned instead of a lookup

	calls map[string]int // username -> GetUser call count
}

func (f *fakeUserGetter) GetUser(_ context.Context, name string, _ bool) (types.User, error) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[name]++
	if err, ok := f.failFor[name]; ok {
		return nil, err
	}
	user, ok := f.users[name]
	if !ok {
		return nil, trace.NotFound("user %q does not exist", name)
	}
	return user, nil
}

func newUserWithTraits(t *testing.T, name string, traits map[string][]string) types.User {
	t.Helper()
	user, err := types.NewUser(name)
	require.NoError(t, err)
	if traits != nil {
		user.SetTraits(traits)
	}
	return user
}

func TestResolveUserDisplays(t *testing.T) {
	t.Parallel()

	t.Run("dedupes and issues one lookup per unique username", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"alice": newUserWithTraits(t, "alice", nil),
			"bob":   newUserWithTraits(t, "bob", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "alice", "bob", "alice"})
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.Contains(t, out, "alice")
		require.Contains(t, out, "bob")
		require.Equal(t, 1, getter.calls["alice"])
		require.Equal(t, 1, getter.calls["bob"])
	})

	t.Run("returns the display value for a found user", func(t *testing.T) {
		t.Parallel()
		alice := newUserWithTraits(t, "alice", map[string][]string{
			"displayName": {"Alice Liddell"},
			"email":       {"alice@example.com"},
		})

		want := alice.GetDisplay()
		// Sanity-check that the chosen traits actually produce a display, so the
		// assertion below is meaningful.
		require.NotEqual(t, types.UserDisplay{}, want)

		getter := &fakeUserGetter{users: map[string]types.User{"alice": alice}}
		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice"})
		require.NoError(t, err)
		require.Equal(t, want, out["alice"])
	})

	t.Run("found user with no display is present with a zero value", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"plain": newUserWithTraits(t, "plain", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"plain"})
		require.NoError(t, err)

		// present with the zero value, not missing from the map
		require.Contains(t, out, "plain")
		require.Equal(t, types.UserDisplay{}, out["plain"])
	})

	t.Run("missing users are absent and do not fail resolution", func(t *testing.T) {
		t.Parallel()
		getter := &fakeUserGetter{users: map[string]types.User{
			"alice": newUserWithTraits(t, "alice", nil),
			"bob":   newUserWithTraits(t, "bob", nil),
		}}

		out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "ghost", "bob"})
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.Contains(t, out, "alice")
		require.Contains(t, out, "bob")
		require.NotContains(t, out, "ghost")
	})

	t.Run("aborts on non-NotFound errors without a partial map", func(t *testing.T) {
		t.Parallel()
		for _, errorCase := range []struct {
			name string
			err  error
		}{
			{"transient backend error", errors.New("backend timeout")},
			// A cancelled/expired context surfaces through the getter as a
			// non-NotFound error and must abort like any other.
			{"context cancellation", context.Canceled},
		} {
			t.Run(errorCase.name, func(t *testing.T) {
				t.Parallel()
				getter := &fakeUserGetter{
					users:   map[string]types.User{"alice": newUserWithTraits(t, "alice", nil)},
					failFor: map[string]error{"bob": errorCase.err},
				}

				out, err := ResolveUserDisplays(context.Background(), getter, []string{"alice", "bob", "carol"})
				require.Error(t, err)
				require.ErrorIs(t, err, errorCase.err)  // original error preserved
				require.Contains(t, err.Error(), "bob") // names the error user
				require.Nil(t, out)                     // no partial map handed back
			})
		}
	})
}
