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
	"time"

	"github.com/gravitational/trace"

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
		roleSpecifiedDisconnect = c.role.GetSpec().GetDefaults().DisconnectExpiredCert
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
