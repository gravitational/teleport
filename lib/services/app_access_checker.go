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
	"cmp"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// AppAccessChecker provides app-specific access checking, abstracting over scoped and unscoped
// identities. It is obtained from [ScopedAccessChecker.App] and should not be constructed directly.
// Methods on this type implement app-specific behavior, branching internally between the scoped and
// unscoped paths of the underlying [ScopedAccessChecker].
type AppAccessChecker struct {
	checker *ScopedAccessChecker
}

// CheckAccessToApp checks access to an application.
func (c *AppAccessChecker) CheckAccessToApp(target types.Application, state AccessState, matchers ...RoleMatcher) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, state, matchers...)
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, state, matchers...)
}

// CanAccessApp checks whether read access to the specified application is possible without regard
// to a specific MFA state. Used for listing/filtering.
func (c *AppAccessChecker) CanAccessApp(target types.Application) error {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.CheckAccess(target, AccessState{MFAVerified: true})
	}
	return c.checker.scopedCompatChecker.CheckAccess(target, AccessState{MFAVerified: true})
}

// AdjustClientIdleTimeout determines the app client idle timeout to apply. The supplied argument must be
// the globally defined most-permissive value. For scoped identities, the most restrictive non-zero value
// across all roles is selected (app.client_idle_timeout takes precedence over defaults.client_idle_timeout).
// If the selected value is more restrictive than the global value it is returned; otherwise the global value
// is returned unchanged. An error is returned if any role contains a non-empty duration string that cannot
// be parsed.
func (c *AppAccessChecker) AdjustClientIdleTimeout(timeout time.Duration) (time.Duration, error) {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustClientIdleTimeout(timeout), nil
	}

	// aggregate stream of all non-empty client_idle_timeout strings
	idleStrs := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (string, bool) {
		// app.client_idle_timeout takes precedence over defaults.client_idle_timeout
		s := cmp.Or(role.GetSpec().GetApp().GetClientIdleTimeout(), role.GetSpec().GetDefaults().GetClientIdleTimeout())
		return s, s != ""
	})

	// convert to fallible stream of parsed durations
	decoded := stream.TryMap(stream.FromSeq(idleStrs), func(s string) (time.Duration, error) {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, trace.Wrap(err, "failed to parse client_idle_timeout %q", s)
		}
		return d, nil
	})

	// add the global timeout to the stream. since we always select the lowest non-zero value, the global
	// needs no special handling.
	durations := stream.Chain(decoded, stream.Once(timeout))

	// select the lowest non-zero duration
	return minNonZeroStream(durations)
}

// AdjustDisconnectExpiredCert adjusts whether to disconnect on certificate expiry. For scoped identities, an explicit
// true in any role takes precedence over an explicit false in any role. The supplied default is only used if no role
// specifies a value (app.disconnect_expired_cert takes precedence over defaults.disconnect_expired_cert).
func (c *AppAccessChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.AdjustDisconnectExpiredCert(disconnect)
	}

	// aggregate all explicitly set disconnect_expired_cert values
	disconnects := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (bool, bool) {
		if app := role.GetSpec().GetApp(); app.HasDisconnectExpiredCert() {
			return app.GetDisconnectExpiredCert(), true
		}
		return role.GetSpec().GetDefaults().GetDisconnectExpiredCert(), role.GetSpec().GetDefaults().HasDisconnectExpiredCert()
	})

	// an explicit true takes precedence over an explicit false.
	if d, ok := highestPrecedence(disconnects, true, false); ok {
		return d
	}

	// the cluster default is only applied if no role specifies a value.
	return disconnect
}

// LockingMode returns the app lock enforcement mode to apply. For scoped identities, strict in any role takes
// precedence over best_effort in any role. The supplied default is only used if no role specifies a mode
// (app.lock takes precedence over defaults.lock).
func (c *AppAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	if !c.checker.isScoped() {
		return c.checker.unscopedChecker.LockingMode(defaultMode)
	}

	// aggregate all explicitly set lock modes
	modes := itertools.FilterMap(c.checker.roles(), func(role *scopedaccessv1.ScopedRole) (constants.LockingMode, bool) {
		lock := role.GetSpec().GetApp().GetLock()
		if lock == nil {
			lock = role.GetSpec().GetDefaults().GetLock()
		}
		return constants.LockingMode(lock.GetMode()), lock.GetMode() != ""
	})

	// strict takes precedence over best effort
	if mode, ok := highestPrecedence(modes, constants.LockingModeStrict, constants.LockingModeBestEffort); ok {
		return mode
	}

	// the cluster default is only applied if no role specifies a recognized mode.
	return defaultMode
}
