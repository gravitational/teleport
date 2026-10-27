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
	"iter"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/constants"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
)

// adjust the client idle timeout value - preferring the default if not set
func (c *ScopedAccessChecker) adjustScopedClientIdleTimeout(idleStr string, timeout time.Duration) (time.Duration, error) {
	if idleStr == "" {
		idleStr = c.role.GetSpec().GetDefaults().GetClientIdleTimeout()
	}
	if idleStr != "" {
		d, err := time.ParseDuration(idleStr)
		if err != nil {
			return 0, trace.Errorf("invalid client_idle_timeout %q in scoped role %q: %w", idleStr, c.role.GetMetadata().GetName(), err)
		}
		if d > 0 && (timeout == 0 || d < timeout) {
			return max(d, 0), nil
		}
	}
	return max(timeout, 0), nil
}

// adjustScopedDisconnectExpiredCert returns the disconnect on Expired Cert condition - - applying the default if applicable.
func (c *ScopedAccessChecker) adjustScopedDisconnectExpiredCert(roleSpecifiedDisconnect *bool, defaultdisconnect bool) bool {
	if roleSpecifiedDisconnect == nil && c.role.GetSpec().GetDefaults() != nil {
		roleSpecifiedDisconnect = proto.ValueOrNil(c.role.GetSpec().GetDefaults().HasDisconnectExpiredCert(), c.role.GetSpec().GetDefaults().GetDisconnectExpiredCert)
	}

	if roleSpecifiedDisconnect != nil {
		return *roleSpecifiedDisconnect
	}
	return defaultdisconnect
}

// LockingMode returns the lock enforcement mode to apply - applying the default if applicable.
func (c *ScopedAccessChecker) scopedLockingMode(lock *accessv1.Lock, defaultMode constants.LockingMode) constants.LockingMode {
	if lock == nil {
		lock = c.role.GetSpec().GetDefaults().GetLock()
	}
	// both protocol specific lock and the default locks are nil, so return the defaultMode.
	if lock == nil {
		return defaultMode
	}

	mode := constants.LockingMode(lock.GetMode())
	switch mode {
	case constants.LockingModeStrict, constants.LockingModeBestEffort:
		return mode
	default:
		return defaultMode
	}
}

// minNonZero returns the smallest positive value in seq, or zero if seq contains no positive values. Useful for
// summing controls such as max_sessions where the smallest nonzero value is always selected.
func minNonZero[T cmp.Ordered](seq iter.Seq[T]) T {
	var m, zero T
	for v := range seq {
		if v > zero && (m == zero || v < m) {
			m = v
		}
	}
	return m
}

// minNonZeroStream is equivalent to minNonZero but works with fallible streams. Useful for summing controls
// which have a fallible decoding step such as client_idle_timeout.
func minNonZeroStream[T cmp.Ordered](seq iter.Seq2[T, error]) (T, error) {
	var m, zero T
	for v, err := range seq {
		if err != nil {
			return zero, trace.Wrap(err)
		}
		if v > zero && (m == zero || v < m) {
			m = v
		}
	}
	return m, nil
}

// highestPrecedence returns the value from seq that appears earliest in the provided precedence list. This helper
// allows easily summing controls that have a finite number of possible values (e.g. enums or bools). For example,
// an enum where `"enabled"` is always overridden by `"disabled"` can be summed with `highestPrecedence(seq, "disabled", "enabled")`.
// Values not in the precedence list are ignored, and ok is false if no value from seq appears in precedence (or if seq is empty).
func highestPrecedence[T comparable](seq iter.Seq[T], precedence ...T) (highest T, ok bool) {
	best := len(precedence)
	for v := range seq {
		if i := slices.Index(precedence, v); i >= 0 && i < best {
			best = i
			if best == 0 {
				break
			}
		}
	}
	if best == len(precedence) {
		return highest, false
	}
	return precedence[best], true
}
