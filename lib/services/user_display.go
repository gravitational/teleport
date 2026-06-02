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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ResolveUserDisplays resolves usernames to display values, keyed by username.
// It dedupes the input and does at most one read per unique name through getter,
// deriving each value from types.User.GetDisplay. It inherits the getter's RBAC
// scope and does no authorization of its own.
//
// A username with no matching user is absent from the result (the "gone" signal
// for an expired or logged-out SSO user). A user that exists but has no distinct
// display value is present with a zero-value types.UserDisplay. Empty and
// whitespace-only usernames are skipped without a lookup.
//
// Only NotFound is swallowed. Any other error aborts resolution and is returned
// wrapped with the problematic username. No partial map is returned.
func ResolveUserDisplays(ctx context.Context, getter UserGetter, usernames []string) (map[string]types.UserDisplay, error) {
	displays := make(map[string]types.UserDisplay)
	seen := make(map[string]struct{})

	for _, username := range usernames {
		if strings.TrimSpace(username) == "" {
			continue // skipping whitespace-only username
		}

		if _, ok := seen[username]; ok {
			continue // skipping duplicate username
		}
		seen[username] = struct{}{}

		user, err := getter.GetUser(ctx, username, false)
		if trace.IsNotFound(err) {
			continue // skipping missing user
		}
		if err != nil {
			return nil, trace.Wrap(err, "resolving display for user %q", username)
		}
		displays[username] = user.GetDisplay()
	}

	return displays, nil
}
