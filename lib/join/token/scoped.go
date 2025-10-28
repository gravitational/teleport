/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package token

import (
	"strings"
	"time"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
)

// Scoped wraps a [joiningv1.ScopedToken] such that it can be used to provision
// resources.
type Scoped struct {
	token      *joiningv1.ScopedToken
	joinMethod types.JoinMethod
	roles      types.SystemRoles
}

// NewScoped returns the wrapped version of the given [joiningv1.ScopedToken].
// It will return an error if the configured join method is not a valid
// [types.JoinMethod] or if any of the configured roles are not a valid
// [types.SystemRole]. The validated join method and roles are cached on the
// [Scoped] wrapper itself so they can be read without repeating validation.
func NewScoped(token *joiningv1.ScopedToken) (*Scoped, error) {
	joinMethod := types.JoinMethod(token.GetSpec().GetJoinMethod())
	if err := types.ValidateJoinMethod(joinMethod); err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := types.NewTeleportRoles(token.GetSpec().GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Scoped{token: token, joinMethod: joinMethod, roles: roles}, nil
}

// GetName returns the name of a [joiningv1.ScopedToken].
func (s *Scoped) GetName() string {
	return s.token.GetMetadata().GetName()
}

// GetJoinMethod returns the cached [types.JoinMethod] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (s *Scoped) GetJoinMethod() types.JoinMethod {
	return s.joinMethod
}

// GetRoles returns the cached [types.SystemRoles] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (s *Scoped) GetRoles() types.SystemRoles {
	return s.roles
}

// GetSafeName returns the name of the scoped token, sanitized appropriately
// for join methods where the name is secret. This should be used when logging
// the token name.
func (s *Scoped) GetSafeName() string {
	return GetSafeScopedTokenName(s.token)
}

// Expiry returns the [time.Time] representing when the wrapped
// [joiningv1.ScopedToken] will expire.
func (s *Scoped) Expiry() time.Time {
	return s.token.GetMetadata().GetExpires().AsTime()
}

// GetSafeScopedTokenName returns the name of the scoped token, sanitized
// appropriately for join methods where the name is secret. This should be used
// when logging the token name.
func GetSafeScopedTokenName(token *joiningv1.ScopedToken) string {
	name := token.GetMetadata().GetName()
	if types.JoinMethod(token.GetSpec().GetJoinMethod()) != types.JoinMethodToken {
		return name
	}

	// If the token name is short, we just blank the whole thing.
	if len(name) < 16 {
		return strings.Repeat("*", len(name))
	}

	// If the token name is longer, we can show the last 25% of it to help
	// the operator identify it.
	hiddenBefore := int(0.75 * float64(len(name)))
	name = name[hiddenBefore:]
	name = strings.Repeat("*", hiddenBefore) + name
	return name
}
